//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

// 来源：Windows process creation flags, CREATE_NO_WINDOW = 0x08000000（syscall 包没有这个常量）。
// DETACHED_PROCESS 不行：没有控制台的 uv 再拉起 python.exe 时 Windows 会给它新开一个可见的控制台窗口，
// 用户一关窗口平台就没了；CREATE_NO_WINDOW 给一个看不见的控制台，子进程沿用它。
const createNoWindow = 0x08000000

// detachProcess starts a long-lived child (the local hub) with no visible console and in its own
// process group, so closing cc-connect's console (Ctrl+C) does not take it down.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow}
}
