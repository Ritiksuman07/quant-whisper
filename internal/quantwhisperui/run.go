package quantwhisperui

import (
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/broker"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/config"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/engine"
)

func Run(options config.Options, brokerClient broker.Client, marketData engine.MarketDataProvider, logger *slog.Logger) error {
	program := tea.NewProgram(newModel(options, brokerClient, marketData, logger), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
