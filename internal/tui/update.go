package tui

import (
    "context"
    "fmt"
    "strconv"
    "strings"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/key"

    "github.com/ritiksuman07/quantflow/internal/process"
    "github.com/ritiksuman07/quantflow/internal/profiles"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.updateTableSizes()
    case processesMsg:
        m.loading = false
        m.processes = msg.entries
        m.applyFilter()
        cmds = append(cmds, listenForProcesses(m.procChan))
    case errMsg:
        m.errMsg = msg.err.Error()
    case statusMsg:
        m.statusMsg = msg.text
    case profileSavedMsg:
        m.statusMsg = "Profile saved"
        m.setProfileRows()
    case profileLaunchMsg:
        m.statusMsg = fmt.Sprintf("Launched profile %s", msg.name)
    case profileLaunchErrMsg:
        m.statusMsg = fmt.Sprintf("Profile launch failed: %v", msg.err)
    case tea.KeyMsg:
        if m.wizard.active {
            return m.updateWizard(msg)
        }

        if key.Matches(msg, m.keys.Quit) {
            return m, tea.Quit
        }

        if m.filtering {
            var cmd tea.Cmd
            m.filterInput, cmd = m.filterInput.Update(msg)
            cmds = append(cmds, cmd)

            if key.Matches(msg, m.keys.Clear) {
                m.filtering = false
                m.filterInput.Blur()
                m.applyFilter()
            } else if msg.Type == tea.KeyEnter {
                m.filtering = false
                m.filterInput.Blur()
                m.applyFilter()
            } else {
                m.applyFilter()
            }
            break
        }

        if m.confirm != confirmNone {
            if strings.ToLower(msg.String()) == "y" {
                target := m.confirmTarget
                mode := m.confirm
                m.confirm = confirmNone
                if mode == confirmKill {
                    cmds = append(cmds, killCmd(target))
                } else if mode == confirmRestart {
                    cmds = append(cmds, restartCmd(target))
                }
            } else if key.Matches(msg, m.keys.Clear) || strings.ToLower(msg.String()) == "n" {
                m.confirm = confirmNone
                m.statusMsg = "Action cancelled"
            }
            break
        }

        switch {
        case key.Matches(msg, m.keys.Tab):
            if m.tabActive() == tabProcesses {
                m.setActiveTab(tabProfiles)
            } else {
                m.setActiveTab(tabProcesses)
            }
        case key.Matches(msg, m.keys.Filter) && m.tabActive() == tabProcesses:
            m.filtering = true
            m.filterInput.Focus()
        case key.Matches(msg, m.keys.Clear):
            m.filterInput.SetValue("")
            m.applyFilter()
            m.statusMsg = "Filter cleared"
        case key.Matches(msg, m.keys.Help):
            m.help.ShowAll = !m.help.ShowAll
        case key.Matches(msg, m.keys.Up):
            if m.tabActive() == tabProcesses {
                m.processTable.MoveUp(1)
            } else {
                m.profileTable.MoveUp(1)
            }
        case key.Matches(msg, m.keys.Down):
            if m.tabActive() == tabProcesses {
                m.processTable.MoveDown(1)
            } else {
                m.profileTable.MoveDown(1)
            }
        case key.Matches(msg, m.keys.Top):
            if m.tabActive() == tabProcesses {
                m.processTable.GotoTop()
            } else {
                m.profileTable.GotoTop()
            }
        case key.Matches(msg, m.keys.Bottom):
            if m.tabActive() == tabProcesses {
                m.processTable.GotoBottom()
            } else {
                m.profileTable.GotoBottom()
            }
        case key.Matches(msg, m.keys.Details) && m.tabActive() == tabProcesses:
            m.detailOpen = !m.detailOpen
        case key.Matches(msg, m.keys.Kill) && m.tabActive() == tabProcesses:
            if entry, ok := m.selectedEntry(); ok {
                m.confirm = confirmKill
                m.confirmTarget = entry
            }
        case key.Matches(msg, m.keys.Restart) && m.tabActive() == tabProcesses:
            if entry, ok := m.selectedEntry(); ok {
                m.confirm = confirmRestart
                m.confirmTarget = entry
            }
        case key.Matches(msg, m.keys.NewProf) && m.tabActive() == tabProfiles:
            m.startWizard()
        case key.Matches(msg, m.keys.Launch) && m.tabActive() == tabProfiles:
            if profile, ok := m.selectedProfile(); ok {
                cmds = append(cmds, launchProfileCmd(profile))
            }
        }
    }

    return m, tea.Batch(cmds...)
}

func (m model) tabActive() tab {
    if m.profileTable.Focused() {
        return tabProfiles
    }
    return tabProcesses
}

func (m *model) setActiveTab(t tab) {
    if t == tabProfiles {
        m.profileTable.Focus()
        m.processTable.Blur()
    } else {
        m.processTable.Focus()
        m.profileTable.Blur()
    }
}

func (m *model) updateTableSizes() {
    height := m.height - 8
    if height < 6 {
        height = 6
    }
    m.processTable.SetHeight(height)
    m.profileTable.SetHeight(height)

    if m.width == 0 {
        return
    }

    procCols := m.processTable.Columns()
    if len(procCols) >= 7 {
        procCols[3].Width = clamp(m.width/3, 12, 28)
        m.processTable.SetColumns(procCols)
    }

    profCols := m.profileTable.Columns()
    if len(profCols) >= 3 {
        profCols[0].Width = clamp(m.width/3, 16, 32)
        profCols[2].Width = clamp(m.width/4, 16, 24)
        m.profileTable.SetColumns(profCols)
    }
}

func clamp(value, min, max int) int {
    if value < min {
        return min
    }
    if value > max {
        return max
    }
    return value
}

func killCmd(entry process.Entry) tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()
        if err := process.KillProcess(ctx, entry.PID); err != nil {
            return statusMsg{text: fmt.Sprintf("Kill failed: %v", err)}
        }
        return statusMsg{text: fmt.Sprintf("Sent SIGTERM to PID %d", entry.PID)}
    }
}

func restartCmd(entry process.Entry) tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()
        if err := process.RestartProcess(ctx, entry); err != nil {
            return statusMsg{text: fmt.Sprintf("Restart failed: %v", err)}
        }
        return statusMsg{text: fmt.Sprintf("Restarted %s", entry.Name)}
    }
}

func launchProfileCmd(profile profiles.Profile) tea.Cmd {
    return func() tea.Msg {
        if err := profiles.Launch(profile); err != nil {
            return profileLaunchErrMsg{err: err}
        }
        return profileLaunchMsg{name: profile.Name}
    }
}

func (m model) updateWizard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    if msg.Type == tea.KeyEsc {
        m.wizard = profileWizard{active: false, stage: wizName, input: m.wizard.input}
        m.statusMsg = "Profile creation cancelled"
        return m, nil
    }

    if m.wizard.stage == wizAddAnother {
        if strings.ToLower(msg.String()) == "y" {
            m.wizard.stage = wizCmdLabel
            m.wizard.input.SetValue("")
            return m, nil
        }
        if strings.ToLower(msg.String()) == "n" || msg.Type == tea.KeyEnter {
            m.wizard.stage = wizDone
        }
    }

    m.wizard.input, cmd = m.wizard.input.Update(msg)

    if msg.Type == tea.KeyEnter {
        input := strings.TrimSpace(m.wizard.input.Value())
        switch m.wizard.stage {
        case wizName:
            if input == "" {
                m.wizard.err = "Name is required"
                return m, nil
            }
            m.wizard.profile.Name = input
            m.wizard.profile.CreatedAt = timeNow()
            m.wizard.stage = wizDescription
        case wizDescription:
            m.wizard.profile.Description = input
            m.wizard.stage = wizCmdLabel
        case wizCmdLabel:
            if input == "" {
                m.wizard.err = "Command label is required"
                return m, nil
            }
            m.wizard.commands = append(m.wizard.commands, profiles.ProfileCommand{Label: input})
            m.wizard.stage = wizCmd
        case wizCmd:
            if input == "" {
                m.wizard.err = "Command is required"
                return m, nil
            }
            m.wizard.commands[len(m.wizard.commands)-1].Cmd = input
            m.wizard.stage = wizCmdPort
        case wizCmdPort:
            if input != "" {
                port, err := strconv.Atoi(input)
                if err != nil {
                    m.wizard.err = "Port must be a number"
                    return m, nil
                }
                m.wizard.commands[len(m.wizard.commands)-1].Port = port
            }
            m.wizard.stage = wizCmdDir
        case wizCmdDir:
            m.wizard.commands[len(m.wizard.commands)-1].Dir = input
            m.wizard.stage = wizAddAnother
        }

        if m.wizard.stage == wizDone {
            m.wizard.profile.Commands = m.wizard.commands
            m.profiles = append(m.profiles, m.wizard.profile)
            _ = profiles.Save(m.profiles)
            m.setProfileRows()
            m.wizard = profileWizard{active: false, stage: wizName, input: m.wizard.input}
            return m, func() tea.Msg { return profileSavedMsg{} }
        }

        m.wizard.input.SetValue("")
        m.wizard.err = ""
    }

    return m, cmd
}

func (m *model) startWizard() {
    m.wizard = profileWizard{
        active: true,
        stage:  wizName,
        input:  m.wizard.input,
    }
    m.wizard.input.SetValue("")
    m.wizard.input.Focus()
}

func timeNow() time.Time {
    return time.Now()
}
