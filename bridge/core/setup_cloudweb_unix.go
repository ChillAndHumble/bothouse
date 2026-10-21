//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

// detachProcess puts a long-lived child (the local hub) in its own process group, so a Ctrl+C or
// a launchd stop of cc-connect's group does not take it down.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
