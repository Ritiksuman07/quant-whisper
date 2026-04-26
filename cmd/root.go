package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "quant-whisper",
	Short: "Terminal-native algorithmic execution engine",
	Long:  "Quant Whisperer runs local-model-driven paper and live trading workflows with broker adapters, risk controls, and SQLite trade logs.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(paperCmd)
	rootCmd.AddCommand(liveCmd)
	rootCmd.AddCommand(tuiCmd)
}
