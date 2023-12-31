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

// MDNSSearch finds new devices via mDNS.
func (d *Discoverer) MDNSSearch(ctx context.Context) ([]*Device, error) {
	c := make(chan *mdns.ServiceEntry, mdnsSearchBuffer)
	params := &mdns.QueryParam{
		Service:             d.mdnsService,
		Domain:              d.mdnsZone,
		Timeout:             d.mdnsSearchTimeout,
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
	workerLimiter := make(chan struct{}, d.mdnsWorkers)
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
	var addr net.IP
	if d.preferIPVersion != "6" && !se.AddrV4.IsUnspecified() {
		addr = se.AddrV4
	} else if d.preferIPVersion != "4" && !se.AddrV6.IsUnspecified() {
		addr = se.AddrV6
	} else {
		// TODO(cbaker) log
		fmt.Println("unknown address format")
		return nil
	}
	var genFound bool
	for _, f := range se.InfoFields {
		k, v, ok := strings.Cut(f, "=")
		if !ok || k != "gen" {
			continue
		}
		genFound = true
		if v != "2" {
			// TODO(cbaker) log
			fmt.Printf("unsupport device `gen`: %q\n", f)
			return nil
		}
	}
	if !genFound {
		// TODO(cbaker) log
		fmt.Println("mdns record missing `gen` field; skipping")
		return nil
	}
	u := url.URL{
		Scheme: "http",
		Path:   "/rpc",
		Host:   net.JoinHostPort(addr.String(), strconv.Itoa(se.Port)),
	}
	dev, err := d.AddDeviceByAddress(ctx, u.String(), sourceIsMDNS)
	if err != nil {
		// TODO(cbaker) log
		fmt.Println(err)
		return nil
	}
	return dev
}
