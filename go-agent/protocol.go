package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Reasoning  string     `json:"reasoning_content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolDefinition struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type Request struct {
	TopP            *float64         `json:"top_p,omitempty"`
	TopK            *int             `json:"top_k,omitempty"`
	MinP            *float64         `json:"min_p,omitempty"`
	Seed            *int             `json:"seed,omitempty"`
	PresencePenalty *float64         `json:"presence_penalty,omitempty"`
	RepeatPenalty   *float64         `json:"repeat_penalty,omitempty"`
	Model           string           `json:"model"`
	Messages        []Message        `json:"messages"`
	Tools           []ToolDefinition `json:"tools"`
	ToolChoice      string           `json:"tool_choice"`
	Parallel        bool             `json:"parallel_tool_calls"`
	Stream          bool             `json:"stream"`
	Temperature     float64          `json:"temperature"`
	MaxTokens       int              `json:"max_tokens"`
	CachePrompt     bool             `json:"cache_prompt"`
}

type Response struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage   json.RawMessage `json:"usage,omitempty"`
	Timings json.RawMessage `json:"timings,omitempty"`
}

type Client struct {
	URL    string
	APIKey string
	HTTP   *http.Client
}

func (c *Client) Complete(ctx context.Context, input Request) (Response, error) {
	var result Response
	body, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.URL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return result, err
	}
	defer res.Body.Close()
	const limit = 8 << 20
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return result, err
	}
	if len(data) > limit {
		return result, fmt.Errorf("response exceeds %d bytes", limit)
	}
	if res.StatusCode != http.StatusOK {
		return result, fmt.Errorf("endpoint returned HTTP %d", res.StatusCode)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("invalid response JSON: %w", err)
	}
	if len(result.Choices) != 1 {
		return result, fmt.Errorf("expected one choice, got %d", len(result.Choices))
	}
	return result, nil
}

func toolDefinitions(withTests, withSearch bool) []ToolDefinition {
	makeTool := func(name, description string, fields map[string]any, required ...string) ToolDefinition {
		if required == nil {
			required = []string{}
		}
		var tool ToolDefinition
		tool.Type = "function"
		tool.Function.Name = name
		tool.Function.Description = description
		tool.Function.Parameters = map[string]any{"type": "object", "properties": fields, "required": required, "additionalProperties": false}
		return tool
	}
	str := func(description string) any { return map[string]any{"type": "string", "description": description} }
	tools := []ToolDefinition{
		makeTool("read", "Read a UTF-8 file or list a directory inside the worktree. Use . to list the root. File output is paginated.",
			map[string]any{"path": str("Relative file or directory path"), "offset": map[string]any{"type": "integer", "minimum": 1, "description": "First line or directory entry, default 1"}}, "path"),
		makeTool("edit", "Replace exactly one occurrence of oldText in an existing UTF-8 file. Read it first. Ambiguous matches are rejected.",
			map[string]any{"path": str("Relative file path"), "oldText": str("Exact non-empty text occurring once"), "newText": str("Replacement text")}, "path", "oldText", "newText"),
	}
	if withTests {
		tools = append(tools, makeTool("run_tests", "Run the preconfigured non-interactive test command. Takes no arguments.", map[string]any{}))
	}
	if withSearch {
		tools = append(tools, makeTool("search", "Find literal text across the repository (or under one file/directory, if path is given). Skips .git, node_modules, and other generated/build directories, and binary files. Returns bounded matching line numbers with nearby source. Use to locate a function or symbol before guessing filenames or rereading whole files. Not a regex search.", map[string]any{"path": str("Relative file or directory path; omit or use . to search the whole worktree"), "text": str("Non-empty literal text to find, such as clone(")}, "text"))
	}
	return tools
}
