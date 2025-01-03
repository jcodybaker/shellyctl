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

	btHome = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "bthome",
			Aliases: []string{"bt-home"},
			Short:   "RPCs related to BTHome",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.BTHomeAddDeviceRequest{},
			&shelly.BTHomeDeleteDeviceRequest{},
			&shelly.BTHomeAddSensorRequest{},
			&shelly.BTHomeDeleteSensorRequest{},
			&shelly.BTHomeGetObjectInfosRequest{},
		},
	}

	btHomeDevice = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "bthome-device",
			Aliases: []string{"bt-home-device"},
			Short:   "RPCs related to BTHome devices",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.BTHomeDeviceGetStatusRequest{},
			&shelly.BTHomeDeviceGetConfigRequest{},
			&shelly.BTHomeDeviceSetConfigRequest{},
			&shelly.BTHomeDeviceGetKnownObjectsRequest{},
		},
	}

	btHomeSensor = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "bthome-sensor",
			Aliases: []string{"bt-home-sensor"},
			Short:   "RPCs related to BTHome sensors",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.BTHomeSensorGetConfigRequest{},
			&shelly.BTHomeSensorGetStatusRequest{},
			&shelly.BTHomeSensorSetConfigRequest{},
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

	devicePowerComponent = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "device-power",
			Short:   "RPCs related to device power configuration and status",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.DevicePowerGetStatusRequest{},
		},
	}

	humidityComponent = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "humidity",
			Short:   "RPCs related to humidity configuration and status",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.HumidityGetStatusRequest{},
			&shelly.HumidityGetConfigRequest{},
			&shelly.HumiditySetConfigRequest{},
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

	scriptComponent = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "script",
			Short:   "RPCs related to managing scripts",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.ScriptCreateRequest{},
			&shelly.ScriptGetConfigRequest{},
			&shelly.ScriptSetConfigRequest{},
			&shelly.ScriptGetStatusRequest{},
			&shelly.ScriptEvalRequest{},
			&shelly.ScriptStartRequest{},
			&shelly.ScriptStopRequest{},
			&shelly.ScriptListRequest{},
			&shelly.ScriptDeleteRequest{},
		},
	}

	shellyComponent = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "shelly",
			Short:   "RPCs related device management, configuration, and status",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.ShellyCheckForUpdateRequest{},
			&shelly.ShellyDetectLocationRequest{},
			&shelly.ShellyFactoryResetRequest{},
			&shelly.ShellyGetConfigRequest{},
			&shelly.ShellyGetDeviceInfoRequest{},
			&shelly.ShellyGetStatusRequest{},
			&shelly.ShellyListMethodsRequest{},
			&shelly.ShellyListProfilesRequest{},
			&shelly.ShellyListTimezonesRequest{},
			&shelly.ShellyRebootRequest{},
			&shelly.ShellyResetWiFiConfigRequest{},
			&shelly.ShellySetProfileRequest{},
			&shelly.ShellyUpdateRequest{},
			// ShellySetAuth requires some calculation as it depends on the device ID.
			// It is implemented in cmd/shelly.go.
			// &shelly.ShellySetAuthRequest{},
			&shelly.ShellyGetComponentsRequest{},
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

	temperatureComponent = &gencobra.Component{
		Parent: &cobra.Command{
			GroupID: "Component RPCs",
			Use:     "temperature",
			Short:   "RPCs related to temperature configuration and status",
		},
		Requests: []shelly.RPCRequestBody{
			&shelly.TemperatureGetStatusRequest{},
			&shelly.TemperatureGetConfigRequest{},
			&shelly.TemperatureSetConfigRequest{},
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
		btHome,
		btHomeDevice,
		btHomeSensor,
		cloudComponent,
		coverComponent,
		devicePowerComponent,
		humidityComponent,
		inputComponent,
		lightComponent,
		mqttComponent,
		scheduleComponent,
		scriptComponent,
		shellyComponent,
		switchComponent,
		sysComponent,
		temperatureComponent,
		wifiComponent,
	}
)

func init() {
	baggage := &gencobra.Baggage{
		Output: Output,
	}
	cmds, err := gencobra.ComponentsToCmd(components, baggage)
	if err != nil {
		log.Panic().Err(err).Msg("generating menu for API commands")
	}
	rootCmd.AddGroup(&cobra.Group{
		ID:    "Component RPCs",
		Title: "Device Component RPCs:",
	})

	for _, c := range components {
		c := c
		rootCmd.AddCommand(c.Parent)
		c.Parent.Run = func(cmd *cobra.Command, args []string) {
			c.Parent.Help()
		}
	}
	for _, childCmd := range cmds {
		parent := childCmd.Parent()
		// Hack 🙄
		if parent.Use == "shelly" && childCmd.Use == "reset-wi-fi-config" {
			childCmd.Use = "reset-wifi-config"
			childCmd.Aliases = append(childCmd.Aliases, "reset-wi-fi-config")
		}
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
			if err := baggage.Discoverer.MQTTConnect(ctx); err != nil {
				l.Fatal().Err(err).Msg("connecting to MQTT broker")
			}
			_, err = baggage.Discoverer.Search(ctx)
			if err != nil {
				l.Fatal().Err(err).Msg("searching for devices")
			}
			if err := discoveryAddDevices(ctx, baggage.Discoverer); err != nil {
				l.Fatal().Err(err).Msg("adding devices")
			}
			return childRun(cmd, args)
		}
	}
}
