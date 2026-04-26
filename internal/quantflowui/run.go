package quantflowui

import tea "github.com/charmbracelet/bubbletea"

func Run() error {
	program := tea.NewProgram(newModel(), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
