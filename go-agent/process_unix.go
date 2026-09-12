//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type CommandResult struct {
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
	TimedOut  bool   `json:"timed_out"`
	WallMS    int64  `json:"wall_ms"`
}

type limitedOutput struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := min(len(p), maxOutputBytes-len(b.data))
	b.data = append(b.data, p[:n]...)
	if n < len(p) {
		b.truncated = true
	}
	return len(p), nil
}

func runCommand(parent context.Context, dir string, argv []string, timeout time.Duration) (CommandResult, error) {
	if len(argv) == 0 || argv[0] == "" {
		return CommandResult{}, fmt.Errorf("test command is empty")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CI=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = time.Second
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	output := &limitedOutput{}
	cmd.Stdout, cmd.Stderr = output, output
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return CommandResult{}, err
	}
	// Also clean up children of a test runner which exits without reaping them.
	defer syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	err := cmd.Wait()
	result := CommandResult{ExitCode: cmd.ProcessState.ExitCode(), Output: string(output.data), Truncated: output.truncated, TimedOut: ctx.Err() != nil, WallMS: time.Since(start).Milliseconds()}
	if err != nil && result.ExitCode == 0 {
		return result, err
	}
	return result, nil
}
