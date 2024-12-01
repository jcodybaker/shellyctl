package cmd

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
		mqttDevices := viper.GetStringSlice("mqtt-device")
		if len(mqttDevices) == 0 && !viper.IsSet("mqtt-topic") {
			dOpts = append(dOpts, discovery.WithMQTTTopicSubscriptions([]string{"+/events/rpc"}))
		}
		disc := discovery.NewDiscoverer(dOpts...)
		fsnChan := disc.GetFullStatusNotifications(50)
		snChan := disc.GetStatusNotifications(50)
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
					Msg("got NotifyStatus")
				Output(
					ctx,
					fmt.Sprintf("Received NotifyStatus frame from %s", sn.Frame.Src),
					"notification",
					sn.Status,
					sn.Frame.Params,
				)
			}
		}
	},
}