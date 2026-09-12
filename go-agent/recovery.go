package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const callAnchor = "<|tool_call>"
const callClose = "<tool_call|>"
const stringMarker = `<|"|>`

var namePattern = regexp.MustCompile(`^[A-Za-z_]\w*$`)

func hasMarkers(text string) bool {
	for _, marker := range []string{callAnchor, callClose, "<|channel>", "<channel|>"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// gemmaRecoverParse adapts recoverCall to the recoverer.parse shape: it ignores allowed
// (the Gemma dialect is never allowlist-checked; see TestObservedGemma4ParserRegressions,
// which fixtures a recovered call named "done" that is not a declared tool) and returns
// at most one call, matching recoverCall's own single-call contract.
func gemmaRecoverParse(text string, allowed map[string]bool) ([]ToolCall, error) {
	call, err := recoverCall(text)
	if err != nil || call == nil {
		return nil, err
	}
	return []ToolCall{*call}, nil
}

// Recover only complete, identical calls. Multiple distinct calls or any incomplete opener are an
// error, never a guess. The caller refuses token-truncated responses before calling this function.
func recoverCall(texts ...string) (*ToolCall, error) {
	var recovered *ToolCall
	for _, text := range texts {
		for {
			index := strings.Index(text, callAnchor)
			if index < 0 {
				break
			}
			text = text[index+len(callAnchor):]
			switch {
			case strings.HasPrefix(text, "call:"):
				text = text[5:]
			case strings.HasPrefix(text, ":"):
				text = text[1:]
			default:
				return nil, fmt.Errorf("unrecognized Gemma tool opener")
			}
			brace := strings.IndexByte(text, '{')
			if brace < 0 {
				return nil, fmt.Errorf("incomplete Gemma tool call")
			}
			name := strings.TrimSpace(text[:brace])
			if !namePattern.MatchString(name) {
				return nil, fmt.Errorf("invalid Gemma tool name")
			}
			p := gemmaParser{text: text, pos: brace}
			value, err := p.value(0)
			if err != nil {
				return nil, err
			}
			p.space()
			if !strings.HasPrefix(text[p.pos:], callClose) {
				return nil, fmt.Errorf("missing Gemma tool closer")
			}
			args, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			call := ToolCall{Type: "function", Function: FunctionCall{Name: name, Arguments: string(args)}}
			if recovered != nil && (recovered.Function.Name != name || recovered.Function.Arguments != string(args)) {
				return nil, fmt.Errorf("multiple distinct leaked calls")
			}
			recovered = &call
			text = text[p.pos+len(callClose):]
		}
	}
	return recovered, nil
}

type gemmaParser struct {
	text string
	pos  int
}

func (p *gemmaParser) space() {
	for p.pos < len(p.text) && unicode.IsSpace(rune(p.text[p.pos])) {
		p.pos++
	}
}
func (p *gemmaParser) value(depth int) (any, error) {
	p.space()
	if depth > 64 || p.pos >= len(p.text) {
		return nil, fmt.Errorf("incomplete or deeply nested Gemma value")
	}
	if strings.HasPrefix(p.text[p.pos:], stringMarker) {
		p.pos += len(stringMarker)
		end := strings.Index(p.text[p.pos:], stringMarker)
		if end < 0 {
			return nil, fmt.Errorf("unterminated Gemma string")
		}
		value := p.text[p.pos : p.pos+end]
		p.pos += end + len(stringMarker)
		return value, nil
	}
	switch p.text[p.pos] {
	case '{':
		p.pos++
		result := map[string]any{}
		p.space()
		if p.pos < len(p.text) && p.text[p.pos] == '}' {
			p.pos++
			return result, nil
		}
		for {
			p.space()
			start := p.pos
			for p.pos < len(p.text) && !strings.ContainsRune(":,{}[]\n", rune(p.text[p.pos])) {
				p.pos++
			}
			if p.pos >= len(p.text) || p.text[p.pos] != ':' {
				return nil, fmt.Errorf("invalid Gemma object key")
			}
			key := strings.TrimSpace(p.text[start:p.pos])
			if key == "" {
				return nil, fmt.Errorf("empty Gemma object key")
			}
			if _, exists := result[key]; exists {
				return nil, fmt.Errorf("duplicate Gemma object key")
			}
			p.pos++
			value, err := p.value(depth + 1)
			if err != nil {
				return nil, err
			}
			result[key] = value
			p.space()
			if p.pos >= len(p.text) {
				return nil, fmt.Errorf("unterminated Gemma object")
			}
			ch := p.text[p.pos]
			p.pos++
			if ch == '}' {
				return result, nil
			}
			if ch != ',' {
				return nil, fmt.Errorf("invalid Gemma object separator")
			}
		}
	case '[':
		p.pos++
		result := []any{}
		p.space()
		if p.pos < len(p.text) && p.text[p.pos] == ']' {
			p.pos++
			return result, nil
		}
		for {
			value, err := p.value(depth + 1)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
			p.space()
			if p.pos >= len(p.text) {
				return nil, fmt.Errorf("unterminated Gemma array")
			}
			ch := p.text[p.pos]
			p.pos++
			if ch == ']' {
				return result, nil
			}
			if ch != ',' {
				return nil, fmt.Errorf("invalid Gemma array separator")
			}
		}
	default:
		decoder := json.NewDecoder(bytes.NewBufferString(p.text[p.pos:]))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("invalid Gemma scalar: %w", err)
		}
		p.pos += int(decoder.InputOffset())
		return value, nil
	}
}
