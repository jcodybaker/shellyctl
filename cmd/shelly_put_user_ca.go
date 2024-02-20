package cmd

import (
	"github.com/jcodybaker/go-shelly"
	"github.com/spf13/cobra"
)

var (
	shellyPutUserCACmd = &cobra.Command{
		Use: "put-user-ca",
	}
)

func init() {
	// Passing a whole cert as a cmd argument is awkward, but viper supports config files which
	// should facilitate multi-line strings in yaml, JSON, etc.
	shellyPutUserCACmd.Flags().String(
		"data", "", "PEM encoded certificate authority data. (one of --data, --data-file, or --remove-ca is required)",
	)
	shellyPutUserCACmd.Flags().String(
		"data-file", "", "path to a file containing PEM encoded certificate authority data.",
	)
	shellyPutUserCACmd.Flags().Bool(
		"remove-ca", false, "remove an existing CA certificate from the device",
	)
	shellyComponent.Parent.AddCommand(shellyPutUserCACmd)
	discoveryFlags(shellyPutUserCACmd.Flags(), discoveryFlagsOptions{interactive: true})
	shellyPutUserCACmd.RunE = newDataCommand(
		func(data *string, append bool) shelly.RPCRequestBody {
			return &shelly.ShellyPutUserCARequest{
				Data:   data,
				Append: append,
			}
		}, "data", "data-file", "remove-ca")
}
