package cmd

import (
	"context"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/spf13/cobra"
)

var liveFlags = defaultRuntimeFlags(quantwhisperer.ModeLive)

var liveCmd = &cobra.Command{
	Use:   "live",
	Short: "Run live mode with execution wall and risk kill-switches",
	RunE: func(cmd *cobra.Command, _ []string) error {
		opts := buildOptions(quantwhisperer.ModeLive, liveFlags)
		return runSession(context.Background(), quantwhisperer.ModeLive, opts, cmd.OutOrStdout())
	},
}

func init() {
	bindRuntimeFlags(liveCmd.Flags(), &liveFlags)
}
