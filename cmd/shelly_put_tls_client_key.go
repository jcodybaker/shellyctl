package cmd

import (
	"github.com/jcodybaker/go-shelly"
	"github.com/spf13/cobra"
)

var (
	shellyPutTLSClientKeyCmd = &cobra.Command{
		Use: "put-tls-client-key",
	}
)

func init() {
	// Passing a whole key as a cmd argument is awkward, but viper supports config files which
	// should facilitate multi-line strings in yaml, JSON, etc.
	shellyPutTLSClientKeyCmd.Flags().String(
		"data", "", "PEM encoded key data. (one of --data, --data-file, or --remove-key is required)",
	)
	shellyPutTLSClientKeyCmd.Flags().String(
		"data-file", "", "path to a file containing PEM encoded key data.",
	)
	shellyPutTLSClientKeyCmd.Flags().Bool(
		"remove-key", false, "remove an existing key from the device",
	)
	shellyComponent.Parent.AddCommand(shellyPutTLSClientKeyCmd)
	discoveryFlags(shellyPutTLSClientKeyCmd.Flags(), discoveryFlagsOptions{interactive: true})
	shellyPutTLSClientKeyCmd.RunE = newDataCommand(
		func(data *string, append bool) shelly.RPCRequestBody {
			return &shelly.ShellyPutTLSClientKeyRequest{
				Data:   data,
				Append: append,
			}
		}, "data", "data-file", "remove-key")
}
