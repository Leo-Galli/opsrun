// SPDX-License-Identifier: Apache-2.0

package executor

import "fmt"

// InvalidTransitionError reports an attempt to move a step/host out of a state it cannot
// leave that way.
type InvalidTransitionError struct {
	From StepState
	To   StepState
}

// Error implements the error interface.
func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid state transition: %s -> %s", e.From, e.To)
}

// validTransitions is the full, explicit state machine. Each key maps to the set of
// states it may legally move to next.
var validTransitions = map[StepState]map[StepState]bool{
	StepPending: {
		StepWaitingApproval: true,
		StepRunning:         true,
		StepSkipped:         true,
	},
	StepWaitingApproval: {
		StepRunning: true,
		StepSkipped: true,
		StepFailed:  true, // abort decision
	},
	StepRunning: {
		StepSucceeded: true,
		StepFailed:    true,
	},
	StepSucceeded: {
		StepRollingBack: true, // a later step failed and this one must be undone
	},
	StepFailed: {
		StepRollingBack: true,
	},
	StepRollingBack: {
		StepRolledBack:     true,
		StepRollbackFailed: true,
	},
	// StepSkipped, StepRolledBack, StepRollbackFailed are terminal: no outgoing edges.
}

// Transition validates and returns the result of moving from to next. It does not
// mutate any state itself; callers apply the returned state to their own bookkeeping.
func Transition(from, to StepState) (StepState, error) {
	if from == to {
		return to, nil
	}
	allowed, ok := validTransitions[from]
	if !ok || !allowed[to] {
		return from, &InvalidTransitionError{From: from, To: to}
	}
	return to, nil
}
