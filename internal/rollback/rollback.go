// SPDX-License-Identifier: Apache-2.0

// Package rollback runs the rollback actions of already-completed steps, in reverse
// dependency order, when a run fails.
package rollback

import (
	"context"

	"github.com/Leo-Galli/opsrun/internal/inventory"
	"github.com/Leo-Galli/opsrun/internal/runbook"
)

// CompletedStep is a step that finished successfully on a given host and is therefore a
// candidate for rollback if a later step fails.
type CompletedStep struct {
	Step *runbook.Step
	Host inventory.Host
}

// Outcome is the result of rolling back a single completed step on a single host.
type Outcome struct {
	CompletedStep CompletedStep
	Err           error // nil on success
}

// Manager rolls back completed steps in reverse order when a run must be undone.
// Implementations must be idempotent: a rollback action may be invoked more than once
// for the same step and host, and must not retry indefinitely on failure.
type Manager interface {
	// Rollback runs the rollback action of each entry in completed, in the order
	// given by the caller (callers must pass entries in reverse completion order).
	// It returns one Outcome per entry and does not stop early on individual
	// failures, so that as much of the system as possible is restored.
	Rollback(ctx context.Context, completed []CompletedStep) []Outcome
}
