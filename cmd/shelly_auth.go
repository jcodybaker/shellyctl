package cmd

import (
	"context"
	"fmt"

	"github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	shellyAuthCmd = &cobra.Command{
		Use:  "set-auth",
		RunE: shellyAuthCmdRunE,
	}
)

func init() {
	shellyAuthCmd.Flags().String(
		"password", "", "password to use for auth. If empty, the password will be cleared.",
	)
	shellyComponent.Parent.AddCommand(shellyAuthCmd)
	discoveryFlags(shellyAuthCmd.Flags(), discoveryFlagsOptions{interactive: true})
}

func shellyAuthCmdRunE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ll := log.Ctx(ctx).With().Str("request", (&shelly.ShellySetAuthRequest{}).Method()).Logger()
	dOpts, err := discoveryOptionsFromFlags(cmd.Flags())
	if err != nil {
		ll.Fatal().Err(err).Msg("parsing flags")
	}

	password := viper.GetString("password")

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
		req, err := shelly.BuildShellyAuthRequest(ctx, conn, password)
		if err != nil {
			return fmt.Errorf("building %s request: %w", req.Method(), err)
		}
		ll.Info().Any("request_body", req).Str("method", req.Method()).Msg("sending request")
		resp := req.NewResponse()
		reqContext := ctx
		cancel := func() {} // no-op
		if dur := viper.GetDuration("rpc-timeout"); dur != 0 {
			reqContext, cancel = context.WithTimeout(ctx, dur)
		}
		raw, err := shelly.Do(reqContext, conn, d.AuthCallback(ctx), req, resp)
		cancel()
		if err != nil {
			return fmt.Errorf("executing %s: %w", req.Method(), err)
		}
		Output(
			ctx,
			fmt.Sprintf("Response to %s command for %s", req.Method(), d.BestName()),
			"response",
			resp,
			raw.Response,
		)
	}
	return nil
}
