package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var sysRequests = []shelly.RPCRequestBody{
	&shelly.SysGetConfigRequest{},
	&shelly.SysSetConfigRequest{},
	&shelly.SysGetStatusRequest{},
}

var sysCmd = &cobra.Command{
	Use: "sys",
}

func init() {
	sysCmd.Run = func(cmd *cobra.Command, args []string) {
		sysCmd.Help()
	}
	discoveryFlags(sysCmd.PersistentFlags(), false)
	for _, req := range sysRequests {
		c, err := gencobra.RequestToCmd(ctx, req, Output)
		if err != nil {
			panic("building sys subcommands: " + err.Error())
		}
		sysCmd.AddCommand(c)
	}
	rootCmd.AddCommand(sysCmd)
}
