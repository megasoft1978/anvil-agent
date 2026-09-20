package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// compactHistory removes old, usually-repeated tool transcripts while preserving the system
// contract, the original task, a short action ledger, and the most recent complete tool turns.
// It is deliberately deterministic: no second model is needed to summarize the repository, and
// the worktree remains the source of truth whenever an exact source span is needed again.
func compactHistory(messages []Message, keepToolTurns int) []Message {
	if len(messages) <= 2 {
		return messages
	}
	if keepToolTurns < 1 {
		keepToolTurns = 1
	}

	starts := make([]int, 0)
	for i := 2; i < len(messages); i++ {
		if messages[i].Role == "assistant" && len(messages[i].ToolCalls) > 0 {
			starts = append(starts, i)
		}
	}
	start := 2
	if len(starts) > keepToolTurns {
		start = starts[len(starts)-keepToolTurns]
	}

	result := make([]Message, 0, len(messages)-start+3)
	result = append(result, messages[0], messages[1])
	if summary := compactedActionLedger(messages[2:start]); summary != "" {
		result = append(result, Message{Role: "user", Content: summary})
	}
	for _, message := range messages[start:] {
		result = append(result, compactedMessage(message))
	}
	return result
}

func compactedActionLedger(messages []Message) string {
	const maxActions = 32
	actions := make([]string, 0, maxActions)
	for _, message := range messages {
		if message.Role != "assistant" {
			continue
		}
		for _, call := range message.ToolCalls {
			if len(actions) == maxActions {
				break
			}
			actions = append(actions, compactToolCallLabel(call))
		}
	}
	if len(actions) == 0 {
		return ""
	}
	return "Earlier tool history was compacted to keep the request within the context budget. " +
		"The current worktree is authoritative; reread exact source text if needed. " +
		"Earlier actions for orientation only: " + strings.Join(actions, ", ") + "."
}

func compactToolCallLabel(call ToolCall) string {
	path := ""
	var fields map[string]any
	if json.Unmarshal([]byte(call.Function.Arguments), &fields) == nil {
		if value, ok := fields["path"].(string); ok {
			path = value
		}
	}
	if path == "" {
		return call.Function.Name
	}
	return fmt.Sprintf("%s(%s)", call.Function.Name, path)
}

func compactedMessage(message Message) Message {
	limit := 8192
	if message.Role == "assistant" {
		limit = 4096
	}
	if len(message.Content) > limit {
		message.Content = compactText(message.Content, limit)
	}
	return message
}

func compactText(value string, limit int) string {
	if limit < 32 || len(value) <= limit {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	left := limit / 2
	truncated := limit - left
	return string(runes[:left]) + "\n… [older tool output compacted] …\n" + string(runes[len(runes)-truncated:])
}
