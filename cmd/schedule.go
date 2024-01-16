package cmd

import (
	shelly "github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/gencobra"
	"github.com/spf13/cobra"
)

var ScheduleRequests = []shelly.RPCRequestBody{
	// TODO(cbaker) Need to make custom flag for schedule.
	//
	// &shelly.ScheduleCreateRequest{},
	// &shelly.ScheduleDeleteRequest{},
	// &shelly.ScheduleDeleteAllRequest{},
	// &shelly.ScheduleUpdateRequest{},
}

var scheduleCmd = &cobra.Command{
	Use: "schedule",
}

func init() {
	scheduleCmd.Run = func(cmd *cobra.Command, args []string) {
		scheduleCmd.Help()
	}
	discoveryFlags(scheduleCmd.PersistentFlags(), false)
	for _, req := range ScheduleRequests {
		c, err := gencobra.RequestToCmd(ctx, req, Output)
		if err != nil {
			panic("building schedule subcommands: " + err.Error())
		}
		scheduleCmd.AddCommand(c)
	}
	rootCmd.AddCommand(scheduleCmd)
}
