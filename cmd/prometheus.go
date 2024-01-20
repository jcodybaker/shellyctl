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
	bindAddr          net.IP
	bindPort          uint16
	promNamespace     string
	promSubsystem     string
	promConcurrency   int
	promDeviceTimeout time.Duration
)

func init() {
	prometheusCmd.Flags().IPVar(&bindAddr, "bind-addr", net.IPv6zero, "local ip address to bind the metrics server to")
	prometheusCmd.Flags().Uint16Var(&bindPort, "bind-port", 8080, "port to bind the metrics server")
	prometheusCmd.Flags().StringVar(&promNamespace, "prometheus-namespace", promserver.DefaultNamespace, "set the namespace string to use for prometheus metric names.")
	prometheusCmd.Flags().StringVar(&promSubsystem, "prometheus-subsystem", promserver.DefaultSubsystem, "set the subsystem section of the prometheus metric names.")
	prometheusCmd.Flags().IntVar(&promConcurrency, "probe-concurrency", promserver.DefaultConcurrency, "set the number of concurrent probes which will be made to service a metrics request.")
	prometheusCmd.Flags().DurationVar(&promDeviceTimeout, "device-timeout", promserver.DefaultDeviceTimeout, "set the maximum time allowed for a device to respond to it probe.")
	discoveryFlags(prometheusCmd.Flags(), true)
	rootCmd.AddCommand(prometheusCmd)
	rootCmd.AddGroup(&cobra.Group{
		ID:    "servers",
		Title: "Servers:",
	})
}

var prometheusCmd = &cobra.Command{
	Use:     "prometheus",
	GroupID: "servers",
	Aliases: []string{"prom"},
	Short:   "Host a prometheus metrics exporter for shelly devices",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, signalStop := signal.NotifyContext(ctx, os.Interrupt)
		defer signalStop()

		l := log.Ctx(ctx)

		dOpts, err := discoveryOptionsFromFlags()
		if err != nil {
			l.Fatal().Err(err).Msg("parsing flags")
		}
		disc := discovery.NewDiscoverer(dOpts...)
		if err := discoveryAddHosts(ctx, disc); err != nil {
			l.Fatal().Err(err).Msg("adding devices")
		}

		ps := promserver.NewServer(
			ctx,
			disc,
			promserver.WithPrometheusNamespace(promNamespace),
			promserver.WithPrometheusSubsystem(promSubsystem),
			promserver.WithConcurrency(promConcurrency),
			promserver.WithDeviceTimeout(promDeviceTimeout),
		)

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
