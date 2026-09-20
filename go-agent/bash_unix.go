//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const maxBashCommandBytes = 16 << 10

var (
	// These checks are intentionally conservative. Bash mode is an experiment interface for
	// disposable worktrees, not a security sandbox; the command still executes with the host
	// user's filesystem permissions. Rejecting the common destructive/network/install paths keeps
	// an accidental model action from doing disproportionate damage without pretending to prove
	// arbitrary shell text safe.
	blockedBashCommand = regexp.MustCompile(`(?i)(^|[;&|()])\s*(?:sudo|rm|rmdir|unlink|shred|mkfs(?:\.[[:alnum:]]+)?|dd|shutdown|reboot|kill|pkill|bash|sh|zsh|fish|curl|wget|ssh|scp|sftp|nc|ncat|telnet|ftp|source|eval)(?:\s|$)`)
	blockedGitCommand  = regexp.MustCompile(`(?i)\bgit\s+(?:reset|clean|checkout|restore|commit|push|pull|fetch|clone)\b`)
	blockedInstallCmd  = regexp.MustCompile(`(?i)\b(?:npm|pnpm|yarn|pip|pip3|uv|go|cargo|brew)\s+(?:install|ci|add|get|update|remove|uninstall|fetch)\b`)
)

func validateBashCommand(command string) error {
	if command == "" || strings.TrimSpace(command) == "" {
		return fmt.Errorf("command must be non-empty")
	}
	if len(command) > maxBashCommandBytes {
		return fmt.Errorf("command exceeds %d bytes", maxBashCommandBytes)
	}
	if strings.IndexByte(command, 0) >= 0 {
		return fmt.Errorf("command contains NUL")
	}
	// Newlines turn one model action into an unbounded script and make trace review harder. Shell
	// operators such as &&, ||, ;, pipes, and command substitutions remain available on one line.
	if strings.ContainsAny(command, "\r\n") {
		return fmt.Errorf("multi-line shell scripts are not allowed; issue one command at a time")
	}
	if blockedBashCommand.MatchString(command) {
		return fmt.Errorf("command contains a blocked shell, destructive, network, or environment operation")
	}
	if blockedGitCommand.MatchString(command) {
		return fmt.Errorf("destructive or remote git operations are blocked")
	}
	if blockedInstallCmd.MatchString(command) {
		return fmt.Errorf("package installation or dependency mutation is blocked")
	}
	return nil
}

func sanitizedBashEnvironment(worktree string) []string {
	result := make([]string, 0, len(os.Environ())+8)
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(key)
		if key == "HOME" || key == "PWD" || key == "OLDPWD" ||
			strings.Contains(upper, "TOKEN") || strings.Contains(upper, "SECRET") ||
			strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "_PASS") ||
			strings.HasSuffix(upper, "_KEY") || strings.HasPrefix(upper, "AWS_") ||
			strings.HasPrefix(upper, "GH_") || strings.HasPrefix(upper, "GITHUB_") ||
			strings.HasPrefix(upper, "OPENAI_") || strings.HasPrefix(upper, "ANTHROPIC_") ||
			strings.HasPrefix(upper, "HF_") {
			continue
		}
		result = append(result, entry)
	}
	result = append(result,
		"HOME="+worktree,
		"CI=1",
		"NO_COLOR=1",
		// HOME intentionally points at the disposable worktree so tools cannot reach the
		// operator's personal config. npm otherwise creates .npm/_logs inside that worktree
		// during an offline test, which is an irrelevant change that invalidates the source
		// allowlist. Keep the cache outside the worktree while retaining the offline policy.
		"npm_config_cache="+filepath.Join(os.TempDir(), "anvil-agent-npm-cache"),
		"npm_config_audit=false",
		"npm_config_fund=false",
		"npm_config_update_notifier=false",
		"npm_config_offline=true",
		"HUSKY=0",
	)
	return result
}

func runBashCommand(parent context.Context, dir, command string, timeout time.Duration) (CommandResult, error) {
	if err := validateBashCommand(command); err != nil {
		return CommandResult{}, err
	}
	if dir == "" {
		return CommandResult{}, fmt.Errorf("bash worktree is empty")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/bash", "--noprofile", "--norc", "-lc", command)
	cmd.Dir = dir
	cmd.Env = sanitizedBashEnvironment(dir)
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
	defer syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	err := cmd.Wait()
	result := CommandResult{
		ExitCode: cmd.ProcessState.ExitCode(),
		Output:   string(output.data), Truncated: output.truncated,
		TimedOut: ctx.Err() != nil, WallMS: time.Since(start).Milliseconds(),
	}
	if err != nil && result.ExitCode == 0 {
		return result, err
	}
	return result, nil
}
