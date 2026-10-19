// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Leo-Galli/opsrun/internal/executor"
)

const timeRoundUnit = time.Millisecond

// TextEmitter prints each Event as a single line, for plain-text (non-TUI) progress
// output. It is safe for concurrent use, since the executor emits from per-host
// goroutines while a step's batch is running.
type TextEmitter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewTextEmitter returns an Emitter that writes human-readable progress lines to w.
func NewTextEmitter(w io.Writer) *TextEmitter {
	return &TextEmitter{w: w}
}

// Emit implements executor.Emitter.
func (t *TextEmitter) Emit(e executor.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch e.Kind {
	case executor.EventRunStarted:
		fmt.Fprintf(t.w, "[RUN] starting run %s\n", e.RunID)
	case executor.EventRunFinished:
		if e.Err != nil {
			fmt.Fprintf(t.w, "[FAIL] run %s finished: %v\n", e.RunID, e.Err)
		} else {
			fmt.Fprintf(t.w, "[OK] run %s finished\n", e.RunID)
		}
	case executor.EventStepStateChanged:
		host := e.Host
		if host == "" {
			host = "-"
		}
		fmt.Fprintf(t.w, "[STEP] %-8s step=%-20s host=%s\n", e.To, e.StepID, host)
	case executor.EventApprovalRequired:
		// The approval gate itself prints the interactive prompt; nothing to add here.
	case executor.EventApprovalDecided:
		fmt.Fprintf(t.w, "[APPROVAL] step=%s decision=%s\n", e.StepID, e.Message)
	case executor.EventCommandResult:
		if e.Message == "dry-run" {
			fmt.Fprintf(t.w, "[DRY-RUN] step=%s host=%s: %s\n", e.StepID, e.Host, e.Command)
			return
		}
		if e.Err != nil {
			fmt.Fprintf(t.w, "[FAIL] step=%s host=%s (%s): %v\n", e.StepID, e.Host, e.Duration.Round(timeRoundUnit), e.Err)
			return
		}
		exit := 0
		if e.ExitCode != nil {
			exit = *e.ExitCode
		}
		fmt.Fprintf(t.w, "[EXEC] step=%s host=%s exit=%d (%s): %s\n", e.StepID, e.Host, exit, e.Duration.Round(timeRoundUnit), e.Command)
	case executor.EventRollbackResult:
		if e.Err != nil {
			fmt.Fprintf(t.w, "[ROLLBACK FAIL] step=%s host=%s: %v\n", e.StepID, e.Host, e.Err)
		} else {
			fmt.Fprintf(t.w, "[ROLLBACK OK] step=%s host=%s\n", e.StepID, e.Host)
		}
	}
}
