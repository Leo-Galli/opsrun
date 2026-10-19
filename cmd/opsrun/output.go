// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Leo-Galli/opsrun/internal/executor"
	"github.com/Leo-Galli/opsrun/internal/parser"
)

type findingJSON struct {
	Severity string `json:"severity"`
	StepID   string `json:"step_id,omitempty"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
}

func hasErrors(findings []parser.ValidationError) bool {
	for _, f := range findings {
		if f.Severity == parser.SeverityError {
			return true
		}
	}
	return false
}

func printFindings(w io.Writer, findings []parser.ValidationError, output string) {
	if output == "json" {
		out := make([]findingJSON, 0, len(findings))
		for _, f := range findings {
			out = append(out, findingJSON{
				Severity: string(f.Severity), StepID: f.StepID, Message: f.Message,
				Line: f.Position.Line, Column: f.Position.Column,
			})
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(w, string(data))
		return
	}

	for _, f := range findings {
		tag := "[OK]"
		switch f.Severity {
		case parser.SeverityError:
			tag = "[FAIL]"
		case parser.SeverityWarning:
			tag = "[WARN]"
		}
		loc := ""
		if f.Position.Line > 0 {
			loc = fmt.Sprintf(" (%d:%d)", f.Position.Line, f.Position.Column)
		}
		step := ""
		if f.StepID != "" {
			step = fmt.Sprintf(" step=%s", f.StepID)
		}
		fmt.Fprintf(w, "%s%s%s %s\n", tag, step, loc, f.Message)
	}
}

type summaryJSON struct {
	RunID       string                       `json:"run_id"`
	Failed      bool                         `json:"failed"`
	FailedStep  string                       `json:"failed_step,omitempty"`
	RollbackRan bool                         `json:"rollback_ran"`
	StepStates  map[string]map[string]string `json:"step_states"`
}

func printSummary(w io.Writer, summary *executor.Summary, output string) {
	states := make(map[string]map[string]string, len(summary.StepStates))
	for stepID, hosts := range summary.StepStates {
		states[stepID] = make(map[string]string, len(hosts))
		for host, state := range hosts {
			states[stepID][host] = string(state)
		}
	}

	if output == "json" {
		data, _ := json.MarshalIndent(summaryJSON{
			RunID: summary.RunID, Failed: summary.Failed, FailedStep: summary.FailedStep,
			RollbackRan: summary.RollbackRan, StepStates: states,
		}, "", "  ")
		fmt.Fprintln(w, string(data))
		return
	}

	fmt.Fprintf(w, "\nRun %s: ", summary.RunID)
	if summary.Failed {
		fmt.Fprintf(w, "[FAIL] failed at step %q\n", summary.FailedStep)
		if summary.RollbackRan {
			fmt.Fprintln(w, "Rollback ran for completed steps.")
		}
	} else {
		fmt.Fprintln(w, "[OK] completed successfully")
	}
	for stepID, hosts := range states {
		for host, state := range hosts {
			label := host
			if label == "" {
				label = "-"
			}
			fmt.Fprintf(w, "  step=%s host=%s state=%s\n", stepID, label, state)
		}
	}
}
