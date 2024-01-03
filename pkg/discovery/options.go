package discovery

import (
	"net"
	"time"
)

// DiscovererOption provides optional parameters for the Discoverer.
type DiscovererOption func(*Discoverer)

// WithMDNSInterface configures a specific network interface to search.
func WithMDNSInterface(iface *net.Interface) DiscovererOption {
	return func(d *Discoverer) {
		d.mdnsInterface = iface
	}
}

// WithMDNSZone configures the mDNS zone to search. This is commonly `.local`.
func WithMDNSZone(zone string) DiscovererOption {
	return func(d *Discoverer) {
		d.mdnsZone = zone
	}
}

// WithMDNSService configures the mDNS service name to search. Shelly devices are commonly
// at `_shelly._tcp`.
func WithMDNSService(service string) DiscovererOption {
	return func(d *Discoverer) {
		d.mdnsService = service
	}
}

// WithSearchTimeout specifies the duration to wait for mDNS responses and the initial probe.
func WithSearchTimeout(timeout time.Duration) DiscovererOption {
	return func(d *Discoverer) {
		d.searchTimeout = timeout
	}
}

// WithConcurrency configures the number of concurrent device probes to evaluate.
func WithConcurrency(concurrency int) DiscovererOption {
	return func(d *Discoverer) {
		d.concurrency = concurrency
	}
}

// WithIPVersion sets a required IP version for discovery. Values "4" or "6" are accepted.
func WithIPVersion(ipVersion string) DiscovererOption {
	return func(d *Discoverer) {
		d.preferIPVersion = ipVersion
	}
}

// WithDeviceTTL configures a time-to-live for devices in long-lived commands like the
// prometheus server.
func WithDeviceTTL(ttl time.Duration) DiscovererOption {
	return func(d *Discoverer) {
		d.deviceTTL = ttl
	}
}

// WithMDNSSearchEnabled allows enabling or disabling mDNS discovery.
func WithMDNSSearchEnabled(enabled bool) DiscovererOption {
	return func(d *Discoverer) {
		d.mdnsEnabled = enabled
	}
}

type DeviceOption func(*Device)
