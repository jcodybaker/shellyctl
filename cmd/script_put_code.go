package cmd

import (
	"github.com/jcodybaker/go-shelly"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	scriptPutCodeCmd = &cobra.Command{
		Use: "put-code",
	}
)

func init() {
	scriptPutCodeCmd.Flags().Int(
		"id", 0, "Script ID",
	)
	scriptPutCodeCmd.Flags().String(
		"code", "", "Code. (one of --code or --code-file is required)",
	)
	scriptPutCodeCmd.Flags().String(
		"code-file", "", "path to a file containing code.",
	)

	scriptComponent.Parent.AddCommand(scriptPutCodeCmd)
	discoveryFlags(scriptPutCodeCmd.Flags(), discoveryFlagsOptions{interactive: true})
	scriptPutCodeCmd.RunE = newDataCommand(
		func(code *string, append bool) shelly.RPCRequestBody {
			r := &shelly.ScriptPutCodeRequest{
				Append: append,
				ID:     viper.GetInt("id"),
			}
			if code != nil {
				r.Code = *code
			}
			return r
		}, "code", "code-file", "")
}
