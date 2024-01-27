package discovery

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/mdns"
)

const (
	mdnsSearchBuffer = 50
)

// searchMDNS finds new devices via mDNS.
func (d *Discoverer) searchMDNS(ctx context.Context, stop chan struct{}) ([]*Device, error) {
	ll := d.logCtx(ctx, "mdns")
	if !d.mdnsSearchEnabled {
		return nil, nil
	}
	c := make(chan *mdns.ServiceEntry, mdnsSearchBuffer)
	params := &mdns.QueryParam{
		Service:             d.mdnsService,
		Domain:              d.mdnsZone,
		Timeout:             d.searchTimeout,
		Entries:             c,
		WantUnicastResponse: true,
		Interface:           d.mdnsInterface,
	}
	switch d.preferIPVersion {
	case "4":
		params.DisableIPv6 = true
	case "6":
		params.DisableIPv4 = true
	}

	var wg sync.WaitGroup
	wg.Add(1)

	approver := newApprover[*mdns.ServiceEntry](d, stop)
	defer approver.done()

	workerLimiter := make(chan struct{}, d.concurrency)
	defer close(workerLimiter)
	var outputLock sync.Mutex
	var output []*Device

	go func() {
		defer wg.Done()
		defer approver.done()
		for se := range c {
			desc := fmt.Sprintf(
				"mDNS device %q (%s)",
				d.mdnsSEName(se),
				d.mdnsSEAddr(se).String())
			approver.submit(ctx, se, desc)
		}
	}()

	// Display the confirmation dialogues if necessary, then pass devices to the approved queue.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := approver.run(ctx); err != nil {
			ll.Err(err).Msg("confirming devices")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			se := approver.getApproved(ctx)
			if se == nil {
				return
			}
			wg.Add(1)
			// Occupy a space in the workerLimiter buffer or block until one is available.
			workerLimiter <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-workerLimiter }()
				if dev := d.processMDNSServiceEntry(ctx, se); dev != nil {
					outputLock.Lock()
					defer outputLock.Unlock()
					output = append(output, dev)
				}
			}()
		}
	}()

	if err := d.mdnsQueryFunc(params); err != nil {
		close(c)
		return nil, fmt.Errorf("querying mdns for devices: %w", err)
	}
	close(c)

	wg.Wait()
	return output, nil
}

func sourceIsMDNS(dev *Device) {
	dev.source = sourceMDNS
}

func (d *Discoverer) mdnsSEAddr(se *mdns.ServiceEntry) net.IP {
	if d.preferIPVersion != "6" && !se.AddrV4.IsUnspecified() {
		return se.AddrV4
	} else if d.preferIPVersion != "4" && !se.AddrV6.IsUnspecified() {
		return se.AddrV6
	}
	return nil
}

func (d *Discoverer) mdnsSEName(se *mdns.ServiceEntry) string {
	return strings.TrimSuffix(se.Host, "."+d.mdnsZone+".")
}

func (d *Discoverer) processMDNSServiceEntry(ctx context.Context, se *mdns.ServiceEntry) *Device {
	ll := d.logCtx(ctx, "mdns").With().
		Str("mdns_name", se.Name).
		Str("mdns_host", se.Host).
		Logger()
	addr := d.mdnsSEAddr(se)
	if addr == nil {
		ll.Warn().
			IPAddr("addr_v4", se.AddrV4).
			IPAddr("addr_v6", se.AddrV6).
			Msg("mDNS advertisement with unknown or missing ")
		return nil
	}
	ll = ll.With().IPAddr("mdns_addr", addr).Logger()
	var genFound bool
	for _, f := range se.InfoFields {
		k, v, ok := strings.Cut(f, "=")
		if !ok || k != "gen" {
			continue
		}
		genFound = true
		switch v {
		case "2", "3":
			// ok
		default:
			ll.Warn().Str("gen", v).Msg("unsupport device `gen`")
			return nil
		}
	}
	if !genFound {
		ll.Warn().Msg("mdns record missing `gen` field; skipping")
		return nil
	}
	u := url.URL{
		Scheme: "http",
		Path:   "/rpc",
		Host:   net.JoinHostPort(addr.String(), strconv.Itoa(se.Port)),
	}
	dev, err := d.AddDeviceByAddress(ctx, u.String(), sourceIsMDNS, WithDeviceName(d.mdnsSEName(se)))
	if err != nil {
		ll.Err(err).Msg("adding device")
		return nil
	}
	return dev
}
