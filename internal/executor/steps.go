// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"

	"github.com/Leo-Galli/opsrun/internal/approval"
	"github.com/Leo-Galli/opsrun/internal/inventory"
	"github.com/Leo-Galli/opsrun/internal/parser"
	"github.com/Leo-Galli/opsrun/internal/rollback"
	"github.com/Leo-Galli/opsrun/internal/runbook"
	"github.com/Leo-Galli/opsrun/internal/ssh"
)

const maxApprovalEditAttempts = 10

// handleApproval decides whether step may proceed, requesting human approval when the
// step, the runbook, or --interactive requires it. It returns approved=false (with a
// nil error) for an operator-requested skip, and a non-nil error only for an abort or a
// gate failure.
func (e *DefaultExecutor) handleApproval(ctx context.Context, rs *runState, emitter Emitter, rb *runbook.Runbook, step *runbook.Step, hosts []inventory.Host, opts Options) (bool, error) {
	policy := rb.Approval
	if step.Approval != nil {
		policy = *step.Approval
	}
	if opts.ApprovalPolicy.Mode != "" {
		policy = opts.ApprovalPolicy
	}

	needsApproval := step.Confirm || opts.Interactive || policy.Mode == runbook.ApprovalModeAll
	if !needsApproval || e.Approval == nil {
		for _, h := range hosts {
			e.setState(rs, emitter, step.ID, h.Name, StepRunning) //nolint:errcheck
		}
		return true, nil
	}

	for _, h := range hosts {
		e.setState(rs, emitter, step.ID, h.Name, StepWaitingApproval) //nolint:errcheck
	}

	for attempt := 0; attempt < maxApprovalEditAttempts; attempt++ {
		cmd := interpolatedRunCommand(rb, step, rs.varsSnapshot())
		emitter.Emit(Event{Kind: EventApprovalRequired, Timestamp: e.now(), RunID: rs.runID, StepID: step.ID, Command: cmd})

		resp, err := e.Approval.RequestApproval(ctx, approval.Request{
			Step:                step,
			Hosts:               hosts,
			InterpolatedCommand: cmd,
			Timeout:             policy.Timeout,
		})
		if err != nil {
			return false, fmt.Errorf("request approval for step %q: %w", step.ID, err)
		}
		emitter.Emit(Event{Kind: EventApprovalDecided, Timestamp: e.now(), RunID: rs.runID, StepID: step.ID, Message: string(resp.Decision)})

		switch resp.Decision {
		case approval.DecisionApprove:
			for _, h := range hosts {
				e.setState(rs, emitter, step.ID, h.Name, StepRunning) //nolint:errcheck
			}
			return true, nil
		case approval.DecisionSkip:
			for _, h := range hosts {
				e.setState(rs, emitter, step.ID, h.Name, StepSkipped) //nolint:errcheck
			}
			return false, nil
		case approval.DecisionAbort:
			for _, h := range hosts {
				e.setState(rs, emitter, step.ID, h.Name, StepFailed) //nolint:errcheck
			}
			return false, fmt.Errorf("step %q aborted by operator", step.ID)
		case approval.DecisionEditVariable:
			rs.setVar(resp.VariableName, resp.VariableValue)
			continue
		default:
			return false, fmt.Errorf("step %q: unknown approval decision %q", step.ID, resp.Decision)
		}
	}
	return false, fmt.Errorf("step %q: too many variable edits without a decision", step.ID)
}

func interpolatedRunCommand(rb *runbook.Runbook, step *runbook.Step, vars map[string]any) string {
	if step.Run == nil {
		return ""
	}
	cmd, err := parser.Interpolate(step.Run.Command, rb.Vars, vars)
	if err != nil {
		return step.Run.Command
	}
	return cmd
}

// runStepAcrossHosts runs step on every host, respecting strategy's batching and
// failure tolerance. It returns true if the step should be considered failed overall.
func (e *DefaultExecutor) runStepAcrossHosts(ctx context.Context, rs *runState, emitter Emitter, rb *runbook.Runbook, step *runbook.Step, hosts []inventory.Host, strategy RolloutStrategy, dryRun bool) bool {
	maxFailures := strategy.MaxFailures(len(hosts))
	failures := 0

	for _, batch := range strategy.Batches(hosts) {
		type hostResult struct {
			host inventory.Host
			err  error
		}
		results := make(chan hostResult, len(batch))
		for _, h := range batch {
			go func(h inventory.Host) {
				err := e.runOnHost(ctx, rs, emitter, rb, step, h, dryRun)
				results <- hostResult{host: h, err: err}
			}(h)
		}
		for i := 0; i < len(batch); i++ {
			r := <-results
			if r.err != nil {
				failures++
				e.setState(rs, emitter, step.ID, r.host.Name, StepFailed) //nolint:errcheck
				continue
			}
			e.setState(rs, emitter, step.ID, r.host.Name, StepSucceeded) //nolint:errcheck
			rs.appendCompleted(rollback.CompletedStep{
				Step:                    step,
				Host:                    r.host,
				ResolvedRollbackCommand: interpolatedRollbackCommand(rb, step, rs.varsSnapshot()),
			})
		}
		if failures > maxFailures {
			return true
		}
	}
	return failures > maxFailures
}

func interpolatedRollbackCommand(rb *runbook.Runbook, step *runbook.Step, vars map[string]any) string {
	if step.Rollback == nil {
		return ""
	}
	cmd, err := parser.Interpolate(step.Rollback.Command, rb.Vars, vars)
	if err != nil {
		return step.Rollback.Command
	}
	return cmd
}

// runOnHost runs step's main action, then its checks, against a single host. A
// previously-checkpointed success for this step/host short-circuits execution.
func (e *DefaultExecutor) runOnHost(ctx context.Context, rs *runState, emitter Emitter, rb *runbook.Runbook, step *runbook.Step, host inventory.Host, dryRun bool) error {
	if cmd, ok := rs.preCompletedCommand(step.ID, host.Name); ok {
		rs.appendCompleted(rollback.CompletedStep{Step: step, Host: host, ResolvedRollbackCommand: cmd})
		return nil
	}

	if step.Run != nil {
		if err := e.runAction(ctx, rs, emitter, rb, step, host, step.Run, dryRun); err != nil {
			return err
		}
	}
	for _, check := range step.Checks {
		if err := e.runAction(ctx, rs, emitter, rb, step, host, check, dryRun); err != nil {
			return err
		}
	}
	return nil
}

func (e *DefaultExecutor) runAction(ctx context.Context, rs *runState, emitter Emitter, rb *runbook.Runbook, step *runbook.Step, host inventory.Host, action *runbook.Action, dryRun bool) error {
	cmd, err := parser.Interpolate(action.Command, rb.Vars, rs.varsSnapshot())
	if err != nil {
		return fmt.Errorf("interpolate: %w", err)
	}

	if dryRun {
		emitter.Emit(Event{Kind: EventCommandResult, Timestamp: e.now(), RunID: rs.runID, StepID: step.ID, Host: host.Name, Command: cmd, Message: "dry-run"})
		return nil
	}

	sess, err := e.Transport.Open(ctx, host)
	if err != nil {
		return fmt.Errorf("open session on %s: %w", host.Name, err)
	}
	defer sess.Close()

	start := e.now()
	result, err := sess.Run(ctx, cmd, ssh.RunOptions{CommandTimeout: action.Timeout})
	duration := e.now().Sub(start)
	if err != nil {
		emitter.Emit(Event{Kind: EventCommandResult, Timestamp: e.now(), RunID: rs.runID, StepID: step.ID, Host: host.Name, Command: cmd, Duration: duration, Err: err})
		return fmt.Errorf("run on %s: %w", host.Name, err)
	}

	exitCode := result.ExitCode
	emitter.Emit(Event{Kind: EventCommandResult, Timestamp: e.now(), RunID: rs.runID, StepID: step.ID, Host: host.Name, Command: cmd, Duration: duration, ExitCode: &exitCode})

	want := 0
	if action.Kind == runbook.ActionKindCheck {
		want = action.ExpectExit
	}
	if result.ExitCode != want {
		return fmt.Errorf("on %s: exit code %d, expected %d", host.Name, result.ExitCode, want)
	}
	return nil
}

// performRollback rolls back every successfully completed step, in reverse order.
func (e *DefaultExecutor) performRollback(ctx context.Context, rs *runState, emitter Emitter) {
	completed := rs.completedSnapshot()
	if len(completed) == 0 || e.Rollback == nil {
		return
	}
	reversed := make([]rollback.CompletedStep, len(completed))
	for i, c := range completed {
		reversed[len(completed)-1-i] = c
	}
	for _, c := range reversed {
		e.setState(rs, emitter, c.Step.ID, c.Host.Name, StepRollingBack) //nolint:errcheck
	}

	outcomes := e.Rollback.Rollback(ctx, reversed)
	for _, o := range outcomes {
		to := StepRolledBack
		if o.Err != nil {
			to = StepRollbackFailed
		}
		e.setState(rs, emitter, o.CompletedStep.Step.ID, o.CompletedStep.Host.Name, to) //nolint:errcheck
		emitter.Emit(Event{
			Kind: EventRollbackResult, Timestamp: e.now(), RunID: rs.runID,
			StepID: o.CompletedStep.Step.ID, Host: o.CompletedStep.Host.Name, Err: o.Err,
		})
	}
}

func (e *DefaultExecutor) saveCheckpoint(opts Options, rb *runbook.Runbook, rs *runState, summary *Summary) error {
	completed := rs.completedSnapshot()
	entries := make([]CompletedEntry, 0, len(completed))
	for _, c := range completed {
		entries = append(entries, CompletedEntry{StepID: c.Step.ID, Host: c.Host.Name, ResolvedRollbackCommand: c.ResolvedRollbackCommand})
	}
	cp := &Checkpoint{
		RunID:          rs.runID,
		RunbookName:    rb.Name,
		RunbookPath:    opts.RunbookPath,
		Vars:           rs.varsSnapshot(),
		StepStates:     summary.StepStates,
		CompletedOrder: entries,
		Failed:         summary.Failed,
		FailedStep:     summary.FailedStep,
		RollbackRan:    summary.RollbackRan,
	}
	return SaveCheckpoint(opts.CheckpointDir, cp)
}
