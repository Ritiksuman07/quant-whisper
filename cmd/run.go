package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	runTicker      string
	runLookback    int
	runOfflineMode bool
	runVerbose     bool
)

var runCmd = &cobra.Command{
	Use:   "run [thesis]",
	Short: "Run QuantFlow pipeline through the Python orchestrator",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		thesis := args[0]
		pythonArgs := []string{
			"-m",
			"quantflow",
			"run",
			thesis,
			"--lookback-days",
			strconv.Itoa(runLookback),
		}
		if runTicker != "" {
			pythonArgs = append(pythonArgs, "--ticker", runTicker)
		}
		if runOfflineMode {
			pythonArgs = append(pythonArgs, "--offline")
		}
		if runVerbose {
			pythonArgs = append(pythonArgs, "--verbose")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Running: python %s\n", joinArgs(pythonArgs))
		return runPythonCommand(pythonArgs...)
	},
}

func init() {
	runCmd.Flags().StringVar(&runTicker, "ticker", "", "override inferred ticker symbol")
	runCmd.Flags().IntVar(&runLookback, "lookback-days", 252, "number of daily bars for backtest")
	runCmd.Flags().BoolVar(&runOfflineMode, "offline", false, "disable external API calls")
	runCmd.Flags().BoolVar(&runVerbose, "verbose", true, "print stage-by-stage progress logs")
}
