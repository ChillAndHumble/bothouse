//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

// detachCmd: DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP, so the guard survives the console
// supervisor and gets no console of its own.
func detachCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000200}
}

// restartOld: nil on Windows — the guard there is a detached process the service script does
// not kill, so it rolls back itself; no startup recovery needed.
var restartOld func(exe string) error
