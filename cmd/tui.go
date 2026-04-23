package cmd

import (
	"github.com/ritiksuman07/quantflow/internal/quantflowui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive QuantFlow terminal interface",
	RunE: func(cmd *cobra.Command, args []string) error {
		return quantflowui.Run()
	},
}
