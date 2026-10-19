// SPDX-License-Identifier: Apache-2.0

// Package executor orchestrates running a runbook's DAG of steps across target hosts,
// including approval gates, rollback on failure, and checkpointing for resume.
package executor

import (
	"context"

	"github.com/Leo-Galli/opsrun/internal/runbook"
)

// StepState is the state of a single step on a single host. Valid transitions are
// enforced by Transition and documented in docs/architecture.md.
type StepState string

// Supported step states.
const (
	StepPending         StepState = "pending"
	StepWaitingApproval StepState = "waiting_approval"
	StepRunning         StepState = "running"
	StepSucceeded       StepState = "succeeded"
	StepFailed          StepState = "failed"
	StepSkipped         StepState = "skipped"
	StepRollingBack     StepState = "rolling_back"
	StepRolledBack      StepState = "rolled_back"
	StepRollbackFailed  StepState = "rollback_failed"
)

// IsTerminal reports whether a step in this state will never transition again.
func (s StepState) IsTerminal() bool {
	switch s {
	case StepSucceeded, StepFailed, StepSkipped, StepRolledBack, StepRollbackFailed:
		return true
	default:
		return false
	}
}

// Options configures a single Executor run.
type Options struct {
	RunID          string // if empty, a new run id is generated
	RunbookPath    string // recorded in checkpoints so Resume can reload the runbook
	DryRun         bool
	Interactive    bool
	Concurrency    int
	Vars           map[string]any
	CheckpointDir  string // if set, a checkpoint is written after every step completes
	HostRollout    RolloutStrategy
	ApprovalPolicy runbook.ApprovalPolicy // overrides the runbook's own policy when Mode is non-empty
}

// Summary is the outcome of a completed or aborted run.
type Summary struct {
	RunID       string
	Failed      bool
	StepStates  map[string]map[string]StepState // step id -> host name -> final state
	FailedStep  string
	RollbackRan bool
}

// Executor runs a parsed, validated runbook to completion or failure.
type Executor interface {
	Run(ctx context.Context, rb *runbook.Runbook, opts Options) (*Summary, error)
	Resume(ctx context.Context, runID string, opts Options) (*Summary, error)
}
