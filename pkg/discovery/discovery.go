package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"tinygo.org/x/bluetooth"
)

const (
	// DefaultDeviceTTL sets the default time-to-live for discovered devices on long-lived commands.
	DefaultDeviceTTL = 5 * time.Minute

	DefaultMDNSZone          = "local"
	DefaultMDNSService       = "_shelly._tcp"
	DefaultMDNSSearchTimeout = 1 * time.Second
	DefaultConcurrency       = 5
)

func NewDiscoverer(opts ...DiscovererOption) *Discoverer {
	d := &Discoverer{
		knownDevices: make(map[string]*Device),
		options: &options{
			bleAdapter:    bluetooth.DefaultAdapter,
			now:           time.Now,
			deviceTTL:     DefaultDeviceTTL,
			mdnsZone:      DefaultMDNSZone,
			mdnsService:   DefaultMDNSService,
			searchTimeout: DefaultMDNSSearchTimeout,
			concurrency:   DefaultConcurrency,
			mdnsQueryFunc: mdns.Query,
		},
	}
	for _, o := range opts {
		o(d)
	}
	d.enableBLEAdapter = sync.OnceValue[error](func() error {
		log.Logger.Debug().Msg("enabling BLE adapter")
		return d.bleAdapter.Enable()
	})
	return d
}

// Discoverer finds shelly gen 2 devices and provides basic metadata.
type Discoverer struct {
	*options
	knownDevices map[string]*Device

	lock sync.Mutex
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
		uri:    u.String(),
		source: sourceManual,
	}
	if err = dev.resolveSpecs(ctx); err != nil {
		return nil, err
	}
	dev.lastSeen = d.now()
	dev = d.addDevice(dev, opts...)
	return dev, nil
}

func (d *Discoverer) addDevice(dev *Device, opts ...DeviceOption) *Device {
	for _, o := range opts {
		o(dev)
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	if existingDev, ok := d.knownDevices[dev.MACAddr]; ok {
		existingDev.lastSeen = dev.lastSeen
		return existingDev
	}
	d.knownDevices[dev.MACAddr] = dev
	return dev
}

// AllDevices returns all known devices.
func (d *Discoverer) AllDevices() []*Device {
	var out []*Device
	d.lock.Lock()
	defer d.lock.Unlock()
	for _, dev := range d.knownDevices {
		out = append(out, dev)
	}
	return out
}

func (d *Discoverer) Search(ctx context.Context) ([]*Device, error) {
	if !d.bleSearchEnabled && !d.mdnsSearchEnabled {
		return nil, nil
	}
	var l sync.Mutex
	var allDevs []*Device
	eg, ctx := errgroup.WithContext(ctx)
	if d.bleSearchEnabled {
		eg.Go(func() error {
			devs, err := d.SearchBLE(ctx)
			l.Lock()
			defer l.Unlock()
			allDevs = append(allDevs, devs...)
			return err
		})
	}
	if d.mdnsSearchEnabled {
		eg.Go(func() error {
			devs, err := d.SearchMDNS(ctx)
			l.Lock()
			defer l.Unlock()
			allDevs = append(allDevs, devs...)
			return err
		})
	}
	return allDevs, eg.Wait()
}
