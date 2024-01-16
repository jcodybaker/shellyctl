package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var CloudRequests = []shelly.RPCRequestBody{
	&shelly.CloudGetStatusRequest{},
	&shelly.CloudGetConfigRequest{},
	&shelly.CloudSetConfigRequest{},
}

var cloudCmd = &cobra.Command{
	Use: "cloud",
}

func init() {
	cloudCmd.Run = func(cmd *cobra.Command, args []string) {
		cloudCmd.Help()
	}
	discoveryFlags(cloudCmd.PersistentFlags(), false)
	for _, req := range CloudRequests {
		c, err := gencobra.RequestToCmd(ctx, req, Output)
		if err != nil {
			panic("building cloud subcommands: " + err.Error())
		}
		cloudCmd.AddCommand(c)
	}
	rootCmd.AddCommand(cloudCmd)
}
