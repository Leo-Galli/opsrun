// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadCheckpoint(t *testing.T) {
	dir := t.TempDir()
	cp := &Checkpoint{
		RunID:       "run-1",
		RunbookName: "example",
		RunbookPath: "/tmp/example.md",
		Vars:        map[string]any{"foo": "bar"},
		StepStates: map[string]map[string]StepState{
			"step-1": {"host-1": StepSucceeded},
		},
		CompletedOrder: []CompletedEntry{
			{StepID: "step-1", Host: "host-1", ResolvedRollbackCommand: "echo rollback"},
		},
		Failed:     true,
		FailedStep: "step-2",
	}
	require.NoError(t, SaveCheckpoint(dir, cp))

	loaded, err := LoadCheckpoint(dir, "run-1")
	require.NoError(t, err)
	require.Equal(t, cp.RunID, loaded.RunID)
	require.Equal(t, cp.RunbookPath, loaded.RunbookPath)
	require.Equal(t, cp.Vars, loaded.Vars)
	require.Equal(t, cp.StepStates, loaded.StepStates)
	require.Equal(t, cp.CompletedOrder, loaded.CompletedOrder)
	require.True(t, loaded.Failed)
	require.Equal(t, "step-2", loaded.FailedStep)
}

func TestLoadCheckpointMissing(t *testing.T) {
	_, err := LoadCheckpoint(t.TempDir(), "nonexistent")
	require.Error(t, err)
}

func TestSaveCheckpointCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	cp := &Checkpoint{RunID: "run-2"}
	require.NoError(t, SaveCheckpoint(dir, cp))
	_, err := LoadCheckpoint(dir, "run-2")
	require.NoError(t, err)
}
