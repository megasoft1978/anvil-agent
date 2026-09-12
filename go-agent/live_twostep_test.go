package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLiveGemma4TwoStep tests a workflow-level change instead of another edit-loop mechanism:
// split repair into two independent agent calls with no shared conversation state.
//
// Step "diagnose": independent navigation (no source preload, no rich-edit-feedback, no
// auto-test-after-edit, no dedup-reads — none of 2026-09-11's opt-in mechanisms), read/search
// tools only (no edit, no run_tests), asked only to identify and explain the root cause.
//
// Step "implement": a completely fresh agent call (no memory of step 1's transcript),
// source-supplied + edit-only tools (the one condition that has produced real edits today),
// given the step-1 model's OWN diagnosis text — explicitly labeled as self-generated, not
// evaluator-authored (contrast with GEMMA_DIAGNOSIS_HINT in realrepo_test.go, which is).
//
// This isolates whether the guided/independent failure mode (never attempting an edit) is a
// workflow-structure problem (too much state at once) or a genuine navigation/reasoning gap
// (the model can't produce a useful diagnosis either, even with room to just think).
func TestLiveGemma4TwoStep(t *testing.T) {
	if os.Getenv("GEMMA_LIVE") != "1" {
		t.Skip("opt-in only: GEMMA_LIVE=1; needs an already-running local Gemma server")
	}
	pilot := os.Getenv("GEMMA_PILOT")
	if pilot == "" {
		t.Fatal("requires GEMMA_PILOT")
	}
	stage := os.Getenv("GEMMA_REPO_STAGE")
	if stage == "" {
		stage = "dayjs-guided"
	}
	endpoint := os.Getenv("GEMMA_LIVE_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8114/v1"
	}
	model := os.Getenv("GEMMA_LIVE_MODEL")
	if model == "" {
		model = "gemma4"
	}
	output := os.Getenv("GEMMA_LIVE_OUTPUT")
	if output == "" {
		output = "live-results"
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	client := &Client{URL: endpoint, HTTP: &http.Client{}}
	stageBudget := 120 * time.Second
	if raw := os.Getenv("GEMMA_STAGE_BUDGET_SECONDS"); raw != "" {
		seconds, err := time.ParseDuration(raw + "s")
		if err != nil {
			t.Fatalf("invalid GEMMA_STAGE_BUDGET_SECONDS: %v", err)
		}
		stageBudget = seconds
	}

	// --- Step 1: diagnose. Independent navigation, no edit/run_tests, no source preload. ---
	root1 := t.TempDir()
	report1 := filepath.Join(t.TempDir(), "oracle1.json")
	task1, _, _ := prepareRepo(t, pilot, stage, root1, report1)
	diagnosePrompt := task1.Report + "\n\nThis is a diagnosis-only task: do not attempt a fix. " +
		"Investigate using read and search. When you have found the root cause, reply in plain " +
		"text naming the exact file and function, and explain precisely why it produces the " +
		"wrong output. Do not propose replacement code."

	root1Opened, err := os.OpenRoot(root1)
	if err != nil {
		t.Fatal(err)
	}
	defer root1Opened.Close()
	diagnoseTrace, err := os.Create(filepath.Join(output, "twostep-"+stage+"-diagnose-trace.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer diagnoseTrace.Close()
	trace1 := &Trace{Writer: diagnoseTrace, Progress: os.Stderr}
	tools1 := &Tools{Root: root1Opened, ToolTimeout: stageBudget, Trace: trace1,
		Edited: map[string]bool{}, SearchEnabled: true}
	cfg1 := Config{Model: model, MaxTurns: 16, MaxTokens: 3072, Temperature: 0, MaxHistoryBytes: 64 << 10, RecoverGemma: true}
	ctx1, cancel1 := context.WithTimeout(context.Background(), stageBudget)
	defer cancel1()
	result1 := runAgent(ctx1, cfg1, diagnosePrompt, client, tools1, trace1)
	t.Logf("diagnose step: status=%s turns=%d tool_calls=%d edited=%v", result1.Status, result1.Turns, result1.ToolCalls, result1.EditedFiles)
	if len(result1.EditedFiles) != 0 {
		t.Errorf("diagnose step edited files despite instruction: %v", result1.EditedFiles)
	}
	if result1.Answer == "" {
		t.Fatalf("diagnose step produced no answer to hand to step 2 (status=%s error=%s)", result1.Status, result1.Error)
	}
	t.Logf("self-diagnosis:\n%s", result1.Answer)

	// --- Step 2: implement. Fresh state, source-supplied + edit-only, fed step 1's own text. ---
	root2 := t.TempDir()
	report2 := filepath.Join(output, "twostep-"+stage+"-oracle.json")
	os.Setenv("GEMMA_PRELOAD_SOURCES", "1")
	task2, command2, before2 := prepareRepo(t, pilot, stage, root2, report2)
	os.Unsetenv("GEMMA_PRELOAD_SOURCES")
	task2.Report += "\n\nSelf-diagnosis from an earlier independent investigation pass by this " +
		"same model (not evaluator-authored; verify it by reading the supplied source and testing, " +
		"not by trusting it alone):\n" + result1.Answer

	root2Opened, err := os.OpenRoot(root2)
	if err != nil {
		t.Fatal(err)
	}
	defer root2Opened.Close()
	implementTrace, err := os.Create(filepath.Join(output, "twostep-"+stage+"-implement-trace.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer implementTrace.Close()
	trace2 := &Trace{Writer: implementTrace, Progress: os.Stderr}
	tools2 := &Tools{Root: root2Opened, TestCommand: command2, ToolTimeout: stageBudget, Trace: trace2,
		Edited: map[string]bool{}, ReadDisabled: true}
	cfg2 := Config{Model: model, MaxTurns: 16, MaxTokens: 3072, Temperature: 0, MaxHistoryBytes: 64 << 10, RecoverGemma: true}
	ctx2, cancel2 := context.WithTimeout(context.Background(), stageBudget)
	defer cancel2()
	result2 := runAgent(ctx2, cfg2, task2.Report, client, tools2, trace2)
	t.Logf("implement step: status=%s turns=%d tool_calls=%d edited=%v", result2.Status, result2.Turns, result2.ToolCalls, result2.EditedFiles)

	after2, err := repoSnapshot(root2)
	if err != nil {
		t.Fatal(err)
	}
	changeErr := checkRepoChanges(stage, before2, after2)
	fixed := changeErr == nil
	reportErr := checkRepoReport(report2, task2, fixed)
	t.Logf("oracle: changes=%v report=%v (fixed=%v)", changeErr, reportErr, fixed)
	if fixed && reportErr == nil {
		t.Logf("VERIFIED FIX via two-step diagnose-then-implement")
	} else {
		t.Logf("not a verified fix this attempt: changes=%v report=%v", changeErr, reportErr)
	}
}
