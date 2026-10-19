// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CompletedEntry records one step that finished successfully on one host, in the order
// it completed, so a resumed run can rebuild correct rollback ordering.
type CompletedEntry struct {
	StepID                  string `json:"step_id"`
	Host                    string `json:"host"`
	ResolvedRollbackCommand string `json:"resolved_rollback_command,omitempty"`
}

// Checkpoint is the on-disk representation of a run's progress, written after every
// step completes when Options.CheckpointDir is set, and read back by Resume.
type Checkpoint struct {
	RunID          string                          `json:"run_id"`
	RunbookName    string                          `json:"runbook_name"`
	RunbookPath    string                          `json:"runbook_path"`
	Vars           map[string]any                  `json:"vars"`
	StepStates     map[string]map[string]StepState `json:"step_states"`
	CompletedOrder []CompletedEntry                `json:"completed_order"`
	Failed         bool                            `json:"failed"`
	FailedStep     string                          `json:"failed_step,omitempty"`
	RollbackRan    bool                            `json:"rollback_ran"`
}

func checkpointPath(dir, runID string) string {
	return filepath.Join(dir, runID+".json")
}

// SaveCheckpoint writes cp to dir, atomically (write-then-rename) so a crash mid-write
// never leaves a corrupt checkpoint behind.
func SaveCheckpoint(dir string, cp *Checkpoint) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}
	final := checkpointPath(dir, cp.RunID)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("finalize checkpoint: %w", err)
	}
	return nil
}

// LoadCheckpoint reads back a checkpoint previously written by SaveCheckpoint.
func LoadCheckpoint(dir, runID string) (*Checkpoint, error) {
	data, err := os.ReadFile(checkpointPath(dir, runID))
	if err != nil {
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("parse checkpoint: %w", err)
	}
	return &cp, nil
}
