package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var addAll bool

type discoveryFlagsOptions struct {
	withTTL                    bool
	interactive                bool
	searchStrictTimeoutDefault bool
}

func discoveryFlags(f *pflag.FlagSet, opts discoveryFlagsOptions) {
	f.String(
		"auth",
		"",
		"password to use for authenticating with devices.",
	)

	f.StringArray(
		"host",
		[]string{},
		"host address of a single device. IP, DNS, or mDNS/BonJour addresses are accepted.\n"+
			"If a URL scheme is provided, only `http` and `https` schemes are supported.\n"+
			"mDNS names must be within the zone specified by the `--mdns-zone` flag (default `local`).\n"+
			"URL formatted auth is supported (ex. `http://admin:password@1.2.3.4/`)")

	f.Bool(
		"mdns-search",
		false,
		"if true, devices will be discovered via mDNS")

	f.Bool(
		"ble-search",
		false,
		"if true, devices will be discovered via Bluetooth Low-Energy")

	f.StringArray(
		"ble-device",
		[]string{},
		"MAC address of a single bluetooth low-energy device. May be specified multiple times to work with multiple devices.")

	f.String(
		"mdns-interface",
		"",
		"if specified, search only the specified network interface for devices.")

	f.String(
		"mdns-zone",
		discovery.DefaultMDNSZone,
		"mDNS zone to search")

	f.String(
		"mdns-service",
		discovery.DefaultMDNSService,
		"mDNS service to search")

	f.Duration(
		"search-timeout",
		discovery.DefaultMDNSSearchTimeout,
		"timeout for devices to respond to the mDNS discovery query.",
	)

	f.Bool(
		"search-strict-timeout",
		opts.searchStrictTimeoutDefault,
		"ignore devices which have been found but completed their initial query within the search-timeout",
	)

	// search-interactive and interactive cannot use the Bool() pattern as the default
	// varies by command and the global be set to whatever the last value was.
	f.Bool(
		"search-interactive",
		opts.interactive,
		"if true confirm devices discovered in search before proceeding with commands. Defers to --interactive if not explicitly set.",
	)

	f.Bool(
		"interactive",
		opts.interactive,
		"if true prompt for confirmation or passwords.",
	)

	f.Int(
		"discovery-concurrency",
		discovery.DefaultConcurrency,
		"number of concurrent discovery threads",
	)

	f.String(
		"prefer-ip-version",
		"",
		"prefer ip version (`4` or `6`)")

	f.Bool(
		"skip-failed-hosts",
		false,
		"continue with other hosts in the face errors.",
	)

	if opts.withTTL {
		f.Duration(
			"device-ttl",
			discovery.DefaultDeviceTTL,
			"time-to-live for discovered devices in long-lived commands like the prometheus server.",
		)
	}

	f.String(
		"mqtt-addr",
		"",
		"mqtt server address (URI format or hostname:port)",
	)
}

func discoveryOptionsFromFlags(flags *pflag.FlagSet) (opts []discovery.DiscovererOption, err error) {
	viper.BindPFlags(flags)
	hosts := viper.GetStringSlice("host")
	bleDevices := viper.GetStringSlice("ble-device")
	mdnsSearch := viper.GetBool("mdns-search")
	bleSearch := viper.GetBool("ble-search")
	mdnsInterface := viper.GetString("mdns-interface")
	preferIPVersion := viper.GetString("prefer-ip-version")

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
	searchInteractive := viper.GetBool("search-interactive")
	explictSearchInteractive := flags.Lookup("search-interactive").Changed
	interactive := viper.GetBool("interactive")

	if !explictSearchInteractive {
		searchInteractive = interactive
	}
	auth := viper.GetString("auth")
	if auth != "" {
		opts = append(opts, discovery.WithAuthCallback(func(_ context.Context, _ string) (passwd string, err error) {
			return auth, nil
		}))
	} else if interactive {
		opts = append(opts, discovery.WithAuthCallback(passwordPrompt))
	}

	if viper.IsSet("mqtt-addr") {
		var mqttConnectOptions mqtt.ClientOptions
		var u *url.URL
		addr := viper.GetString("mqtt-addr")
		if strings.Contains(addr, "://") {
			var err error
			u, err = url.Parse(addr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse `mqtt-addr`: %v", err)
			}
			if u.User != nil {
				mqttConnectOptions.Username = u.User.Username()
				mqttConnectOptions.Password, _ = u.User.Password()
			}
		} else {
			u = &url.URL{
				Host: addr,
			}
		}
		mqttPort := u.Port()
		if strings.EqualFold(u.Scheme, "mqtt") {
			u.Scheme = "tcp"
		}
		if strings.EqualFold(u.Scheme, "mqtts") {
			u.Scheme = "tcps"
		}
		if u.Scheme == "" {
			if mqttPort == "1883" {
				u.Scheme = "tcp"
			} else {
				u.Scheme = "tcps"
			}
		}
		if mqttPort == "" {
			if u.Scheme == "tcp" {
				mqttPort = "1883"
			} else {
				mqttPort = "8883"
			}
		}
		u.Host = net.JoinHostPort(u.Hostname(), mqttPort)
		if viper.IsSet("mqtt-user") {
			mqttConnectOptions.Username = viper.GetString("mqtt-user")
		}
		if viper.IsSet("mqtt-password") {
			mqttConnectOptions.Password = viper.GetString("mqtt-password")
		}
		if viper.IsSet("mqtt-tls-ca-cert") || viper.GetBool("mqtt-tls-insecure") {
			mqttConnectOptions.TLSConfig = &tls.Config{
				InsecureSkipVerify: viper.GetBool("mqtt-tls-insecure"),
			}
			if viper.IsSet("mqtt-tls-ca-cert") {
				mqttConnectOptions.TLSConfig.RootCAs = x509.NewCertPool()
				certs, err := os.ReadFile(viper.GetString("mqtt-tls-ca-cert"))
				if err != nil {
					return nil, fmt.Errorf("reading `mqtt-tls-ca-cert`: %w", err)
				}
				if ok := mqttConnectOptions.TLSConfig.RootCAs.AppendCertsFromPEM(certs); !ok {
					return nil, fmt.Errorf("failed to parse `mqtt-tls-ca-cert` as PEM cert bundle")
				}
			}
		}
		opts = append(opts, discovery.WithMQTTConnectOptions(mqttConnectOptions))
	} else {
		if viper.IsSet("mqtt-user") {
			return nil, errors.New("mqtt-user is invalid without mqtt-addr")
		}
		if viper.IsSet("mqtt-password") {
			return nil, errors.New("mqtt-password is invalid without mqtt-addr")
		}
		if viper.IsSet("mqtt-tls-ca-cert") {
			return nil, errors.New("mqtt-tls-ca-cert is invalid without mqtt-addr")
		}
		if viper.IsSet("mqtt-tls-insecure") {
			return nil, errors.New("mqtt-tls-insecure is invalid without mqtt-addr")
		}
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
		discovery.WithMDNSZone(viper.GetString("mdns-zone")),
		discovery.WithMDNSService(viper.GetString("mdns-service")),
		discovery.WithSearchTimeout(viper.GetDuration("search-timeout")),
		discovery.WithSearchStrictTimeout(viper.GetBool("search-strict-timeout")),
		discovery.WithConcurrency(viper.GetInt("discovery-concurrency")),
		discovery.WithDeviceTTL(viper.GetDuration("device-ttl")),
		discovery.WithMDNSSearchEnabled(mdnsSearch),
		discovery.WithBLESearchEnabled(bleSearch),
	)
	return opts, err
}

func discoveryAddDevices(ctx context.Context, d *discovery.Discoverer) error {
	l := log.Ctx(ctx)
	var wg sync.WaitGroup
	concurrencyLimit := make(chan struct{}, viper.GetInt("discovery-concurrency"))
	defer close(concurrencyLimit)
	defer wg.Wait()
	hosts := viper.GetStringSlice("host")
	bleDevices := viper.GetStringSlice("ble-device")
	skipFailedHosts := viper.GetBool("skip-failed-hosts")
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
	skipFailedHosts := viper.GetBool("skip-failed-hosts")
	bleDevices := viper.GetStringSlice("ble-device")
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
	if addAll {
		return true, true, nil
	}
	for {
		fmt.Printf("\nFound %s\n", desc)
		fmt.Println("y - Add device and continue search")
		fmt.Println("n - Skip this device and continue search")
		fmt.Println("a - Add this device and all other devices found")
		fmt.Println("u - Use this device and stop searching for additional devices")
		fmt.Println("s - Stop search without this device")
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
			case "a", "A":
				addAll = true
				return true, true, nil
			case "n", "N":
				return false, true, nil
			case "u", "U":
				return true, false, nil
			case "s", "S":
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

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("failed to convert terminal to raw mode for password entry")
	} else {
		defer func() {
			if err := term.Restore(int(os.Stdin.Fd()), oldState); err != nil {
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
