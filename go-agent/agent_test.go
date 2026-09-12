package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func call(name, args string) ToolCall {
	return ToolCall{ID: "call-1", Type: "function", Function: FunctionCall{Name: name, Arguments: args}}
}

func reply(w http.ResponseWriter, message Message, reason string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": reason}}})
}

func newTools(t *testing.T) (*Tools, *bytes.Buffer) {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	trace := &bytes.Buffer{}
	return &Tools{Root: root, ToolTimeout: time.Second, Trace: &Trace{Writer: trace}, Edited: map[string]bool{}}, trace
}

func config() Config {
	return Config{Model: "gemma4", MaxTurns: 8, MaxTokens: 3072, MaxHistoryBytes: 64 << 10, RecoverToolCalls: true}
}

func TestCLIEndToEnd(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "work")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sum.txt"), []byte("wrong"), 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != "POST" {
			t.Errorf("bad endpoint: %s %s", r.Method, r.URL.Path)
		}
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.ToolChoice != "auto" || request.Parallel || request.Stream || request.MaxTokens != 8192 {
			t.Errorf("unexpected request settings: %+v", request)
		}
		for _, tool := range request.Tools {
			if tool.Function.Name == "done" {
				t.Error("must not register done")
			}
		}
		requests++
		if requests == 2 {
			last := request.Messages[len(request.Messages)-1]
			if last.Role != "tool" || last.ToolCallID != "call-1" {
				t.Errorf("missing tool result linkage: %+v", last)
			}
		}
		switch requests {
		case 1:
			reply(w, Message{ToolCalls: []ToolCall{call("read", `{"path":"sum.txt"}`)}}, "tool_calls")
		case 2:
			reply(w, Message{ToolCalls: []ToolCall{call("edit", `{"path":"sum.txt","oldText":"wrong","newText":"right"}`)}}, "tool_calls")
		case 3:
			reply(w, Message{Content: "Fixed and tested."}, "stop")
		default:
			t.Error("unexpected extra request")
			http.Error(w, "unexpected", 500)
		}
	}))
	defer server.Close()
	var stdout, stderr bytes.Buffer
	exit := cli(context.Background(), []string{"--root", root, "--output", filepath.Join(dir, "runs"), "--endpoint", server.URL + "/v1",
		"--prompt", "Fix the sum fixture", "--test-command", `["/bin/sh","-c","test \"$(cat sum.txt)\" = right"]`, "--timeout", "5s"}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit %d: %s\n%s", exit, stdout.String(), stderr.String())
	}
	var summary Summary
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Status != "completed" || summary.Turns != 3 || summary.ToolCalls != 2 || summary.Verification == nil || summary.Verification.ExitCode != 0 {
		t.Fatalf("summary: %+v", summary)
	}
	data, _ := os.ReadFile(filepath.Join(root, "sum.txt"))
	if string(data) != "right" {
		t.Fatalf("edit not applied: %s", data)
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "runs", "*", "trace.jsonl"))
	if len(paths) != 1 {
		t.Fatalf("traces: %v", paths)
	}
	trace, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range bytes.Split(bytes.TrimSpace(trace), []byte("\n")) {
		if !json.Valid(line) {
			t.Fatalf("invalid trace line: %s", line)
		}
	}
	if !bytes.Contains(trace, []byte(`"before":"wrong"`)) || !bytes.Contains(trace, []byte(`"type":"summary"`)) {
		t.Fatalf("missing replay evidence: %s", trace)
	}
}

func TestRecovery(t *testing.T) {
	span := `<|tool_call>:read{path:<|"|>a.ts<|"|>}<tool_call|>`
	for _, tc := range []struct {
		name, input string
		want        string
		bad         bool
	}{
		{"missing_call", span, `{"path":"a.ts"}`, false},
		{"canonical", `<|tool_call>call:read{path:<|"|>a.ts<|"|>}<tool_call|>`, `{"path":"a.ts"}`, false},
		{"repeated", strings.Repeat(span, 60), `{"path":"a.ts"}`, false},
		{"closer_in_string", `<|tool_call>:read{path:<|"|>literal <tool_call|><|"|>}<tool_call|>`, `{"path":"literal <tool_call|>"}`, false},
		{"nested", `<|tool_call>:read{meta:{values:[true,null,1e3,-2.5]}}<tool_call|>`, `{"meta":{"values":[true,null,1e3,-2.5]}}`, false},
		{"partial", `<|tool_call>:read{path:<|"|>a`, "", true},
		{"junk", `<|tool_call>:read{path:<|"|>a<|"|>}junk<tool_call|>`, "", true},
		{"conflicting", span + strings.Replace(span, "a.ts", "b.ts", 1), "", true},
		{"partial_after_complete", span + `<|tool_call>:read{`, "", true},
		{"duplicate_key", `<|tool_call>:read{path:1,path:2}<tool_call|>`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := recoverCall(tc.input)
			if tc.bad {
				if err == nil {
					t.Fatalf("expected rejection: %+v", got)
				}
				return
			}
			if err != nil || got == nil {
				t.Fatalf("got %+v %v", got, err)
			}
			var actual, expected any
			if err := json.Unmarshal([]byte(got.Function.Arguments), &actual); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.want), &expected); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("got %v, want %v", actual, expected)
			}
		})
	}
	if got, err := recoverCall("<tool_call|>"); got != nil || err != nil {
		t.Fatalf("closer-only is not a call: %+v %v", got, err)
	}
	if got, err := recoverCall(span, span); got == nil || err != nil {
		t.Fatal("cross-channel duplicate not recovered")
	}
}

func TestAgentFailureBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, finish, status string
		message              Message
		turns, calls         int
	}{
		{"truncated", "length", "truncated", Message{ToolCalls: []ToolCall{call("edit", `{"path":"a","oldText":"x","newText":"y"}`)}}, 1, 0},
		{"empty", "stop", "empty_completion", Message{}, 1, 0},
		{"multiple", "tool_calls", "stalled", Message{ToolCalls: []ToolCall{call("read", `{"path":"a"}`), call("read", `{"path":"b"}`)}}, 3, 2},
		{"malformed", "stop", "malformed_tool_call", Message{Content: "<tool_call|>"}, 2, 0},
		{"stalled", "tool_calls", "stalled", Message{ToolCalls: []ToolCall{call("read", `{"path":"a"}`)}}, 3, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tools, trace := newTools(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reply(w, tc.message, tc.finish) }))
			defer server.Close()
			result := runAgent(context.Background(), config(), "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
			if result.Status != tc.status || result.Turns != tc.turns || result.ToolCalls != tc.calls {
				t.Fatalf("got %+v", result)
			}
		})
	}
}

func TestRepeatWarningAllowsProgress(t *testing.T) {
	tools, trace := newTools(t)
	if err := os.WriteFile(filepath.Join(tools.Root.Name(), "a"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		switch requests {
		case 1, 2:
			if requests == 2 {
				last := request.Messages[len(request.Messages)-1]
				if last.Role != "tool" || !strings.HasPrefix(last.Content, "File: a\n") || !strings.HasSuffix(last.Content, "\n\nx") {
					t.Errorf("read result is not literal source: %q", last.Content)
				}
			}
			reply(w, Message{ToolCalls: []ToolCall{call("read", `{"path":"a"}`)}}, "tool_calls")
		case 3:
			last := request.Messages[len(request.Messages)-1]
			if last.Role != "user" || !strings.Contains(last.Content, "You repeated the identical tool request") {
				t.Error("missing repeat warning")
			}
			// The repeated tool is banned for exactly this one turn, not merely
			// discouraged: the model cannot re-request "read" here even if it ignores
			// the correction message, because it is not in the declared tool set.
			for _, tool := range request.Tools {
				if tool.Function.Name == "read" {
					t.Error("repeated tool must be excluded from this turn's tool list")
				}
			}
			reply(w, Message{ToolCalls: []ToolCall{call("edit", `{"path":"a","oldText":"x","newText":"y"}`)}}, "tool_calls")
		default:
			reply(w, Message{Content: "Fixed"}, "stop")
		}
	}))
	defer server.Close()
	result := runAgent(context.Background(), config(), "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "completed" || result.ToolCalls != 3 || len(result.EditedFiles) != 1 {
		t.Fatalf("got %+v", result)
	}
	if strings.Count(trace.String(), `"reason":"repeated tool request"`) != 1 {
		t.Fatal("repeat correction not recorded once")
	}
	if !strings.Contains(trace.String(), `"tool_banned_for_turn"`) || !strings.Contains(trace.String(), `"tool":"read"`) {
		t.Fatal("expected read to be traced as banned for one turn")
	}
}

// TestBannedToolClearsAfterOneTurn confirms the exclusion is scoped to exactly the
// turn immediately following the second identical call, not the rest of the run: a
// later, unrelated repeat of the same tool name must be allowed to execute again.
func TestBannedToolClearsAfterOneTurn(t *testing.T) {
	tools, trace := newTools(t)
	if err := os.WriteFile(filepath.Join(tools.Root.Name(), "a"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		switch requests {
		case 1, 2:
			reply(w, Message{ToolCalls: []ToolCall{call("read", `{"path":"a"}`)}}, "tool_calls")
		case 3:
			for _, tool := range request.Tools {
				if tool.Function.Name == "read" {
					t.Error("read must be banned on the turn right after the 2nd identical call")
				}
			}
			reply(w, Message{ToolCalls: []ToolCall{call("edit", `{"path":"a","oldText":"x","newText":"y"}`)}}, "tool_calls")
		case 4:
			found := false
			for _, tool := range request.Tools {
				if tool.Function.Name == "read" {
					found = true
				}
			}
			if !found {
				t.Error("read ban must not persist past the one turn it applied to")
			}
			reply(w, Message{ToolCalls: []ToolCall{call("read", `{"path":"a"}`)}}, "tool_calls")
		default:
			reply(w, Message{Content: "done"}, "stop")
		}
	}))
	defer server.Close()
	result := runAgent(context.Background(), config(), "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "completed" {
		t.Fatalf("got %+v", result)
	}
}

func TestPromptProfiles(t *testing.T) {
	for _, profile := range []string{"baseline", "focused", "test-first"} {
		t.Run(profile, func(t *testing.T) {
			tools, trace := newTools(t)
			cfg := config()
			cfg.PromptProfile = profile
			cfg.Instructions = "EXPLICIT_PROJECT_INSTRUCTIONS"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request Request
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				want := systemPrompt
				if profile == "focused" {
					want = focusedPrompt
				}
				if profile == "test-first" {
					want = testFirstPrompt
				}
				if request.Messages[0].Content != want+"\nEXPLICIT_PROJECT_INSTRUCTIONS" {
					t.Error("incorrect initial prompt")
				}
				if request.Messages[1].Content != "USER_REPORT" {
					t.Error("task report changed")
				}
				reply(w, Message{Content: "Done"}, "stop")
			}))
			defer server.Close()
			result := runAgent(context.Background(), cfg, "USER_REPORT", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
			if result.Status != "completed" {
				t.Fatalf("%+v", result)
			}
		})
	}
}

func TestDeduplicatedReadSeesEdits(t *testing.T) {
	tools, trace := newTools(t)
	if err := os.WriteFile(filepath.Join(tools.Root.Name(), "a"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config()
	cfg.DedupReads = true
	turn := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turn++
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		lastTool := ""
		for _, message := range request.Messages {
			if message.Role == "tool" {
				lastTool = message.Content
			}
		}
		switch turn {
		case 1:
			reply(w, Message{ToolCalls: []ToolCall{call("read", `{"path":"a"}`)}}, "tool_calls")
		case 2:
			reply(w, Message{ToolCalls: []ToolCall{call("read", `{"path":"a","offset":1}`)}}, "tool_calls")
		case 3:
			if !strings.HasPrefix(lastTool, "Unchanged read.") {
				t.Errorf("duplicate content not suppressed: %q", lastTool)
			}
			if request.Messages[len(request.Messages)-1].Role != "user" {
				t.Error("equivalent offsets did not trigger repeat warning")
			}
			reply(w, Message{ToolCalls: []ToolCall{call("edit", `{"path":"a","oldText":"x","newText":"y"}`)}}, "tool_calls")
		case 4:
			reply(w, Message{ToolCalls: []ToolCall{call("read", `{"path":"a"}`)}}, "tool_calls")
		default:
			if !strings.HasSuffix(lastTool, "\n\ny") {
				t.Errorf("stale read after edit: %q", lastTool)
			}
			reply(w, Message{Content: "Done"}, "stop")
		}
	}))
	defer server.Close()
	result := runAgent(context.Background(), cfg, "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "completed" || result.ToolCalls != 4 {
		t.Fatalf("%+v", result)
	}
	if strings.Count(trace.String(), `"type":"read_deduplicated"`) != 1 {
		t.Fatal("dedup event missing or repeated")
	}
}

// TestCompletionVerificationRetry exercises the retry loop: the model declares done with
// a wrong fix, the harness's own end-of-run test catches it and hands the failure back
// instead of ending the run, and the model gets a bounded chance to actually fix it.
func TestCompletionVerificationRetry(t *testing.T) {
	tools, trace := newTools(t)
	if err := os.WriteFile(filepath.Join(tools.Root.Name(), "a"), []byte("wrong"), 0600); err != nil {
		t.Fatal(err)
	}
	tools.TestCommand = []string{"/bin/sh", "-c", `test "$(cat a)" = right`}
	cfg := config()
	turn := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turn++
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		switch turn {
		case 1:
			reply(w, Message{ToolCalls: []ToolCall{call("edit", `{"path":"a","oldText":"wrong","newText":"still-wrong"}`)}}, "tool_calls")
		case 2:
			reply(w, Message{Content: "Fixed."}, "stop")
		case 3:
			last := request.Messages[len(request.Messages)-1]
			if last.Role != "user" || !strings.Contains(last.Content, "exit_code=1") {
				t.Fatalf("missing completion-failure message: %+v", last)
			}
			reply(w, Message{ToolCalls: []ToolCall{call("edit", `{"path":"a","oldText":"still-wrong","newText":"right"}`)}}, "tool_calls")
		default:
			reply(w, Message{Content: "Fixed and verified."}, "stop")
		}
	}))
	defer server.Close()
	result := runAgent(context.Background(), cfg, "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "completed" || result.ToolCalls != 2 || turn != 4 {
		t.Fatalf("%+v turn=%d", result, turn)
	}
	if !strings.Contains(trace.String(), `"type":"completion_verification_failed"`) {
		t.Fatal("auto_test event not traced")
	}
}

func TestTaskReminder(t *testing.T) {
	tools, trace := newTools(t)
	if err := os.WriteFile(filepath.Join(tools.Root.Name(), "a"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config()
	cfg.TaskReminder = true
	turn := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turn++
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if turn == 1 {
			reply(w, Message{ToolCalls: []ToolCall{call("read", `{"path":"a"}`)}}, "tool_calls")
			return
		}
		last := request.Messages[len(request.Messages)-1]
		if last.Role != "user" || !strings.HasSuffix(last.Content, "Original task:\nORIGINAL_TASK") {
			t.Error("missing exact task reminder")
		}
		if request.Messages[len(request.Messages)-2].Role != "tool" {
			t.Error("reminder replaced the tool result")
		}
		reply(w, Message{Content: "done"}, "stop")
	}))
	defer server.Close()
	result := runAgent(context.Background(), cfg, "ORIGINAL_TASK", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "completed" || result.ToolCalls != 1 {
		t.Fatalf("%+v", result)
	}
	if !strings.Contains(trace.String(), `"type":"task_reminder"`) {
		t.Fatal("reminder not traced")
	}
}

func TestEditOnlyDiagnostic(t *testing.T) {
	tools, trace := newTools(t)
	tools.ReadDisabled = true
	if err := os.WriteFile(filepath.Join(tools.Root.Name(), "a"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := tools.Execute(context.Background(), call("read", `{"path":"a"}`)); err == nil {
		t.Fatal("read was not disabled")
	}
	tools.SearchEnabled = true
	if _, err := tools.Execute(context.Background(), call("search", `{"path":"a","text":"x"}`)); err == nil {
		t.Fatal("search was not disabled")
	}
	turn := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turn++
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if len(request.Tools) != 1 || request.Tools[0].Function.Name != "edit" {
			t.Error("read/search exposed in repair mode")
		}
		if turn == 1 {
			reply(w, Message{ToolCalls: []ToolCall{call("edit", `{"path":"a","oldText":"x","newText":"y"}`)}}, "tool_calls")
		} else {
			reply(w, Message{Content: "done"}, "stop")
		}
	}))
	defer server.Close()
	result := runAgent(context.Background(), config(), "Current a contains x. Replace x with y.", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "completed" || len(result.EditedFiles) != 1 {
		t.Fatalf("%+v", result)
	}
}

func TestToolReasoningHistory(t *testing.T) {
	for _, tc := range []struct {
		name, reasoning, want string
		preserve              bool
	}{
		{"default", "Inspect the source.", "", false},
		{"preserved", "Inspect the source.", "Inspect the source.", true},
		{"leaked", "<|tool_call>:read{path:a}<tool_call|>", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tools, trace := newTools(t)
			if err := os.WriteFile(filepath.Join(tools.Root.Name(), "a"), []byte("x"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg := config()
			cfg.PreserveToolReasoning = tc.preserve
			turn := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				turn++
				var request Request
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if turn == 1 {
					reply(w, Message{Reasoning: tc.reasoning, ToolCalls: []ToolCall{call("read", `{"path":"a"}`)}}, "tool_calls")
					return
				}
				if request.Messages[2].Reasoning != tc.want {
					t.Errorf("reasoning got %q", request.Messages[2].Reasoning)
				}
				reply(w, Message{Content: "done"}, "stop")
			}))
			defer server.Close()
			result := runAgent(context.Background(), cfg, "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
			if result.Status != "completed" {
				t.Fatalf("%+v", result)
			}
		})
	}
}

func TestRecoveredCallExecutesOnce(t *testing.T) {
	tools, trace := newTools(t)
	if err := os.WriteFile(filepath.Join(tools.Root.Name(), "a"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			span := `<|tool_call>:edit{path:<|"|>a<|"|>,oldText:<|"|>x<|"|>,newText:<|"|>y<|"|>}<tool_call|>`
			reply(w, Message{Content: strings.Repeat(span, 60), Reasoning: span}, "stop")
		} else {
			reply(w, Message{Content: "Finished"}, "stop")
		}
	}))
	defer server.Close()
	result := runAgent(context.Background(), config(), "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "completed" || result.Recoveries != 1 || result.ToolCalls != 1 {
		t.Fatalf("got %+v", result)
	}
	content, _ := os.ReadFile(filepath.Join(tools.Root.Name(), "a"))
	if string(content) != "y" {
		t.Fatalf("content %s", content)
	}
}

func TestObservedGemma4AgentRegressions(t *testing.T) {
	for _, fixture := range regressions(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			tools, trace := newTools(t)
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if fixture.RecoveredName == "" || requests == 1 {
					content := fixture.Content
					if fixture.Finish == "length" {
						content = strings.Repeat(content, 60)
					}
					reply(w, Message{Content: content, Reasoning: fixture.Reasoning}, fixture.Finish)
				} else {
					reply(w, Message{Content: "Finished"}, "stop")
				}
			}))
			defer server.Close()
			result := runAgent(context.Background(), config(), "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
			switch {
			case fixture.Finish == "length":
				if result.Status != "truncated" || result.ToolCalls != 0 {
					t.Fatalf("executed from truncated output: %+v", result)
				}
			case fixture.RecoveredName == "":
				if result.Status != "malformed_tool_call" || result.ToolCalls != 0 {
					t.Fatalf("invented call from closer: %+v", result)
				}
			default:
				if result.Recoveries != 1 || !strings.Contains(trace.String(), "unknown tool") {
					t.Fatalf("unexpected done execution: %+v", result)
				}
			}
		})
	}
}

func TestHTTPTimeout(t *testing.T) {
	tools, trace := newTools(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the request before waiting so the server observes connection cancellation.
		var request Request
		_ = json.NewDecoder(r.Body).Decode(&request)
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := runAgent(ctx, config(), "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "timeout" || result.WallMS > 1000 {
		t.Fatalf("got %+v", result)
	}
}

func TestLimitsBeforeRequest(t *testing.T) {
	tools, trace := newTools(t)
	cfg := config()
	cfg.MaxHistoryBytes = 1
	result := runAgent(context.Background(), cfg, "task", nil, tools, &Trace{Writer: trace})
	if result.Status != "context_limit" || result.Turns != 0 {
		t.Fatalf("got %+v", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = runAgent(ctx, config(), "task", nil, tools, &Trace{Writer: trace})
	if result.Status != "cancelled" {
		t.Fatalf("got %+v", result)
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestTraceFailurePreventsRequest(t *testing.T) {
	tools, _ := newTools(t)
	result := runAgent(context.Background(), config(), "task", nil, tools, &Trace{Writer: brokenWriter{}})
	if result.Status != "trace_error" || result.Turns != 0 {
		t.Fatalf("got %+v", result)
	}
}

func TestHTTPProtocolErrors(t *testing.T) {
	for _, body := range []string{`not json`, `{}`, `{"choices":[]}`, `{"choices":[{},{}]}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			client := Client{URL: server.URL, HTTP: server.Client()}
			if _, err := client.Complete(context.Background(), Request{}); err == nil {
				t.Fatal("expected protocol error")
			}
		})
	}
}
