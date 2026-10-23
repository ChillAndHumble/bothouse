//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

// detach puts `dsh web` in its own session so it outlives the wizard and its terminal.
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

func processAlive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func stopProcess(pid int) { _ = syscall.Kill(pid, syscall.SIGTERM) }
