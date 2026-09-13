package main

import (
	"strings"
	"testing"
	"time"
)

// These test the tuiModel/traceEvent wiring directly rather than through a real
// tea.Program: bubbletea's Program.Run() expects a real terminal (it queries the output
// for capabilities and waits on the input reader), which is unreliable to drive headlessly
// in CI. A true headless/TUI parity run and PTY behavior (resize, paste, cancel-during-tool,
// terminal restoration) still need manual verification in an actual terminal per TESTING.md;
// what is verified here is that the model renders the exact events runAgent/Trace emit,
// without duplicating or reinterpreting them.

func TestTraceSinkReceivesEveryEvent(t *testing.T) {
	var received []string
	trace := &Trace{Sink: func(kind string, data any, at time.Time) {
		received = append(received, kind)
	}}
	if err := trace.Event("request", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if err := trace.Event("tool_start", &ToolCall{Function: FunctionCall{Name: "edit"}}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[0] != "request" || received[1] != "tool_start" {
		t.Fatalf("sink did not receive the same events as the JSONL logger: %v", received)
	}
}

func TestTUIModelUpdateFromTraceEvents(t *testing.T) {
	m := newTUIModel("task", "test-model", "/repo", nil)

	m2, _ := m.Update(traceEvent{kind: "request", at: time.Now()})
	m = m2.(tuiModel)
	if m.status != "waiting for model" || m.turns != 1 {
		t.Fatalf("request event: %+v", m)
	}

	m2, _ = m.Update(traceEvent{kind: "tool_start", data: &ToolCall{Function: FunctionCall{Name: "edit"}}, at: time.Now()})
	m = m2.(tuiModel)
	if m.status != "running: edit" || m.toolCalls != 1 {
		t.Fatalf("tool_start event: %+v", m)
	}

	applied := map[string]any{"result": map[string]any{"applied": true}}
	m2, _ = m.Update(traceEvent{kind: "tool_result", data: map[string]any{"name": "edit", "output": applied}, at: time.Now()})
	m = m2.(tuiModel)
	if m.status != "waiting for model" || m.edits != 1 {
		t.Fatalf("tool_result event: %+v", m)
	}
	if len(m.transcript) == 0 || !strings.Contains(m.transcript[len(m.transcript)-1], "<- edit ok") {
		t.Fatalf("transcript missing tool_result line: %v", m.transcript)
	}

	m2, _ = m.Update(agentDoneMsg{result: Summary{Status: "completed", Answer: "Fixed it."}})
	m = m2.(tuiModel)
	if !m.finished || m.status != "done: completed" {
		t.Fatalf("agentDoneMsg: %+v", m)
	}
	if !strings.Contains(strings.Join(m.transcript, "\n"), "Fixed it.") {
		t.Fatal("final answer not rendered in transcript")
	}
}

func TestTUITranscriptBounded(t *testing.T) {
	m := newTUIModel("task", "test-model", "/repo", nil)
	m.maxLines = 5
	for i := 0; i < 20; i++ {
		m.append("line")
	}
	if len(m.transcript) != 5 {
		t.Fatalf("transcript not bounded: %d lines", len(m.transcript))
	}
}
