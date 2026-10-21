//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
)

// restartProcess: Windows has no exec(2), so the new cc-connect runs as a child and this
// process stays as a thin parent until it exits, then exits with its code. Exiting right
// away (as before) handed the console back to the shell while the child kept writing to it,
// and under `cc-connect daemon` the Task Scheduler wrapper saw exit 0, stopped supervising,
// and `daemon stop` no longer reached the orphaned child.
func restartProcess(execPath string) error {
	cmd := exec.Command(execPath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	// Ctrl+C / Ctrl+Break reach every process on the console: the child shuts down cleanly, the
	// parent only waits for it. Notify (not Ignore): an ignored console event falls through to
	// Windows' default handler, which would kill this parent at once.
	signal.Notify(make(chan os.Signal, 1), os.Interrupt)
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
