package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

var (
	hosts                []string
	mdnsSearch           bool
	bleSearch            bool
	bleDevices           []string
	mdnsInterface        string
	mdnsZone             string
	mdnsService          string
	discoveryDeviceTTL   time.Duration
	searchTimeout        time.Duration
	discoveryConcurrency int
	skipFailedHosts      bool

	preferIPVersion string
)

func discoveryFlags(f *pflag.FlagSet, withTTL bool) {
	f.StringArrayVar(
		&hosts,
		"host",
		[]string{},
		"host address of a single device. IP, DNS, or mDNS/BonJour addresses are accepted. If a URL scheme is provided, only `http` and `https` are supported. mDNS names must be within the zone specified by the `--mdns-zone` flag (default `local`).")

	f.BoolVar(
		&mdnsSearch,
		"mdns-search",
		false,
		"if true, devices will be discovered via mDNS")

	f.BoolVar(
		&bleSearch,
		"ble-search",
		false,
		"if true, devices will be discovered via Bluetooth Low-Energy")

	f.StringArrayVar(
		&bleDevices,
		"ble-device",
		[]string{},
		"MAC address of a single bluetooth low-energy device. May be specified multiple times to work with multiple devices.")

	f.StringVar(
		&mdnsInterface,
		"mdns-interface",
		"",
		"if specified, search only the specified network interface for devices.")

	f.StringVar(
		&mdnsZone,
		"mdns-zone",
		discovery.DefaultMDNSZone,
		"mDNS zone to search")

	f.StringVar(
		&mdnsService,
		"mdns-service",
		discovery.DefaultMDNSService,
		"mDNS service to search")

	f.DurationVar(
		&searchTimeout,
		"search-timeout",
		discovery.DefaultMDNSSearchTimeout,
		"timeout for devices to respond to the mDNS discovery query.",
	)

	f.IntVar(
		&discoveryConcurrency,
		"discovery-concurrency",
		discovery.DefaultConcurrency,
		"number of concurrent ",
	)

	f.StringVar(
		&preferIPVersion,
		"prefer-ip-version",
		"",
		"prefer ip version (`4` or `6`)")

	f.BoolVar(
		&skipFailedHosts,
		"skip-failed-hosts",
		false,
		"continue with other hosts in the face errors.",
	)

	if withTTL {
		f.DurationVar(
			&discoveryDeviceTTL,
			"device-ttl",
			discovery.DefaultDeviceTTL,
			"time-to-live for discovered devices in long-lived commands like the prometheus server.",
		)
	}
}

func discoveryOptionsFromFlags() (opts []discovery.DiscovererOption, err error) {
	if len(hosts) == 0 && len(bleDevices) == 0 && !mdnsSearch && !bleSearch {
		return nil, errors.New("no hosts and or discovery (mDNS)")
	}
	if mdnsInterface != "" {
		i, err := net.InterfaceByName(mdnsInterface)
		if err != nil {
			return nil, fmt.Errorf("resolving interface name: %w", err)
		}
		opts = append(opts, discovery.WithMDNSInterface(i))
	}
	switch preferIPVersion {
	case "":
		// no action
	case "4", "6":
		opts = append(opts, discovery.WithIPVersion(preferIPVersion))
	default:
		return nil, errors.New("invalid value for --prefer-ip-version; must be `4` or `6`")
	}
	opts = append(opts,
		discovery.WithMDNSZone(mdnsZone),
		discovery.WithMDNSService(mdnsService),
		discovery.WithSearchTimeout(searchTimeout),
		discovery.WithConcurrency(discoveryConcurrency),
		discovery.WithDeviceTTL(discoveryDeviceTTL),
		discovery.WithMDNSSearchEnabled(mdnsSearch),
	)
	return opts, err
}

func discoveryAddDevices(ctx context.Context, d *discovery.Discoverer) error {
	l := log.Ctx(ctx)
	var wg sync.WaitGroup
	concurrencyLimit := make(chan struct{}, discoveryConcurrency)
	defer close(concurrencyLimit)
	defer wg.Wait()
	if len(bleDevices) > 0 {
		select {
		case concurrencyLimit <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				<-concurrencyLimit
			}()
			discoveryAddBLEDevices(ctx, d)
		}()
	}
	for _, h := range hosts {
		// This chan send will block if the we exceed discoveryConcurrency.
		select {
		case concurrencyLimit <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				<-concurrencyLimit
			}()
			if _, err := d.AddDeviceByAddress(ctx, h); err != nil {
				if !skipFailedHosts {
					l.Fatal().Err(err).Msg("adding device")
					return
				}
				l.Warn().Err(err).Msg("adding device; continuing because `skip-failed-hosts=true`")
			}
		}()
	}
	return nil
}

func discoveryAddBLEDevices(ctx context.Context, d *discovery.Discoverer) error {
	l := log.Ctx(ctx)
	for _, mac := range bleDevices {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := d.AddBLE(ctx, mac)
		if err == nil {
			continue
		}
		if !skipFailedHosts {
			l.Fatal().Err(err).Msg("adding device")
		}
		l.Warn().Err(err).Msg("adding device; continuing because `skip-failed-hosts=true`")
	}
	return nil
}
