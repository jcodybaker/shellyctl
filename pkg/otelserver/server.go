package otelserver

import (
	"context"
	"fmt"
	"time"

	"github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkMetric "go.opentelemetry.io/otel/sdk/metric"
)

const (
	// DefaultStopWait is the maximum duration we're willing to wait a clean shutdown.
	DefaultStopWait time.Duration = 5 * time.Second

	// DefeaultMeterName is the default name of the meter.
	DefeaultMeterName = "codybaker.com/shellyctl"
)

type Server struct {
	discoverer           *discovery.Discoverer
	onStop               []func(ctx context.Context) error
	stopWait             time.Duration
	meterP               *sdkMetric.MeterProvider
	meter                metric.Meter
	metrics              metrics
	meterProviderOptions []sdkMetric.Option
}

func NewServer(
	ctx context.Context,
	discoverer *discovery.Discoverer,
	opts ...Option,
) *Server {
	s := &Server{
		discoverer: discoverer,
		stopWait:   DefaultStopWait,
	}

	s.meterP = sdkMetric.NewMeterProvider(s.meterProviderOptions...)
	s.onStop = append(s.onStop, s.meterP.Shutdown)
	for _, o := range opts {
		o(s)
	}
	if s.meter == nil {
		s.meter = s.meterP.Meter(DefeaultMeterName)
	}

	s.metrics.initMeters(s.meter)
	return s
}

func (m *metrics) initMeters(meter metric.Meter) error {
	var err error
	m.switches.output, err = meter.Int64Gauge("switch.output",
		metric.WithUnit("%"),
		metric.WithDescription("1 if the switch output is on; 0 if it is off."))
	if err != nil {
		return fmt.Errorf("failed to create switch.output metric: %w", err)
	}
	m.covers.position, err = meter.Float64Gauge("cover.position",
		metric.WithUnit("%"),
		metric.WithDescription("Only present if Cover is calibrated. Represents current position in percent from 0 (fully closed) to 100 (fully open); null if the position is unknown."))
	if err != nil {
		return fmt.Errorf("failed to create cover.position metric: %w", err)
	}
	m.covers.positionControlEnabled, err = meter.Int64Gauge("cover.position_control_enabled",
		metric.WithUnit("enabled"),
		metric.WithDescription("1 if position control is enabled; 0 otherwise."))
	if err != nil {
		return fmt.Errorf("failed to create cover.position metric: %w", err)
	}
	m.input.enabled, err = meter.Int64Gauge("input.enabled",
		metric.WithUnit("enabled"),
		metric.WithDescription("1 if the input is enabled; 0 otherwise."))
	if err != nil {
		return fmt.Errorf("failed to create input.enabled metric: %w", err)
	}
	m.input.percent, err = meter.Float64Gauge("input.percent",
		metric.WithUnit("%"),
		metric.WithDescription("Current input state in percent."))
	if err != nil {
		return fmt.Errorf("failed to create input.percent metric: %w", err)
	}
	m.input.xPercent, err = meter.Float64Gauge("input.x_percent",
		metric.WithUnit("%"),
		metric.WithDescription("Percent transformed with config.xpercent.expr. Present only when both config.xpercent.expr and config.xpercent.unit are set to non-empty values."))
	if err != nil {
		return fmt.Errorf("failed to create input.x_percent metric: %w", err)
	}
	// NOTE these will need to be converted from watt-hours to joules (x 3600)
	// Also technically these would be counters but since we're just relaying it it we need to use gauges.
	m.energy.total, err = meter.Float64Gauge("energy.total",
		metric.WithUnit("J"),
		metric.WithDescription("Total energy consumption."))
	if err != nil {
		return fmt.Errorf("failed to create energy.total metric: %w", err)
	}
	m.energy.totalReturned, err = meter.Float64Gauge("energy.total_returned",
		metric.WithUnit("J"),
		metric.WithDescription("Total energy returned."))
	if err != nil {
		return fmt.Errorf("failed to create energy.total_returned metric: %w", err)
	}
	m.energy.networkFrequency, err = meter.Float64Gauge("energy.network_frequency",
		metric.WithUnit("Hz"),
		metric.WithDescription("Frequency of the electrical network."))
	if err != nil {
		return fmt.Errorf("failed to create energy.network_frequency metric: %w", err)
	}
	m.energy.powerFactor, err = meter.Float64Gauge("energy.power_factor",
		metric.WithDescription("Last measured power factor."))
	if err != nil {
		return fmt.Errorf("failed to create energy.power_factor metric: %w", err)
	}
	m.energy.voltage, err = meter.Float64Gauge("energy.voltage",
		metric.WithUnit("V"),
		metric.WithDescription("Last measured voltage."))
	if err != nil {
		return fmt.Errorf("failed to create energy.voltage metric: %w", err)
	}
	m.energy.current, err = meter.Float64Gauge("energy.current",
		metric.WithUnit("A"),
		metric.WithDescription("Last measured current."))
	if err != nil {
		return fmt.Errorf("failed to create energy.current metric: %w", err)
	}
	m.temperature.celsius, err = meter.Float64Gauge("temperature.celsius",
		metric.WithUnit("Cel"),
		metric.WithDescription("Temperature in degrees Celsius."))
	if err != nil {
		return fmt.Errorf("failed to create temperature.celsius metric: %w", err)
	}
	m.temperature.fahrenheit, err = meter.Float64Gauge("temperature.fahrenheit",
		metric.WithUnit("°F"),
		metric.WithDescription("Temperature in degrees Fahrenheit."))
	if err != nil {
		return fmt.Errorf("failed to create temperature.fahrenheit metric: %w", err)
	}
	m.humidity.relative, err = meter.Float64Gauge("humidity.relative",
		metric.WithUnit("%"),
		metric.WithDescription("Relative humidity."))
	if err != nil {
		return fmt.Errorf("failed to create humidity.relative metric: %w", err)
	}
	return nil
}

func (s *Server) Run(ctx context.Context) error {
	defer s.stop(ctx)
	snc := s.discoverer.GetStatusNotifications(100)
	fsnc := s.discoverer.GetStatusNotifications(100)
	for ctx.Err() == nil {
		var sn discovery.StatusNotification
		select {
		case <-ctx.Done():
			return nil
		case sn = <-snc:
		case sn = <-fsnc:
		}
		for _, sw := range sn.Status.Switches {
			s.metrics.setSwitch(ctx, sw, sn.Frame.Src)
		}
		for _, t := range sn.Status.Temperatures {
			s.metrics.setTemperature(ctx, t, sn.Frame.Src)
		}
		for _, h := range sn.Status.Humidities {
			s.metrics.setHumidity(ctx, h, sn.Frame.Src)
		}
	}
	return nil
}

func (s *Server) stop(ctx context.Context) {
	ll := log.Ctx(ctx)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.stopWait)
	defer cancel()
	for _, stop := range s.onStop {
		if err := stop(ctx); err != nil {
			ll.Err(err).Msg("shutting down")
		}
	}
}

func bool2int64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

type metrics struct {
	switches struct {
		output metric.Int64Gauge
	}
	covers struct {
		position               metric.Float64Gauge
		positionControlEnabled metric.Int64Gauge
	}
	input struct {
		enabled  metric.Int64Gauge
		percent  metric.Float64Gauge
		xPercent metric.Float64Gauge
	}
	energy struct {
		total                    metric.Float64Gauge
		totalReturned            metric.Float64Gauge
		networkFrequency         metric.Float64Gauge
		powerFactor              metric.Float64Gauge
		voltage                  metric.Float64Gauge
		current                  metric.Float64Gauge
		instantaneousActivePower metric.Float64Gauge
	}
	temperature struct {
		celsius    metric.Float64Gauge
		fahrenheit metric.Float64Gauge
	}
	humidity struct {
		relative metric.Float64Gauge
	}
}

func (m *metrics) setSwitch(ctx context.Context, s *shelly.SwitchStatus, src string) {
	attrs := []attribute.KeyValue{
		attribute.Int("id", s.ID),
		attribute.String("instance", src),
	}
	attrSet := attribute.NewSet(attrs...)
	if s.Output != nil {
		m.switches.output.Record(ctx, bool2int64(*s.Output), metric.WithAttributeSet(attrSet))
	}
	attrs = append(attrs, attribute.String("type", "switch"))
	attrSet = attribute.NewSet(attrs...)
	if s.APower != nil {
		m.energy.instantaneousActivePower.Record(ctx, *s.APower, metric.WithAttributeSet(attrSet))
	}
	if s.Voltage != nil {
		m.energy.voltage.Record(ctx, *s.Voltage, metric.WithAttributeSet(attrSet))
	}
	if s.Current != nil {
		m.energy.current.Record(ctx, *s.Current, metric.WithAttributeSet(attrSet))
	}
	if s.PF != nil {
		m.energy.powerFactor.Record(ctx, *s.PF, metric.WithAttributeSet(attrSet))
	}
	if s.Freq != nil {
		m.energy.networkFrequency.Record(ctx, *s.Freq, metric.WithAttributeSet(attrSet))
	}
	if s.AEnergy != nil {
		m.energy.total.Record(ctx, s.AEnergy.Total*3600, metric.WithAttributeSet(attrSet))
	}
	if s.RetAEnergy != nil {
		m.energy.totalReturned.Record(ctx, s.RetAEnergy.Total*3600, metric.WithAttributeSet(attrSet))
	}
	if s.Temperature != nil {
		if s.Temperature.C != nil {
			m.temperature.celsius.Record(ctx, *s.Temperature.C, metric.WithAttributeSet(attrSet))
		}
		if s.Temperature.F != nil {
			m.temperature.fahrenheit.Record(ctx, *s.Temperature.F, metric.WithAttributeSet(attrSet))
		}
	}
}

func (m *metrics) setTemperature(ctx context.Context, t *shelly.TemperatureStatus, src string) {
	attrs := []attribute.KeyValue{
		attribute.Int("id", t.ID),
		attribute.String("instance", src),
		attribute.String("type", "temperature"),
	}
	attrSet := attribute.NewSet(attrs...)
	if t.TC != nil {
		m.temperature.celsius.Record(ctx, *t.TC, metric.WithAttributeSet(attrSet))
	}
	if t.TF != nil {
		m.temperature.fahrenheit.Record(ctx, *t.TF, metric.WithAttributeSet(attrSet))
	}
}

func (m *metrics) setHumidity(ctx context.Context, h *shelly.HumidityStatus, src string) {
	attrs := []attribute.KeyValue{
		attribute.Int("id", h.ID),
		attribute.String("instance", src),
		attribute.String("type", "humidity"),
	}
	attrSet := attribute.NewSet(attrs...)
	if h.RH != nil {
		m.humidity.relative.Record(ctx, *h.RH, metric.WithAttributeSet(attrSet))
	}
}
