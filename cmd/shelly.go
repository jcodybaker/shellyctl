package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var switchRequests = []shelly.RPCRequestBody{
	&shelly.SwitchGetConfigRequest{},
	&shelly.SwitchSetConfigRequest{},
	&shelly.SwitchSetRequest{},
	&shelly.SwitchToggleRequest{},
	&shelly.SwitchGetStatusRequest{},
}

var switchCmd = &cobra.Command{
	Use: "switch",
}

func init() {
	switchCmd.Run = func(cmd *cobra.Command, args []string) {
		switchCmd.Help()
	}
	discoveryFlags(switchCmd.PersistentFlags(), false)
	for _, req := range switchRequests {
		c, err := gencobra.RequestToCmd(ctx, req, Output)
		if err != nil {
			panic("building switch subcommands: " + err.Error())
		}
		switchCmd.AddCommand(c)
	}
	rootCmd.AddCommand(switchCmd)
}
