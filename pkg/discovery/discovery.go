package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
)

const (
	defaultTimeout           = 5 * time.Minute
	DefaultMDNSZone          = "local"
	DefaultMDNSService       = "_shelly._tcp"
	DefaultMDNSSearchTimeout = 1 * time.Second
	DefaultMDNSWorkers       = 5
)

func NewDiscoverer(opts ...DiscovererOption) *Discoverer {
	d := &Discoverer{
		timeout:           defaultTimeout,
		now:               time.Now,
		knownDevices:      make(map[string]*Device),
		mdnsZone:          DefaultMDNSZone,
		mdnsService:       DefaultMDNSService,
		mdnsSearchTimeout: DefaultMDNSSearchTimeout,
		mdnsWorkers:       DefaultMDNSWorkers,
		mdnsQueryFunc:     mdns.Query,
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

// Discoverer finds shelly gen 2 devices and provides basic metadata.
type Discoverer struct {
	knownDevices map[string]*Device

	mdnsInterface     *net.Interface
	mdnsZone          string
	mdnsService       string
	mdnsSearchTimeout time.Duration
	mdnsWorkers       int

	preferIPVersion string

	mdnsQueryFunc func(*mdns.QueryParam) error

	lock sync.Mutex

	now func() time.Time

	// timeout is relevant for long-lived commands (like prometheus metrics server) when
	// mixed with mDNS or other ephemeral discovery.
	timeout time.Duration
}

// AddDeviceByAddress attempts to parse a user-provided URI and add the device.
// An error will be generated if the address cannot be parsed or is unreachable.
// If no schema is provided, `http` is assumed. Any non-empty URI path other than
// `/rpc` is invalid and will be rejected. If the hostname ends in the mDNS zone
// (.local by default) the name will be resolved via mDNS.
func (d *Discoverer) AddDeviceByAddress(ctx context.Context, addr string, opts ...DeviceOption) (*Device, error) {
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("parsing URI: %w", err)
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return nil, fmt.Errorf("unsupported URI scheme %q", u.Scheme)
	}
	if u.Path == "" {
		u.Path = "/rpc"
	}
	if u.Path != "/rpc" {
		return nil, fmt.Errorf("unsupported URI path %q", u.Path)
	}
	if u.RawQuery != "" {
		return nil, errors.New("URI query parameters are not supported")
	}

	if strings.HasSuffix(strings.ToLower(u.Hostname()), "."+d.mdnsZone) {
		// URI is mDNS, we want to resolve it to an IP.
		return nil, nil
	}

	dev := &Device{
		URI:    u.String(),
		source: sourceManual,
	}
	if err = dev.resolveSpecs(ctx); err != nil {
		return nil, err
	}
	dev.lastSeen = d.now()
	for _, o := range opts {
		o(dev)
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	if existingDev, ok := d.knownDevices[dev.MACAddr]; ok {
		existingDev.lastSeen = dev.lastSeen
		return existingDev, nil
	}
	d.knownDevices[dev.MACAddr] = dev
	return dev, nil
}
