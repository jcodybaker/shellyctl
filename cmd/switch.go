package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var shellyRequests = []shelly.RPCRequestBody{
	&shelly.ShellyGetStatusRequest{},
	&shelly.ShellyGetConfigRequest{},
	&shelly.ShellyCheckForUpdateRequest{},
	&shelly.ShellyRebootRequest{},
	&shelly.ShellySetAuthRequest{},
}

var shellyCmd = &cobra.Command{
	Use: "shelly",
}

func init() {
	shellyCmd.Run = func(cmd *cobra.Command, args []string) {
		shellyCmd.Help()
	}
	discoveryFlags(shellyCmd.PersistentFlags(), false)
	for _, req := range shellyRequests {
		c, err := gencobra.RequestToCmd(ctx, req, Output)
		if err != nil {
			panic("building shelly subcommands: " + err.Error())
		}
		shellyCmd.AddCommand(c)
	}
	rootCmd.AddCommand(shellyCmd)
}
