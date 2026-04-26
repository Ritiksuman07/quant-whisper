package tui

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/charmbracelet/bubbles/help"
    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/bubbles/table"
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    "github.com/ritiksuman07/quantflow/internal/process"
    "github.com/ritiksuman07/quantflow/internal/profiles"
    "github.com/ritiksuman07/quantflow/internal/util"
)

type tab int

const (
    tabProcesses tab = iota
    tabProfiles
)

type confirmMode int

const (
    confirmNone confirmMode = iota
    confirmKill
    confirmRestart
)

type wizardStage int

const (
    wizName wizardStage = iota
    wizDescription
    wizCmdLabel
    wizCmd
    wizCmdPort
    wizCmdDir
    wizAddAnother
    wizDone
)

type profileWizard struct {
    active   bool
    stage    wizardStage
    input    textinput.Model
    profile  profiles.Profile
    commands []profiles.ProfileCommand
    err      string
}

type model struct {
    ctx           context.Context
    refresh       time.Duration
    procChan      <-chan []process.Entry
    keys          keyMap
    help          help.Model
    spinner       spinner.Model
    filterInput   textinput.Model
    filtering     bool
    processes     []process.Entry
    filtered      []process.Entry
    profiles      []profiles.Profile
    processTable  table.Model
    profileTable  table.Model
    statusMsg     string
    confirm       confirmMode
    confirmTarget process.Entry
    detailOpen    bool
    width         int
    height        int
    wizard        profileWizard
    errMsg        string
    loading       bool
}

type processesMsg struct {
    entries []process.Entry
}

type errMsg struct {
    err error
}

type statusMsg struct {
    text string
}

type profileSavedMsg struct{}

type profileLaunchMsg struct{
    name string
}

type profileLaunchErrMsg struct{
    err error
}

func NewModel(ctx context.Context, refresh time.Duration) model {
    spin := spinner.New()
    spin.Spinner = spinner.Line

    filter := textinput.New()
    filter.Placeholder = "filter by name or port"
    filter.Prompt = "/ "
    filter.CharLimit = 40

    procCols := []table.Column{
        {Title: "PORT", Width: 7},
        {Title: "PROTO", Width: 6},
        {Title: "PID", Width: 7},
        {Title: "NAME", Width: 18},
        {Title: "STATUS", Width: 12},
        {Title: "CPU", Width: 7},
        {Title: "MEM", Width: 8},
    }

    profCols := []table.Column{
        {Title: "PROFILE", Width: 24},
        {Title: "COMMANDS", Width: 10},
        {Title: "LAST LAUNCH", Width: 20},
    }

    procTable := table.New(table.WithColumns(procCols), table.WithHeight(12))
    procTable.SetStyles(defaultTableStyles())

    profileTable := table.New(table.WithColumns(profCols), table.WithHeight(12))
    profileTable.SetStyles(defaultTableStyles())

    profs, _ := profiles.Load()

    wizInput := textinput.New()
    wizInput.Placeholder = ""
    wizInput.CharLimit = 80
    wizInput.Prompt = "> "

    scanner := process.NewScanner(refresh)
    ch := scanner.Start(ctx)

    m := model{
        ctx:          ctx,
        refresh:      refresh,
        procChan:     ch,
        keys:         newKeyMap(),
        help:         help.New(),
        spinner:      spin,
        filterInput:  filter,
        processes:    []process.Entry{},
        filtered:     []process.Entry{},
        profiles:     profs,
        processTable: procTable,
        profileTable: profileTable,
        loading:      true,
        wizard: profileWizard{
            active: false,
            stage:  wizName,
            input:  wizInput,
        },
    }

    m.applyFilter()
    m.setProfileRows()
    return m
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        listenForProcesses(m.procChan),
    )
}

func listenForProcesses(ch <-chan []process.Entry) tea.Cmd {
    return func() tea.Msg {
        entries, ok := <-ch
        if !ok {
            return errMsg{err: fmt.Errorf("scanner stopped")}
        }
        return processesMsg{entries: entries}
    }
}

func (m *model) applyFilter() {
    if m.filterInput.Value() == "" {
        m.filtered = m.processes
    } else {
        query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
        filtered := make([]process.Entry, 0, len(m.processes))
        for _, entry := range m.processes {
            if strings.Contains(strings.ToLower(entry.Name), query) || strings.Contains(fmt.Sprint(entry.Port), query) {
                filtered = append(filtered, entry)
            }
        }
        m.filtered = filtered
    }
    m.setProcessRows()
}

func (m *model) setProcessRows() {
    rows := make([]table.Row, 0, len(m.filtered))
    for _, entry := range m.filtered {
        rows = append(rows, table.Row{
            fmt.Sprint(entry.Port),
            entry.Protocol,
            fmt.Sprint(entry.PID),
            entry.Name,
            entry.Status,
            util.FormatPercent(entry.CPUPercent),
            util.FormatBytes(entry.MemRSS),
        })
    }
    m.processTable.SetRows(rows)
}

func (m *model) setProfileRows() {
    rows := make([]table.Row, 0, len(m.profiles))
    for _, profile := range m.profiles {
        last := "never"
        if profile.LastLaunchedAt != nil {
            last = profile.LastLaunchedAt.Format("2006-01-02 15:04")
        }
        rows = append(rows, table.Row{
            profile.Name,
            fmt.Sprint(len(profile.Commands)),
            last,
        })
    }
    m.profileTable.SetRows(rows)
}

func (m model) selectedEntry() (process.Entry, bool) {
    if len(m.filtered) == 0 {
        return process.Entry{}, false
    }
    idx := m.processTable.Cursor()
    if idx < 0 || idx >= len(m.filtered) {
        return process.Entry{}, false
    }
    return m.filtered[idx], true
}

func (m model) selectedProfile() (profiles.Profile, bool) {
    if len(m.profiles) == 0 {
        return profiles.Profile{}, false
    }
    idx := m.profileTable.Cursor()
    if idx < 0 || idx >= len(m.profiles) {
        return profiles.Profile{}, false
    }
    return m.profiles[idx], true
}

func defaultTableStyles() table.Styles {
    styles := table.DefaultStyles()
    styles.Header = styles.Header.
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("33")).
        BorderBottom(true).
        Bold(true)
    styles.Selected = styles.Selected.
        Foreground(lipgloss.Color("230")).
        Background(lipgloss.Color("33")).
        Bold(true)
    return styles
}
