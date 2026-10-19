// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/audit"
)

func TestAuditEmitterWritesRecognizedEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := audit.NewFileSink(path, nil)
	require.NoError(t, err)

	emitter := &AuditEmitter{Sink: sink}
	emitter.Emit(Event{Kind: EventRunStarted, Timestamp: time.Now(), RunID: "run-1"})
	emitter.Emit(Event{Kind: EventCommandResult, Timestamp: time.Now(), RunID: "run-1", StepID: "s1", Host: "h1", Command: "echo hi"})
	emitter.Emit(Event{Kind: EventApprovalDecided, Timestamp: time.Now(), RunID: "run-1", StepID: "s1", Message: "approve"})
	require.NoError(t, sink.Close())

	records, err := audit.ReadAll(path)
	require.NoError(t, err)
	require.Len(t, records, 3)
	require.Equal(t, audit.EventRunStarted, records[0].Type)
	require.Equal(t, audit.EventStepFinished, records[1].Type)
	require.Equal(t, "echo hi", records[1].Command)
	require.Equal(t, audit.EventApprovalGiven, records[2].Type)
	require.Equal(t, "approve", records[2].Approver)
}

func TestAuditEmitterNilSinkDoesNotPanic(t *testing.T) {
	emitter := &AuditEmitter{}
	require.NotPanics(t, func() {
		emitter.Emit(Event{Kind: EventRunStarted})
	})
}

func TestMultiEmitterFansOut(t *testing.T) {
	r1, r2 := &Recorder{}, &Recorder{}
	m := MultiEmitter{r1, r2}
	m.Emit(Event{Kind: EventRunStarted, RunID: "run-1"})
	require.Len(t, r1.Snapshot(), 1)
	require.Len(t, r2.Snapshot(), 1)
}
