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
	"github.com/rs/zerolog/log"
)

const (
	mdnsSearchBuffer = 50
)

// SearchMDNS finds new devices via mDNS.
func (d *Discoverer) SearchMDNS(ctx context.Context) ([]*Device, error) {
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
	workerLimiter := make(chan struct{}, d.concurrency)
	defer close(workerLimiter)
	var outputLock sync.Mutex
	var output []*Device

	go func() {
		defer wg.Done()
		for se := range c {
			se := se
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

func (d *Discoverer) processMDNSServiceEntry(ctx context.Context, se *mdns.ServiceEntry) *Device {
	l := log.Ctx(ctx).With().
		Str("component", "discovery").
		Str("mdns_name", se.Name).
		Str("mdns_host", se.Host).
		Logger()
	var addr net.IP
	if d.preferIPVersion != "6" && !se.AddrV4.IsUnspecified() {
		addr = se.AddrV4
	} else if d.preferIPVersion != "4" && !se.AddrV6.IsUnspecified() {
		addr = se.AddrV6
	} else {
		l.Warn().
			IPAddr("addr_v4", se.AddrV4).
			IPAddr("addr_v6", se.AddrV6).
			Msg("mDNS advertisement with unknown or missing ")
		return nil
	}
	l = l.With().IPAddr("mdns_addr", addr).Logger()
	var genFound bool
	for _, f := range se.InfoFields {
		k, v, ok := strings.Cut(f, "=")
		if !ok || k != "gen" {
			continue
		}
		genFound = true
		if v != "2" {
			l.Warn().Str("gen", v).Msg("unsupport device `gen`")
			return nil
		}
	}
	if !genFound {
		l.Warn().Msg("mdns record missing `gen` field; skipping")
		return nil
	}
	u := url.URL{
		Scheme: "http",
		Path:   "/rpc",
		Host:   net.JoinHostPort(addr.String(), strconv.Itoa(se.Port)),
	}
	dev, err := d.AddDeviceByAddress(ctx, u.String(), sourceIsMDNS)
	if err != nil {
		l.Err(err).Msg("adding device")
		return nil
	}
	return dev
}
