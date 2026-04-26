package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
    Up       key.Binding
    Down     key.Binding
    Top      key.Binding
    Bottom   key.Binding
    Kill     key.Binding
    Restart  key.Binding
    Details  key.Binding
    Filter   key.Binding
    Clear    key.Binding
    Tab      key.Binding
    NewProf  key.Binding
    Launch   key.Binding
    Help     key.Binding
    Quit     key.Binding
}

func newKeyMap() keyMap {
    return keyMap{
        Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
        Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
        Top:     key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
        Bottom:  key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
        Kill:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "kill")),
        Restart: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
        Details: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "details")),
        Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
        Clear:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear")),
        Tab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "tab")),
        NewProf: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new profile")),
        Launch:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "launch")),
        Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
        Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
    }
}

func (k keyMap) ShortHelp() []key.Binding {
    return []key.Binding{k.Up, k.Down, k.Filter, k.Kill, k.Restart, k.Tab, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
    return [][]key.Binding{
        {k.Up, k.Down, k.Top, k.Bottom},
        {k.Filter, k.Clear, k.Details},
        {k.Kill, k.Restart},
        {k.Tab, k.NewProf, k.Launch},
        {k.Help, k.Quit},
    }
}
