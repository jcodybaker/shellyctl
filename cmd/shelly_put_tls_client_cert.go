package cmd

import (
	"github.com/jcodybaker/go-shelly"
	"github.com/spf13/cobra"
)

var (
	shellyPutTLSClientCertCmd = &cobra.Command{
		Use: "put-tls-client-cert",
	}
)

func init() {
	// Passing a whole cert as a cmd argument is awkward, but viper supports config files which
	// should facilitate multi-line strings in yaml, JSON, etc.
	shellyPutTLSClientCertCmd.Flags().String(
		"data", "", "PEM encoded certificate data. (one of --data, --data-file, or --remove-cert is required)",
	)
	shellyPutTLSClientCertCmd.Flags().String(
		"data-file", "", "path to a file containing PEM encoded certificate data.",
	)
	shellyPutTLSClientCertCmd.Flags().String(
		"data-url", "", "url containing PEM encoded certificate authority data.",
	)
	shellyPutTLSClientCertCmd.Flags().Bool(
		"remove-cert", false, "remove an existing certificate from the device",
	)

	shellyComponent.Parent.AddCommand(shellyPutTLSClientCertCmd)
	discoveryFlags(shellyPutTLSClientCertCmd.Flags(), discoveryFlagsOptions{interactive: true})
	shellyPutTLSClientCertCmd.RunE = newDataCommand(
		func(data *string, append bool) shelly.RPCRequestBody {
			return &shelly.ShellyPutTLSClientCertRequest{
				Data:   data,
				Append: append,
			}
		}, dataCommandOptions{
			strParam:  "data",
			fileParam: "data-file",
			urlParam:  "data-url",
			nullParam: "remove-cert",
		})
}
