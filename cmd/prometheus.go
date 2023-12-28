package cmd

import (
	"fmt"
	"net"

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
		fmt.Println("Hugo Static Site Generator v0.9 -- HEAD")
	},
}
