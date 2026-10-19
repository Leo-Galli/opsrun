// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInvalidTransitionErrorMessage(t *testing.T) {
	err := &InvalidTransitionError{From: StepPending, To: StepSucceeded}
	require.Equal(t, "invalid state transition: pending -> succeeded", err.Error())
}

func TestEmitterFunc(t *testing.T) {
	var got Event
	f := EmitterFunc(func(e Event) { got = e })
	f.Emit(Event{Kind: EventRunStarted, RunID: "run-1"})
	require.Equal(t, EventRunStarted, got.Kind)
	require.Equal(t, "run-1", got.RunID)
}

func TestNoopEmitter(t *testing.T) {
	require.NotPanics(t, func() {
		NoopEmitter{}.Emit(Event{Kind: EventRunStarted})
	})
}
