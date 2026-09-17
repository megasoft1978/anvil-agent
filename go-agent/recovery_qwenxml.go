package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Qwen3-family models use a separate-entity XML dialect when the server fails to
// turn the text into an OpenAI tool_calls entry:
//
//   <tool_call>
//   <function=read>
//   <parameter=path>src/file.go</parameter>
//   </function>
//   </tool_call>
//
// This is deliberately a whole-message, fail-closed recovery path. It exists for
// the observed llama.cpp/Qwen parser failure mode; it must never search arbitrary
// source text for a promising tag or silently recover a truncated call.

var qwenFunctionOpen = regexp.MustCompile(`^<function=([A-Za-z_]\w*)>`)
var qwenParameterOpen = regexp.MustCompile(`^<parameter=([A-Za-z_]\w*)>`)

func qwenXMLDetect(text string) bool {
	return strings.Contains(text, "<tool_call>") || strings.Contains(text, "</tool_call>") || strings.Contains(text, "<function=")
}

func qwenXMLParse(text string, allowed map[string]bool) ([]ToolCall, error) {
	return qwenXMLParseWithSchema(text, allowed, nil)
}

func qwenXMLParseWithTools(text string, allowed map[string]bool, tools []ToolDefinition) ([]ToolCall, error) {
	return qwenXMLParseWithSchema(text, allowed, tools)
}

func qwenXMLParseWithSchema(text string, allowed map[string]bool, tools []ToolDefinition) ([]ToolCall, error) {
	rest := strings.TrimSpace(text)
	if rest == "" {
		return nil, fmt.Errorf("empty Qwen XML tool output")
	}
	var calls []ToolCall
	for rest != "" {
		if strings.HasPrefix(rest, "<tool_call>") {
			const close = "</tool_call>"
			end := strings.Index(rest[len("<tool_call>"):], close)
			if end < 0 {
				return nil, fmt.Errorf("incomplete Qwen XML tool-call wrapper")
			}
			end += len("<tool_call>")
			inner := rest[len("<tool_call>"):end]
			innerCalls, err := qwenFunctions(inner, allowed, tools)
			if err != nil {
				return nil, err
			}
			calls = append(calls, innerCalls...)
			rest = strings.TrimSpace(rest[end+len(close):])
			continue
		}

		// Some server parsers hand the model's <function=...> entity to the
		// formatter without the outer <tool_call> marker. Accept that exact
		// whole-message shape as well, but never accept surrounding narration.
		if strings.HasPrefix(rest, "<function=") {
			innerCalls, err := qwenFunctions(rest, allowed, tools)
			if err != nil {
				return nil, err
			}
			calls = append(calls, innerCalls...)
			break
		}
		return nil, fmt.Errorf("Qwen XML tool output contains non-whitespace outside a tool call")
	}
	if len(calls) == 0 {
		return nil, fmt.Errorf("Qwen XML tool output contains no function")
	}
	return calls, nil
}

func qwenFunctions(text string, allowed map[string]bool, tools []ToolDefinition) ([]ToolCall, error) {
	rest := strings.TrimSpace(text)
	var calls []ToolCall
	for rest != "" {
		call, remaining, err := qwenFunction(rest, allowed, tools)
		if err != nil {
			return nil, err
		}
		calls = append(calls, *call)
		rest = strings.TrimSpace(remaining)
	}
	return calls, nil
}

func qwenFunction(text string, allowed map[string]bool, tools []ToolDefinition) (*ToolCall, string, error) {
	match := qwenFunctionOpen.FindStringSubmatchIndex(text)
	if match == nil {
		return nil, "", fmt.Errorf("incomplete or invalid Qwen XML function opener")
	}
	name := text[match[2]:match[3]]
	if allowed != nil && !allowed[name] {
		return nil, "", fmt.Errorf("Qwen XML tool name %q is not a declared tool", name)
	}
	rest := text[match[1]:]
	args := map[string]any{}
	properties, hasSchema := qwenToolProperties(tools, name)
	for {
		rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
		if strings.HasPrefix(rest, "</function>") {
			rest = rest[len("</function>"):]
			encoded, err := json.Marshal(args)
			if err != nil {
				return nil, "", err
			}
			return &ToolCall{Type: "function", Function: FunctionCall{Name: name, Arguments: string(encoded)}}, rest, nil
		}
		parameter := qwenParameterOpen.FindStringSubmatchIndex(rest)
		if parameter == nil {
			return nil, "", fmt.Errorf("Qwen XML function %q contains an unexpected tag or is missing </function>", name)
		}
		parameterName := rest[parameter[2]:parameter[3]]
		if hasSchema {
			if _, ok := properties[parameterName]; !ok {
				return nil, "", fmt.Errorf("Qwen XML parameter %q is not declared for tool %q", parameterName, name)
			}
		}
		rest = rest[parameter[1]:]
		const close = "</parameter>"
		end := strings.Index(rest, close)
		if end < 0 {
			return nil, "", fmt.Errorf("Qwen XML parameter %q is incomplete", parameterName)
		}
		raw := qwenParameterValue(rest[:end])
		if _, duplicate := args[parameterName]; duplicate {
			return nil, "", fmt.Errorf("duplicate Qwen XML parameter %q", parameterName)
		}
		value, err := qwenConvertParameter(raw, properties[parameterName])
		if err != nil {
			return nil, "", fmt.Errorf("invalid Qwen XML parameter %q: %w", parameterName, err)
		}
		args[parameterName] = value
		rest = rest[end+len(close):]
	}
}

func qwenParameterValue(value string) string {
	if strings.HasPrefix(value, "\r\n") {
		value = value[2:]
	} else if strings.HasPrefix(value, "\n") {
		value = value[1:]
	}
	if strings.HasSuffix(value, "\r\n") {
		value = value[:len(value)-2]
	} else if strings.HasSuffix(value, "\n") {
		value = value[:len(value)-1]
	}
	return value
}

func qwenToolProperties(tools []ToolDefinition, name string) (map[string]any, bool) {
	for _, tool := range tools {
		if tool.Function.Name != name {
			continue
		}
		properties, ok := tool.Function.Parameters["properties"].(map[string]any)
		return properties, ok
	}
	return nil, false
}

func qwenConvertParameter(raw string, schema any) (any, error) {
	property, _ := schema.(map[string]any)
	typeName, nullable := qwenSchemaType(property)
	if strings.EqualFold(strings.TrimSpace(raw), "null") && nullable {
		return nil, nil
	}
	switch {
	case typeName == "string" || typeName == "str" || typeName == "text" || typeName == "varchar" || typeName == "char" || typeName == "enum":
		return raw, nil
	case strings.HasPrefix(typeName, "uint") || strings.HasPrefix(typeName, "unsigned"):
		value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid unsigned integer literal %q", raw)
		}
		return value, nil
	case strings.HasPrefix(typeName, "int") || strings.HasPrefix(typeName, "long") || strings.HasPrefix(typeName, "short"):
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer literal %q", raw)
		}
		return value, nil
	case strings.HasPrefix(typeName, "num") || strings.HasPrefix(typeName, "float"):
		value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("invalid numeric literal %q", raw)
		}
		return value, nil
	case typeName == "boolean" || typeName == "bool" || typeName == "binary":
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid boolean literal %q", raw)
		}
	case typeName == "object" || typeName == "array" || typeName == "arr" || strings.HasPrefix(typeName, "dict") || strings.HasPrefix(typeName, "list"):
		value, err := decodeQwenJSONValue(raw)
		if err != nil {
			return nil, err
		}
		if typeName == "object" || strings.HasPrefix(typeName, "dict") {
			if _, ok := value.(map[string]any); !ok {
				return nil, fmt.Errorf("expected a JSON object")
			}
		}
		if typeName == "array" || typeName == "arr" || strings.HasPrefix(typeName, "list") {
			if _, ok := value.([]any); !ok {
				return nil, fmt.Errorf("expected a JSON array")
			}
		}
		return value, nil
	default:
		// With no schema (the compatibility test path), preserve the model's
		// literal as a string. Production calls always pass the declared schema.
		return raw, nil
	}
}

func qwenSchemaType(property map[string]any) (string, bool) {
	if property == nil {
		return "", false
	}
	switch value := property["type"].(type) {
	case string:
		name := strings.ToLower(strings.TrimSpace(value))
		return name, name == "null"
	case []any:
		nullable := false
		for _, item := range value {
			name, ok := item.(string)
			if !ok {
				continue
			}
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "null" {
				nullable = true
				continue
			}
			if name != "" {
				return name, nullable
			}
		}
		return "", nullable
	default:
		return "", false
	}
}

func decodeQwenJSONValue(raw string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("expected valid JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != nil {
		if err == io.EOF {
			return value, nil
		}
		return nil, fmt.Errorf("trailing content after JSON value: %w", err)
	}
	return nil, fmt.Errorf("trailing content after JSON value")
}
