package promserver

import "time"

const (
	// DefaultConcurrency is the default probe concurrency.
	DefaultConcurrency = 10
	// DefaultDeviceTimeout is the default max time for devices to respond to probes.
	DefaultDeviceTimeout = 5 * time.Second
	// DefaultScrapeDurationWarning sets the default value for scrape duration warning. By default
	// prometheus scrape_timeout is 10s, so 8s is 80% of this value.
	DefaultScrapeDurationWarning = 8 * time.Second
	// DefaultNamespace is the default namespace for metrics.
	DefaultNamespace = "shelly"
	// DefaultSubsystem is the default subsystem for metrics.
	DefaultSubsystem = "status"
)

type Option func(*Server)

// WithConcurrency defines the number of concurrent probes which will be made.
func WithConcurrency(c int) Option {
	return func(s *Server) {
		s.concurrency = c
	}
}

// WithDeviceTimeout describes the maximum time allowed for a device to respond to it probe.
func WithDeviceTimeout(t time.Duration) Option {
	return func(s *Server) {
		s.deviceTimeout = t
	}
}

// WithScrapeDurationWarning sets the value for scrape duration warning.
func WithScrapeDurationWarning(t time.Duration) Option {
	return func(s *Server) {
		s.scrapeDurationWaring = t
	}
}

// WithPrometheusNamespace sets the namespace string to use for prometheus metric names.
func WithPrometheusNamespace(ns string) Option {
	return func(s *Server) {
		s.namespace = ns
	}
}

// WithPrometheusSubsystem sets the subsystem section of the prometheus metric names.
func WithPrometheusSubsystem(subsystem string) Option {
	return func(s *Server) {
		s.subsystem = subsystem
	}
}
