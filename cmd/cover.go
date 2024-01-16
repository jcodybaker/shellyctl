package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var CoverRequests = []shelly.RPCRequestBody{
	&shelly.CoverGetStatusRequest{},
	&shelly.CoverGetConfigRequest{},
	&shelly.CoverSetConfigRequest{},
	&shelly.CoverCloseRequest{},
	&shelly.CoverCalibrateRequest{},
	&shelly.CoverGoToPositionRequest{},
	&shelly.CoverOpenRequest{},
	&shelly.CoverResetCountersRequest{},
	&shelly.CoverStopRequest{},
}

var coverCmd = &cobra.Command{
	Use: "cover",
}

func init() {
	coverCmd.Run = func(cmd *cobra.Command, args []string) {
		coverCmd.Help()
	}
	discoveryFlags(coverCmd.PersistentFlags(), false)
	for _, req := range CoverRequests {
		c, err := gencobra.RequestToCmd(ctx, req, Output)
		if err != nil {
			panic("building cover subcommands: " + err.Error())
		}
		coverCmd.AddCommand(c)
	}
	rootCmd.AddCommand(coverCmd)
}
