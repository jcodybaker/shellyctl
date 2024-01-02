package cmd

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/jcodybaker/shellyctl/pkg/promserver"
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
		ctx, signalStop := signal.NotifyContext(ctx, os.Interrupt)
		defer signalStop()

		l := log.Ctx(ctx)

		disc := discovery.NewDiscoverer()

		ps := promserver.NewServer(ctx, disc)

		hs := http.Server{
			Handler: ps,
			Addr:    net.JoinHostPort(bindAddr.String(), strconv.Itoa(int(bindPort))),
		}
		go func() {
			<-ctx.Done()
			sCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := hs.Shutdown(sCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				l.Err(err).Msg("shutting down http server")
			}
		}()
		l.Info().Msg("starting metrics server")
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.Err(err).Msg("starting http server")
		}
	},
}
