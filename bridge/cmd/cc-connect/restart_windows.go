//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
)

// Windows has no exec(2), so a restart cannot replace the process in place.
//
//   - Under the service (the Task Scheduler script sets CC_SUPERVISED=1) and in a child started
//     by the supervisor below (CC_RESTART_CHILD=1), a restart just exits with
//     restartExitCode; whoever started us starts the (possibly updated) binary again. No extra
//     process is left behind.
//   - In a console, the first cc-connect becomes the supervisor: it starts the new cc-connect as
//     a child, waits, and starts another one each time the child asks for a restart. At most one
//     parent + one child, however many restarts. Exiting right away instead would hand the
//     console back to the shell while the child keeps writing to it.
//
// restartExitCode — 来源：sysexits.h EX_TEMPFAIL（「临时失败，可重试」）；只在 cc-connect 自己
// 的父子进程和 daemon/windows.go 的服务脚本之间约定。
const (
	restartExitCode = 75
	envRestartChild = "CC_RESTART_CHILD"
	envSupervised   = "CC_SUPERVISED"
)

func restartProcess(execPath string) error {
	if os.Getenv(envRestartChild) == "1" || os.Getenv(envSupervised) == "1" {
		os.Exit(restartExitCode)
	}
	// Ctrl+C / Ctrl+Break reach every process on the console: the child shuts down cleanly, the
	// supervisor only waits for it. Notify (not Ignore): an ignored console event falls through
	// to Windows' default handler, which would kill the supervisor at once.
	signal.Notify(make(chan os.Signal, 1), os.Interrupt)
	for {
		cmd := exec.Command(execPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = append(os.Environ(), envRestartChild+"=1")
		err := cmd.Run()
		code := 0
		var exitErr *exec.ExitError
		switch {
		case errors.As(err, &exitErr):
			code = exitErr.ExitCode()
		case err != nil:
			return err
		}
		if code != restartExitCode {
			os.Exit(code)
		}
	}
}
