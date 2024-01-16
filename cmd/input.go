package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var InputRequests = []shelly.RPCRequestBody{
	&shelly.InputGetStatusRequest{},
	&shelly.InputGetConfigRequest{},
	&shelly.InputSetConfigRequest{},
	&shelly.InputCheckExpressionRequest{},
}

var inputCmd = &cobra.Command{
	Use: "input",
}

func init() {
	inputCmd.Run = func(cmd *cobra.Command, args []string) {
		inputCmd.Help()
	}
	discoveryFlags(inputCmd.PersistentFlags(), false)
	for _, req := range InputRequests {
		c, err := gencobra.RequestToCmd(ctx, req, Output)
		if err != nil {
			panic("building input subcommands: " + err.Error())
		}
		inputCmd.AddCommand(c)
	}
	rootCmd.AddCommand(inputCmd)
}
