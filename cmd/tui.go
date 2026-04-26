package cmd

import (
	"strings"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperui"
	"github.com/spf13/cobra"
)

var (
	tuiMode  string
	tuiFlags = defaultRuntimeFlags(quantwhisperer.ModePaper)
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch live Quant Whisperer dashboard",
	RunE: func(cmd *cobra.Command, _ []string) error {
		mode := quantwhisperer.Mode(strings.ToLower(strings.TrimSpace(tuiMode)))
		if mode != quantwhisperer.ModePaper && mode != quantwhisperer.ModeLive {
			mode = quantwhisperer.ModePaper
		}
		opts := buildOptions(mode, tuiFlags)
		logger := newLogger(tuiFlags.logFormat)
		brokerClient, marketData, err := buildSessionDependencies(mode, opts, logger)
		if err != nil {
			return err
		}
		return quantwhisperui.Run(opts, brokerClient, marketData, logger)
	},
}

func init() {
	tuiCmd.Flags().StringVar(&tuiMode, "mode", "paper", "run mode for dashboard (paper|live)")
	bindRuntimeFlags(tuiCmd.Flags(), &tuiFlags)
}
