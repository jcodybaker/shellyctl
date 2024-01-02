package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/jcodybaker/shellyctl/pkg/logcompat"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	hosts             []string
	logLevel          string
	mdnsDiscover      bool
	mdnsInterface     string
	mdnsZone          string
	mdnsService       string
	mdnsSearchTimeout time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "shellyctl",
	Short: "shellyctl provides a cli interface for discovering and working with shelly gen 2 devices",
}

func init() {
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		rootCmd.Help()
	}
	rootCmd.PersistentFlags().StringArrayVar(
		&hosts,
		"host",
		[]string{},
		"host address of a single device. IP, DNS, or mDNS/BonJour addresses are accepted. If a URL scheme is provided, only `http` and `https` are supported. mDNS names must be within the zone specified by the `--mdns-zone` flag (default `local`).")
	rootCmd.PersistentFlags().BoolVar(
		&mdnsDiscover,
		"mdns-discover",
		false,
		"if true, devices will be discovered via mDNS")
	rootCmd.PersistentFlags().StringVar(
		&mdnsInterface,
		"mdns-interface",
		"",
		"if specified, search only the specified network interface for devices.")
	rootCmd.PersistentFlags().StringVar(
		&mdnsZone,
		"mdns-zone",
		discovery.DefaultMDNSZone,
		"mDNS zone to search")
	rootCmd.PersistentFlags().StringVar(
		&mdnsService,
		"mdns-service",
		discovery.DefaultMDNSService,
		"mDNS service to search")
	rootCmd.PersistentFlags().DurationVar(
		&mdnsSearchTimeout,
		"mdns-search-timeout",
		discovery.DefaultMDNSSearchTimeout,
		"timeout for devices to respond to the mDNS discovery query.",
	)
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "warn", "threshold for outputing logs: trace, debug, info, warn, error, fatal, panic")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()

		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		switch strings.ToLower(logLevel) {
		case "trace":
			log.Logger = log.Level(zerolog.TraceLevel)
		case "debug":
			log.Logger = log.Level(zerolog.DebugLevel)
		case "info":
			log.Logger = log.Level(zerolog.InfoLevel)
		case "warn":
			log.Logger = log.Level(zerolog.WarnLevel)
		case "error":
			log.Logger = log.Level(zerolog.ErrorLevel)
		case "fatal":
			log.Logger = log.Level(zerolog.FatalLevel)
		case "panic":
			log.Logger = log.Level(zerolog.PanicLevel)
		default:
			return errors.New("unknown value for --log-level")
		}

		logcompat.Init(&log.Logger)
		return nil
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
