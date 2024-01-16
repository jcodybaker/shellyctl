package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var MQTTRequests = []shelly.RPCRequestBody{
	&shelly.MQTTGetStatusRequest{},
	&shelly.MQTTGetConfigRequest{},
	&shelly.MQTTSetConfigRequest{},
}

var mqttCmd = &cobra.Command{
	Use: "mqtt",
}

func init() {
	mqttCmd.Run = func(cmd *cobra.Command, args []string) {
		mqttCmd.Help()
	}
	discoveryFlags(mqttCmd.PersistentFlags(), false)
	for _, req := range MQTTRequests {
		c, err := gencobra.RequestToCmd(ctx, req, Output)
		if err != nil {
			panic("building mqtt subcommands: " + err.Error())
		}
		mqttCmd.AddCommand(c)
	}
	rootCmd.AddCommand(mqttCmd)
}
