package quantwhisperui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/config"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/engine"
)

type sessionStartedMsg struct {
	events <-chan quantwhisperer.Event
	done   <-chan error
	cancel context.CancelFunc
}

type sessionEventMsg quantwhisperer.Event

type sessionDoneMsg struct {
	err error
}

type model struct {
	options config.Options

	eventCh <-chan quantwhisperer.Event
	doneCh  <-chan error
	cancel  context.CancelFunc

	running  bool
	finished bool
	err      error

	status       string
	lastTick     *quantwhisperer.Tick
	lastDecision *quantwhisperer.Decision
	lastSnapshot *quantwhisperer.Snapshot
	tradeHistory []quantwhisperer.Trade
	logLines     []string
	width        int
	spinner      spinner.Model
}

func newModel(options config.Options) model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))

	return model{
		options:      options,
		status:       "initializing session",
		tradeHistory: make([]quantwhisperer.Trade, 0, 8),
		logLines:     make([]string, 0, 14),
		spinner:      spin,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.startSessionCmd(), m.spinner.Tick)
}

func (m model) startSessionCmd() tea.Cmd {
	return func() tea.Msg {
		session, err := engine.NewSession(m.options)
		if err != nil {
			return sessionDoneMsg{err: err}
		}

		ctx, cancel := context.WithCancel(context.Background())
		events := make(chan quantwhisperer.Event, 128)
		dones := make(chan error, 1)
		go func() {
			dones <- session.Run(ctx, events)
			close(events)
			_ = session.Close()
		}()
		return sessionStartedMsg{events: events, done: dones, cancel: cancel}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		return m, nil
	case tea.KeyMsg:
		if typed.String() == "q" || typed.String() == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		return m, nil
	case sessionStartedMsg:
		m.eventCh = typed.events
		m.doneCh = typed.done
		m.cancel = typed.cancel
		m.running = true
		m.status = "broker stream active"
		return m, tea.Batch(waitForEvent(typed.events), waitForDone(typed.done), m.spinner.Tick)
	case sessionEventMsg:
		event := quantwhisperer.Event(typed)
		m.consumeEvent(event)
		if m.eventCh != nil {
			return m, waitForEvent(m.eventCh)
		}
		return m, nil
	case sessionDoneMsg:
		m.running = false
		m.finished = true
		m.err = typed.err
		if typed.err != nil {
			m.status = "session failed"
			m.appendLog("[ERROR] " + typed.err.Error())
		} else {
			m.status = "session complete"
			m.appendLog("[SYSTEM] session complete")
		}
		return m, nil
	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func waitForEvent(events <-chan quantwhisperer.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return sessionDoneMsg{}
		}
		return sessionEventMsg(event)
	}
}

func waitForDone(done <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-done
		if !ok {
			return sessionDoneMsg{}
		}
		return sessionDoneMsg{err: err}
	}
}

func (m *model) consumeEvent(event quantwhisperer.Event) {
	if event.Message != "" {
		m.appendLog(fmt.Sprintf("[%s] %s", strings.ToUpper(event.Type), event.Message))
	}
	if event.Tick != nil {
		tick := *event.Tick
		m.lastTick = &tick
	}
	if event.Decision != nil {
		decision := *event.Decision
		m.lastDecision = &decision
	}
	if event.Snapshot != nil {
		snapshot := *event.Snapshot
		m.lastSnapshot = &snapshot
	}
	if event.Trade != nil {
		trade := *event.Trade
		m.tradeHistory = append([]quantwhisperer.Trade{trade}, m.tradeHistory...)
		if len(m.tradeHistory) > 8 {
			m.tradeHistory = m.tradeHistory[:8]
		}
	}
	if event.Type == "status" || event.Type == "risk" || event.Type == "error" {
		m.status = event.Message
	}
}

func (m *model) appendLog(line string) {
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > 14 {
		m.logLines = m.logLines[len(m.logLines)-14:]
	}
}

func (m model) View() string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true).Render("QUANT WHISPERER")
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("246")).Render("Local AI trading engine | q to quit")

	statusLine := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Status: " + m.status)
	if m.running {
		statusLine += " " + m.spinner.View()
	}

	brokerLine := fmt.Sprintf("Broker: %s | Mode: %s | Symbol: %s", m.options.Broker, m.options.Mode, m.options.Symbol)
	wallLine := fmt.Sprintf("Execution Wall: confidence >= %.2f | DD <= %.2f%% | Trades/min <= %d | Position <= %.2f%%", m.options.ConfidenceThreshold, m.options.MaxDrawdownPct, m.options.MaxTradesPerMinute, m.options.PositionSizePct)

	market := "Market: waiting for ticks"
	if m.lastTick != nil {
		market = fmt.Sprintf("Market: last=%.2f bid=%.2f ask=%.2f vol=%d", m.lastTick.LastPrice, m.lastTick.Bid, m.lastTick.Ask, m.lastTick.Volume)
	}

	decision := "AI: waiting for decisions"
	if m.lastDecision != nil {
		decision = fmt.Sprintf("AI: %s confidence=%.2f reason=%s", m.lastDecision.Action, m.lastDecision.Confidence, m.lastDecision.Reasoning)
	}

	pnl := "P&L: waiting for snapshots"
	if m.lastSnapshot != nil {
		pnl = fmt.Sprintf("P&L: equity=%.2f cash=%.2f qty=%.2f drawdown=%.2f%%", m.lastSnapshot.Equity, m.lastSnapshot.Cash, m.lastSnapshot.PositionQty, m.lastSnapshot.DrawdownPct)
	}

	trades := []string{"Trades:"}
	if len(m.tradeHistory) == 0 {
		trades = append(trades, "- no trades yet")
	} else {
		for _, trade := range m.tradeHistory {
			trades = append(trades, fmt.Sprintf("- %s %.0f @ %.2f (conf %.2f)", trade.Side, trade.Quantity, trade.Price, trade.Confidence))
		}
	}

	logs := []string{"Logs:"}
	if len(m.logLines) == 0 {
		logs = append(logs, "- waiting for session output")
	} else {
		for _, line := range m.logLines {
			logs = append(logs, "- "+line)
		}
	}

	blocks := []string{
		title,
		subtitle,
		"",
		statusLine,
		brokerLine,
		wallLine,
		"",
		market,
		decision,
		pnl,
		"",
		strings.Join(trades, "\n"),
		"",
		strings.Join(logs, "\n"),
	}

	if m.finished {
		if m.err != nil {
			blocks = append(blocks, "", lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("Session ended with error: "+m.err.Error()))
		} else {
			blocks = append(blocks, "", lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true).Render("Session completed successfully."))
		}
	}

	return strings.Join(blocks, "\n")
}
