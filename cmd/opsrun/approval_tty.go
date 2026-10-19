// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Leo-Galli/opsrun/internal/approval"
)

// TTYGate implements approval.Gate by prompting on In/Out, suitable for a plain
// terminal (and used as the TUI's fallback when stdout is not a real TTY).
type TTYGate struct {
	In  io.Reader
	Out io.Writer
}

// RequestApproval implements approval.Gate.
func (g *TTYGate) RequestApproval(_ context.Context, req approval.Request) (approval.Response, error) {
	fmt.Fprintf(g.Out, "\n=== Approval required: step %q ===\n", req.Step.ID)
	hostNames := make([]string, 0, len(req.Hosts))
	for _, h := range req.Hosts {
		hostNames = append(hostNames, h.Name)
	}
	fmt.Fprintf(g.Out, "Hosts: %s\n", strings.Join(hostNames, ", "))
	fmt.Fprintf(g.Out, "Command: %s\n", req.InterpolatedCommand)
	fmt.Fprint(g.Out, "[a]pprove, [s]kip, [A]bort, [e]dit variable? ")

	scanner := bufio.NewScanner(g.In)
	if !scanner.Scan() {
		return approval.Response{}, fmt.Errorf("no input available for approval prompt on step %q", req.Step.ID)
	}
	switch strings.TrimSpace(scanner.Text()) {
	case "a", "approve", "":
		return approval.Response{Decision: approval.DecisionApprove}, nil
	case "s", "skip":
		return approval.Response{Decision: approval.DecisionSkip}, nil
	case "A", "abort":
		return approval.Response{Decision: approval.DecisionAbort}, nil
	case "e", "edit":
		fmt.Fprint(g.Out, "variable name: ")
		if !scanner.Scan() {
			return approval.Response{Decision: approval.DecisionAbort}, nil
		}
		name := strings.TrimSpace(scanner.Text())
		fmt.Fprint(g.Out, "new value: ")
		if !scanner.Scan() {
			return approval.Response{Decision: approval.DecisionAbort}, nil
		}
		value := strings.TrimSpace(scanner.Text())
		return approval.Response{Decision: approval.DecisionEditVariable, VariableName: name, VariableValue: value}, nil
	default:
		fmt.Fprintln(g.Out, "unrecognized response; treating as abort")
		return approval.Response{Decision: approval.DecisionAbort}, nil
	}
}

// NonInteractiveGate always aborts: it is used when --non-interactive is set so that a
// step requiring approval fails fast instead of silently proceeding or hanging on input
// that will never come.
type NonInteractiveGate struct{}

// RequestApproval implements approval.Gate.
func (NonInteractiveGate) RequestApproval(_ context.Context, req approval.Request) (approval.Response, error) {
	return approval.Response{}, fmt.Errorf("step %q requires approval but --non-interactive was set", req.Step.ID)
}
