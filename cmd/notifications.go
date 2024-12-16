package cmd

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	discoveryFlags(notificationsCmd.Flags(), discoveryFlagsOptions{
		withTTL:                    true,
		interactive:                false,
		searchStrictTimeoutDefault: true,
	})
	rootCmd.AddCommand(notificationsCmd)
	rootCmd.AddGroup(&cobra.Group{
		ID:    "notifications",
		Title: "Notifications:",
	})
}

var notificationsCmd = &cobra.Command{
	Use:     "watch",
	GroupID: "notifications",
	Aliases: []string{""},
	// TODO - Support subscriptions via BLE + Websocket Server
	Short: "Subscribe to status notifications (via MQTT)",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, signalStop := signal.NotifyContext(ctx, os.Interrupt)
		defer signalStop()

		l := log.Ctx(ctx)

		dOpts, err := discoveryOptionsFromFlags(cmd.Flags())
		if err != nil {
			l.Fatal().Err(err).Msg("parsing flags")
		}
		disc := discovery.NewDiscoverer(dOpts...)
		fsnChan := disc.GetFullStatusNotifications(50)
		snChan := disc.GetStatusNotifications(50)
		enChan := disc.GetEventNotifications(50)
		if err := disc.MQTTConnect(ctx); err != nil {
			l.Fatal().Err(err).Msg("connecting to MQTT broker")
		}
		if err := discoveryAddDevices(ctx, disc); err != nil {
			l.Fatal().Err(err).Msg("adding devices")
		}
		for {
			select {
			case <-ctx.Done():
				l.Info().Msg("shutting down notification watch")
				return
			case fsn := <-fsnChan:
				log.Debug().
					Str("src", fsn.Frame.Src).
					Str("dst", fsn.Frame.Dst).
					Str("method", fsn.Frame.Method).
					Any("msg", fsn.Status).
					Float64("timestamp", fsn.Status.TS).
					Str("raw", string(fsn.Frame.Params)).
					Msg("got NotifyFullStatus")
				Output(
					ctx,
					fmt.Sprintf("Received NotifyFullStatus frame from %s", fsn.Frame.Src),
					"notification",
					fsn.Status,
					fsn.Frame.Params,
				)
			case sn := <-snChan:
				log.Debug().
					Str("src", sn.Frame.Src).
					Str("dst", sn.Frame.Dst).
					Str("method", sn.Frame.Method).
					Any("msg", sn.Status).
					Float64("timestamp", sn.Status.TS).
					Str("raw", string(sn.Frame.Params)).
					Msg("got NotifyStatus")
				Output(
					ctx,
					fmt.Sprintf("Received NotifyStatus frame from %s", sn.Frame.Src),
					"notification",
					sn.Status,
					sn.Frame.Params,
				)
			case en := <-enChan:
				log.Debug().
					Str("src", en.Frame.Src).
					Str("dst", en.Frame.Dst).
					Str("method", en.Frame.Method).
					Any("msg", en.Event).
					Float64("timestamp", en.Event.TS).
					Str("raw", string(en.Frame.Params)).
					Msg("got NotifyStatus")
				Output(
					ctx,
					fmt.Sprintf("Received NotifyStatus frame from %s", en.Frame.Src),
					"notification",
					en.Event,
					en.Frame.Params,
				)
			}
		}
	},
}
