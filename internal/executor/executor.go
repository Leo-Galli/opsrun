// SPDX-License-Identifier: Apache-2.0

// Package executor orchestrates running a runbook's DAG of steps across target hosts,
// including approval gates, rollback on failure, and checkpointing for resume.
package executor

import (
	"context"

	"github.com/Leo-Galli/opsrun/internal/runbook"
)

// StepState is the state of a single step on a single host.
type StepState string

// Supported step states and their valid transitions are documented in
// docs/architecture.md and enforced by the Phase 4 state machine implementation.
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

// Options configures a single Executor run.
type Options struct {
	DryRun      bool
	Interactive bool
	Concurrency int
	Vars        map[string]any
}

// Summary is the outcome of a completed or aborted run.
type Summary struct {
	RunID  string
	Failed bool
	States map[string]StepState // step id -> final state
}

// Executor runs a parsed, validated runbook to completion or failure.
type Executor interface {
	Run(ctx context.Context, rb *runbook.Runbook, opts Options) (*Summary, error)
}
