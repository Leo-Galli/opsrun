// SPDX-License-Identifier: Apache-2.0

// Package tui implements the Bubble Tea terminal UI: a DAG/step status view, a host
// grid, a streaming log pane, and approval prompts, driven by the same executor.Event
// stream the audit log consumes. Model.Update and Model.View are plain functions over
// plain data, so they are testable without a real terminal; see model_test.go.
package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Leo-Galli/opsrun/internal/approval"
	"github.com/Leo-Galli/opsrun/internal/executor"
	"github.com/Leo-Galli/opsrun/internal/inventory"
)

const maxLogLines = 500
const visibleLogLines = 15

// Model is a Bubble Tea model driven by executor.Event values and approval.Request
// values, both delivered over channels so it can run concurrently with the executor.
type Model struct {
	eventCh    chan executor.Event
	approvalCh chan approval.Request
	respCh     chan approval.Response

	RunID    string
	order    []string
	states   map[string]map[string]executor.StepState
	logLines []string
	pending  *approval.Request

	Done   bool
	Failed bool
	Err    error
}

// NewModel returns a Model ready to be driven by Emitter and Gate.
func NewModel() *Model {
	return &Model{
		eventCh:    make(chan executor.Event, 256),
		approvalCh: make(chan approval.Request),
		respCh:     make(chan approval.Response),
		states:     make(map[string]map[string]executor.StepState),
	}
}

// Emitter returns an executor.Emitter that feeds events to this model.
func (m *Model) Emitter() executor.Emitter {
	return executor.EmitterFunc(func(e executor.Event) { m.eventCh <- e })
}

// Gate returns an approval.Gate backed by this model's interactive prompt.
func (m *Model) Gate() approval.Gate {
	return tuiGate{m}
}

type tuiGate struct{ m *Model }

// RequestApproval implements approval.Gate.
func (g tuiGate) RequestApproval(ctx context.Context, req approval.Request) (approval.Response, error) {
	select {
	case g.m.approvalCh <- req:
	case <-ctx.Done():
		return approval.Response{}, ctx.Err()
	}
	select {
	case resp := <-g.m.respCh:
		return resp, nil
	case <-ctx.Done():
		return approval.Response{}, ctx.Err()
	}
}

type eventMsg executor.Event
type approvalReqMsg approval.Request

func waitForEvent(ch chan executor.Event) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return nil
		}
		return eventMsg(e)
	}
}

func waitForApproval(ch chan approval.Request) tea.Cmd {
	return func() tea.Msg {
		req, ok := <-ch
		if !ok {
			return nil
		}
		return approvalReqMsg(req)
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(waitForEvent(m.eventCh), waitForApproval(m.approvalCh))
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case eventMsg:
		e := executor.Event(msg)
		m.applyEvent(e)
		if e.Kind == executor.EventRunFinished {
			m.Done = true
			m.Failed = e.Err != nil
			m.Err = e.Err
			return m, tea.Quit
		}
		return m, waitForEvent(m.eventCh)

	case approvalReqMsg:
		req := approval.Request(msg)
		m.pending = &req
		return m, nil

	case tea.KeyMsg:
		if m.pending != nil {
			return m.handleApprovalKey(msg)
		}
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.Done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) handleApprovalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var resp approval.Response
	switch msg.String() {
	case "a":
		resp = approval.Response{Decision: approval.DecisionApprove}
	case "s":
		resp = approval.Response{Decision: approval.DecisionSkip}
	case "A", "ctrl+c":
		resp = approval.Response{Decision: approval.DecisionAbort}
	default:
		// Editing a variable requires a text prompt the compact TUI does not offer;
		// operators who need that can rerun with a plain terminal (no TTY) instead.
		return m, nil
	}
	m.pending = nil
	respCh := m.respCh
	return m, func() tea.Msg {
		respCh <- resp
		return nil
	}
}

func (m *Model) applyEvent(e executor.Event) {
	if e.RunID != "" {
		m.RunID = e.RunID
	}
	switch e.Kind {
	case executor.EventStepStateChanged:
		if _, ok := m.states[e.StepID]; !ok {
			m.order = append(m.order, e.StepID)
			m.states[e.StepID] = make(map[string]executor.StepState)
		}
		m.states[e.StepID][e.Host] = e.To
	case executor.EventCommandResult:
		if e.Message == "dry-run" {
			m.appendLog(fmt.Sprintf("[DRY-RUN] %s/%s: %s", e.StepID, e.Host, e.Command))
			return
		}
		line := fmt.Sprintf("[EXEC] %s/%s (%s): %s", e.StepID, e.Host, e.Duration, e.Command)
		if e.Err != nil {
			line = fmt.Sprintf("[FAIL] %s/%s: %v", e.StepID, e.Host, e.Err)
		} else if e.ExitCode != nil {
			line += fmt.Sprintf(" [exit %d]", *e.ExitCode)
		}
		m.appendLog(line)
	case executor.EventRollbackResult:
		line := fmt.Sprintf("[ROLLBACK OK] %s/%s", e.StepID, e.Host)
		if e.Err != nil {
			line = fmt.Sprintf("[ROLLBACK FAIL] %s/%s: %v", e.StepID, e.Host, e.Err)
		}
		m.appendLog(line)
	case executor.EventApprovalDecided:
		m.appendLog(fmt.Sprintf("[APPROVAL] %s: %s", e.StepID, e.Message))
	}
}

func (m *Model) appendLog(line string) {
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > maxLogLines {
		m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
	}
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	failStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	pendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	approvalStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")).BorderStyle(lipgloss.NormalBorder()).Padding(0, 1)
)

func indicator(s executor.StepState) string {
	switch s {
	case executor.StepSucceeded, executor.StepRolledBack:
		return okStyle.Render("[OK]")
	case executor.StepFailed, executor.StepRollbackFailed:
		return failStyle.Render("[FAIL]")
	case executor.StepSkipped:
		return dimStyle.Render("[SKIP]")
	case executor.StepRunning, executor.StepRollingBack:
		return pendingStyle.Render("[RUN]")
	case executor.StepWaitingApproval:
		return pendingStyle.Render("[WAIT]")
	default:
		return dimStyle.Render("[....]")
	}
}

// View implements tea.Model.
func (m *Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("OpsRun - run %s", m.RunID)))
	b.WriteString("\n\n")

	for _, stepID := range m.order {
		hosts := m.states[stepID]
		for _, host := range sortedKeys(hosts) {
			label := host
			if label == "" {
				label = "-"
			}
			fmt.Fprintf(&b, "%s %-22s %-18s %s\n", indicator(hosts[host]), stepID, label, hosts[host])
		}
	}

	b.WriteString("\n" + dimStyle.Render("--- log ---") + "\n")
	start := 0
	if len(m.logLines) > visibleLogLines {
		start = len(m.logLines) - visibleLogLines
	}
	for _, line := range m.logLines[start:] {
		b.WriteString(line + "\n")
	}

	if m.pending != nil {
		b.WriteString("\n")
		b.WriteString(approvalStyle.Render(fmt.Sprintf(
			"APPROVAL REQUIRED: step %q\nhosts: %s\ncommand: %s\n[a]pprove  [s]kip  [A]bort",
			m.pending.Step.ID, hostNames(m.pending.Hosts), m.pending.InterpolatedCommand,
		)))
		b.WriteString("\n")
	}

	b.WriteString("\n" + dimStyle.Render("q: quit"))
	return b.String()
}

func hostNames(hosts []inventory.Host) string {
	names := make([]string, 0, len(hosts))
	for _, h := range hosts {
		names = append(names, h.Name)
	}
	return strings.Join(names, ", ")
}

func sortedKeys(m map[string]executor.StepState) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
