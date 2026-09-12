package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestArgumentContracts(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `"x"`, `{`, `{} trailing`, `{"path":1}`, `{"path":"a","unexpected":1}`, `{"path":"a","offset":1.2}`, `{"path":"a","offset":0}`, `{"path":"a","offset":-1}`} {
		t.Run(raw, func(t *testing.T) {
			tools, _ := newTools(t)
			if _, err := tools.Execute(context.Background(), call("read", raw)); err == nil {
				t.Fatal("invalid arguments accepted")
			}
		})
	}
}

func TestStrictArgumentNamesAndDuplicates(t *testing.T) {
	for _, raw := range []string{`{"path":"a","path":"b"}`, `{"Path":"a"}`, `{"path":null}`, `{"offset":null}`, `{"offset":1,"offset":2}`} {
		var args struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
		}
		if err := decodeArguments(raw, &args); err == nil {
			t.Fatalf("ambiguous arguments accepted: %s", raw)
		}
	}
}

func TestModelCannotChooseTestCommand(t *testing.T) {
	tools, _ := newTools(t)
	tools.TestCommand = []string{"/bin/sh", "-c", "touch allowed"}
	if _, err := tools.Execute(context.Background(), call("run_tests", `{"command":"touch injected"}`)); err == nil {
		t.Fatal("model-supplied command accepted")
	}
	if _, err := os.Stat(filepath.Join(tools.Root.Name(), "allowed")); !os.IsNotExist(err) {
		t.Fatal("command ran despite invalid arguments")
	}
	if _, err := tools.Execute(context.Background(), call("run_tests", `{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tools.Root.Name(), "allowed")); err != nil {
		t.Fatal("fixed command did not run")
	}
}

func TestFileLimitsAndSpecialFiles(t *testing.T) {
	tools, _ := newTools(t)
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"large", bytes.Repeat([]byte("a"), maxFileBytes+1)},
		{"long-line", bytes.Repeat([]byte("a"), maxOutputBytes+1)},
		{"invalid-utf8", []byte{0xff}},
	} {
		if err := os.WriteFile(filepath.Join(tools.Root.Name(), tc.name), tc.data, 0600); err != nil {
			t.Fatal(err)
		}
		args, _ := json.Marshal(map[string]string{"path": tc.name})
		if _, err := tools.Execute(context.Background(), call("read", string(args))); err == nil {
			t.Fatalf("accepted %s", tc.name)
		}
	}
	pipe := filepath.Join(tools.Root.Name(), "pipe")
	if err := syscall.Mkfifo(pipe, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"read", "edit"} {
		args := `{"path":"pipe"}`
		if name == "edit" {
			args = `{"path":"pipe","oldText":"a","newText":"b"}`
		}
		start := time.Now()
		if _, err := tools.Execute(context.Background(), call(name, args)); err == nil {
			t.Fatal("FIFO accepted")
		}
		if time.Since(start) > time.Second {
			t.Fatal("special file blocked tool execution")
		}
	}
}

func TestNativeCallWinsOverLeakedText(t *testing.T) {
	tools, trace := newTools(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			reply(w, Message{Content: `<|tool_call>:edit{path:<|"|>x<|"|>,oldText:<|"|>a<|"|>,newText:<|"|>b<|"|>}<tool_call|>`, ToolCalls: []ToolCall{call("read", `{"path":"."}`)}}, "tool_calls")
		} else {
			reply(w, Message{Content: "Finished"}, "stop")
		}
	}))
	defer server.Close()
	result := runAgent(context.Background(), config(), "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "completed" || result.Recoveries != 0 || result.ToolCalls != 1 || len(result.EditedFiles) != 0 {
		t.Fatalf("got %+v", result)
	}
}

func TestToolErrorCanBeCorrected(t *testing.T) {
	tools, trace := newTools(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request Request
		_ = json.NewDecoder(r.Body).Decode(&request)
		switch requests {
		case 1:
			reply(w, Message{ToolCalls: []ToolCall{call("bash", `{"command":"pwd"}`)}}, "tool_calls")
		case 2:
			if !strings.Contains(request.Messages[len(request.Messages)-1].Content, "unknown tool") {
				t.Error("error not returned to model")
			}
			reply(w, Message{ToolCalls: []ToolCall{call("read", `{"path":"."}`)}}, "tool_calls")
		default:
			reply(w, Message{Content: "Finished"}, "stop")
		}
	}))
	defer server.Close()
	result := runAgent(context.Background(), config(), "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "completed" || result.ToolCalls != 2 {
		t.Fatalf("got %+v", result)
	}
}

func TestTurnLimitAndNativeOnlyMode(t *testing.T) {
	tools, trace := newTools(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply(w, Message{Content: `<|tool_call>:read{path:<|"|>.<|"|>}<tool_call|>`}, "stop")
	}))
	defer server.Close()
	cfg := config()
	cfg.MaxTurns = 1
	result := runAgent(context.Background(), cfg, "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "turn_limit" || result.ToolCalls != 1 {
		t.Fatalf("got %+v", result)
	}
	cfg = config()
	cfg.RecoverToolCalls = false
	result = runAgent(context.Background(), cfg, "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "malformed_tool_call" || result.ToolCalls != 0 {
		t.Fatalf("native-only got %+v", result)
	}
}

func TestVerificationOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		command      []string
	}{
		{"passes", "completed", []string{"/bin/sh", "-c", "exit 0"}},
		{"fails", "verification_failed", []string{"/bin/sh", "-c", "exit 1"}},
		{"missing", "verification_error", []string{"/nonexistent-gemma-test-command"}},
		{"tool-timeout", "verification_failed", []string{"/bin/sh", "-c", "sleep 10"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tools, trace := newTools(t)
			tools.TestCommand = tc.command
			tools.ToolTimeout = 50 * time.Millisecond
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reply(w, Message{Content: "Done"}, "stop") }))
			defer server.Close()
			result := runAgent(context.Background(), config(), "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
			if result.Status != tc.status {
				t.Fatalf("got %+v", result)
			}
		})
	}
}

func TestProtocolStatusAndBodyLimit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"rate-limit", 429, `{"error":"busy"}`},
		{"server-error", 500, `{"error":"failed"}`},
		{"too-large", 200, strings.Repeat(" ", (8<<20)+1)},
		{"array-content", 200, `{"choices":[{"message":{"content":[]},"finish_reason":"stop"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client := Client{URL: server.URL, HTTP: server.Client()}
			if _, err := client.Complete(context.Background(), Request{}); err == nil {
				t.Fatal("expected failure")
			}
			if requests != 1 {
				t.Fatal("unexpected hidden retry")
			}
		})
	}
}

func TestUnknownFinishReason(t *testing.T) {
	tools, trace := newTools(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reply(w, Message{Content: "blocked"}, "content_filter") }))
	defer server.Close()
	result := runAgent(context.Background(), config(), "task", &Client{URL: server.URL, HTTP: server.Client()}, tools, &Trace{Writer: trace})
	if result.Status != "protocol_error" || result.ToolCalls != 0 {
		t.Fatalf("got %+v", result)
	}
}
