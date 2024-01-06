package promserver

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

var (
	// baseKnownSwitchErrors describes documented error conditions which may be reported on switch components.
	baseKnownSwitchErrors = []string{"overtemp", "overpower", "overvoltage", "undervoltage"}
	// baseKnownInputErrors describes documented error conditions which may be reported on input components.
	baseKnownInputErrors = []string{"out_of_range", "read"}
	// baseKnownCoverErrors describes documented error conditions which may be reported on cover components.
	baseKnownCoverErrors = []string{
		"safety_switch",
		"overpower",
		"overvoltage",
		"undervoltage",
		"overcurrent",
		"obstruction",
		"overtemp",
		"bad_feedback:rotating_in_wrong_direction",
		"bad_feedback:both_directions_active",
		"bad_feedback:failed_to_halt",
		"cal_abort:timeout_open",
		"cal_abort:timeout_close",
		"cal_abort:safety",
		"cal_abort:ext_command",
		"cal_abort:bad_feedback",
		"cal_abort:implausible_time_to_fully_close",
		"cal_abort:implausible_time_to_fully_open",
		"cal_abort:implausible_power_consumption_in_close_dir",
		"cal_abort:implausible_power_consumption_in_open_dir",
		"cal_abort:too_many_steps_to_close",
		"cal_abort:too_few_steps_to_close",
		"cal_abort:implausible_time_to_fully_close_w_steps",
		"cal_abort:implausible_step_duration_in_open_dir",
		"cal_abort:too_many_steps_to_open",
		"cal_abort:too_few_steps_to_open",
		"cal_abort:implausible_time_to_fully_open_w_steps",
	}

	// coverStates describe the documented cover component state.
	coverStates = []string{"open", "closed", "opening", "closing", "stopped", "calibrating"}
)

func NewServer(ctx context.Context, discoverer *discovery.Discoverer, opts ...Option) http.Handler {
	s := &Server{
		discoverer:    discoverer,
		promReg:       prometheus.NewRegistry(),
		ctx:           ctx,
		concurrency:   DefaultConcurrency,
		deviceTimeout: DefaultDeviceTimeout,
		namespace:     DefaultNamespace,
		subsystem:     DefaultSubsystem,
	}
	for _, o := range opts {
		o(s)
	}
	s.Handler = promhttp.HandlerFor(s.promReg, promhttp.HandlerOpts{})
	s.initDescs()
	s.promReg.MustRegister(s)
	for _, e := range baseKnownSwitchErrors {
		s.knownSwitchErrors.Store(e, struct{}{})
	}
	for _, e := range baseKnownInputErrors {
		s.knownInputErrors.Store(e, struct{}{})
	}
	for _, e := range baseKnownCoverErrors {
		s.knownCoverErrors.Store(e, struct{}{})
	}
	return s
}

type Server struct {
	ctx        context.Context
	discoverer *discovery.Discoverer
	promReg    *prometheus.Registry
	http.Handler
	namespace     string
	subsystem     string
	concurrency   int
	deviceTimeout time.Duration

	switchOutputOnDesc                *prometheus.Desc
	coverPositionDesc                 *prometheus.Desc
	coverStateDesc                    *prometheus.Desc
	coverPositionControlEnabled       *prometheus.Desc
	inputEnabledDesc                  *prometheus.Desc
	inputStateOnDesc                  *prometheus.Desc
	inputPercentDesc                  *prometheus.Desc
	inputXPercentDesc                 *prometheus.Desc
	totalEnergyWattHoursDesc          *prometheus.Desc
	totalReturnedEnergyWattHoursDesc  *prometheus.Desc
	temperatureCelsiusDesc            *prometheus.Desc
	temperatureFahrenheitDesc         *prometheus.Desc
	networkFrequencyHertzDesc         *prometheus.Desc
	powerFactorDesc                   *prometheus.Desc
	voltageDesc                       *prometheus.Desc
	currentAmperesDesc                *prometheus.Desc
	instantaneousActivePowerWattsDesc *prometheus.Desc
	componentErrorDesc                *prometheus.Desc

	allDescs []*prometheus.Desc
	// knownSwitchErrors tracks all known switch error states, both those documented and any unexpected
	// error codes seen. If an unknown code is seen we want to retain it so we can report it as cleared.
	knownSwitchErrors sync.Map
	knownInputErrors  sync.Map
	knownCoverErrors  sync.Map
}

func (s *Server) initDescs() {
	s.switchOutputOnDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "switch_output_on"),
		`1 if the switch output is on; 0 if it is off.`,
		[]string{"instance", "mac", "device_name", "component_name", "id"},
		nil,
	)
	s.coverPositionDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "cover_position_percent"),
		`Only present if Cover is calibrated. Represents current position in percent from 0 (fully closed) to 100 (fully open); null if the position is unknown`,
		[]string{"instance", "mac", "device_name", "component_name", "id"},
		nil,
	)
	s.coverStateDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "cover_state"),
		`Only present if Cover is calibrated. Represents current position in percent from 0 (fully closed) to 100 (fully open); null if the position is unknown`,
		[]string{"instance", "mac", "device_name", "component_name", "id", "state"},
		nil,
	)
	s.coverPositionControlEnabled = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "cover_position_enabled"),
		`1 if position control is enabled; 0 otherwise.`,
		[]string{"instance", "mac", "device_name", "component_name", "id", "state"},
		nil,
	)
	s.inputEnabledDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "input_enabled"),
		`1 if the input is enabled; 0 if it is disabled.`,
		[]string{"instance", "mac", "device_name", "component_name", "id"},
		nil,
	)
	s.inputStateOnDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "input_state_on"),
		`1 if the input is active; 0 if it is off.`,
		[]string{"instance", "mac", "device_name", "component_name", "id"},
		nil,
	)
	s.inputPercentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "input_percent"),
		`Analog value in percent.`,
		[]string{"instance", "mac", "device_name", "component_name", "id"},
		nil,
	)
	s.inputXPercentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "input_xpercent"),
		`percent transformed with config.xpercent.expr. Present only when both config.xpercent.expr and config.xpercent.unit are set to non-empty values.`,
		[]string{"instance", "mac", "device_name", "component_name", "id"},
		nil,
	)
	s.totalEnergyWattHoursDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "total_energy_watt_hours"),
		`Total energy consumed in Watt-hours.`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id"},
		nil,
	)
	s.totalReturnedEnergyWattHoursDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "total_returned_energy_watt_hours"),
		`Total returned energy consumed in Watt-hours`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id"},
		nil,
	)
	s.temperatureCelsiusDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "temperature_celsius"),
		`Temperature in degrees celsius.`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id"},
		nil,
	)
	s.temperatureFahrenheitDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "temperature_fahrenheit"),
		`Temperature in degrees farenheit.`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id"},
		nil,
	)
	s.networkFrequencyHertzDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "network_frequency_hertz"),
		`Last measured network frequency in Hz.`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id"},
		nil,
	)
	s.powerFactorDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "power_factor"),
		`Last measured power factor.`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id"},
		nil,
	)
	s.voltageDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "voltage"),
		`Last measured voltage.`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id"},
		nil,
	)
	s.currentAmperesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "current_amperes"),
		`Last measured current in amperes.`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id"},
		nil,
	)
	s.instantaneousActivePowerWattsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "instantaneous_active_power_watts"),
		`Last measured instantaneous active power (in Watts) delivered to the attached load (shown if applicable)`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id"},
		nil,
	)
	s.componentErrorDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "component_error"),
		`1 if the error condition ("error" label) is active; 0 or omitted if the error has cleared.`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id", "error"},
		nil,
	)
	s.allDescs = append(s.allDescs,
		s.switchOutputOnDesc,
		s.inputStateOnDesc,
		s.inputPercentDesc,
		s.inputXPercentDesc,
		s.totalEnergyWattHoursDesc,
		s.totalReturnedEnergyWattHoursDesc,
		s.temperatureCelsiusDesc,
		s.temperatureFahrenheitDesc,
		s.networkFrequencyHertzDesc,
		s.powerFactorDesc,
		s.voltageDesc,
		s.currentAmperesDesc,
		s.instantaneousActivePowerWattsDesc,
		s.componentErrorDesc)
}

// Describe implements prometheus.Collector.
func (s *Server) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range s.allDescs {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (s *Server) Collect(ch chan<- prometheus.Metric) {
	l := log.Ctx(s.ctx)
	if _, err := s.discoverer.MDNSSearch(s.ctx); err != nil {
		l.Err(err).Msg("finding new mdns devices")
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	concurrencyLimit := make(chan struct{}, s.concurrency)
	defer close(concurrencyLimit)
	for _, d := range s.discoverer.AllDevices() {
		d := d
		select {
		case <-s.ctx.Done():
			return
		case concurrencyLimit <- struct{}{}:
		}
		wg.Add(1)
		go func() {
			ctx, cancel := context.WithTimeout(s.ctx, s.deviceTimeout)
			defer func() {
				cancel()
				<-concurrencyLimit
				wg.Done()
			}()
			s.collectDevice(ctx, d, ch)
		}()
	}
}

func (s *Server) collectDevice(ctx context.Context, d *discovery.Device, ch chan<- prometheus.Metric) {
	l := log.Ctx(ctx).With().
		Str("mac", d.MACAddr).
		Str("uri", d.URI).
		Logger()
	c, err := d.Open(s.ctx)
	if err != nil {
		l.Err(err).Msg("connecting to device")
	}
	defer func() {
		if err = c.Disconnect(s.ctx); err != nil {
			l.Err(err).Msg("disconnecting from device")
		}
	}()
	status, _, err := (&shelly.ShellyGetStatusRequest{}).Do(ctx, c)
	if err != nil {
		l.Err(err).Msg("querying device status")
		return
	}
	config, _, err := (&shelly.ShellyGetConfigRequest{}).Do(ctx, c)
	if err != nil {
		l.Err(err).Msg("querying device status")
		return
	}
	deviceName := d.MACAddr
	if config.System != nil && config.System.Device != nil && config.System.Device.Name != nil {
		deviceName = *config.System.Device.Name
	}

	if len(config.Switches) != len(status.Switches) {
		l.Error().
			Int("config_len", len(config.Switches)).
			Int("status_len", len(status.Switches)).
			Msg("mismatch between Shelly.GetConfig.Switch and Shelly.GetStatus.Switch")
		return
	}
	for i, swc := range config.Switches {
		sws := status.Switches[i]
		swc := swc

		componentType := "switch"
		componentName := fmt.Sprintf("%s:%d", componentType, i)
		if swc.Name != nil {
			componentName = *swc.Name
		}

		// switch_output_on
		m, err := prometheus.NewConstMetric(
			s.switchOutputOnDesc,
			prometheus.GaugeValue,
			ptrBoolToFloat64(sws.Output),
			d.URI,
			d.MACAddr,
			deviceName,
			componentName,
			strconv.Itoa(swc.ID),
		)
		if err != nil {
			l.Err(err).Msg("encoding metric")
		}
		ch <- m

		if sws.AEnergy != nil {
			// total_energy_watt_hours
			m, err := prometheus.NewConstMetric(
				s.totalEnergyWattHoursDesc,
				prometheus.CounterValue,
				sws.AEnergy.Total,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(swc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if sws.RetAEnergy != nil {
			// total_returned_energy_watt_hours
			m, err := prometheus.NewConstMetric(
				s.totalReturnedEnergyWattHoursDesc,
				prometheus.CounterValue,
				sws.RetAEnergy.Total,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(swc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if sws.Temperature != nil && sws.Temperature.C != nil {
			// temperature_celsius
			m, err := prometheus.NewConstMetric(
				s.temperatureCelsiusDesc,
				prometheus.GaugeValue,
				*sws.Temperature.C,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(swc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if sws.Temperature != nil && sws.Temperature.F != nil {
			// temperature_fahrenheit
			m, err := prometheus.NewConstMetric(
				s.temperatureFahrenheitDesc,
				prometheus.GaugeValue,
				*sws.Temperature.F,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(swc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if sws.Freq != nil {
			// network_frequency_hertz
			m, err := prometheus.NewConstMetric(
				s.networkFrequencyHertzDesc,
				prometheus.GaugeValue,
				*sws.Freq,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(swc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if sws.PF != nil {
			// power_factor
			m, err := prometheus.NewConstMetric(
				s.powerFactorDesc,
				prometheus.GaugeValue,
				*sws.PF,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(swc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if sws.Voltage != nil {
			// voltage
			m, err := prometheus.NewConstMetric(
				s.voltageDesc,
				prometheus.GaugeValue,
				*sws.Voltage,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(swc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if sws.Current != nil {
			// current_amperes
			m, err := prometheus.NewConstMetric(
				s.currentAmperesDesc,
				prometheus.GaugeValue,
				*sws.Current,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(swc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if sws.APower != nil {
			// instantaneous_active_power_watts
			m, err := prometheus.NewConstMetric(
				s.instantaneousActivePowerWattsDesc,
				prometheus.GaugeValue,
				*sws.APower,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(swc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}

		// component_error
		// We obviously want to emit metrics with a 1.0 value for any error that is seen. BUT we
		// want to ensure we send 0.0 for all errors which are inactive to clear any alerts.
		var seenErrors map[string]struct{}
		for _, e := range sws.Errors {
			if seenErrors == nil {
				// This should be rare so only init if we need it.
				seenErrors = make(map[string]struct{})
			}
			seenErrors[e] = struct{}{}
			if _, newError := s.knownSwitchErrors.LoadOrStore(e, struct{}{}); newError {
				// This metric isn't documented. We want to
				l.Warn().
					Str("error_code", e).
					Msg("unknown error code was reported by switch; metric will be retained for future reporting")
			}
		}
		s.knownSwitchErrors.Range(func(eAny, _ any) bool {
			e := eAny.(string)
			var eValue float64
			if _, ok := seenErrors[e]; ok {
				eValue = 1
			}
			m, err := prometheus.NewConstMetric(
				s.componentErrorDesc,
				prometheus.GaugeValue,
				eValue,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(swc.ID),
				e,
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
			return true
		})
	}
	for i, cc := range config.Covers {
		cs := status.Covers[i]

		componentType := "cover"
		componentName := fmt.Sprintf("%s:%d", componentType, i)
		if cc.Name != nil {
			componentName = *cc.Name
		}

		// cover_position
		var currentPos float64 = math.NaN()
		if cs.CurrentPos != nil {
			currentPos = *cs.CurrentPos
		}
		m, err := prometheus.NewConstMetric(
			s.coverPositionDesc,
			prometheus.GaugeValue,
			currentPos,
			d.URI,
			d.MACAddr,
			deviceName,
			componentName,
			strconv.Itoa(cc.ID),
		)
		if err != nil {
			l.Err(err).Msg("encoding metric")
		}
		ch <- m

		// cover_position_control_enabled
		m, err = prometheus.NewConstMetric(
			s.coverPositionControlEnabled,
			prometheus.GaugeValue,
			ptrBoolToFloat64(cs.PosControl),
			d.URI,
			d.MACAddr,
			deviceName,
			componentName,
			strconv.Itoa(cc.ID),
		)
		if err != nil {
			l.Err(err).Msg("encoding metric")
		}
		ch <- m

		// cover_state
		for _, state := range coverStates {
			var stateActive float64
			if cs.State != nil && *cs.State == state {
				stateActive = 1
			}
			m, err := prometheus.NewConstMetric(
				s.coverStateDesc,
				prometheus.GaugeValue,
				stateActive,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				strconv.Itoa(cc.ID),
				state,
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}

		if cs.AEnergy != nil {
			// total_energy_watt_hours
			m, err := prometheus.NewConstMetric(
				s.totalEnergyWattHoursDesc,
				prometheus.CounterValue,
				cs.AEnergy.Total,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(cc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if cs.Temperature != nil && cs.Temperature.C != nil {
			// temperature_celsius
			m, err := prometheus.NewConstMetric(
				s.temperatureCelsiusDesc,
				prometheus.GaugeValue,
				*cs.Temperature.C,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(cc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if cs.Temperature != nil && cs.Temperature.F != nil {
			// temperature_fahrenheit
			m, err := prometheus.NewConstMetric(
				s.temperatureFahrenheitDesc,
				prometheus.GaugeValue,
				*cs.Temperature.F,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(cc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if cs.Freq != nil {
			// network_frequency_hertz
			m, err := prometheus.NewConstMetric(
				s.networkFrequencyHertzDesc,
				prometheus.GaugeValue,
				*cs.Freq,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(cc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if cs.PF != nil {
			// power_factor
			m, err := prometheus.NewConstMetric(
				s.powerFactorDesc,
				prometheus.GaugeValue,
				*cs.PF,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(cc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if cs.Voltage != nil {
			// voltage
			m, err := prometheus.NewConstMetric(
				s.voltageDesc,
				prometheus.GaugeValue,
				*cs.Voltage,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(cc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if cs.Current != nil {
			// current_amperes
			m, err := prometheus.NewConstMetric(
				s.currentAmperesDesc,
				prometheus.GaugeValue,
				*cs.Current,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(cc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		if cs.APower != nil {
			// instantaneous_active_power_watts
			m, err := prometheus.NewConstMetric(
				s.instantaneousActivePowerWattsDesc,
				prometheus.GaugeValue,
				*cs.APower,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(cc.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}

		// component_error
		// We obviously want to emit metrics with a 1.0 value for any error that is seen. BUT we
		// want to ensure we send 0.0 for all errors which are inactive to clear any alerts.
		var seenErrors map[string]struct{}
		for _, e := range cs.Errors {
			if seenErrors == nil {
				// This should be rare so only init if we need it.
				seenErrors = make(map[string]struct{})
			}
			seenErrors[e] = struct{}{}
			if _, newError := s.knownCoverErrors.LoadOrStore(e, struct{}{}); newError {
				// This metric isn't documented. We want to
				l.Warn().
					Str("error_code", e).
					Msg("unknown error code was reported by cover; metric will be retained for future reporting")
			}
		}
		s.knownCoverErrors.Range(func(eAny, _ any) bool {
			e := eAny.(string)
			var eValue float64
			if _, ok := seenErrors[e]; ok {
				eValue = 1
			}
			m, err := prometheus.NewConstMetric(
				s.componentErrorDesc,
				prometheus.GaugeValue,
				eValue,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(cc.ID),
				e,
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
			return true
		})
	}

	for i, ic := range config.Inputs {
		is := status.Inputs[i]

		componentType := "input"
		componentName := fmt.Sprintf("%s:%d", componentType, i)
		if ic.Name != nil {
			componentName = *ic.Name
		}

		// input_enabled
		if ic.Enable != nil {
			m, err := prometheus.NewConstMetric(
				s.inputEnabledDesc,
				prometheus.GaugeValue,
				ptrBoolToFloat64(ic.Enable),
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				strconv.Itoa(ic.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}

		// input_state_on
		if is.State != nil {
			m, err := prometheus.NewConstMetric(
				s.inputStateOnDesc,
				prometheus.GaugeValue,
				ptrBoolToFloat64(is.State),
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				strconv.Itoa(ic.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}

		// input_percent
		if is.Percent != nil {
			m, err := prometheus.NewConstMetric(
				s.inputPercentDesc,
				prometheus.GaugeValue,
				*is.Percent,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				strconv.Itoa(ic.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}
		// input_xpercent
		if is.XPercent != nil {
			m, err := prometheus.NewConstMetric(
				s.inputXPercentDesc,
				prometheus.GaugeValue,
				*is.XPercent,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				strconv.Itoa(ic.ID),
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
		}

		// component_error
		// We obviously want to emit metrics with a 1.0 value for any error that is seen. BUT we
		// want to ensure we send 0.0 for all errors which are inactive to clear any alerts.
		var seenErrors map[string]struct{}
		for _, e := range is.Errors {
			if seenErrors == nil {
				// This should be rare so only init if we need it.
				seenErrors = make(map[string]struct{})
			}
			seenErrors[e] = struct{}{}
			if _, newError := s.knownInputErrors.LoadOrStore(e, struct{}{}); newError {
				// This metric isn't documented. We want to
				l.Warn().
					Str("error_code", e).
					Msg("unknown error code was reported by switch; metric will be retained for future reporting")
			}
		}
		s.knownInputErrors.Range(func(eAny, _ any) bool {
			e := eAny.(string)
			var eValue float64
			if _, ok := seenErrors[e]; ok {
				eValue = 1
			}
			m, err := prometheus.NewConstMetric(
				s.componentErrorDesc,
				prometheus.GaugeValue,
				eValue,
				d.URI,
				d.MACAddr,
				deviceName,
				componentName,
				componentType,
				strconv.Itoa(ic.ID),
				e,
			)
			if err != nil {
				l.Err(err).Msg("encoding metric")
			}
			ch <- m
			return true
		})
	}
}

func ptrBoolToFloat64(b *bool) float64 {
	if b == nil || !*b {
		return 0
	}
	return 1
}
