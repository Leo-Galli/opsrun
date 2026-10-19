// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"

	"github.com/Leo-Galli/opsrun/internal/audit"
)

// AuditEmitter adapts an audit.Sink into an Emitter, translating executor events into
// audit.Event records. Command is already redacted by the time it reaches an Event (see
// runAction), so AuditEmitter never needs to redact anything itself.
type AuditEmitter struct {
	Sink audit.Sink
}

// Emit implements Emitter. Write errors are intentionally swallowed here: a failing
// audit write must not take down an otherwise-healthy run. Callers who need to know
// about audit failures should wrap Sink with one that also reports write errors
// elsewhere (logs, metrics).
func (a *AuditEmitter) Emit(e Event) {
	if a.Sink == nil {
		return
	}
	auditType, ok := auditEventType(e.Kind)
	if !ok {
		return
	}
	ev := audit.Event{
		Type:      auditType,
		Timestamp: e.Timestamp.UTC(),
		RunID:     e.RunID,
		Host:      e.Host,
		StepID:    e.StepID,
		Command:   e.Command,
		ExitCode:  e.ExitCode,
		Duration:  e.Duration,
	}
	if e.Kind == EventApprovalDecided {
		ev.Approver = e.Message
	}
	_ = a.Sink.Write(context.Background(), ev)
}

func auditEventType(k EventKind) (audit.EventType, bool) {
	switch k {
	case EventRunStarted:
		return audit.EventRunStarted, true
	case EventRunFinished:
		return audit.EventRunFinished, true
	case EventStepStateChanged:
		return audit.EventStepFinished, true
	case EventApprovalRequired:
		return audit.EventApprovalAsked, true
	case EventApprovalDecided:
		return audit.EventApprovalGiven, true
	case EventCommandResult:
		return audit.EventStepFinished, true
	case EventRollbackResult:
		return audit.EventRollbackResult, true
	default:
		return "", false
	}
}
