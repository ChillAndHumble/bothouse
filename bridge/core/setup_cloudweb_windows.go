//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

// 来源：Windows process creation flags, DETACHED_PROCESS = 0x00000008（syscall 包没有这个常量）。
const detachedProcess = 0x00000008

// detachProcess starts a long-lived child (the local hub) without cc-connect's console and in its
// own process group, so closing cc-connect does not take it down.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess}
}
