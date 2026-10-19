// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/approval"
	"github.com/Leo-Galli/opsrun/internal/executor"
	"github.com/Leo-Galli/opsrun/internal/inventory"
	"github.com/Leo-Galli/opsrun/internal/runbook"
)

func TestModelAppliesStepStateChanged(t *testing.T) {
	m := NewModel()
	_, _ = m.Update(eventMsg(executor.Event{
		Kind: executor.EventStepStateChanged, RunID: "run-1", StepID: "s1", Host: "h1", To: executor.StepRunning,
	}))
	require.Equal(t, "run-1", m.RunID)
	require.Equal(t, []string{"s1"}, m.order)
	require.Equal(t, executor.StepRunning, m.states["s1"]["h1"])
}

func TestModelAppliesCommandResultSuccess(t *testing.T) {
	m := NewModel()
	exit := 0
	_, _ = m.Update(eventMsg(executor.Event{
		Kind: executor.EventCommandResult, StepID: "s1", Host: "h1", Command: "echo hi", ExitCode: &exit, Duration: time.Second,
	}))
	require.Len(t, m.logLines, 1)
	require.Contains(t, m.logLines[0], "echo hi")
	require.Contains(t, m.logLines[0], "exit 0")
}

func TestModelAppliesCommandResultFailure(t *testing.T) {
	m := NewModel()
	_, _ = m.Update(eventMsg(executor.Event{
		Kind: executor.EventCommandResult, StepID: "s1", Host: "h1", Err: errors.New("boom"),
	}))
	require.Contains(t, m.logLines[0], "FAIL")
	require.Contains(t, m.logLines[0], "boom")
}

func TestModelAppliesDryRun(t *testing.T) {
	m := NewModel()
	_, _ = m.Update(eventMsg(executor.Event{
		Kind: executor.EventCommandResult, StepID: "s1", Host: "h1", Command: "echo hi", Message: "dry-run",
	}))
	require.Contains(t, m.logLines[0], "DRY-RUN")
}

func TestModelAppliesRollbackResult(t *testing.T) {
	m := NewModel()
	_, _ = m.Update(eventMsg(executor.Event{Kind: executor.EventRollbackResult, StepID: "s1", Host: "h1"}))
	require.Contains(t, m.logLines[0], "ROLLBACK OK")

	_, _ = m.Update(eventMsg(executor.Event{Kind: executor.EventRollbackResult, StepID: "s1", Host: "h1", Err: errors.New("x")}))
	require.Contains(t, m.logLines[1], "ROLLBACK FAIL")
}

func TestModelLogLinesCapped(t *testing.T) {
	m := NewModel()
	for i := 0; i < maxLogLines+50; i++ {
		m.appendLog("line")
	}
	require.Len(t, m.logLines, maxLogLines)
}

func TestModelRunFinishedQuits(t *testing.T) {
	m := NewModel()
	_, cmd := m.Update(eventMsg(executor.Event{Kind: executor.EventRunFinished}))
	require.True(t, m.Done)
	require.False(t, m.Failed)
	require.NotNil(t, cmd)
	require.IsType(t, tea.QuitMsg{}, cmd())
}

func TestModelRunFinishedWithErrorMarksFailed(t *testing.T) {
	m := NewModel()
	_, _ = m.Update(eventMsg(executor.Event{Kind: executor.EventRunFinished, Err: errors.New("boom")}))
	require.True(t, m.Failed)
	require.Error(t, m.Err)
}

func TestModelQuitKey(t *testing.T) {
	m := NewModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	require.True(t, m.Done)
	require.NotNil(t, cmd)
}

func TestModelApprovalFlow(t *testing.T) {
	m := NewModel()
	req := approval.Request{
		Step:                &runbook.Step{ID: "s1"},
		Hosts:               []inventory.Host{{Name: "h1"}},
		InterpolatedCommand: "echo hi",
	}
	_, _ = m.Update(approvalReqMsg(req))
	require.NotNil(t, m.pending)
	require.Equal(t, "s1", m.pending.Step.ID)

	view := m.View()
	require.Contains(t, view, "APPROVAL REQUIRED")
	require.Contains(t, view, "echo hi")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	require.Nil(t, m.pending)
	require.NotNil(t, cmd)

	go func() { cmd() }()
	resp := <-m.respCh
	require.Equal(t, approval.DecisionApprove, resp.Decision)
}

func TestModelApprovalKeysAllDecisions(t *testing.T) {
	tests := []struct {
		key  string
		want approval.Decision
	}{
		{"a", approval.DecisionApprove},
		{"s", approval.DecisionSkip},
		{"A", approval.DecisionAbort},
	}
	for _, tt := range tests {
		m := NewModel()
		m.pending = &approval.Request{Step: &runbook.Step{ID: "s1"}}
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
		require.NotNil(t, cmd)
		go func() { cmd() }()
		resp := <-m.respCh
		require.Equal(t, tt.want, resp.Decision)
	}
}

func TestModelApprovalUnknownKeyIgnored(t *testing.T) {
	m := NewModel()
	m.pending = &approval.Request{Step: &runbook.Step{ID: "s1"}}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	require.Nil(t, cmd)
	require.NotNil(t, m.pending)
}

func TestModelViewShowsStepsAndLog(t *testing.T) {
	m := NewModel()
	_, _ = m.Update(eventMsg(executor.Event{Kind: executor.EventStepStateChanged, RunID: "run-1", StepID: "s1", Host: "h1", To: executor.StepSucceeded}))
	view := m.View()
	require.Contains(t, view, "run-1")
	require.Contains(t, view, "s1")
	require.Contains(t, view, "h1")
	require.Contains(t, view, "q: quit")
}

func TestModelInitReturnsBatchedCmd(t *testing.T) {
	m := NewModel()
	cmd := m.Init()
	require.NotNil(t, cmd)
}

func TestGateRequestApprovalRoundTrip(t *testing.T) {
	m := NewModel()
	gate := m.Gate()

	go func() {
		req := <-m.approvalCh
		require.Equal(t, "s1", req.Step.ID)
		m.respCh <- approval.Response{Decision: approval.DecisionApprove}
	}()

	resp, err := gate.RequestApproval(context.Background(), approval.Request{Step: &runbook.Step{ID: "s1"}})
	require.NoError(t, err)
	require.Equal(t, approval.DecisionApprove, resp.Decision)
}

func TestGateRequestApprovalContextCanceled(t *testing.T) {
	m := NewModel()
	gate := m.Gate()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := gate.RequestApproval(ctx, approval.Request{Step: &runbook.Step{ID: "s1"}})
	require.Error(t, err)
}

func TestEmitterFeedsEventChannel(t *testing.T) {
	m := NewModel()
	emitter := m.Emitter()
	emitter.Emit(executor.Event{Kind: executor.EventRunStarted, RunID: "run-1"})
	e := <-m.eventCh
	require.Equal(t, "run-1", e.RunID)
}

func TestIndicatorAllStates(t *testing.T) {
	states := []executor.StepState{
		executor.StepPending, executor.StepWaitingApproval, executor.StepRunning,
		executor.StepSucceeded, executor.StepFailed, executor.StepSkipped,
		executor.StepRollingBack, executor.StepRolledBack, executor.StepRollbackFailed,
	}
	for _, s := range states {
		require.NotEmpty(t, indicator(s))
	}
}
