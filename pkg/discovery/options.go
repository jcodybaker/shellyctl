package discovery

import (
	"context"
	"net"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/hashicorp/mdns"
	"tinygo.org/x/bluetooth"
)

type options struct {
	bleAdapter       *bluetooth.Adapter
	bleLock          sync.Mutex
	enableBLEAdapter func() error
	bleSearchEnabled bool

	authCallback AuthCallback

	now func() time.Time

	mdnsInterface     *net.Interface
	mdnsZone          string
	mdnsService       string
	mdnsSearchEnabled bool

	mqttConnectOptions mqtt.ClientOptions

	searchStrictTimeout bool
	searchTimeout       time.Duration
	searchConfirm       SearchConfirm
	concurrency         int

	// deviceTTL is relevant for long-lived commands (like prometheus metrics server) when
	// mixed with mDNS or other ephemeral discovery.
	deviceTTL time.Duration

	preferIPVersion string

	mdnsQueryFunc func(context.Context, *mdns.QueryParam) error
}

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
		d.mdnsSearchEnabled = enabled
	}
}

// WithBLEAdapter configures a BLE adapter for use in discovery.
func WithBLEAdapter(ble *bluetooth.Adapter) DiscovererOption {
	return func(d *Discoverer) {
		d.bleAdapter = ble
	}
}

// WithMDNSSearchEnabled allows enabling or disabling BLE discovery.
func WithBLESearchEnabled(enabled bool) DiscovererOption {
	return func(d *Discoverer) {
		d.bleSearchEnabled = enabled
	}
}

// WithSearchConfirm sets a callback to confirm search results.
func WithSearchConfirm(confirm SearchConfirm) DiscovererOption {
	return func(d *Discoverer) {
		d.searchConfirm = confirm
	}
}

// WithAuthCallback sets a default callback for authenticating .
func WithAuthCallback(authCallback AuthCallback) DiscovererOption {
	return func(d *Discoverer) {
		d.authCallback = authCallback
	}
}

// WithSearchStrictTimeout will force devices which have been discovered, but not resolved and added
// to finish within the search timeout or be cancelled.
func WithSearchStrictTimeout(strictTimeoutMode bool) DiscovererOption {
	return func(d *Discoverer) {
		d.searchStrictTimeout = strictTimeoutMode
	}
}

// WithMQTTConnectOptions sets connection parameters for MQTT.
func WithMQTTConnectOptions(c mqtt.ClientOptions) DiscovererOption {
	return func(d *Discoverer) {
		d.mqttConnectOptions = c
	}
}

type DeviceOption func(*Device)
