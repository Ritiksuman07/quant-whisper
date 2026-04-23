package tui

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/lipgloss"

    "github.com/ritiksuman07/quantflow/internal/util"
)

var (
    colorBg     = lipgloss.Color("235")
    colorAccent = lipgloss.Color("39")
    colorText   = lipgloss.Color("252")
    colorMuted  = lipgloss.Color("244")
    colorWarn   = lipgloss.Color("214")
    colorDanger = lipgloss.Color("203")
)

func (m model) View() string {
    if m.loading {
        return lipgloss.NewStyle().Foreground(colorAccent).Render(m.spinner.View() + " Scanning processes...")
    }

    if m.wizard.active {
        return m.viewWizard()
    }

    top := m.viewTopBar()
    body := m.viewBody()
    bottom := m.viewBottomBar()

    return strings.Join([]string{top, body, bottom}, "\n")
}

func (m model) viewTopBar() string {
    title := lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("portman")
    tabs := []string{
        m.tabLabel(tabProcesses, "Processes"),
        m.tabLabel(tabProfiles, "Profiles"),
    }
    filter := ""
    if m.filterInput.Value() != "" {
        filter = lipgloss.NewStyle().Foreground(colorWarn).Render("Filter: " + m.filterInput.Value())
    }
    line := lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", strings.Join(tabs, " "), "  ", filter)
    return lipgloss.NewStyle().Padding(0, 1).Background(colorBg).Foreground(colorText).Render(line)
}

func (m model) viewBody() string {
    if m.tabActive() == tabProfiles {
        return m.profileTable.View()
    }

    tableView := m.processTable.View()
    if m.detailOpen {
        detail := m.viewDetailPanel()
        return lipgloss.JoinHorizontal(lipgloss.Top, tableView, detail)
    }
    return tableView
}

func (m model) viewBottomBar() string {
    var left string
    if m.confirm != confirmNone {
        action := "Kill"
        if m.confirm == confirmRestart {
            action = "Restart"
        }
        left = lipgloss.NewStyle().Foreground(colorWarn).Render(fmt.Sprintf("%s %s (PID %d)? [y/N]", action, m.confirmTarget.Name, m.confirmTarget.PID))
    } else if m.filtering {
        left = m.filterInput.View()
    } else if m.statusMsg != "" {
        left = lipgloss.NewStyle().Foreground(colorAccent).Render(m.statusMsg)
    }

    keys := m.help.View(m.keys)
    right := lipgloss.NewStyle().Foreground(colorMuted).Render(keys)

    space := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
    if space < 1 {
        space = 1
    }
    gap := strings.Repeat(" ", space)

    return lipgloss.NewStyle().Padding(0, 1).Background(colorBg).Foreground(colorText).Render(left + gap + right)
}

func (m model) tabLabel(t tab, label string) string {
    style := lipgloss.NewStyle().Foreground(colorMuted)
    if m.tabActive() == t {
        style = style.Foreground(colorAccent).Bold(true)
    }
    return style.Render(label)
}

func (m model) viewDetailPanel() string {
    entry, ok := m.selectedEntry()
    if !ok {
        return ""
    }
    panelWidth := clamp(m.width/3, 24, 40)

    lines := []string{
        lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Render("Details"),
        fmt.Sprintf("Name: %s", entry.Name),
        fmt.Sprintf("PID: %d", entry.PID),
        fmt.Sprintf("Port: %d", entry.Port),
        fmt.Sprintf("Status: %s", entry.Status),
        fmt.Sprintf("User: %s", entry.User),
        fmt.Sprintf("CPU: %s", util.FormatPercent(entry.CPUPercent)),
        fmt.Sprintf("Mem: %s", util.FormatBytes(entry.MemRSS)),
    }
    if entry.Exe != "" {
        lines = append(lines, fmt.Sprintf("Exe: %s", entry.Exe))
    }
    if entry.Cwd != "" {
        lines = append(lines, fmt.Sprintf("Cwd: %s", entry.Cwd))
    }
    if entry.ParentName != "" {
        lines = append(lines, fmt.Sprintf("Parent: %s", entry.ParentName))
    }
    if len(entry.Cmdline) > 0 {
        lines = append(lines, "Cmd: "+strings.Join(entry.Cmdline, " "))
    }

    content := strings.Join(lines, "\n")
    return lipgloss.NewStyle().
        Width(panelWidth).
        Padding(1, 1).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(colorAccent).
        Render(content)
}

func (m model) viewWizard() string {
    prompt := ""
    switch m.wizard.stage {
    case wizName:
        prompt = "Profile name"
    case wizDescription:
        prompt = "Profile description (optional)"
    case wizCmdLabel:
        prompt = "Command label"
    case wizCmd:
        prompt = "Command (shell)"
    case wizCmdPort:
        prompt = "Expected port (optional)"
    case wizCmdDir:
        prompt = "Working dir (optional)"
    case wizAddAnother:
        prompt = "Add another command? (y/N)"
    }

    title := lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("New Profile")
    desc := lipgloss.NewStyle().Foreground(colorMuted).Render(prompt)
    errLine := ""
    if m.wizard.err != "" {
        errLine = lipgloss.NewStyle().Foreground(colorDanger).Render(m.wizard.err)
    }
    input := m.wizard.input.View()

    card := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(colorAccent).
        Padding(1, 2).
        Width(clamp(m.width-10, 40, 80)).
        Render(strings.Join([]string{title, desc, input, errLine}, "\n"))

    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}
