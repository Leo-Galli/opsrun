// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package ssh

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the spawned shell in its own process group, so that
// killProcessGroup can terminate it together with any children it spawns (for example a
// grandchild process started by a pipeline), not just the shell itself.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGKILL to the process group started by setProcessGroup.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) //nolint:errcheck
}
