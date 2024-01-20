package cmd

import (
	"github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	bleComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "ble",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.BLEGetStatusRequest{},
			&shelly.BLEGetConfigRequest{},
			&shelly.BLESetConfigRequest{},
		},
	}

	cloudComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "cloud",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.CloudGetStatusRequest{},
			&shelly.CloudGetConfigRequest{},
			&shelly.CloudSetConfigRequest{},
		},
	}

	coverComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "cover",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.CoverGetStatusRequest{},
			&shelly.CoverGetConfigRequest{},
			&shelly.CoverSetConfigRequest{},
			&shelly.CoverCloseRequest{},
			&shelly.CoverCalibrateRequest{},
			&shelly.CoverGoToPositionRequest{},
			&shelly.CoverOpenRequest{},
			&shelly.CoverResetCountersRequest{},
			&shelly.CoverStopRequest{},
		},
	}

	inputComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "input",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.InputGetStatusRequest{},
			&shelly.InputGetConfigRequest{},
			&shelly.InputSetConfigRequest{},
			&shelly.InputCheckExpressionRequest{},
		},
	}

	lightComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "light",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.LightGetStatusRequest{},
			&shelly.LightGetConfigRequest{},
			&shelly.LightSetConfigRequest{},
			&shelly.LightSetRequest{},
			&shelly.LightToggleRequest{},
		},
	}

	mqttComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "mqtt",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.MQTTGetStatusRequest{},
			&shelly.MQTTGetConfigRequest{},
			&shelly.MQTTSetConfigRequest{},
		},
	}

	scheduleComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "schedule",
		},
		Requests: []shelly.RPCRequestBody{
			// &shelly.ScheduleCreateRequest{},
			// &shelly.ScheduleUpdateRequest{},
			&shelly.ScheduleDeleteRequest{},
			&shelly.ScheduleDeleteAllRequest{},
		},
	}

	shellyComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "shelly",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.ShellyGetStatusRequest{},
			&shelly.ShellyGetConfigRequest{},
			&shelly.ShellyCheckForUpdateRequest{},
			&shelly.ShellyRebootRequest{},
			&shelly.ShellySetAuthRequest{},
		},
	}

	switchComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "switch",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.SwitchGetConfigRequest{},
			&shelly.SwitchSetConfigRequest{},
			&shelly.SwitchSetRequest{},
			&shelly.SwitchToggleRequest{},
			&shelly.SwitchGetStatusRequest{},
		},
	}

	sysComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "sys",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.SysGetConfigRequest{},
			&shelly.SysSetConfigRequest{},
			&shelly.SysGetStatusRequest{},
		},
	}

	wifiComponent = &gencobra.Component{
		Parent: &cobra.Command{
			Use: "wifi",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.WifiGetStatusRequest{},
			&shelly.WifiGetConfigRequest{},
			&shelly.WifiSetConfigRequest{},
		},
	}

	components = []*gencobra.Component{
		bleComponent,
		cloudComponent,
		coverComponent,
		inputComponent,
		lightComponent,
		mqttComponent,
		scheduleComponent,
		shellyComponent,
		switchComponent,
		sysComponent,
		wifiComponent,
	}
)

func init() {
	baggage := &gencobra.Baggage{
		Output: Output,
	}
	if err := gencobra.ComponentsToCmd(components, baggage); err != nil {
		log.Panic().Err(err).Msg("generating menu for API commands")
	}
	for _, c := range components {
		discoveryFlags(c.Parent.PersistentFlags(), false)
		rootCmd.AddCommand(c.Parent)
		c.Parent.Run = func(cmd *cobra.Command, args []string) {
			c.Parent.Help()
		}
		c.Parent.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
				return err
			}
			ctx := cmd.Context()
			l := log.Ctx(ctx)
			dOpts, err := discoveryOptionsFromFlags()
			if err != nil {
				l.Fatal().Err(err).Msg("parsing flags")
			}

			baggage.Discoverer = discovery.NewDiscoverer(dOpts...)
			if err := discoveryAddHosts(ctx, baggage.Discoverer); err != nil {
				l.Fatal().Err(err).Msg("adding devices")
			}
			return nil
		}
	}
}
