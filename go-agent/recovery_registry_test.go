package main

import "testing"

func TestDetectRecoverersGarbage(t *testing.T) {
	names := detectRecoverers("the quick brown fox jumps over the lazy dog")
	if len(names) != 0 {
		t.Fatalf("expected no detectors to fire on plain text, got %v", names)
	}
}

func TestRecoverAnyNoDetectorFires(t *testing.T) {
	call, name, err := recoverAny(map[string]bool{"read": true}, "plain narration, no tool call at all")
	if call != nil || name != "" || err != nil {
		t.Fatalf("expected (nil, \"\", nil), got (%v, %q, %v)", call, name, err)
	}
}

func TestRecoverAnyXMLAttrSucceeds(t *testing.T) {
	call, name, err := recoverAny(map[string]bool{"read": true}, `<function name="read" arguments='{"path":"a"}'/>`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if call == nil || name != "xml_attr" {
		t.Fatalf("expected xml_attr recovery, got call=%v name=%q", call, name)
	}
	if call.Function.Name != "read" || call.Function.Arguments != `{"path":"a"}` {
		t.Fatalf("unexpected call: %+v", call)
	}
}

func TestRecoverAnyLeakedMarkupAndXMLAttrAgree(t *testing.T) {
	leakedMarkup := `<|tool_call>call:read{path:<|"|>a<|"|>}<tool_call|>`
	xmlAttr := `<function name="read" arguments='{"path":"a"}'/>`
	call, _, err := recoverAny(map[string]bool{"read": true}, leakedMarkup, xmlAttr)
	if err != nil {
		t.Fatalf("unexpected error when both formats agree: %v", err)
	}
	if call == nil || call.Function.Name != "read" || call.Function.Arguments != `{"path":"a"}` {
		t.Fatalf("unexpected call: %+v", call)
	}
}

func TestRecoverAnyLeakedMarkupAndXMLAttrDisagree(t *testing.T) {
	leakedMarkup := `<|tool_call>call:read{path:<|"|>a<|"|>}<tool_call|>`
	xmlAttr := `<function name="read" arguments='{"path":"b"}'/>`
	call, name, err := recoverAny(map[string]bool{"read": true}, leakedMarkup, xmlAttr)
	if err == nil {
		t.Fatalf("expected ambiguity error, got call=%v name=%q", call, name)
	}
}

func TestRecoverAnyLeakedMarkupFallsThroughOnXMLAttrRefusal(t *testing.T) {
	// The xml_attr detector fires (a <function ...> tag is present), but the tag is
	// embedded in narration so xmlAttrParse refuses it. Since a firing detector whose
	// parser errors must propagate that error immediately (never silently skip to try
	// another format), this is a hard failure, not a leaked-markup-only recovery.
	leakedMarkup := `<|tool_call>call:read{path:<|"|>a<|"|>}<tool_call|>`
	embedded := "I'll do this now: <function name=\"read\" arguments='{\"path\":\"a\"}'/>"
	_, _, err := recoverAny(map[string]bool{"read": true}, leakedMarkup, embedded)
	if err == nil {
		t.Fatal("expected error: a firing-but-refusing parser must not be silently bypassed")
	}
}
