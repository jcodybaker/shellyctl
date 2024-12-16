package otelserver

import (
	"time"

	sdkMetric "go.opentelemetry.io/otel/sdk/metric"
)

type Option func(*Server)

// WithStopWait sets the maximum duration we're willing to wait a clean shutdown.
func WithStopWait(stopWait time.Duration) Option {
	return func(s *Server) {
		s.stopWait = stopWait
	}
}

// WithMeterName sets the name of the meter.
func WithMeterName(name string) Option {
	return func(s *Server) {
		s.meter = s.meterP.Meter(name)
	}
}

// WithMetricsExporter enables metrics exporter for a meter provider.
func WithMetricsExporter(e sdkMetric.Exporter, interval time.Duration) Option {
	return func(s *Server) {
		s.meterProviderOptions = append(
			s.meterProviderOptions,
			sdkMetric.WithReader(sdkMetric.NewPeriodicReader(e, sdkMetric.WithInterval(interval))))
	}
}

// WithMetricsReader enables metrics reader for a meter provider.
func WithMetricsReader(r sdkMetric.Reader) Option {
	return func(s *Server) {
		s.meterProviderOptions = append(
			s.meterProviderOptions,
			sdkMetric.WithReader(r))
	}
}
