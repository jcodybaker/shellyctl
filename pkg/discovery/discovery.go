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
	"github.com/jcodybaker/go-shelly"
	"github.com/rs/zerolog"
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
			mdnsQueryFunc: mdns.QueryContext,
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
	// ioLock ensures only one approver can read/write to stdio concurrently. This is needed
	// so mDNS/BLE can opperate simultaneously.
	ioLock sync.Mutex
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
	ll := d.logCtx(ctx, "").With().Str("host", u.Host).Logger()
	// Default to the global auth callback, but if there's a un/pw on the URL we'll use that.
	authCallback := d.authCallback
	if u.User != nil {
		if pw, ok := u.User.Password(); ok {
			if u.User.Username() != "" && u.User.Username() != shelly.DefaultAuthenticationUsername {
				ll.Warn().
					Str("username", u.User.Username()).
					Msg("host URI includes username/password with unsupported username; `" + shelly.DefaultAuthenticationUsername + "` expected")
			}
			authCallback = func(ctx context.Context, desc string) (string, error) {
				return pw, nil
			}
		} else if u.User.Username() != "" {
			// Since only one username is allowed, as a special case we'll treat a URL with only
			// a username section (no password) as the password. Ex: `http://mypassword@192.168.1.1/`.
			pw := u.User.Username()
			authCallback = func(ctx context.Context, desc string) (string, error) {
				return pw, nil
			}
		}
		u.User = nil
	}

	if strings.HasSuffix(strings.ToLower(u.Hostname()), "."+d.mdnsZone) {
		// TODO(cbaker) - URI is mDNS, we want to resolve it to an IP.
		return nil, nil
	}

	dev := &Device{
		uri:          u.String(),
		source:       sourceManual,
		authCallback: authCallback,
	}

	if err = dev.resolveSpecs(ctx); err != nil {
		return nil, err
	}
	dev.lastSeen = d.now()
	dev, _ = d.addDevice(ctx, dev, opts...)
	return dev, nil
}

func (d *Discoverer) addDevice(ctx context.Context, dev *Device, opts ...DeviceOption) (*Device, bool) {
	ll := d.logCtx(ctx, "").With().Str("instance", dev.Instance()).Logger()
	for _, o := range opts {
		o(dev)
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	if existingDev, ok := d.knownDevices[dev.MACAddr]; ok {
		ll.Debug().Msg("known device was rediscovered; reusing existing reference")
		existingDev.lastSeen = dev.lastSeen
		return existingDev, false
	}
	d.knownDevices[strings.ToUpper(dev.MACAddr)] = dev
	ll.Info().Msg("new device added")
	return dev, true
}

func (d *Discoverer) isKnownDevice(mac string) bool {
	d.lock.Lock()
	defer d.lock.Unlock()
	_, ok := d.knownDevices[strings.ToUpper(mac)]
	return ok
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
	stop := make(chan struct{})
	defer func() {
		// This is gross. `stop` may be closed during the search by the confirmation
		// dialogue. That's concurrency safe by virtue of the ioLock. We could pass
		// something along so we know the channel is open/closed, or we just do a
		// non-blocking read on it here and close in the block case.
		select {
		case <-stop:
			// stop is already closed
		default:
			close(stop)
		}
	}()
	if d.bleSearchEnabled {
		eg.Go(func() error {
			devs, err := d.searchBLE(ctx, stop)
			l.Lock()
			defer l.Unlock()
			allDevs = append(allDevs, devs...)
			return err
		})
	}
	if d.mdnsSearchEnabled {
		eg.Go(func() error {
			devs, err := d.searchMDNS(ctx, stop)
			l.Lock()
			defer l.Unlock()
			allDevs = append(allDevs, devs...)
			return err
		})
	}
	return allDevs, eg.Wait()
}

func (d *Discoverer) logCtx(ctx context.Context, sub string) zerolog.Logger {
	ll := log.Ctx(ctx).With().Str("component", "discovery")
	if sub != "" {
		ll = ll.Str("subcomponent", sub)
	}
	return ll.Logger()
}
