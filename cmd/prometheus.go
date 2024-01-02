package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	bindAddr net.IP
	bindPort uint16
)

func init() {
	prometheusCmd.Flags().IPVar(&bindAddr, "bind-addr", net.IPv6zero, "local ip address to bind the metrics server to")
	prometheusCmd.Flags().Uint16Var(&bindPort, "bind-port", 8080, "port to bind the metrics server")

	rootCmd.AddCommand(prometheusCmd)
}

var prometheusCmd = &cobra.Command{
	Use:     "prometehus",
	Aliases: []string{"prom"},
	Short:   "host a prometheus metrics exporter for shelly devices",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, signalStop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer signalStop()

		l := log.Logger
		ctx = l.WithContext(ctx)

		d := discovery.NewDiscoverer()
		devs, err := d.MDNSSearch(ctx)
		if err != nil {
			l.Err(err).Msg("searching for devices via mDNS")
			return
		}
		for _, dev := range devs {
			fmt.Printf("%s:\n", dev.MACAddr)
			c, err := dev.Open(ctx)
			if err != nil {
				l.Err(err).Str("mac", dev.MACAddr).Msg("opening channel to device")
				continue
			}
			status, _, err := (&shelly.ShellyGetStatusRequest{}).Do(ctx, c)
			if err != nil {
				l.Err(err).Str("mac", dev.MACAddr).Msg("querying device for status")
				continue
			}
			for _, s := range status.Switches {
				fmt.Printf("  switch %d - output=%v\n", s.ID, *s.Output)
			}
			if err := c.Disconnect(ctx); err != nil {
				l.Warn().Err(err).Str("mac", dev.MACAddr).Msg("disconnecting from device")
				continue
			}
		}
	},
}
