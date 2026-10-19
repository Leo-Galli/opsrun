// SPDX-License-Identifier: Apache-2.0

// Package audit defines the append-only event sink used to build a tamper-evident
// record of every OpsRun run, for post-mortems and compliance.
package audit

import (
	"context"
	"time"
)

// EventType classifies an audit event.
type EventType string

// Supported event types.
const (
	EventRunStarted      EventType = "run_started"
	EventRunFinished     EventType = "run_finished"
	EventStepStarted     EventType = "step_started"
	EventStepFinished    EventType = "step_finished"
	EventApprovalAsked   EventType = "approval_asked"
	EventApprovalGiven   EventType = "approval_given"
	EventRollbackStarted EventType = "rollback_started"
	EventRollbackResult  EventType = "rollback_result"
)

// Event is a single immutable fact recorded to the audit log. Command is the fully
// interpolated command with any secret values redacted before this struct is
// constructed; sinks must never perform redaction themselves.
type Event struct {
	Type       EventType
	Timestamp  time.Time // always UTC
	RunID      string
	LocalUser  string
	Host       string
	StepID     string
	Command    string
	Approver   string
	ExitCode   *int
	Duration   time.Duration
	OutputHash string // hex-encoded SHA-256 of the captured output, if any
}

// Sink persists audit events. A conforming Sink must preserve the order in which events
// are written and must not silently drop events.
type Sink interface {
	Write(ctx context.Context, event Event) error
	Close() error
}
