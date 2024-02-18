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
	"github.com/spf13/viper"
)

func init() {
	prometheusCmd.Flags().IP("bind-addr", net.IPv6zero, "local ip address to bind the metrics server to")
	prometheusCmd.Flags().Uint16("bind-port", 8080, "port to bind the metrics server")
	prometheusCmd.Flags().String("prometheus-namespace", promserver.DefaultNamespace, "set the namespace string to use for prometheus metric names.")
	prometheusCmd.Flags().String("prometheus-subsystem", promserver.DefaultSubsystem, "set the subsystem section of the prometheus metric names.")
	prometheusCmd.Flags().Int("probe-concurrency", promserver.DefaultConcurrency, "set the number of concurrent probes which will be made to service a metrics request.")
	prometheusCmd.Flags().Duration("device-timeout", promserver.DefaultDeviceTimeout, "set the maximum time allowed for a device to respond to it probe.")
	prometheusCmd.Flags().Duration("scrape-duration-warning", promserver.DefaultScrapeDurationWarning, "sets the value for scrape duration warning. Scrapes which exceed this duration will log a warning generate. Default value 8s is 80% of the 10s default prometheus scrape_timeout.")
	discoveryFlags(prometheusCmd.Flags(), true, false)
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

		dOpts, err := discoveryOptionsFromFlags(cmd.Flags())
		if err != nil {
			l.Fatal().Err(err).Msg("parsing flags")
		}
		disc := discovery.NewDiscoverer(dOpts...)
		if err := discoveryAddDevices(ctx, disc); err != nil {
			l.Fatal().Err(err).Msg("adding devices")
		}

		ps := promserver.NewServer(
			ctx,
			disc,
			promserver.WithPrometheusNamespace(viper.GetString("prometheus-namespace")),
			promserver.WithPrometheusSubsystem(viper.GetString("prometheus-subsystem")),
			promserver.WithConcurrency(viper.GetInt("probe-concurrency")),
			promserver.WithScrapeDurationWarning(viper.GetDuration("scrape-duration-warning")),
			promserver.WithDeviceTimeout(viper.GetDuration("device-timeout")),
		)

		hs := http.Server{
			Handler: ps,
			Addr:    net.JoinHostPort(viper.GetString("bind-addr"), strconv.Itoa(int(viper.GetUint16("bind-port")))),
		}
		go func() {
			<-ctx.Done()
			sCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := hs.Shutdown(sCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				l.Err(err).Msg("shutting down http server")
			}
		}()
		l.Info().Str("bind_address", hs.Addr).Msg("starting metrics server")
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.Err(err).Msg("starting http server")
		}
	},
}
