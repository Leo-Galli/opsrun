// SPDX-License-Identifier: Apache-2.0

// Command opsrun is the OpsRun CLI entry point. The Cobra command tree is implemented in
// Phase 5; for now this prints a placeholder so the module builds and runs end to end.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stdout, "opsrun: command-line interface not yet implemented (see CHANGELOG.md)")
}
