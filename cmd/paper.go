package cmd

import (
	"context"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/spf13/cobra"
)

var paperFlags = defaultRuntimeFlags(quantwhisperer.ModePaper)

var paperCmd = &cobra.Command{
	Use:   "paper",
	Short: "Run paper-trading mode with SQLite logging",
	RunE: func(cmd *cobra.Command, _ []string) error {
		opts := buildOptions(quantwhisperer.ModePaper, paperFlags)
		return runSession(context.Background(), quantwhisperer.ModePaper, opts, cmd.OutOrStdout())
	},
}

func init() {
	bindRuntimeFlags(paperCmd.Flags(), &paperFlags)
}
