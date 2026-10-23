//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

// detachCmd: own session, so the guard outlives the restart and launchd's process-group cleanup.
func detachCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
