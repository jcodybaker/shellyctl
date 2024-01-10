package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var wifiRequests = []shelly.RPCRequestBody{
	&shelly.WifiGetStatusRequest{},
	&shelly.WifiGetConfigRequest{},
	&shelly.WifiSetConfigRequest{},
}

var wifiCmd = &cobra.Command{
	Use: "wifi",
}

func init() {
	wifiCmd.Run = func(cmd *cobra.Command, args []string) {
		wifiCmd.Help()
	}
	discoveryFlags(wifiCmd.PersistentFlags(), false)
	for _, req := range wifiRequests {
		c, err := gencobra.RequestToCmd(req)
		if err != nil {
			panic("building wifi subcommands: " + err.Error())
		}
		wifiCmd.AddCommand(c)
	}
	rootCmd.AddCommand(wifiCmd)
}
