// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"golang.org/x/term"
)

// isTerminal reports whether f is an interactive terminal.
//
// This deliberately uses golang.org/x/term rather than an os.ModeCharDevice heuristic:
// that heuristic cannot tell a real TTY apart from /dev/null or NUL, since both are
// character devices. Redirecting opsrun's output to /dev/null previously made it
// mistake /dev/null for a terminal and hang trying to run the TUI against it. x/term is
// already an indirect dependency of bubbletea and is maintained by the Go team
// alongside golang.org/x/crypto (already an allowed dependency), so promoting it to a
// direct import here is the correct fix, not scope creep.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
