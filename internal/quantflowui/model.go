package quantflowui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type stageState int

const (
	stagePending stageState = iota
	stageRunning
	stageDone
	stageFailed
)

type stageItem struct {
	Name   string
	State  stageState
	Detail string
}

type logLineMsg string
type logsClosedMsg struct{}
type runDoneMsg struct {
	err error
}

type model struct {
	width    int
	height   int
	focus    int
	offline  bool
	running  bool
	finished bool
	err      error

	thesisInput textinput.Model
	tickerInput textinput.Model
	logs        []string
	stages      []stageItem
	spinner     spinner.Model

	lineCh <-chan string
	doneCh <-chan error
	cancel context.CancelFunc
}

func newModel() model {
	thesisInput := textinput.New()
	thesisInput.Placeholder = `short small-cap biotech on FDA rejection patterns`
	thesisInput.SetValue(`short small-cap biotech on FDA rejection patterns`)
	thesisInput.CharLimit = 200
	thesisInput.Width = 70
	thesisInput.Focus()

	tickerInput := textinput.New()
	tickerInput.Placeholder = "XBI"
	tickerInput.SetValue("XBI")
	tickerInput.CharLimit = 6
	tickerInput.Width = 10

	loadSpinner := spinner.New()
	loadSpinner.Spinner = spinner.Dot
	loadSpinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	return model{
		focus:       0,
		offline:     true,
		thesisInput: thesisInput,
		tickerInput: tickerInput,
		logs:        []string{},
		stages:      defaultStages(),
		spinner:     loadSpinner,
	}
}

func defaultStages() []stageItem {
	return []stageItem{
		{Name: "SEC Filing Agent"},
		{Name: "Reddit Sentiment Agent"},
		{Name: "Strategy Orchestrator"},
		{Name: "Backtest Engine"},
		{Name: "Artifact Export"},
		{Name: "DuckDB Analytics"},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(typed)
	case logLineMsg:
		line := string(typed)
		m.appendLog(line)
		m.progressStages(line)
		if m.lineCh != nil {
			return m, waitForLine(m.lineCh)
		}
		return m, nil
	case logsClosedMsg:
		return m, nil
	case runDoneMsg:
		m.running = false
		m.finished = true
		m.cancel = nil
		if typed.err != nil {
			m.err = typed.err
			m.markCurrentFailed(typed.err.Error())
		} else {
			m.markAllDone()
		}
		return m, nil
	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var command tea.Cmd
		m.spinner, command = m.spinner.Update(msg)
		return m, command
	default:
		return m, nil
	}
}

func (m model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)

	var builder strings.Builder
	builder.WriteString(titleStyle.Render("QuantFlow TUI"))
	builder.WriteString("\n")
	builder.WriteString(helpStyle.Render("Interactive agent pipeline runner built with Bubble Tea"))
	builder.WriteString("\n\n")

	builder.WriteString(sectionStyle.Render("Config"))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("Thesis: %s\n", m.thesisInput.View()))
	builder.WriteString(fmt.Sprintf("Ticker: %s\n", m.tickerInput.View()))
	builder.WriteString(fmt.Sprintf("Offline fixtures: %s\n", checkbox(m.offline)))
	builder.WriteString(helpStyle.Render("Keys: tab/up/down switch field • o toggle offline • enter run • r rerun • q quit"))
	builder.WriteString("\n\n")

	builder.WriteString(sectionStyle.Render("Pipeline Stages"))
	builder.WriteString("\n")
	for _, stage := range m.stages {
		icon, style := stageIcon(stage.State)
		line := fmt.Sprintf("%s %s", icon, stage.Name)
		if stage.Detail != "" {
			line = fmt.Sprintf("%s — %s", line, stage.Detail)
		}
		builder.WriteString(style.Render(line))
		builder.WriteString("\n")
	}
	builder.WriteString("\n")

	builder.WriteString(sectionStyle.Render("Live Log"))
	builder.WriteString("\n")
	if len(m.logs) == 0 {
		if m.running {
			builder.WriteString(helpStyle.Render(m.spinner.View() + " waiting for first log line..."))
		} else {
			builder.WriteString(helpStyle.Render("No run yet. Press enter to start."))
		}
		builder.WriteString("\n")
	} else {
		for _, line := range m.logs {
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n")
	if m.running {
		builder.WriteString(helpStyle.Render(m.spinner.View() + " QuantFlow is running..."))
		builder.WriteString("\n")
	}
	if m.finished && m.err == nil {
		builder.WriteString(okStyle.Render("Run finished successfully."))
		builder.WriteString("\n")
	}
	if m.err != nil {
		builder.WriteString(errorStyle.Render("Run failed: " + m.err.Error()))
		builder.WriteString("\n")
	}

	return builder.String()
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" || key == "q" {
		if m.running && m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	}

	if m.running {
		return m, nil
	}

	switch key {
	case "tab", "down":
		m.focus = (m.focus + 1) % 2
		m.syncFocus()
		return m, nil
	case "shift+tab", "up":
		m.focus = (m.focus + 1) % 2
		m.syncFocus()
		return m, nil
	case "o":
		m.offline = !m.offline
		return m, nil
	case "r":
		if strings.TrimSpace(m.thesisInput.Value()) == "" {
			return m, nil
		}
		return m.startRun()
	case "enter":
		if strings.TrimSpace(m.thesisInput.Value()) == "" {
			return m, nil
		}
		return m.startRun()
	}

	var command tea.Cmd
	if m.focus == 0 {
		m.thesisInput, command = m.thesisInput.Update(msg)
		return m, command
	}
	m.tickerInput, command = m.tickerInput.Update(msg)
	m.tickerInput.SetValue(strings.ToUpper(m.tickerInput.Value()))
	return m, command
}

func (m model) startRun() (tea.Model, tea.Cmd) {
	thesis := strings.TrimSpace(m.thesisInput.Value())
	ticker := strings.ToUpper(strings.TrimSpace(m.tickerInput.Value()))
	lineCh := make(chan string, 128)
	doneCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())

	m.running = true
	m.finished = false
	m.err = nil
	m.logs = []string{"[SYSTEM] starting QuantFlow pipeline..."}
	m.stages = defaultStages()
	m.stages[0].State = stageRunning
	m.lineCh = lineCh
	m.doneCh = doneCh
	m.cancel = cancel

	go executePipeline(ctx, thesis, ticker, m.offline, lineCh, doneCh)

	return m, tea.Batch(waitForLine(lineCh), waitForDone(doneCh), m.spinner.Tick)
}

func executePipeline(
	ctx context.Context,
	thesis string,
	ticker string,
	offline bool,
	lineCh chan<- string,
	doneCh chan<- error,
) {
	defer close(lineCh)
	defer close(doneCh)

	args := []string{
		"-m",
		"quantflow",
		"run",
		thesis,
		"--lookback-days",
		strconv.Itoa(252),
		"--verbose",
	}
	if ticker != "" {
		args = append(args, "--ticker", ticker)
	}
	if offline {
		args = append(args, "--offline")
	}

	lineCh <- "[SYSTEM] python " + joinArgs(args)

	command := exec.CommandContext(ctx, "python", args...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		doneCh <- err
		return
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		doneCh <- err
		return
	}

	if err := command.Start(); err != nil {
		doneCh <- err
		return
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go streamPipe(&waitGroup, stdout, "", lineCh)
	go streamPipe(&waitGroup, stderr, "[stderr] ", lineCh)
	waitGroup.Wait()

	runErr := command.Wait()
	if ctx.Err() == context.Canceled {
		doneCh <- nil
		return
	}
	doneCh <- runErr
}

func streamPipe(waitGroup *sync.WaitGroup, reader io.Reader, prefix string, lineCh chan<- string) {
	defer waitGroup.Done()
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		lineCh <- prefix + scanner.Text()
	}
}

func waitForLine(lineCh <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-lineCh
		if !ok {
			return logsClosedMsg{}
		}
		return logLineMsg(line)
	}
}

func waitForDone(doneCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-doneCh
		if !ok {
			return runDoneMsg{}
		}
		return runDoneMsg{err: err}
	}
}

func (m *model) appendLog(line string) {
	const maxLines = 16
	m.logs = append(m.logs, line)
	if len(m.logs) > maxLines {
		m.logs = m.logs[len(m.logs)-maxLines:]
	}
}

func (m *model) progressStages(line string) {
	stageMap := map[string]int{
		"[SEC]":       0,
		"[REDDIT]":    1,
		"[STRATEGY]":  2,
		"[BACKTEST]":  3,
		"[ARTIFACTS]": 4,
		"[DUCKDB]":    5,
	}
	mark := -1
	for key, index := range stageMap {
		if strings.Contains(line, key) {
			mark = index
			break
		}
	}
	if mark >= 0 {
		for index := 0; index < mark; index++ {
			if m.stages[index].State != stageDone {
				m.stages[index].State = stageDone
			}
		}
		if m.stages[mark].State == stagePending {
			m.stages[mark].State = stageRunning
		}
		if strings.Contains(strings.ToLower(line), "done") || strings.Contains(strings.ToLower(line), "saved") {
			m.stages[mark].State = stageDone
		}
		m.stages[mark].Detail = trimStagePrefix(line)
	}
	if strings.Contains(line, "QuantFlow run complete") {
		m.markAllDone()
	}
}

func trimStagePrefix(line string) string {
	for _, prefix := range []string{
		"[SEC] ",
		"[REDDIT] ",
		"[STRATEGY] ",
		"[BACKTEST] ",
		"[DUCKDB] ",
		"[ARTIFACTS] ",
		"[SYSTEM] ",
	} {
		line = strings.TrimPrefix(line, prefix)
	}
	return line
}

func (m *model) markAllDone() {
	for index := range m.stages {
		if m.stages[index].State != stageFailed {
			m.stages[index].State = stageDone
		}
	}
}

func (m *model) markCurrentFailed(detail string) {
	for index := range m.stages {
		if m.stages[index].State == stageRunning {
			m.stages[index].State = stageFailed
			m.stages[index].Detail = detail
			return
		}
	}
	if len(m.stages) > 0 {
		last := len(m.stages) - 1
		m.stages[last].State = stageFailed
		m.stages[last].Detail = detail
	}
}

func (m *model) syncFocus() {
	if m.focus == 0 {
		m.thesisInput.Focus()
		m.tickerInput.Blur()
		return
	}
	m.tickerInput.Focus()
	m.thesisInput.Blur()
}

func checkbox(enabled bool) string {
	if enabled {
		return "[x]"
	}
	return "[ ]"
}

func stageIcon(state stageState) (string, lipgloss.Style) {
	pendingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	failedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	switch state {
	case stageRunning:
		return "◐", runningStyle
	case stageDone:
		return "●", doneStyle
	case stageFailed:
		return "✖", failedStyle
	default:
		return "○", pendingStyle
	}
}

func joinArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, value := range args {
		if strings.ContainsRune(value, ' ') {
			quoted = append(quoted, `"`+value+`"`)
			continue
		}
		quoted = append(quoted, value)
	}
	return strings.Join(quoted, " ")
}
