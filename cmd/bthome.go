package cmd

import (
	"context"
	"time"

	"github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	btHomeDiscoverCmd = &cobra.Command{
		Use:  "discover",
		RunE: btHomeDiscoverCmdRunE,
	}
)

func init() {
	btHomeDiscoverCmd.Flags().Int(
		"duration", 30, "duration of search in seconds",
	)
	btHome.Parent.AddCommand(btHomeDiscoverCmd)
	discoveryFlags(btHomeDiscoverCmd.Flags(), discoveryFlagsOptions{interactive: true})
}

func btHomeDiscoverCmdRunE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ll := log.Ctx(ctx).With().Str("request", (&shelly.BTHomeStartDeviceDiscoveryRequest{}).Method()).Logger()
	dOpts, err := discoveryOptionsFromFlags(cmd.Flags())
	if err != nil {
		ll.Fatal().Err(err).Msg("parsing flags")
	}

	discoverer := discovery.NewDiscoverer(dOpts...)
	if err := discoverer.MQTTConnect(ctx); err != nil {
		ll.Fatal().Err(err).Msg("connecting to MQTT broker")
	}
	if err := discoveryAddDevices(ctx, discoverer); err != nil {
		ll.Fatal().Err(err).Msg("adding devices")
	}
	if _, err := discoverer.Search(ctx); err != nil {
		return err
	}

	for _, d := range discoverer.AllDevices() {
		ll := d.Log(ll)
		conn, err := d.Open(ctx)
		if err != nil {
			return err
		}
		defer func() {
			if err := conn.Disconnect(ctx); err != nil {
				ll.Warn().Err(err).Msg("disconnecting from device")
			}
		}()
		req := &shelly.BTHomeStartDeviceDiscoveryRequest{
			Duration: viper.GetInt("duration"),
		}
		ll.Info().Any("request_body", req).Str("method", req.Method()).Msg("starting bthome discovery")
		resp := req.NewResponse()
		reqContext := ctx
		cancel := func() {} // no-op
		if dur := viper.GetDuration("rpc-timeout"); dur != 0 {
			reqContext, cancel = context.WithTimeout(ctx, time.Duration(req.Duration)*time.Second)
		}

		events := discoverer.GetEventNotifications(50)

		_, err = shelly.Do(reqContext, conn, d.AuthCallback(ctx), req, resp)
		cancel()
		if err != nil {
			ll.Fatal().Err(err).Msg("executing BTHome.StartDeviceDiscovery")
		}

		timeout := time.NewTimer(time.Duration(req.Duration) * time.Second)

	discoveryLoop:
		for {
			select {
			case e := <-events:
				// how do we match this to the particular channel.
				ll.Info().Str("event", string(e.Frame.Params)).Msg("got event")
			case <-ctx.Done():
				break discoveryLoop
			case <-timeout.C:
				break discoveryLoop
			}
		}
		timeout.Stop()

	}
	return nil
}
