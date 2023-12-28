package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	hosts             []string
	mdnsDiscover      bool
	mdnsInterface     string
	mdnsZone          string
	mdnsService       string
	mdnsSearchTimeout time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "shellyctl",
	Short: "shellyctl provides a cli interface for discovering and working with shelly gen 2 devices",
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

func init() {
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
		"local",
		"mDNS zone to search")
	rootCmd.PersistentFlags().StringVar(
		&mdnsService,
		"mdns-service",
		"_shelly._tcp",
		"mDNS service to search")
	rootCmd.PersistentFlags().DurationVar(
		&mdnsSearchTimeout,
		"mdns-search-timeout",
		1*time.Second,
		"timeout for devices to respond to the mDNS discovery query.",
	)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
