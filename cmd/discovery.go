package cmd

import (
	"os"
	"os/signal"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	discoveryFlags(discoveryCmd.Flags(), true)
	rootCmd.AddCommand(discoveryCmd)
}

var discoveryCmd = &cobra.Command{
	Use:   "discovery",
	Short: "List discoverable devices.",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, signalStop := signal.NotifyContext(ctx, os.Interrupt)
		defer signalStop()

		l := log.Ctx(ctx)

		dOpts, err := discoveryOptionsFromFlags()
		if err != nil {
			l.Fatal().Err(err).Msg("parsing flags")
		}
		disc := discovery.NewDiscoverer(dOpts...)
		if err := discoveryAddDevices(ctx, disc); err != nil {
			l.Fatal().Err(err).Msg("adding devices")
		}

		for _, mac := range bleDevices {
			if _, err := disc.AddBLE(ctx, mac); err != nil {
				l.Fatal().Err(err).Msg("adding BLE device")
			}
		}

		if bleSearch {
			if err := disc.SearchBLE(ctx); err != nil {
				l.Fatal().Err(err).Msg("searching bluetooth")
			}
		}

	},
}
