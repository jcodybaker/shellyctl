package promserver

import (
	"context"
	"fmt"
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

func NewServer(ctx context.Context, discoverer *discovery.Discoverer, opts ...Option) http.Handler {
	s := &Server{
		discoverer:    discoverer,
		promReg:       prometheus.NewRegistry(),
		ctx:           ctx,
		concurrency:   DefaultConcurrency,
		deviceTimeout: DefaultDeviceTimeout,
	}
	for _, o := range opts {
		o(s)
	}
	s.Handler = promhttp.HandlerFor(s.promReg, promhttp.HandlerOpts{})
	s.initDescs()
	s.promReg.MustRegister(s)
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
}

func (s *Server) initDescs() {
	s.switchOutputOnDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "switch_output_on"),
		`1 if the switch output is on; 0 if it is off.`,
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
		[]string{"instance", "mac", "device_name", "component_name", "component", "id", "direction"},
		nil,
	)
	s.totalReturnedEnergyWattHoursDesc = prometheus.NewDesc(
		prometheus.BuildFQName(s.namespace, s.subsystem, "total_returned_energy_watt_hours"),
		`Total returned energy consumed in Watt-hours`,
		[]string{"instance", "mac", "device_name", "component_name", "component", "id", "direction"},
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
		var output float64
		if sws.Output != nil && *sws.Output {
			output = 1
		}
		componentName := fmt.Sprintf("switch:%d", i)
		if swc.Name != nil {
			componentName = *swc.Name
		}
		m, err := prometheus.NewConstMetric(
			s.switchOutputOnDesc,
			prometheus.GaugeValue,
			output,
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
	}
}
