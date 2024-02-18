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
			GroupID: "Component RPCs",
			Use:     "ble",
			Short:   "RPCs related to Bluetooth Low-Energy",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.BLEGetStatusRequest{},
			&shelly.BLEGetConfigRequest{},
			&shelly.BLESetConfigRequest{},
		},
	}

	cloudComponent = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "cloud",
			Short:   "RPCs related to Shelly Cloud",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.CloudGetStatusRequest{},
			&shelly.CloudGetConfigRequest{},
			&shelly.CloudSetConfigRequest{},
		},
	}

	coverComponent = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "cover",
			Short:   "RPCs related to Cover components",
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
			GroupID: "Component RPCs",
			Use:     "input",
			Short:   "RPCs related to input components",
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
			GroupID: "Component RPCs",
			Use:     "light",
			Short:   "RPCs related to light components",
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
			GroupID: "Component RPCs",
			Use:     "mqtt",
			Short:   "RPCs related to MQTT configuration and status",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.MQTTGetStatusRequest{},
			&shelly.MQTTGetConfigRequest{},
			&shelly.MQTTSetConfigRequest{},
		},
	}

	scheduleComponent = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "schedule",
			Short:   "RPCs related to managing schedules",
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
			GroupID: "Component RPCs",
			Use:     "shelly",
			Short:   "RPCs related device management, configuration, and status",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.ShellyGetDeviceInfoRequest{},
			&shelly.ShellyGetStatusRequest{},
			&shelly.ShellyGetConfigRequest{},
			&shelly.ShellyCheckForUpdateRequest{},
			&shelly.ShellyRebootRequest{},
			// ShellySetAuth requires some calculation as it depends on the device ID.
			// &shelly.ShellySetAuthRequest{},
		},
	}

	switchComponent = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "switch",
			Short:   "RPCs related to switch components",
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
			GroupID: "Component RPCs",
			Use:     "sys",
			Short:   "RPCs related to system management and status",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.SysGetConfigRequest{},
			&shelly.SysSetConfigRequest{},
			&shelly.SysGetStatusRequest{},
		},
	}

	wifiComponent = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "wifi",
			Short:   "RPCs related to wifi configuration and status.",
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
	rootCmd.AddGroup(&cobra.Group{
		ID:    "Component RPCs",
		Title: "Device Component RPCs:",
	})
	for _, c := range components {

		rootCmd.AddCommand(c.Parent)
		c.Parent.Run = func(cmd *cobra.Command, args []string) {
			c.Parent.Help()
		}
		for _, childCmd := range c.Parent.Commands() {
			childRun := childCmd.RunE
			discoveryFlags(childCmd.Flags(), discoveryFlagsOptions{interactive: true})
			childCmd.RunE = func(cmd *cobra.Command, args []string) error {
				if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
					return err
				}
				ctx := cmd.Context()
				l := log.Ctx(ctx)
				dOpts, err := discoveryOptionsFromFlags(cmd.Flags())
				if err != nil {
					l.Fatal().Err(err).Msg("parsing flags")
				}

				baggage.Discoverer = discovery.NewDiscoverer(dOpts...)
				if err := discoveryAddDevices(ctx, baggage.Discoverer); err != nil {
					l.Fatal().Err(err).Msg("adding devices")
				}
				return childRun(cmd, args)
			}
		}
	}
}
