package main

import "errors"

var errAmbiguousRecovery = errors.New("multiple distinct recovered calls disagree across formats")
var errMultipleDistinctCalls = errors.New("multiple distinct leaked calls")

// A recoverer turns one known malformed tool-call emission format back into a real
// ToolCall. detect is a cheap, format-specific substring gate; parse must be strict:
// any syntax problem is an error, never a partial success, even if an earlier call in
// the same text parsed cleanly. Adding a new malformed format observed from a new
// model means adding one more recoverer here, never widening an existing one.
type recoverer struct {
	name   string
	detect func(text string) bool
	// parse returns every call found in text. allowed is the set of tool names declared
	// for this turn; a recoverer that opts into useAllowed must reject any other name.
	parse      func(text string, allowed map[string]bool) ([]ToolCall, error)
	useAllowed bool
}

var recoverers = []recoverer{
	{name: "leaked_markup", detect: hasMarkers, parse: leakedMarkupRecoverParse, useAllowed: false},
	{name: "xml_attr", detect: xmlAttrDetect, parse: xmlAttrParse, useAllowed: true},
	{name: "json_fence", detect: jsonFenceDetect, parse: jsonFenceParse, useAllowed: true},
}

// declaredToolNames returns the set of tool names available this turn, so a recovered
// call can be rejected if it names a tool the model was never offered.
func declaredToolNames(tools []ToolDefinition) map[string]bool {
	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		names[tool.Function.Name] = true
	}
	return names
}

// detectRecoverers returns the name of every format whose marker appears in any text.
// An empty result means the content is not a known malformed tool-call shape at all —
// callers must not treat that as a leak requiring correction.
func detectRecoverers(texts ...string) []string {
	var names []string
	for _, r := range recoverers {
		for _, text := range texts {
			if r.detect(text) {
				names = append(names, r.name)
				break
			}
		}
	}
	return names
}

// recoverAny runs every recoverer whose detector fires against all texts and returns
// exactly one call, the parser name that produced it, or an error. Two or more
// detectors firing is treated as ambiguity, not a priority order to break: recovery
// succeeds only if every firing parser independently agrees on the same call.
func recoverAny(allowed map[string]bool, texts ...string) (*ToolCall, string, error) {
	var name string
	var result *ToolCall
	for _, r := range recoverers {
		var calls []ToolCall
		fires := false
		for _, text := range texts {
			// Only hand this recoverer a text where its own marker actually appears;
			// a text belonging to a different format is not this parser's concern.
			if !r.detect(text) {
				continue
			}
			fires = true
			found, err := r.parse(text, allowed)
			if err != nil {
				return nil, "", err
			}
			calls = append(calls, found...)
		}
		if !fires {
			continue
		}
		call, err := collapse(calls)
		if err != nil {
			return nil, "", err
		}
		if call == nil {
			continue
		}
		if result == nil {
			result = call
			name = r.name
		} else if result.Function.Name != call.Function.Name || result.Function.Arguments != call.Function.Arguments {
			return nil, "", errAmbiguousRecovery
		}
	}
	if result == nil {
		return nil, "", nil
	}
	return result, name, nil
}

// collapse folds a list of recovered calls into one: identical (name, arguments)
// repeats collapse to a single call, but any distinct pair is a refusal — never a guess
// at which one the model meant.
func collapse(calls []ToolCall) (*ToolCall, error) {
	var result *ToolCall
	for i := range calls {
		call := calls[i]
		if result == nil {
			result = &call
			continue
		}
		if result.Function.Name != call.Function.Name || result.Function.Arguments != call.Function.Arguments {
			return nil, errMultipleDistinctCalls
		}
	}
	return result, nil
}
