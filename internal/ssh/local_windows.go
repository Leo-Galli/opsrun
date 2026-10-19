// SPDX-License-Identifier: Apache-2.0

//go:build windows

package ssh

import "os/exec"

// setProcessGroup is a no-op on Windows: os/exec does not expose a portable way to
// terminate a process tree, so killProcessGroup only terminates the direct child.
// Commands that spawn long-running grandchildren (for example a backgrounded pipeline
// stage) may continue running after a timeout on this platform; OpsRun's primary
// control-node target is Linux/macOS, where setProcessGroup enforces this properly.
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup terminates the direct child process.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	cmd.Process.Kill() //nolint:errcheck
}
