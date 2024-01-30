package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/term"
)

var (
	auth                 string
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

func discoveryFlags(f *pflag.FlagSet, withTTL, interactive bool) {
	f.StringVar(
		&auth,
		"auth",
		"",
		"password to use for authenticating with devices.",
	)

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

	// search-interactive and interactive cannot use the BoolVar() pattern as the default
	// varies by command and the global be set to whatever the last value was.
	f.Bool(
		"search-interactive",
		interactive,
		"if true confirm devices discovered in search before proceeding with commands. Defers to --interactive if not explicitly set.",
	)

	f.Bool(
		"interactive",
		interactive,
		"if true prompt for confirmation or passwords.",
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

func discoveryOptionsFromFlags(flags *pflag.FlagSet) (opts []discovery.DiscovererOption, err error) {
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
	searchInteractive, err := flags.GetBool("search-interactive")
	if err != nil {
		return nil, err
	}
	explictSearchInteractive := flags.Lookup("search-interactive").Changed
	interactive, err := flags.GetBool("interactive")
	if err != nil {
		return nil, err
	}
	if !explictSearchInteractive {
		searchInteractive = interactive
	}
	auth, err := flags.GetString("auth")
	if err != nil {
		return nil, err
	}
	if auth != "" {
		opts = append(opts, discovery.WithAuthCallback(func(_ context.Context, _ string) (passwd string, err error) {
			return auth, nil
		}))
	} else if interactive {
		opts = append(opts, discovery.WithAuthCallback(passwordPrompt))
	}
	if searchInteractive {
		if (bleSearch || mdnsSearch) &&
			!term.IsTerminal(int(os.Stdin.Fd())) &&
			!explictSearchInteractive {
			log.Logger.Fatal().Msg("Search is configured w/ default `--search-interactive=true` but stdin looks" +
				" non-interactive. shellyctl will likely stall when devices are detected. If you're" +
				" certain your search will only find the indented devices you may set " +
				" `--search-interactive=false` to use all discovered devices. " +
				" If your terminal can responde to the interactive promps, you may explicitly" +
				" set --search-interactive=true.")
		}
		opts = append(opts, discovery.WithSearchConfirm(searchConfirm))
	}
	opts = append(opts,
		discovery.WithMDNSZone(mdnsZone),
		discovery.WithMDNSService(mdnsService),
		discovery.WithSearchTimeout(searchTimeout),
		discovery.WithConcurrency(discoveryConcurrency),
		discovery.WithDeviceTTL(discoveryDeviceTTL),
		discovery.WithMDNSSearchEnabled(mdnsSearch),
		discovery.WithBLESearchEnabled(bleSearch),
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

func searchConfirm(desc string) (approveDevice bool, continueSearch bool, err error) {
	for {
		fmt.Printf("\nFound device %s\n", desc)
		fmt.Println("y - Add device and continue search")
		fmt.Println("n - Skip this device and continue search")
		fmt.Println("u - Use this device and stop searching for additional devices")
		fmt.Println("a - Abort search without this device")
		fmt.Println("q - Quit without acting on this device or any others")
		fmt.Println("Use this device [y,n,u,a,q]?")
		for {
			in := []byte{0}
			if _, err := os.Stdin.Read(in); err != nil {
				if errors.Is(err, io.EOF) {
					return false, false, nil
				}
				return false, false, fmt.Errorf("reading prompt response: %w", err)
			}
			switch string(in) {
			case "y", "Y":
				return true, true, nil
			case "n", "N":
				return false, true, nil
			case "u", "U":
				return true, false, nil
			case "a", "A":
				return false, false, nil
			case "q", "Q":
				os.Exit(0)
				return
			case "\n":
				// quietly read another byte
				continue
			default:
				fmt.Printf("Unexpected response %q\n", in)
			}
		}
	}
}

func passwordPrompt(ctx context.Context, desc string) (w string, err error) {
	var password bytes.Buffer
	fmt.Printf("\nDevice %s requires authentication. Please enter a password:\n", desc)
	log.Ctx(ctx)

	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to convert terminal to raw mode for password entry")
	} else {
		defer func() {
			if err := terminal.Restore(int(os.Stdin.Fd()), oldState); err != nil {
				log.Ctx(ctx).Warn().Err(err).Msg("failed to convert terminal to raw mode for password entry")
			}
			fmt.Println()
			fmt.Println()
		}()
	}

	for i := 0; ; i++ {
		b := []byte{0}
		if _, err := os.Stdin.Read(b); err != nil {
			if errors.Is(err, io.EOF) {
				if i == 0 {
					return "", errors.New("input is closed")
				}
				return password.String(), nil
			}
			return "", err
		}
		switch b[0] {
		case '\n', '\r':
			return password.String(), nil
		}
		if err := password.WriteByte(b[0]); err != nil {
			return "", err
		}
		if _, err := os.Stdout.Write([]byte("*")); err != nil {
			return "", fmt.Errorf("writing to stdout")
		}
	}
}
