package io

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
)

type LocalExecDriver struct{}

func NewLocalExecDriver() *LocalExecDriver {
	return &LocalExecDriver{}
}

func (d *LocalExecDriver) Exec(ctx context.Context, cmdStr string) (string, string, int, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// We return err as nil if it's just an exit error, since we captured the exit code
	if exitCode != 0 && exitCode != -1 {
		err = nil
	}

	return stdout.String(), stderr.String(), exitCode, err
}
