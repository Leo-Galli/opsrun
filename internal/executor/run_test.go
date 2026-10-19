// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/approval"
	"github.com/Leo-Galli/opsrun/internal/inventory"
	"github.com/Leo-Galli/opsrun/internal/parser"
	"github.com/Leo-Galli/opsrun/internal/rollback"
	"github.com/Leo-Galli/opsrun/internal/runbook"
	"github.com/Leo-Galli/opsrun/internal/ssh"
)

// fakeApprovalGate returns a scripted sequence of responses, one per RequestApproval
// call (repeating the last one if exhausted).
type fakeApprovalGate struct {
	mu        sync.Mutex
	responses []approval.Response
	i         int
	requests  []approval.Request
}

func (g *fakeApprovalGate) RequestApproval(_ context.Context, req approval.Request) (approval.Response, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requests = append(g.requests, req)
	if len(g.responses) == 0 {
		return approval.Response{Decision: approval.DecisionApprove}, nil
	}
	idx := g.i
	if idx >= len(g.responses) {
		idx = len(g.responses) - 1
	}
	g.i++
	return g.responses[idx], nil
}

//nolint:unparam // target is part of this helper's general-purpose API; every current test uses runbook-level target fallback instead
func runAction(target, command string) *runbook.Action {
	return &runbook.Action{Kind: runbook.ActionKindRun, Target: target, Command: command}
}

func newTestHosts(names ...string) []inventory.Host {
	hosts := make([]inventory.Host, len(names))
	for i, n := range names {
		hosts[i] = inventory.Host{Name: n, Groups: []string{"all"}}
	}
	return hosts
}

func newExecutor(transport ssh.Transport, hosts []inventory.Host, gate approval.Gate) (*DefaultExecutor, *Recorder) {
	rec := &Recorder{}
	exec := &DefaultExecutor{
		Transport: transport,
		Inventory: inventory.NewStatic(hosts),
		Approval:  gate,
		Rollback:  &rollback.DefaultManager{Transport: transport},
		Emitter:   rec,
		Now:       time.Now,
	}
	return exec, rec
}

func TestExecutorRunSuccess(t *testing.T) {
	tr := ssh.NewFakeTransport(nil)
	exec, _ := newExecutor(tr, newTestHosts("web-1"), nil)

	rb := &runbook.Runbook{
		Name:    "test",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", Run: runAction("", "echo a")},
			{ID: "b", DependsOn: []string{"a"}, Run: runAction("", "echo b")},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{})
	require.NoError(t, err)
	require.False(t, summary.Failed)
	require.Equal(t, StepSucceeded, summary.StepStates["a"]["web-1"])
	require.Equal(t, StepSucceeded, summary.StepStates["b"]["web-1"])
}

func TestExecutorComplexDAGOrdering(t *testing.T) {
	var mu sync.Mutex
	var order []string
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		mu.Lock()
		order = append(order, command)
		mu.Unlock()
		return &ssh.Result{ExitCode: 0}, nil
	})
	exec, _ := newExecutor(tr, newTestHosts("web-1"), nil)

	// Diamond: a -> {b, c} -> d
	rb := &runbook.Runbook{
		Name:    "diamond",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", Run: runAction("", "cmd-a")},
			{ID: "b", DependsOn: []string{"a"}, Run: runAction("", "cmd-b")},
			{ID: "c", DependsOn: []string{"a"}, Run: runAction("", "cmd-c")},
			{ID: "d", DependsOn: []string{"b", "c"}, Run: runAction("", "cmd-d")},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{})
	require.NoError(t, err)
	require.False(t, summary.Failed)

	require.Equal(t, "cmd-a", order[0])
	require.Equal(t, "cmd-d", order[len(order)-1])
	require.ElementsMatch(t, []string{"cmd-b", "cmd-c"}, order[1:3])
}

func TestExecutorMidRunFailureTriggersRollbackAndAbortsBeforeDependents(t *testing.T) {
	var rollbackCalls []string
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		switch command {
		case "run-fail":
			return &ssh.Result{ExitCode: 1}, nil
		case "rollback-a":
			rollbackCalls = append(rollbackCalls, "a")
			return &ssh.Result{ExitCode: 0}, nil
		default:
			return &ssh.Result{ExitCode: 0}, nil
		}
	})
	exec, _ := newExecutor(tr, newTestHosts("web-1"), nil)

	rb := &runbook.Runbook{
		Name:    "fail-rollback",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", Run: runAction("", "run-a"), Rollback: runAction("", "rollback-a")},
			{ID: "b", DependsOn: []string{"a"}, Run: runAction("", "run-fail")},
			{ID: "c", DependsOn: []string{"b"}, Run: runAction("", "run-c")},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{})
	require.Error(t, err)
	require.True(t, summary.Failed)
	require.Equal(t, "b", summary.FailedStep)
	require.True(t, summary.RollbackRan)

	require.Equal(t, StepRolledBack, summary.StepStates["a"]["web-1"])
	require.Equal(t, StepFailed, summary.StepStates["b"]["web-1"])
	_, cVisited := summary.StepStates["c"]
	require.False(t, cVisited, "an aborting failure must stop the run before its dependents are even visited")
	require.Equal(t, []string{"a"}, rollbackCalls)
}

func TestExecutorRollbackOrderIsReversed(t *testing.T) {
	var mu sync.Mutex
	var rollbackOrder []string
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		switch command {
		case "run-fail":
			return &ssh.Result{ExitCode: 1}, nil
		case "rollback-1", "rollback-2", "rollback-3":
			mu.Lock()
			rollbackOrder = append(rollbackOrder, command)
			mu.Unlock()
			return &ssh.Result{ExitCode: 0}, nil
		default:
			return &ssh.Result{ExitCode: 0}, nil
		}
	})
	exec, _ := newExecutor(tr, newTestHosts("web-1"), nil)

	rb := &runbook.Runbook{
		Name:    "reverse-rollback",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "s1", Run: runAction("", "run-1"), Rollback: runAction("", "rollback-1")},
			{ID: "s2", DependsOn: []string{"s1"}, Run: runAction("", "run-2"), Rollback: runAction("", "rollback-2")},
			{ID: "s3", DependsOn: []string{"s2"}, Run: runAction("", "run-3"), Rollback: runAction("", "rollback-3")},
			{ID: "s4", DependsOn: []string{"s3"}, Run: runAction("", "run-fail")},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{})
	require.Error(t, err)
	require.True(t, summary.Failed)
	require.Equal(t, []string{"rollback-3", "rollback-2", "rollback-1"}, rollbackOrder)
}

func TestExecutorFailingRollback(t *testing.T) {
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		switch command {
		case "run-fail":
			return &ssh.Result{ExitCode: 1}, nil
		case "rollback-a":
			return &ssh.Result{ExitCode: 1, Stderr: "rollback blew up"}, nil
		default:
			return &ssh.Result{ExitCode: 0}, nil
		}
	})
	exec, _ := newExecutor(tr, newTestHosts("web-1"), nil)

	rb := &runbook.Runbook{
		Name:    "failing-rollback",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", Run: runAction("", "run-a"), Rollback: runAction("", "rollback-a")},
			{ID: "b", DependsOn: []string{"a"}, Run: runAction("", "run-fail")},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{})
	require.Error(t, err)
	require.Equal(t, StepRollbackFailed, summary.StepStates["a"]["web-1"])
}

func TestExecutorApprovalSkip(t *testing.T) {
	tr := ssh.NewFakeTransport(nil)
	gate := &fakeApprovalGate{responses: []approval.Response{{Decision: approval.DecisionSkip}}}
	exec, _ := newExecutor(tr, newTestHosts("web-1"), gate)

	rb := &runbook.Runbook{
		Name:    "approval-skip",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", Confirm: true, Run: runAction("", "run-a")},
			{ID: "b", DependsOn: []string{"a"}, Run: runAction("", "run-b")},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{})
	require.NoError(t, err)
	require.False(t, summary.Failed)
	require.Equal(t, StepSkipped, summary.StepStates["a"]["web-1"])
	require.Equal(t, StepSkipped, summary.StepStates["b"]["web-1"])
	require.Empty(t, tr.Opens())
}

func TestExecutorApprovalAbort(t *testing.T) {
	tr := ssh.NewFakeTransport(nil)
	gate := &fakeApprovalGate{responses: []approval.Response{{Decision: approval.DecisionAbort}}}
	exec, _ := newExecutor(tr, newTestHosts("web-1"), gate)

	rb := &runbook.Runbook{
		Name:    "approval-abort",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", Run: runAction("", "run-a"), Rollback: runAction("", "rollback-a")},
			{ID: "b", DependsOn: []string{"a"}, Confirm: true, Run: runAction("", "run-b")},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{})
	require.Error(t, err)
	require.True(t, summary.Failed)
	require.Equal(t, StepFailed, summary.StepStates["b"]["web-1"])
	require.Equal(t, StepRolledBack, summary.StepStates["a"]["web-1"])
}

func TestExecutorApprovalEditVariable(t *testing.T) {
	var gotCommand string
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		gotCommand = command
		return &ssh.Result{ExitCode: 0}, nil
	})
	gate := &fakeApprovalGate{responses: []approval.Response{
		{Decision: approval.DecisionEditVariable, VariableName: "greeting", VariableValue: "hola"},
		{Decision: approval.DecisionApprove},
	}}
	exec, _ := newExecutor(tr, newTestHosts("web-1"), gate)

	rb := &runbook.Runbook{
		Name:    "edit-variable",
		Targets: []runbook.Target{{Group: "all"}},
		Vars:    map[string]runbook.Variable{"greeting": {Name: "greeting", Type: runbook.VariableTypeString, Default: "hi"}},
		Steps: []*runbook.Step{
			{ID: "a", Confirm: true, Run: runAction("", "echo {{ .Vars.greeting }}")},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{})
	require.NoError(t, err)
	require.False(t, summary.Failed)
	require.Equal(t, "echo 'hola'", gotCommand)
}

func TestExecutorRedactsSecretVariablesFromEvents(t *testing.T) {
	var gotCommand string
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		gotCommand = command
		return &ssh.Result{ExitCode: 0}, nil
	})
	exec, rec := newExecutor(tr, newTestHosts("web-1"), nil)

	rb := &runbook.Runbook{
		Name:    "secret-redaction",
		Targets: []runbook.Target{{Group: "all"}},
		Vars:    map[string]runbook.Variable{"token": {Name: "token", Type: runbook.VariableTypeString, Secret: true}},
		Steps: []*runbook.Step{
			{ID: "a", Run: runAction("", "curl -H 'Authorization: {{ .Vars.token }}' example.com")},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{Vars: map[string]any{"token": "super-secret-value"}})
	require.NoError(t, err)
	require.False(t, summary.Failed)

	require.Contains(t, gotCommand, "super-secret-value", "the real command sent to the transport must still carry the real value")

	for _, e := range rec.Snapshot() {
		require.NotContains(t, e.Command, "super-secret-value", "event %q must never carry a secret value", e.Kind)
	}
}

func TestExecutorContinueOnErrorSkipsDependentsButNotWholeRun(t *testing.T) {
	var ranC bool
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		if command == "run-fail" {
			return &ssh.Result{ExitCode: 1}, nil
		}
		if command == "run-c" {
			ranC = true
		}
		return &ssh.Result{ExitCode: 0}, nil
	})
	exec, _ := newExecutor(tr, newTestHosts("web-1"), nil)

	rb := &runbook.Runbook{
		Name:    "continue-on-error",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", ContinueOnError: true, Run: runAction("", "run-fail")},
			{ID: "b", DependsOn: []string{"a"}, Run: runAction("", "run-b")},
			{ID: "c", Run: runAction("", "run-c")}, // independent of a
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{})
	require.NoError(t, err)
	require.False(t, summary.Failed)
	require.Equal(t, StepFailed, summary.StepStates["a"]["web-1"])
	require.Equal(t, StepSkipped, summary.StepStates["b"]["web-1"])
	require.True(t, ranC)
}

func TestExecutorContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		if command == "run-a" {
			cancel()
		}
		return &ssh.Result{ExitCode: 0}, nil
	})
	exec, _ := newExecutor(tr, newTestHosts("web-1"), nil)

	rb := &runbook.Runbook{
		Name:    "cancellation",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", Run: runAction("", "run-a"), Rollback: runAction("", "rollback-a")},
			{ID: "b", DependsOn: []string{"a"}, Run: runAction("", "run-b")},
		},
	}

	summary, err := exec.Run(ctx, rb, Options{})
	require.Error(t, err)
	require.True(t, summary.Failed)
	// "a" succeeded before cancellation was observed, then got rolled back; "b" never ran.
	require.Equal(t, StepRolledBack, summary.StepStates["a"]["web-1"])
	_, bRan := summary.StepStates["b"]["web-1"]
	require.False(t, bRan, "step b must not run after the context is canceled")
}

func TestExecutorRollingBatchTeratesFailuresWithinLimit(t *testing.T) {
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		if host.Name == "web-2" {
			return &ssh.Result{ExitCode: 1}, nil
		}
		return &ssh.Result{ExitCode: 0}, nil
	})
	exec, _ := newExecutor(tr, newTestHosts("web-1", "web-2", "web-3", "web-4"), nil)

	rb := &runbook.Runbook{
		Name:    "rolling-tolerate",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", Run: runAction("", "run-a")},
			{ID: "b", DependsOn: []string{"a"}, Run: runAction("", "run-b")},
		},
	}

	opts := Options{HostRollout: RollingStrategy{BatchSize: 1, MaxFailurePercent: 50}}
	summary, err := exec.Run(context.Background(), rb, opts)
	require.NoError(t, err)
	require.False(t, summary.Failed)
	require.Equal(t, StepFailed, summary.StepStates["a"]["web-2"])
	require.Equal(t, StepSucceeded, summary.StepStates["b"]["web-1"])
}

func TestExecutorRollingBatchExceedsTolerance(t *testing.T) {
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		if host.Name == "web-1" || host.Name == "web-2" {
			return &ssh.Result{ExitCode: 1}, nil
		}
		return &ssh.Result{ExitCode: 0}, nil
	})
	exec, _ := newExecutor(tr, newTestHosts("web-1", "web-2", "web-3", "web-4"), nil)

	rb := &runbook.Runbook{
		Name:    "rolling-exceed",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", Run: runAction("", "run-a")},
			{ID: "b", DependsOn: []string{"a"}, Run: runAction("", "run-b")},
		},
	}

	opts := Options{HostRollout: RollingStrategy{BatchSize: 1, MaxFailurePercent: 10}}
	summary, err := exec.Run(context.Background(), rb, opts)
	require.Error(t, err)
	require.True(t, summary.Failed)
	require.Equal(t, "a", summary.FailedStep)
}

func TestExecutorDryRun(t *testing.T) {
	tr := ssh.NewFakeTransport(nil)
	exec, rec := newExecutor(tr, newTestHosts("web-1"), nil)

	rb := &runbook.Runbook{
		Name:    "dry-run",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{ID: "a", Run: runAction("", "echo hi")},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{DryRun: true})
	require.NoError(t, err)
	require.False(t, summary.Failed)
	require.Empty(t, tr.Opens(), "dry-run must not open any transport session")

	found := false
	for _, e := range rec.Snapshot() {
		if e.Kind == EventCommandResult && e.Message == "dry-run" {
			found = true
		}
	}
	require.True(t, found)
}

func TestExecutorCheckFailure(t *testing.T) {
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		if command == "check-cmd" {
			return &ssh.Result{ExitCode: 2}, nil
		}
		return &ssh.Result{ExitCode: 0}, nil
	})
	exec, _ := newExecutor(tr, newTestHosts("web-1"), nil)

	rb := &runbook.Runbook{
		Name:    "check-fail",
		Targets: []runbook.Target{{Group: "all"}},
		Steps: []*runbook.Step{
			{
				ID:     "a",
				Run:    runAction("", "run-a"),
				Checks: []*runbook.Action{{Kind: runbook.ActionKindCheck, Command: "check-cmd", ExpectExit: 0}},
			},
		},
	}

	summary, err := exec.Run(context.Background(), rb, Options{})
	require.Error(t, err)
	require.Equal(t, StepFailed, summary.StepStates["a"]["web-1"])
}

func TestExecutorResumeFromCheckpoint(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	opensByHost := map[string]int{}
	fail := true
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		mu.Lock()
		opensByHost[command]++
		mu.Unlock()
		if command == "run-b" && fail {
			return &ssh.Result{ExitCode: 1}, nil
		}
		return &ssh.Result{ExitCode: 0}, nil
	})

	runbookContent := "---\n" +
		"name: resume-test\n" +
		"targets:\n" +
		"  - group: all\n" +
		"---\n\n" +
		"## Step: a - First\n\n" +
		"```bash opsrun:run target=all\nrun-a\n```\n\n" +
		"## Step: b - Second\n" +
		"depends_on: [a]\n\n" +
		"```bash opsrun:run target=all\nrun-b\n```\n"
	runbookPath := writeTempRunbook(t, runbookContent)
	rb := parseRunbookFile(t, runbookPath)

	exec, _ := newExecutor(tr, newTestHosts("web-1"), nil)
	exec.Parser = parser.NewMarkdownParser()
	opts := Options{RunID: "resume-1", CheckpointDir: dir, RunbookPath: runbookPath}

	summary, err := exec.Run(context.Background(), rb, opts)
	require.Error(t, err)
	require.True(t, summary.Failed)
	require.Equal(t, 1, opensByHost["run-a"])
	require.Equal(t, 1, opensByHost["run-b"])

	fail = false
	summary2, err := exec.Resume(context.Background(), "resume-1", Options{CheckpointDir: dir})
	require.NoError(t, err)
	require.False(t, summary2.Failed)
	require.Equal(t, 1, opensByHost["run-a"], "step a must not be re-executed on resume")
	require.Equal(t, 2, opensByHost["run-b"])
	require.Equal(t, StepSucceeded, summary2.StepStates["b"]["web-1"])
}

// --- test helpers for Resume, which needs a real parser and a runbook on disk ---

func writeTempRunbook(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "runbook.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func parseRunbookFile(t *testing.T, path string) *runbook.Runbook {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	rb, err := parser.NewMarkdownParser().Parse(context.Background(), f)
	require.NoError(t, err)
	return rb
}
