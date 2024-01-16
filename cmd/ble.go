package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var BLERequests = []shelly.RPCRequestBody{
	&shelly.BLEGetStatusRequest{},
	&shelly.BLEGetConfigRequest{},
	&shelly.BLESetConfigRequest{},
}

var bleCmd = &cobra.Command{
	Use: "ble",
}

func init() {
	bleCmd.Run = func(cmd *cobra.Command, args []string) {
		bleCmd.Help()
	}
	discoveryFlags(bleCmd.PersistentFlags(), false)
	for _, req := range BLERequests {
		c, err := gencobra.RequestToCmd(ctx, req, Output)
		if err != nil {
			panic("building ble subcommands: " + err.Error())
		}
		bleCmd.AddCommand(c)
	}
	rootCmd.AddCommand(bleCmd)
}
