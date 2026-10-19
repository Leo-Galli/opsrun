// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"sync"
	"time"
)

// EventKind classifies an Event.
type EventKind string

// Supported event kinds.
const (
	EventRunStarted       EventKind = "run_started"
	EventRunFinished      EventKind = "run_finished"
	EventStepStateChanged EventKind = "step_state_changed"
	EventApprovalRequired EventKind = "approval_required"
	EventApprovalDecided  EventKind = "approval_decided"
	EventCommandResult    EventKind = "command_result"
	EventRollbackResult   EventKind = "rollback_result"
)

// Event is a single, immutable fact about a run, emitted on every state transition so
// the TUI and the audit log can both observe progress without coupling to each other.
type Event struct {
	Kind      EventKind
	Timestamp time.Time
	RunID     string
	StepID    string
	Host      string
	From      StepState
	To        StepState
	Command   string
	ExitCode  *int
	Duration  time.Duration
	Err       error
	Message   string
}

// Emitter receives Events as a run progresses. Implementations must not block for long,
// since the executor emits synchronously on its own goroutine.
type Emitter interface {
	Emit(Event)
}

// EmitterFunc adapts a function to the Emitter interface.
type EmitterFunc func(Event)

// Emit implements Emitter.
func (f EmitterFunc) Emit(e Event) { f(e) }

// NoopEmitter discards every event.
type NoopEmitter struct{}

// Emit implements Emitter.
func (NoopEmitter) Emit(Event) {}

// MultiEmitter fans a single Event out to every Emitter in the slice, in order. Use it
// to drive the TUI (or a Recorder) and the audit log from the same run.
type MultiEmitter []Emitter

// Emit implements Emitter.
func (m MultiEmitter) Emit(e Event) {
	for _, em := range m {
		em.Emit(e)
	}
}

// Recorder is an Emitter that stores every event it receives, in order. It is intended
// for tests and for the TUI's internal event log, not for production audit storage. It
// is safe for concurrent use: the executor emits command-result events from per-host
// goroutines while a step's batch is running.
type Recorder struct {
	mu     sync.Mutex
	Events []Event
}

// Emit implements Emitter.
func (r *Recorder) Emit(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Events = append(r.Events, e)
}

// Snapshot returns a copy of the events recorded so far.
func (r *Recorder) Snapshot() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.Events...)
}
