// SPDX-License-Identifier: Apache-2.0

// Package approval implements human-in-the-loop confirmation of runbook steps before
// they execute.
package approval

import (
	"context"
	"time"

	"github.com/Leo-Galli/opsrun/internal/inventory"
	"github.com/Leo-Galli/opsrun/internal/runbook"
)

// Decision is the operator's response to an approval request.
type Decision string

// Supported decisions.
const (
	DecisionApprove      Decision = "approve"
	DecisionSkip         Decision = "skip"
	DecisionAbort        Decision = "abort"
	DecisionEditVariable Decision = "edit_variable"
)

// Request describes the step awaiting approval, including its fully interpolated
// command and the hosts it would run against, so the operator can make an informed
// decision.
type Request struct {
	Step                *runbook.Step
	Hosts               []inventory.Host
	InterpolatedCommand string
	Timeout             time.Duration
}

// Response is the outcome of an approval request. When Decision is DecisionEditVariable,
// VariableName and VariableValue carry the requested override.
type Response struct {
	Decision      Decision
	VariableName  string
	VariableValue string
	Approver      string
	DecidedAt     time.Time
}

// Gate asks a human to approve, skip, abort, or edit a variable before a step runs. An
// extension point for two-person approval on critical steps is implementing a Gate that
// collects more than one Response before returning.
type Gate interface {
	RequestApproval(ctx context.Context, req Request) (Response, error)
}
