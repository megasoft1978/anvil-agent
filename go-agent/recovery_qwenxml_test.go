package main

import "testing"

func TestQwenXMLRecoveryUsesDeclaredArgumentTypes(t *testing.T) {
	tools := toolDefinitions(true, true)
	input := "<tool_call>\n<function=read>\n<parameter=path>src/file.go</parameter>\n<parameter=offset>2</parameter>\n</function>\n</tool_call>"
	call, name, err := recoverAnyWithTools(tools, declaredToolNames(tools), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if call == nil || name != "qwen3_xml" {
		t.Fatalf("expected qwen3_xml recovery, got call=%v name=%q", call, name)
	}
	if call.Function.Name != "read" || call.Function.Arguments != `{"offset":2,"path":"src/file.go"}` {
		t.Fatalf("unexpected recovered call: %+v", call)
	}
}

func TestQwenXMLRecoveryAcceptsBareFunctionEntity(t *testing.T) {
	tools := toolDefinitions(true, true)
	input := "<function=search><parameter=text>clone(</parameter><parameter=path>.</parameter></function>"
	call, name, err := recoverAnyWithTools(tools, declaredToolNames(tools), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if call == nil || name != "qwen3_xml" || call.Function.Name != "search" {
		t.Fatalf("unexpected recovered call: call=%v name=%q", call, name)
	}
	if call.Function.Arguments != `{"path":".","text":"clone("}` {
		t.Fatalf("unexpected arguments: %s", call.Function.Arguments)
	}
}

func TestQwenXMLRecoveryRefusesNarrationAndMalformedCalls(t *testing.T) {
	tools := toolDefinitions(true, true)
	allowed := declaredToolNames(tools)
	for _, input := range []string{
		"I will inspect the file first. <tool_call><function=read><parameter=path>a</parameter></function></tool_call>",
		"<tool_call><function=read><parameter=path>a</function></tool_call>",
		"<tool_call><function=read><parameter=unknown>a</parameter></function></tool_call>",
		"<tool_call><function=read><parameter=offset>one</parameter></function></tool_call>",
	} {
		calls, err := qwenXMLParseWithTools(input, allowed, tools)
		if err == nil || len(calls) != 0 {
			t.Fatalf("expected fail-closed refusal for %q, got calls=%v err=%v", input, calls, err)
		}
	}
}

func TestQwenXMLRecoveryAgreesAcrossChannels(t *testing.T) {
	tools := toolDefinitions(true, true)
	input := "<tool_call><function=read><parameter=path>a</parameter></function></tool_call>"
	call, name, err := recoverAnyWithTools(tools, declaredToolNames(tools), input, input)
	if err != nil {
		t.Fatalf("unexpected duplicate-channel error: %v", err)
	}
	if call == nil || name != "qwen3_xml" {
		t.Fatalf("expected one collapsed recovery, got call=%v name=%q", call, name)
	}
}
