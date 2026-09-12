package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"
)

// isTerminal reports whether w is a real terminal file descriptor, not a redirected file,
// pipe, or the in-memory buffer tests use. The TUI must never emit control sequences onto
// redirected output, so callers fall back to headless JSON whenever this is false.
func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(file.Fd())
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func cli(parent context.Context, argv []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gemma-agent", flag.ContinueOnError)
	flags.SetOutput(stderr)
	rootPath := flags.String("root", ".", "existing worktree to inspect and edit")
	endpoint := flags.String("endpoint", "http://127.0.0.1:8114/v1", "OpenAI-compatible base URL of an already-running server")
	prompt := flags.String("prompt", "", "task description (choose one prompt source)")
	promptFile := flags.String("prompt-file", "", "file containing the task description")
	taskFile := flags.String("task-file", "", "research task JSON; sends only its report field to the model")
	instructions := flags.String("instructions", "", "optional explicit instructions file; no ambient AGENTS.md discovery")
	output := flags.String("output", "", "artifact parent directory outside the worktree; default: sibling .<worktree>-gemma-runs")
	testCommand := flags.String("test-command", "", `fixed test argv as JSON, e.g. '["node","test.mjs"]'; no shell parsing`)
	timeout := flags.Duration("timeout", 120*time.Second, "total deadline, including requests, tools, and final verification")
	toolTimeout := flags.Duration("tool-timeout", 30*time.Second, "per-test-command deadline, bounded by the total deadline")
	config := Config{}
	flags.StringVar(&config.SamplingProfile, "sampling-profile", "baseline", "sampling defaults: baseline (temperature 0) or gemma (1.0, top_p .95, top_k 64, min_p 0)")
	seed := flags.Int("seed", -1, "optional non-negative server sampling seed; -1 uses server default")
	flags.StringVar(&config.PromptProfile, "prompt-profile", "baseline", "initial prompt: baseline, focused, or test-first (experimental)")
	flags.StringVar(&config.ReadFormat, "read-format", "text", "file-read response format: text, json, or xml")
	flags.BoolVar(&config.DedupReads, "dedup-reads", false, "experimental: reference earlier unchanged read pages instead of repeating their content")
	flags.BoolVar(&config.EnableSearch, "enable-search", false, "experimental: enable literal search within a source file")
	flags.BoolVar(&config.TaskReminder, "task-reminder", false, "experimental: restate the original task after successful reads")
	flags.BoolVar(&config.EditOnly, "edit-only", false, "diagnostic: only edit/test tools; caller must supply complete source in the task")
	flags.BoolVar(&config.RichEditFeedback, "rich-edit-feedback", false, "experimental: return current source context and a reason on every edit outcome instead of a bare error, and on success")
	flags.BoolVar(&config.AutoTestAfterEdit, "auto-test-after-edit", false, "experimental: run the configured test command immediately after a successful edit instead of waiting for the model to call run_tests")
	flags.BoolVar(&config.DetectRepeatedEdits, "detect-repeated-edits", false, "experimental: tell the model when it repeats an exact edit (same path/oldText/newText) it already applied and later reverted or replaced in this run")
	tui := flags.Bool("tui", false, "launch the interactive terminal UI instead of one-shot JSON output; silently falls back to headless when stdout is not a terminal")
	flags.BoolVar(&config.PreserveToolReasoning, "preserve-tool-reasoning", false, "preserve native tool-turn reasoning in history; excludes leaked tool markup")
	flags.StringVar(&config.Model, "model", "gemma4", "server model ID")
	flags.IntVar(&config.MaxTokens, "max-tokens", 3072, "maximum completion tokens per request")
	flags.IntVar(&config.MaxTurns, "max-turns", 16, "maximum model requests")
	flags.Float64Var(&config.Temperature, "temperature", 0, "sampling temperature")
	flags.IntVar(&config.MaxHistoryBytes, "max-history-bytes", 64<<10, "request byte limit (not a tokenizer); stops without truncating history")
	flags.BoolVar(&config.RecoverGemma, "recover-gemma", true, "recover complete Gemma tool spans; disable for native-only comparisons")
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 2 }
	if err := flags.Parse(argv); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		return fail(fmt.Errorf("unexpected positional arguments"))
	}
	if config.SamplingProfile != "baseline" && config.SamplingProfile != "gemma" {
		return fail(fmt.Errorf("sampling-profile must be baseline or gemma"))
	}
	if *seed < -1 {
		return fail(fmt.Errorf("seed must be -1 or non-negative"))
	}
	if *seed >= 0 {
		config.Seed = seed
	}
	if config.SamplingProfile == "gemma" {
		explicitTemperature := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "temperature" {
				explicitTemperature = true
			}
		})
		if !explicitTemperature {
			config.Temperature = 1
		}
		topP, topK, minP := .95, 64, 0.0
		config.TopP, config.TopK, config.MinP = &topP, &topK, &minP
	}
	if config.PromptProfile != "baseline" && config.PromptProfile != "focused" && config.PromptProfile != "test-first" {
		return fail(fmt.Errorf("prompt-profile must be baseline, focused, or test-first"))
	}
	if config.ReadFormat != "text" && config.ReadFormat != "json" && config.ReadFormat != "xml" {
		return fail(fmt.Errorf("read-format must be text, json, or xml"))
	}
	if *timeout <= 0 || *toolTimeout <= 0 || config.MaxTurns < 1 || config.MaxTokens < 1 || config.MaxHistoryBytes < 1 || config.Temperature < 0 || config.Temperature > 2 || math.IsNaN(config.Temperature) || math.IsInf(config.Temperature, 0) || config.Model == "" {
		return fail(fmt.Errorf("invalid limits, temperature, or model"))
	}
	parsedURL, err := url.Parse(*endpoint)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return fail(fmt.Errorf("endpoint must be an http(s) base URL without credentials, query, or fragment"))
	}
	sources := 0
	for _, value := range []string{*prompt, *promptFile, *taskFile} {
		if value != "" {
			sources++
		}
	}
	if sources != 1 {
		return fail(fmt.Errorf("provide exactly one of --prompt, --prompt-file, or --task-file"))
	}
	readInput := func(path string) (string, error) {
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer file.Close()
		return readText(file)
	}
	taskID := ""
	if *promptFile != "" {
		*prompt, err = readInput(*promptFile)
		if err != nil {
			return fail(err)
		}
	}
	if *taskFile != "" {
		data, err := readInput(*taskFile)
		if err != nil {
			return fail(err)
		}
		var task struct {
			ID     string `json:"id"`
			Report string `json:"report"`
		}
		if err := json.Unmarshal([]byte(data), &task); err != nil {
			return fail(err)
		}
		*prompt, taskID = task.Report, task.ID
	}
	if strings.TrimSpace(*prompt) == "" {
		return fail(fmt.Errorf("task description is empty"))
	}
	if *instructions != "" {
		config.Instructions, err = readInput(*instructions)
		if err != nil {
			return fail(err)
		}
	}
	var command []string
	if *testCommand != "" {
		if err := json.Unmarshal([]byte(*testCommand), &command); err != nil {
			return fail(err)
		}
		if len(command) == 0 || command[0] == "" {
			return fail(fmt.Errorf("test command must contain an executable"))
		}
	}
	absRoot, err := filepath.Abs(*rootPath)
	if err != nil {
		return fail(err)
	}
	absRoot, err = filepath.EvalSymlinks(absRoot)
	if err != nil {
		return fail(err)
	}
	root, err := os.OpenRoot(absRoot)
	if err != nil {
		return fail(err)
	}
	defer root.Close()
	if *output == "" {
		*output = filepath.Join(filepath.Dir(absRoot), "."+filepath.Base(absRoot)+"-gemma-runs")
	}
	if err := os.MkdirAll(*output, 0700); err != nil {
		return fail(err)
	}
	absOutput, err := filepath.Abs(*output)
	if err != nil {
		return fail(err)
	}
	absOutput, err = filepath.EvalSymlinks(absOutput)
	if err != nil {
		return fail(err)
	}
	if relative, err := filepath.Rel(absRoot, absOutput); err != nil || filepath.IsLocal(relative) {
		return fail(fmt.Errorf("artifact directory must be outside the worktree"))
	}
	runDir, err := os.MkdirTemp(absOutput, time.Now().UTC().Format("20060102T150405Z")+"-")
	if err != nil {
		return fail(err)
	}
	fmt.Fprintln(stderr, "artifacts:", runDir)
	traceFile, err := os.OpenFile(filepath.Join(runDir, "trace.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fail(err)
	}
	defer traceFile.Close()
	trace := &Trace{Writer: traceFile, Progress: stderr}
	if err := trace.Event("run_start", map[string]any{"schema_version": 1, "config": config, "root": absRoot, "endpoint": *endpoint, "task_id": taskID, "prompt": *prompt, "test_command": command, "timeout": timeout.String(), "tool_timeout": toolTimeout.String(), "go_version": runtime.Version()}); err != nil {
		return fail(err)
	}
	ctx, cancel := context.WithTimeout(parent, *timeout)
	defer cancel()
	client := &Client{URL: *endpoint, APIKey: os.Getenv("GEMMA_API_KEY"), HTTP: &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
	tools := &Tools{Root: root, TestCommand: command, ToolTimeout: *toolTimeout, Trace: trace, Edited: map[string]bool{}, SearchEnabled: config.EnableSearch, ReadDisabled: config.EditOnly, RichEditFeedback: config.RichEditFeedback}
	var result Summary
	if *tui && isTerminal(stdout) {
		result, err = runTUI(ctx, config, *prompt, client, tools, trace)
		if err != nil {
			return fail(err)
		}
	} else {
		result = runAgent(ctx, config, *prompt, client, tools, trace)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "summary.json"), append(data, '\n'), 0600); err != nil {
		return fail(err)
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return fail(err)
	}
	switch result.Status {
	case "completed":
		return 0
	case "timeout":
		return 124
	case "cancelled":
		return 130
	default:
		return 1
	}
}
