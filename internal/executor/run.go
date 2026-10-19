// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Leo-Galli/opsrun/internal/approval"
	"github.com/Leo-Galli/opsrun/internal/dag"
	"github.com/Leo-Galli/opsrun/internal/inventory"
	"github.com/Leo-Galli/opsrun/internal/parser"
	"github.com/Leo-Galli/opsrun/internal/rollback"
	"github.com/Leo-Galli/opsrun/internal/runbook"
	"github.com/Leo-Galli/opsrun/internal/ssh"
)

// DefaultExecutor is the production Executor: it drives a runbook's DAG to completion
// using a real (or fake, in tests) Transport, Inventory, approval.Gate, and
// rollback.Manager.
type DefaultExecutor struct {
	Transport ssh.Transport
	Inventory inventory.Inventory
	Approval  approval.Gate
	Rollback  rollback.Manager
	Emitter   Emitter
	Parser    parser.RunbookParser // only required to call Resume

	// Now returns the current time; overridable in tests. Defaults to time.Now.
	Now func() time.Time
}

func (e *DefaultExecutor) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e *DefaultExecutor) emitter() Emitter {
	if e.Emitter != nil {
		return e.Emitter
	}
	return NoopEmitter{}
}

func (e *DefaultExecutor) genRunID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return "run-" + hex.EncodeToString(buf)
}

// runState holds all mutable bookkeeping for a single Run/Resume call. Independent
// steps within one DAG level run in concurrent goroutines, so every accessor below
// takes mu; callers must never reach into the fields directly.
type runState struct {
	mu sync.Mutex

	runID        string
	states       map[string]map[string]StepState
	failedSteps  map[string]bool
	skippedSteps map[string]bool
	vars         map[string]any
	completed    []rollback.CompletedStep

	// preCompleted holds steps already successfully completed in a prior attempt, as
	// recorded in a loaded Checkpoint; resuming re-marks them succeeded without
	// re-executing anything.
	preCompleted map[string]map[string]string // stepID -> host -> resolved rollback command
}

func newRunState(runID string, vars map[string]any) *runState {
	return &runState{
		runID:        runID,
		states:       make(map[string]map[string]StepState),
		failedSteps:  make(map[string]bool),
		skippedSteps: make(map[string]bool),
		vars:         vars,
		preCompleted: make(map[string]map[string]string),
	}
}

func (rs *runState) dependsOnFailedOrSkipped(step *runbook.Step) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, dep := range step.DependsOn {
		if rs.failedSteps[dep] || rs.skippedSteps[dep] {
			return true
		}
	}
	return false
}

func (rs *runState) markFailed(stepID string) {
	rs.mu.Lock()
	rs.failedSteps[stepID] = true
	rs.mu.Unlock()
}

func (rs *runState) markSkippedStep(stepID string) {
	rs.mu.Lock()
	rs.skippedSteps[stepID] = true
	rs.mu.Unlock()
}

func (rs *runState) varsSnapshot() map[string]any {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	out := make(map[string]any, len(rs.vars))
	for k, v := range rs.vars {
		out[k] = v
	}
	return out
}

func (rs *runState) setVar(key string, value any) {
	rs.mu.Lock()
	rs.vars[key] = value
	rs.mu.Unlock()
}

func (rs *runState) preCompletedCommand(stepID, host string) (string, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	cmd, ok := rs.preCompleted[stepID][host]
	return cmd, ok
}

func (rs *runState) appendCompleted(c rollback.CompletedStep) {
	rs.mu.Lock()
	rs.completed = append(rs.completed, c)
	rs.mu.Unlock()
}

func (rs *runState) completedSnapshot() []rollback.CompletedStep {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]rollback.CompletedStep(nil), rs.completed...)
}

// setState validates and applies a state transition, emitting an event on success. A
// transition error is itself only logged via the event stream, not returned, since a
// single bookkeeping inconsistency should not take down an otherwise-healthy run; tests
// exercise Transition directly for the strict behavior.
func (e *DefaultExecutor) setState(rs *runState, emitter Emitter, stepID, host string, to StepState) error {
	rs.mu.Lock()
	if rs.states[stepID] == nil {
		rs.states[stepID] = make(map[string]StepState)
	}
	from, ok := rs.states[stepID][host]
	if !ok {
		from = StepPending
	}
	newState, err := Transition(from, to)
	if err == nil {
		rs.states[stepID][host] = newState
	}
	rs.mu.Unlock()

	if err != nil {
		emitter.Emit(Event{Kind: EventStepStateChanged, Timestamp: e.now(), RunID: rs.runID, StepID: stepID, Host: host, From: from, To: from, Err: err})
		return err
	}
	emitter.Emit(Event{Kind: EventStepStateChanged, Timestamp: e.now(), RunID: rs.runID, StepID: stepID, Host: host, From: from, To: newState})
	return nil
}

// Run implements Executor.
func (e *DefaultExecutor) Run(ctx context.Context, rb *runbook.Runbook, opts Options) (*Summary, error) {
	runID := opts.RunID
	if runID == "" {
		runID = e.genRunID()
	}
	vars := map[string]any{}
	for k, v := range opts.Vars {
		vars[k] = v
	}
	rs := newRunState(runID, vars)
	return e.run(ctx, rb, opts, rs)
}

// Resume implements Executor: it reloads a checkpointed run's state and the original
// runbook, then continues driving the DAG, skipping steps/hosts already succeeded.
func (e *DefaultExecutor) Resume(ctx context.Context, runID string, opts Options) (*Summary, error) {
	if opts.CheckpointDir == "" {
		return nil, fmt.Errorf("resume requires Options.CheckpointDir")
	}
	cp, err := LoadCheckpoint(opts.CheckpointDir, runID)
	if err != nil {
		return nil, fmt.Errorf("load checkpoint: %w", err)
	}
	if cp.RunbookPath == "" {
		return nil, fmt.Errorf("checkpoint %q does not record a runbook path", runID)
	}
	if e.Parser == nil {
		return nil, fmt.Errorf("executor has no Parser configured, cannot resume")
	}

	f, err := os.Open(cp.RunbookPath)
	if err != nil {
		return nil, fmt.Errorf("open runbook %q: %w", cp.RunbookPath, err)
	}
	defer f.Close()
	rb, err := e.Parser.Parse(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("parse runbook %q: %w", cp.RunbookPath, err)
	}

	vars := map[string]any{}
	for k, v := range cp.Vars {
		vars[k] = v
	}
	for k, v := range opts.Vars {
		vars[k] = v
	}

	rs := newRunState(runID, vars)
	for _, entry := range cp.CompletedOrder {
		if rs.preCompleted[entry.StepID] == nil {
			rs.preCompleted[entry.StepID] = make(map[string]string)
		}
		rs.preCompleted[entry.StepID][entry.Host] = entry.ResolvedRollbackCommand
	}

	resumeOpts := opts
	resumeOpts.RunID = runID
	resumeOpts.RunbookPath = cp.RunbookPath
	return e.run(ctx, rb, resumeOpts, rs)
}

func (e *DefaultExecutor) run(ctx context.Context, rb *runbook.Runbook, opts Options, rs *runState) (*Summary, error) {
	emitter := e.emitter()
	emitter.Emit(Event{Kind: EventRunStarted, Timestamp: e.now(), RunID: rs.runID})

	g, err := dag.Build(rb.Steps)
	if err != nil {
		return nil, fmt.Errorf("build DAG: %w", err)
	}

	strategy := opts.HostRollout
	if strategy == nil {
		strategy = AllAtOnceStrategy{}
	}

	summary := &Summary{RunID: rs.runID, StepStates: rs.states}

	var runErr error
loop:
	for _, level := range g.Levels() {
		if ctx.Err() != nil {
			runErr = ctx.Err()
			break loop
		}
		for _, outcome := range e.runLevel(ctx, rs, emitter, rb, level, strategy, opts) {
			if outcome.err != nil {
				runErr = outcome.err
				summary.FailedStep = outcome.step.ID
			}
		}
		if runErr != nil {
			break loop
		}
	}

	if runErr != nil {
		summary.Failed = true
		// Rollback must not inherit the run's own cancellation: if the operator hit
		// Ctrl+C, the whole point of rolling back is to undo side effects despite
		// that cancellation, so it runs on a context detached from ctx's Done channel
		// (context values, such as trace IDs, are still preserved).
		e.performRollback(context.WithoutCancel(ctx), rs, emitter)
		summary.RollbackRan = len(rs.completedSnapshot()) > 0
	}

	emitter.Emit(Event{Kind: EventRunFinished, Timestamp: e.now(), RunID: rs.runID, Err: runErr})

	if opts.CheckpointDir != "" {
		if cerr := e.saveCheckpoint(opts, rb, rs, summary); cerr != nil {
			emitter.Emit(Event{Kind: EventRunFinished, Timestamp: e.now(), RunID: rs.runID, Message: "checkpoint save failed", Err: cerr})
		}
	}

	return summary, runErr
}

// stepOutcome is the result of driving one step to completion (or to a point where the
// whole run must abort).
type stepOutcome struct {
	step *runbook.Step
	err  error // non-nil aborts the run
}

// runLevel runs every step in level concurrently, bounded by opts.Concurrency (or the
// level's size if unset), so independent steps do not wait on one another.
func (e *DefaultExecutor) runLevel(ctx context.Context, rs *runState, emitter Emitter, rb *runbook.Runbook, level []*runbook.Step, strategy RolloutStrategy, opts Options) []stepOutcome {
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = len(level)
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)

	outcomes := make([]stepOutcome, len(level))
	var wg sync.WaitGroup
	for i, step := range level {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, step *runbook.Step) {
			defer wg.Done()
			defer func() { <-sem }()
			outcomes[i] = stepOutcome{step: step, err: e.runOneStep(ctx, rs, emitter, rb, step, strategy, opts)}
		}(i, step)
	}
	wg.Wait()
	return outcomes
}

// runOneStep drives a single step: dependency-skip propagation, approval, host
// rollout, and failure classification. A non-nil return means the whole run aborts.
func (e *DefaultExecutor) runOneStep(ctx context.Context, rs *runState, emitter Emitter, rb *runbook.Runbook, step *runbook.Step, strategy RolloutStrategy, opts Options) error {
	if ctx.Err() != nil {
		return nil
	}

	if rs.dependsOnFailedOrSkipped(step) {
		e.markSkipped(rs, emitter, rb, step)
		return nil
	}

	hosts, herr := e.resolveHosts(ctx, rb, step)
	if herr != nil {
		return fmt.Errorf("step %q: %w", step.ID, herr)
	}

	approved, aerr := e.handleApproval(ctx, rs, emitter, rb, step, hosts, opts)
	if aerr != nil {
		return aerr
	}
	if !approved {
		rs.markSkippedStep(step.ID)
		return nil
	}

	failed := e.runStepAcrossHosts(ctx, rs, emitter, rb, step, hosts, strategy, opts.DryRun)
	if failed {
		rs.markFailed(step.ID)
		if !step.ContinueOnError {
			return fmt.Errorf("step %q failed", step.ID)
		}
	}
	return nil
}

func (e *DefaultExecutor) markSkipped(rs *runState, emitter Emitter, rb *runbook.Runbook, step *runbook.Step) {
	rs.markSkippedStep(step.ID)
	hosts, _ := e.resolveHosts(context.Background(), rb, step)
	if len(hosts) == 0 {
		e.setState(rs, emitter, step.ID, "", StepSkipped) //nolint:errcheck
		return
	}
	for _, h := range hosts {
		e.setState(rs, emitter, step.ID, h.Name, StepSkipped) //nolint:errcheck
	}
}

func targetString(t runbook.Target) string {
	if t.Group != "" {
		return t.Group
	}
	return t.Host
}

func (e *DefaultExecutor) resolveHosts(ctx context.Context, rb *runbook.Runbook, step *runbook.Step) ([]inventory.Host, error) {
	spec := ""
	if step.Run != nil && step.Run.Target != "" {
		spec = step.Run.Target
	} else if len(rb.Targets) > 0 {
		spec = targetString(rb.Targets[0])
	}
	if spec == "" {
		return nil, fmt.Errorf("no target host or group specified")
	}
	if e.Inventory == nil {
		return nil, fmt.Errorf("no inventory configured")
	}
	return e.Inventory.Resolve(ctx, spec)
}
