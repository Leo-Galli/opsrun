// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransitionValid(t *testing.T) {
	tests := []struct {
		from, to StepState
	}{
		{StepPending, StepWaitingApproval},
		{StepPending, StepRunning},
		{StepPending, StepSkipped},
		{StepWaitingApproval, StepRunning},
		{StepWaitingApproval, StepSkipped},
		{StepWaitingApproval, StepFailed},
		{StepRunning, StepSucceeded},
		{StepRunning, StepFailed},
		{StepSucceeded, StepRollingBack},
		{StepFailed, StepRollingBack},
		{StepRollingBack, StepRolledBack},
		{StepRollingBack, StepRollbackFailed},
		{StepPending, StepPending}, // self-transition always allowed
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			got, err := Transition(tt.from, tt.to)
			require.NoError(t, err)
			require.Equal(t, tt.to, got)
		})
	}
}

func TestTransitionInvalid(t *testing.T) {
	tests := []struct {
		from, to StepState
	}{
		{StepPending, StepSucceeded},
		{StepPending, StepFailed},
		{StepPending, StepRolledBack},
		{StepSucceeded, StepRunning},
		{StepSucceeded, StepFailed},
		{StepFailed, StepRunning},
		{StepFailed, StepSucceeded},
		{StepSkipped, StepRunning},
		{StepRolledBack, StepRunning},
		{StepRollbackFailed, StepRolledBack},
		{StepRunning, StepWaitingApproval},
		{StepWaitingApproval, StepPending},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			_, err := Transition(tt.from, tt.to)
			require.Error(t, err)
			var ite *InvalidTransitionError
			require.ErrorAs(t, err, &ite)
			require.Equal(t, tt.from, ite.From)
			require.Equal(t, tt.to, ite.To)
		})
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []StepState{StepSucceeded, StepFailed, StepSkipped, StepRolledBack, StepRollbackFailed}
	for _, s := range terminal {
		require.True(t, s.IsTerminal(), s)
	}
	nonTerminal := []StepState{StepPending, StepWaitingApproval, StepRunning, StepRollingBack}
	for _, s := range nonTerminal {
		require.False(t, s.IsTerminal(), s)
	}
}
