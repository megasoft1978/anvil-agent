package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFileTools(t *testing.T) {
	tools, trace := newTools(t)
	path := filepath.Join(tools.Root.Name(), "code.txt")
	if err := os.WriteFile(path, []byte("before before"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range []string{
		`{"path":"code.txt","oldText":"before","newText":"after"}`,
		`{"path":"code.txt","oldText":"missing","newText":"after"}`,
		`{"path":"code.txt","oldText":"before before"}`,
		`{"path":"code.txt","oldText":"before before","newText":"after","extra":true}`,
		`{"path":"code.txt","oldText":"","newText":"after"}`,
	} {
		if _, err := tools.Execute(context.Background(), call("edit", args)); err == nil {
			t.Fatalf("accepted bad edit: %s", args)
		}
	}
	if _, err := tools.Execute(context.Background(), call("edit", `{"path":"code.txt","oldText":"before before","newText":"after"}`)); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "after" {
		t.Fatal(string(data))
	}
	if !strings.Contains(trace.String(), `"before":"before before"`) {
		t.Fatal("missing backup")
	}
	if _, err := tools.Execute(context.Background(), call("read", `{"path":"."}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := tools.Execute(context.Background(), call("read", `{"path":"code.txt"} {}`)); err == nil {
		t.Fatal("trailing arguments accepted")
	}
	if _, err := tools.Execute(context.Background(), call("bash", `{"command":"echo nope"}`)); err == nil {
		t.Fatal("unknown tool accepted")
	}
}

func TestBashIsExplicitlyGatedAndBounded(t *testing.T) {
	tools, _ := newTools(t)
	tools.Worktree = tools.Root.Name()
	if _, err := tools.Execute(context.Background(), call("bash", `{"command":"printf 'disabled'"}`)); err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("default Bash gate changed: %v", err)
	}

	tools.BashMode = "guarded"
	t.Setenv("ANVIL_API_KEY", "must-not-leak")
	value, err := tools.Execute(context.Background(), call("bash", `{"command":"printf '%s|%s' \"$ANVIL_API_KEY\" \"$PWD\""}`))
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	output, _ := result["output"].(string)
	if result["exit_code"] != 0 || !strings.HasPrefix(output, "|") || strings.Contains(output, "must-not-leak") {
		t.Fatalf("unexpected guarded Bash result: %+v", result)
	}
	if _, err := tools.Execute(context.Background(), call("bash", `{"command":"rm -f should-not-exist"}`)); err == nil {
		t.Fatal("destructive Bash command accepted")
	}

	tools.BashMode = "only"
	if _, err := tools.Execute(context.Background(), call("read", `{"path":"."}`)); err == nil || !strings.Contains(err.Error(), "bash-only") {
		t.Fatalf("native tool was accepted in bash-only mode: %v", err)
	}
}

func TestRichEditFeedback(t *testing.T) {
	tools, _ := newTools(t)
	tools.RichEditFeedback = true
	path := filepath.Join(tools.Root.Name(), "code.txt")
	content := strings.Repeat("pad\n", 20) + "function ins() {\n  let ins = this\n}\n" + strings.Repeat("pad\n", 20)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// Not found: no error, but a reason and a current_context snippet instead.
	value, err := tools.Execute(context.Background(), call("edit", `{"path":"code.txt","oldText":"nonexistent snippet","newText":"x"}`))
	if err != nil {
		t.Fatalf("not-found should not be a Go error under RichEditFeedback: %v", err)
	}
	result := value.(map[string]any)
	if result["applied"] != false || result["reason"] != "oldText was not found in the current file" {
		t.Fatalf("unexpected not-found result: %+v", result)
	}
	if _, ok := result["current_context"].(string); !ok {
		t.Fatalf("missing current_context: %+v", result)
	}

	// Not found, but with a stale first line (e.g. after an earlier edit changed it): the
	// fallback must still locate the block via a later line, not fall back to the file start.
	value, err = tools.Execute(context.Background(), call("edit", `{"path":"code.txt","oldText":"function renamed_elsewhere() {\n  let ins = this\n}","newText":"x"}`))
	if err != nil {
		t.Fatalf("stale-first-line not-found should not be a Go error: %v", err)
	}
	result = value.(map[string]any)
	staleContext := result["current_context"].(string)
	if result["applied"] != false || !strings.Contains(staleContext, "let ins = this") {
		t.Fatalf("fallback did not locate the block via a later line: %+v", result)
	}

	// Ambiguous: both "pad" lines duplicated many times.
	value, err = tools.Execute(context.Background(), call("edit", `{"path":"code.txt","oldText":"pad","newText":"x"}`))
	if err != nil {
		t.Fatalf("ambiguous should not be a Go error under RichEditFeedback: %v", err)
	}
	result = value.(map[string]any)
	if result["applied"] != false || !strings.Contains(result["reason"].(string), "occurs") {
		t.Fatalf("unexpected ambiguous result: %+v", result)
	}

	// No-op: oldText found once, but newText identical to oldText.
	value, err = tools.Execute(context.Background(), call("edit", `{"path":"code.txt","oldText":"let ins = this","newText":"let ins = this"}`))
	if err != nil {
		t.Fatalf("no-op should not be a Go error under RichEditFeedback: %v", err)
	}
	result = value.(map[string]any)
	if result["applied"] != false || result["reason"] != "newText produced no change to the file" {
		t.Fatalf("unexpected no-op result: %+v", result)
	}
	snippet := result["current_context"].(string)
	if !strings.Contains(snippet, "let ins = this") {
		t.Fatalf("current_context missing the edit site: %q", snippet)
	}

	// Success: current_context reflects the new text, not the old.
	value, err = tools.Execute(context.Background(), call("edit", `{"path":"code.txt","oldText":"let ins = this","newText":"const ins = this"}`))
	if err != nil {
		t.Fatal(err)
	}
	result = value.(map[string]any)
	if result["applied"] != true {
		t.Fatalf("expected applied edit: %+v", result)
	}
	snippet = result["current_context"].(string)
	if !strings.Contains(snippet, "const ins = this") || strings.Contains(snippet, "let ins = this") {
		t.Fatalf("current_context does not reflect the edit: %q", snippet)
	}
}

func TestPathEscapes(t *testing.T) {
	tools, _ := newTools(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tools.Root.Name(), "link")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{outside, "../outside", "link", ".git/config"} {
		args, _ := json.Marshal(map[string]any{"path": path})
		if _, err := tools.Execute(context.Background(), call("read", string(args))); err == nil {
			t.Fatalf("read escaped: %s", path)
		}
		args, _ = json.Marshal(map[string]any{"path": path, "oldText": "untouched", "newText": "bad"})
		if _, err := tools.Execute(context.Background(), call("edit", string(args))); err == nil {
			t.Fatalf("edit escaped: %s", path)
		}
	}
	data, _ := os.ReadFile(outside)
	if string(data) != "untouched" {
		t.Fatal("outside file modified")
	}
}

func TestBackupFailurePreventsEdit(t *testing.T) {
	tools, _ := newTools(t)
	tools.Trace.Writer = brokenWriter{}
	path := filepath.Join(tools.Root.Name(), "a")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := tools.Execute(context.Background(), call("edit", `{"path":"a","oldText":"before","newText":"after"}`)); err == nil {
		t.Fatal("accepted failed backup")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "before" {
		t.Fatal("modified without backup")
	}
}

func TestReadPaginationAndBinary(t *testing.T) {
	tools, _ := newTools(t)
	path := filepath.Join(tools.Root.Name(), "a")
	if err := os.WriteFile(path, []byte(strings.Repeat("line\n", 300)), 0600); err != nil {
		t.Fatal(err)
	}
	value, err := tools.Execute(context.Background(), call("read", `{"path":"a"}`))
	if err != nil {
		t.Fatal(err)
	}
	if value.(map[string]any)["next_offset"] != 201 {
		t.Fatalf("pagination: %+v", value)
	}
	if err := os.WriteFile(path, []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := tools.Execute(context.Background(), call("read", `{"path":"a"}`)); err == nil {
		t.Fatal("binary accepted")
	}
}

func TestCommandDeadlineAndOutputCap(t *testing.T) {
	result, err := runCommand(context.Background(), t.TempDir(), []string{"/bin/sh", "-c", "sleep 30 & wait"}, 80*time.Millisecond)
	if err != nil || !result.TimedOut || result.WallMS > 1500 {
		t.Fatalf("timeout: %+v %v", result, err)
	}
	result, err = runCommand(context.Background(), t.TempDir(), []string{"/bin/sh", "-c", "yes output | head -c 100000"}, 2*time.Second)
	if err != nil || !result.Truncated || len(result.Output) != maxOutputBytes {
		t.Fatalf("output cap: %d bytes %+v %v", len(result.Output), result, err)
	}
}

func TestCommandKillsDescendants(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	result, err := runCommand(context.Background(), dir, []string{"/bin/sh", "-c", "sleep 30 & echo $! > child.pid; wait"}, 150*time.Millisecond)
	if err != nil || !result.TimedOut {
		t.Fatalf("result: %+v %v", result, err)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	// A zombie is already dead but may await reaping by init in a container. ps distinguishes it.
	check, err := runCommand(context.Background(), dir, []string{"ps", "-o", "stat=", "-p", strconv.Itoa(pid)}, time.Second)
	if err == nil && check.ExitCode == 0 && !strings.HasPrefix(strings.TrimSpace(check.Output), "Z") {
		t.Fatalf("child survived deadline: %s", check.Output)
	}
}
