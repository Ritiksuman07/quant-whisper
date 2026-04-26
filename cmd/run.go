package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/spf13/cobra"
)

var (
	runMode  string
	runFlags = defaultRuntimeFlags(quantwhisperer.ModePaper)
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run Quant Whisperer in paper or live mode",
	RunE: func(cmd *cobra.Command, _ []string) error {
		mode := quantwhisperer.Mode(strings.ToLower(strings.TrimSpace(runMode)))
		if mode != quantwhisperer.ModePaper && mode != quantwhisperer.ModeLive {
			return fmt.Errorf("invalid mode %q (use paper or live)", runMode)
		}
		opts := buildOptions(mode, runFlags)
		return runSession(context.Background(), mode, opts, cmd.OutOrStdout(), runFlags.logFormat)
	},
}

func init() {
	runCmd.Flags().StringVar(&runMode, "mode", "paper", "run mode (paper|live)")
	bindRuntimeFlags(runCmd.Flags(), &runFlags)
}
