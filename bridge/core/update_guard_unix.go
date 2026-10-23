//go:build !windows

package core

import (
	"os"
	"os/exec"
	"syscall"
)

// detachCmd: own session, so the guard outlives the restart and launchd's process-group cleanup.
func detachCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// restartOld replaces this process with the old binary, same arguments (same PID, so a
// supervisor keeps tracking it).
var restartOld = func(exe string) error {
	return syscall.Exec(exe, append([]string{exe}, os.Args[1:]...), os.Environ())
}
