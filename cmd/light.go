package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var LightRequests = []shelly.RPCRequestBody{
	&shelly.LightGetStatusRequest{},
	&shelly.LightGetConfigRequest{},
	&shelly.LightSetConfigRequest{},
	&shelly.LightSetRequest{},
	&shelly.LightToggleRequest{},
}

var lightCmd = &cobra.Command{
	Use: "light",
}

func init() {
	lightCmd.Run = func(cmd *cobra.Command, args []string) {
		lightCmd.Help()
	}
	discoveryFlags(lightCmd.PersistentFlags(), false)
	for _, req := range LightRequests {
		c, err := gencobra.RequestToCmd(ctx, req, Output)
		if err != nil {
			panic("building light subcommands: " + err.Error())
		}
		lightCmd.AddCommand(c)
	}
	rootCmd.AddCommand(lightCmd)
}
