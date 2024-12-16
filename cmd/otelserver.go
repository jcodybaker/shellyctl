package cmd

import (
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/jcodybaker/shellyctl/pkg/otelserver"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
)

func init() {
	otelCmd.Flags().Duration("stop-wait", otelserver.DefaultStopWait, "maximum duration to wait for a clean shutdown")
	otelCmd.Flags().String("meter-name", otelserver.DefeaultMeterName, "name of the meter")

	otelCmd.Flags().Duration("otel-exporter-interval", 10*time.Second, "OTEL exporter interval. This is the time between sending batches of metrics.")
	otelCmd.Flags().String("otel-exporter-protocol", "grpc", "OTEL exporter protocol. This may be one of `grpc`, `https` or `http`.")
	otelCmd.Flags().String("otel-exporter-endpoint", "", "OTEL endpoint to send metrics to. This may be in the format of `example.com:4317` or URI `https://example.com:4318/v1/traces`.")
	otelCmd.Flags().Bool("otel-exporter-insecure", false, "OTEL exporter insecure flag. This is needed if the endpoint does not support TLS.")
	otelCmd.Flags().Bool("otel-exporter-gzip", false, "OTEL exporter gzip flag. This will enable gzip compression on the request body.")
	otelCmd.Flags().StringArray("otel-exporter-header", nil, "OTEL exporter headers specified as `k=v` to add to the request. This may be specified multiple times.")
	otelCmd.Flags().Bool("otel-exporter-retry", true, "OTEL exporter retry flag. This will enable retry logic on the exporter.")
	otelCmd.Flags().Duration("otel-exporter-retry-initial-interval", 5*time.Second, "OTEL exporter retry initial interval. This is the time to wait between retries.")
	otelCmd.Flags().Duration("otel-exporter-retry-max-interval", 30*time.Second, "OTEL exporter retry initial interval. This is the upper bound on backoff interval. Once this value is reached the delay between consecutive retries will always be the max-interval.")
	otelCmd.Flags().Duration("otel-exporter-retry-max-elapsed-time", 1*time.Minute, "OTEL exporter retry initial interval. This is the maximum amount of time (including retries) spent trying to send a request/batch. Once this value is reached, the data is discarded.")
	otelCmd.Flags().Duration("otel-exporter-timeout", 10*time.Second, "OTEL exporter timeout. This is the maximum time to wait for a request to complete.")

	otelCmd.Flags().IP("prometheus-bind-addr", net.IPv6zero, "local ip address to bind the metrics server to")
	otelCmd.Flags().Uint16("prometheus-bind-port", 8080, "port to bind the metrics server")
	otelCmd.Flags().String("prometheus-namespace", "shelly_status", "set the namespace/subsystem string to use for prometheus metric names.")
	// otelCmd.Flags().Int("probe-concurrency", promserver.DefaultConcurrency, "set the number of concurrent probes which will be made to service a metrics request.")
	// otelCmd.Flags().Duration("device-timeout", promserver.DefaultDeviceTimeout, "set the maximum time allowed for a device to respond to it probe.")
	// otelCmd.Flags().Duration("scrape-duration-warning", promserver.DefaultScrapeDurationWarning, "sets the value for scrape duration warning. Scrapes which exceed this duration will log a warning generate. Default value 8s is 80% of the 10s default prometheus scrape_timeout.")

	discoveryFlags(otelCmd.Flags(), discoveryFlagsOptions{
		withTTL:                    true,
		interactive:                false,
		searchStrictTimeoutDefault: true,
	})
	rootCmd.AddCommand(otelCmd)
	rootCmd.AddGroup(&cobra.Group{
		ID:    "servers",
		Title: "Servers:",
	})
}

var otelCmd = &cobra.Command{
	Use:     "otel",
	GroupID: "servers",
	Short:   "Host a otel metrics exporter for shelly devices",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, signalStop := signal.NotifyContext(ctx, os.Interrupt)
		defer signalStop()

		l := log.Ctx(ctx)

		dOpts, err := discoveryOptionsFromFlags(cmd.Flags())
		if err != nil {
			l.Fatal().Err(err).Msg("parsing flags")
		}
		disc := discovery.NewDiscoverer(dOpts...)
		if err := disc.MQTTConnect(ctx); err != nil {
			l.Fatal().Err(err).Msg("connecting to MQTT broker")
		}
		if err := discoveryAddDevices(ctx, disc); err != nil {
			l.Fatal().Err(err).Msg("adding devices")
		}

		opts := []otelserver.Option{}
		hOpts := []otlpmetrichttp.Option{}
		switch viper.GetString("otel-exporter-protocol") {
		case "grpc":
			gOpts := []otlpmetricgrpc.Option{}
			if viper.IsSet("otel-exporter-endpoint") {
				v := viper.GetString("otel-exporter-endpoint")
				if strings.Contains(v, "://") {
					gOpts = append(gOpts, otlpmetricgrpc.WithEndpointURL(v))
				} else {
					gOpts = append(gOpts, otlpmetricgrpc.WithEndpoint(v))
				}
			}
			if viper.GetBool("otel-exporter-insecure") {
				gOpts = append(gOpts, otlpmetricgrpc.WithInsecure())
			}
			if h := viper.GetStringSlice("otel-exporter-header"); len(h) > 0 {
				headers := make(map[string]string, len(h))
				for _, v := range h {
					parts := strings.SplitN(v, "=", 2)
					if len(parts) != 2 {
						l.Fatal().Str("header", v).Msg("invalid header")
					}
				}
				gOpts = append(gOpts, otlpmetricgrpc.WithHeaders(headers))
			}
			r := otlpmetricgrpc.RetryConfig{
				Enabled:         viper.GetBool("otel-exporter-retry"),
				InitialInterval: viper.GetDuration("otel-exporter-retry-initial-interval"),
				MaxInterval:     viper.GetDuration("otel-exporter-retry-max-interval"),
				MaxElapsedTime:  viper.GetDuration("otel-exporter-retry-max-elapsed-time"),
			}
			gOpts = append(gOpts, otlpmetricgrpc.WithRetry(r))
			e, err := otlpmetricgrpc.New(ctx, gOpts...)
			if err != nil {
				l.Fatal().Err(err).Msg("creating otel exporter")
			}
			opts = append(opts, otelserver.WithMetricsExporter(e, viper.GetDuration("otel-exporter-interval")))
		case "http":
			hOpts = append(hOpts, otlpmetrichttp.WithInsecure())
			fallthrough
		case "https":
			if viper.IsSet("otel-exporter-endpoint") {
				v := viper.GetString("otel-exporter-endpoint")
				if strings.Contains(v, "://") {
					hOpts = append(hOpts, otlpmetrichttp.WithEndpointURL(v))
				} else {
					hOpts = append(hOpts, otlpmetrichttp.WithEndpoint(v))
				}
			}
			if viper.GetBool("otel-exporter-insecure") {
				hOpts = append(hOpts, otlpmetrichttp.WithInsecure())
			}
			if h := viper.GetStringSlice("otel-exporter-header"); len(h) > 0 {
				headers := make(map[string]string, len(h))
				for _, v := range h {
					parts := strings.SplitN(v, "=", 2)
					if len(parts) != 2 {
						l.Fatal().Str("header", v).Msg("invalid header")
					}
				}
				hOpts = append(hOpts, otlpmetrichttp.WithHeaders(headers))
			}
			r := otlpmetrichttp.RetryConfig{
				Enabled:         viper.GetBool("otel-exporter-retry"),
				InitialInterval: viper.GetDuration("otel-exporter-retry-initial-interval"),
				MaxInterval:     viper.GetDuration("otel-exporter-retry-max-interval"),
				MaxElapsedTime:  viper.GetDuration("otel-exporter-retry-max-elapsed-time"),
			}
			hOpts = append(hOpts, otlpmetrichttp.WithRetry(r))
			e, err := otlpmetrichttp.New(ctx, hOpts...)
			if err != nil {
				l.Fatal().Err(err).Msg("creating otel exporter")
			}
			if viper.GetBool("otel-exporter-gzip") {
				hOpts = append(hOpts, otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression))
			}
			opts = append(opts, otelserver.WithMetricsExporter(e, viper.GetDuration("otel-exporter-interval")))
		case "prometheus":
			pOpts := []prometheus.Option{}
			if viper.IsSet("prometheus-namespace") {
				pOpts = append(pOpts, prometheus.WithNamespace(viper.GetString("prometheus-namespace")))
			} else {
				pOpts = append(pOpts, prometheus.WithNamespace("shelly_status"))
			}
			e, err := prometheus.New(pOpts...)
			if err != nil {
				l.Fatal().Err(err).Msg("creating prometheus otel exporter")
			}
			opts = append(opts, otelserver.WithMetricsReader(e))
		default:
			l.Fatal().Str("protocol", viper.GetString("otel-exporter-protocol")).Msg("unknown protocol")
		}
		os := otelserver.NewServer(ctx, disc, opts...)
		if err := os.Run(ctx); err != nil {
			l.Fatal().Err(err).Msg("starting otel server")
		}
	},
}
