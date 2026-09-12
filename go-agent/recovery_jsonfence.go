package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Qwen2.5-Coder-14B-Instruct (through this llama-server build) emits the intended tool
// call as a fenced JSON object in plain content instead of a native tool_calls entry,
// e.g.:
//
//	To fix the sum fixture, I need to inspect the relevant files first.
//
//	```json
//	{
//	  "name": "read",
//	  "arguments": {
//	    "path": "."
//	  }
//	}
//	```
//
// Two verbatim captures this session show narration both before and after the fence —
// unlike xml_attr's tag, this model never emits the call as the entire message, so a
// whole-message-match rule (as used for xml_attr) would recover nothing real here.
// Instead: there must be exactly one ```json-fenced block in the message, and its
// content must parse as exactly {"name": string, "arguments": object} with no other
// top-level keys. Surrounding narration is tolerated; a second fenced block, or any
// non-conforming shape in the one block found, is a refusal, not a best-effort parse.

var jsonFencePattern = regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```")

func jsonFenceDetect(text string) bool {
	return jsonFencePattern.MatchString(text)
}

func jsonFenceParse(text string, allowed map[string]bool) ([]ToolCall, error) {
	matches := jsonFencePattern.FindAllStringSubmatch(text, -1)
	if len(matches) != 1 {
		return nil, fmt.Errorf("expected exactly one ```json-fenced block, found %d", len(matches))
	}
	body := strings.TrimSpace(matches[0][1])
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	var raw map[string]json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON in fenced block: %w", err)
	}
	if decoder.More() {
		return nil, fmt.Errorf("trailing content after fenced JSON object")
	}
	nameRaw, hasName := raw["name"]
	argsRaw, hasArgs := raw["arguments"]
	if !hasName || !hasArgs || len(raw) != 2 {
		return nil, fmt.Errorf("fenced object must have exactly \"name\" and \"arguments\"")
	}
	var name string
	if err := json.Unmarshal(nameRaw, &name); err != nil {
		return nil, fmt.Errorf("\"name\" is not a string: %w", err)
	}
	if !namePattern.MatchString(name) {
		return nil, fmt.Errorf("invalid fenced tool name")
	}
	if allowed != nil && !allowed[name] {
		return nil, fmt.Errorf("fenced tool name %q is not a declared tool", name)
	}
	var args map[string]any
	argsDecoder := json.NewDecoder(strings.NewReader(string(argsRaw)))
	argsDecoder.UseNumber()
	if err := argsDecoder.Decode(&args); err != nil {
		return nil, fmt.Errorf("\"arguments\" is not a JSON object: %w", err)
	}
	if argsDecoder.More() {
		return nil, fmt.Errorf("trailing content after \"arguments\" object")
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	return []ToolCall{{Type: "function", Function: FunctionCall{Name: name, Arguments: string(encoded)}}}, nil
}
