package tui

import (
    "context"
    "time"

    "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, refresh time.Duration) error {
    model := NewModel(ctx, refresh)
    p := tea.NewProgram(model, tea.WithAltScreen())
    _, err := p.Run()
    return err
}
