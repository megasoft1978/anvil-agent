package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Some compatible model/server combinations emit the intended tool call as an
// XML-attribute-style tag in plain content instead of
// a native tool_calls entry, e.g.:
//
//	```xml
//	<function name="read" arguments='{"path": "test.txt"}'/>
//	```
//
// xmlAttrDetect/xmlAttrParse recover exactly that shape. To keep the false-positive
// risk low — source files the agent reads back into the conversation can legitimately
// contain a similarly-shaped tag as quoted code — recovery requires the tag (plus an
// optional surrounding ```xml fence and whitespace) to be the ENTIRE message content,
// not merely present somewhere inside it. A model that narrates and then calls in the
// same message is refused, on purpose: that is the safer failure mode here.

var xmlAttrFence = regexp.MustCompile("^```(?:xml)?\\s*\\n(.*?)\\n```\\s*$")
var xmlAttrTag = regexp.MustCompile(`^<(?:function|tool_call)\s+name=(?:"([^"]*)"|'([^']*)')\s+arguments=(?:"((?:[^"])*)"|'([^']*)')\s*/>$`)

func xmlAttrDetect(text string) bool {
	return strings.Contains(text, "<function ") || strings.Contains(text, "<tool_call ")
}

func xmlAttrParse(text string, allowed map[string]bool) ([]ToolCall, error) {
	body := strings.TrimSpace(text)
	if match := xmlAttrFence.FindStringSubmatch(body); match != nil {
		body = strings.TrimSpace(match[1])
	}
	match := xmlAttrTag.FindStringSubmatch(body)
	if match == nil {
		return nil, fmt.Errorf("xml-attribute tool call is not the entire message, or is incomplete")
	}
	name := firstNonEmpty(match[1], match[2])
	argsText := firstNonEmpty(match[3], match[4])
	if !namePattern.MatchString(name) {
		return nil, fmt.Errorf("invalid xml-attribute tool name")
	}
	if allowed != nil && !allowed[name] {
		return nil, fmt.Errorf("xml-attribute tool name %q is not a declared tool", name)
	}
	if strings.Contains(argsText, "&") {
		return nil, fmt.Errorf("xml-attribute arguments contain an HTML entity; refusing to guess an unescaping policy")
	}
	decoder := json.NewDecoder(strings.NewReader(argsText))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("invalid xml-attribute arguments JSON: %w", err)
	}
	if decoder.More() {
		return nil, fmt.Errorf("trailing content after xml-attribute arguments JSON")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return []ToolCall{{Type: "function", Function: FunctionCall{Name: name, Arguments: string(encoded)}}}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
