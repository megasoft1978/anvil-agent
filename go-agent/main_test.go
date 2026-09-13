package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCLIInvalidInputs(t *testing.T) {
	for _, args := range [][]string{
		{}, {"--prompt", "a", "--prompt-file", "b"}, {"--prompt", " "}, {"--prompt", "a", "extra"},
		{"--prompt", "a", "--timeout", "0s"}, {"--prompt", "a", "--max-tokens", "0"},
		{"--prompt", "a", "--model", ""}, {"--prompt", "a", "--endpoint", "file:///tmp/model"},
		{"--prompt", "a", "--endpoint", "http://user:secret@localhost/v1"},
		{"--prompt", "a", "--endpoint", "http://localhost/v1?secret=x"},
		{"--prompt-file", "/nonexistent-prompt"},
		{"--prompt", "a", "--instructions", "/nonexistent-instructions"},
		{"--prompt", "a", "--root", "/nonexistent-worktree"}, {"--unknown"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errout bytes.Buffer
			if code := cli(context.Background(), args, &out, &errout); code != 2 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, out.String(), errout.String())
			}
		})
	}
	if code := cli(context.Background(), []string{"--help"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatal("help failed")
	}
}

func TestCLIPromptProvenanceAndPrivateArtifacts(t *testing.T) {
	t.Setenv("ANVIL_API_KEY", "fixture-secret-never-log")
	dir := t.TempDir()
	root := filepath.Join(dir, "work")
	output := filepath.Join(dir, "runs")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("ambient-instruction-do-not-load"), 0600); err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(dir, "task.json")
	if err := os.WriteFile(task, []byte(`{"id":"fixture","report":"task-description","fix_commit":"gold-do-not-send","test_files":{"gold":"hidden"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	instructions := filepath.Join(dir, "instructions.txt")
	if err := os.WriteFile(instructions, []byte("explicit-instructions"), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-secret-never-log" {
			t.Error("missing API authorization")
		}
		var request Request
		_ = json.NewDecoder(r.Body).Decode(&request)
		all, _ := json.Marshal(request)
		if bytes.Contains(all, []byte("gold-do-not-send")) || bytes.Contains(all, []byte("ambient-instruction")) {
			t.Errorf("unintended context: %s", all)
		}
		if !strings.Contains(request.Messages[0].Content, "explicit-instructions") || request.Messages[1].Content != "task-description" {
			t.Error("explicit context missing")
		}
		reply(w, Message{Content: "Finished"}, "stop")
	}))
	defer server.Close()
	for i := 0; i < 2; i++ {
		var out, errout bytes.Buffer
		code := cli(context.Background(), []string{"--root", root, "--output", output, "--task-file", task, "--instructions", instructions, "--endpoint", server.URL}, &out, &errout)
		if code != 0 {
			t.Fatalf("exit=%d %s %s", code, out.String(), errout.String())
		}
	}
	paths, _ := filepath.Glob(filepath.Join(output, "*", "trace.jsonl"))
	if len(paths) != 2 {
		t.Fatalf("attempts were overwritten: %v", paths)
	}
	for _, path := range paths {
		data, _ := os.ReadFile(path)
		if bytes.Contains(data, []byte("fixture-secret-never-log")) {
			t.Fatal("credential in trace")
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatal("trace readable by other users")
		}
		data, err = os.ReadFile(filepath.Join(filepath.Dir(path), "summary.json"))
		if err != nil || !json.Valid(data) {
			t.Fatal("missing valid summary")
		}
	}
}

func TestCLIRejectsArtifactsInsideWorktree(t *testing.T) {
	root := t.TempDir()
	var out, errout bytes.Buffer
	code := cli(context.Background(), []string{"--root", root, "--output", filepath.Join(root, "runs"), "--prompt", "task"}, &out, &errout)
	if code != 2 || !strings.Contains(errout.String(), "outside the worktree") {
		t.Fatalf("got %d %s", code, errout.String())
	}
}

func TestCLIExitDeadlineAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request Request
		_ = json.NewDecoder(r.Body).Decode(&request)
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer server.Close()
	for _, cancelled := range []bool{false, true} {
		dir := t.TempDir()
		root := filepath.Join(dir, "work")
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			cancel()
		}
		var out, errout bytes.Buffer
		code := cli(ctx, []string{"--root", root, "--output", filepath.Join(dir, "runs"), "--prompt", "task", "--endpoint", server.URL, "--timeout", "50ms"}, &out, &errout)
		cancel()
		want := 124
		if cancelled {
			want = 130
		}
		if code != want {
			t.Fatalf("exit=%d want=%d %s %s", code, want, out.String(), errout.String())
		}
	}
}

func TestCLIDoesNotFollowRedirects(t *testing.T) {
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls++; reply(w, Message{Content: "no"}, "stop") }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	dir := t.TempDir()
	root := filepath.Join(dir, "work")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	code := cli(context.Background(), []string{"--root", root, "--output", filepath.Join(dir, "runs"), "--prompt", "task", "--endpoint", server.URL}, &out, &errout)
	if code != 1 || targetCalls != 0 {
		t.Fatalf("redirect followed: code=%d calls=%d", code, targetCalls)
	}
}
