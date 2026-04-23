package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "quantflow",
	Short: "Agentic quantitative finance framework",
	Long: "QuantFlow provides an end-to-end agentic pipeline for quantitative finance.\n" +
		"It combines SEC filing signals, Reddit sentiment, strategy generation,\n" +
		"backtesting, and DuckDB analytics.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(tuiCmd)
}
