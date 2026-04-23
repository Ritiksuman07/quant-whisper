package quantwhisperui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/config"
)

func Run(options config.Options) error {
	program := tea.NewProgram(newModel(options), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
