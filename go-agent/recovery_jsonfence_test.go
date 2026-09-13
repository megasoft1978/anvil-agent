package main

import "testing"

func TestJSONFenceRecovery(t *testing.T) {
	allowed := map[string]bool{"read": true, "edit": true}
	for _, tc := range []struct {
		name    string
		input   string
		wantErr bool
		wantFn  string
		wantArg string
	}{
		{
			// Regression capture: fence first, narration after.
			name:    "captured_fence_then_narration",
			input:   "```json\n{\n  \"name\": \"read\",\n  \"arguments\": {\n    \"path\": \"test.txt\"\n  }\n}\n```\n\nAfter reading the file, you can use the `edit` function to modify its contents.",
			wantFn:  "read",
			wantArg: `{"path":"test.txt"}`,
		},
		{
			// Second verbatim capture: narration first, fence last.
			name:    "captured_narration_then_fence",
			input:   "To fix the sum fixture, I need to inspect the relevant files first. Let's start by listing the directory to see what files are present.\n\n```json\n{\n  \"name\": \"read\",\n  \"arguments\": {\n    \"path\": \".\"\n  }\n}\n```",
			wantFn:  "read",
			wantArg: `{"path":"."}`,
		},
		{
			name:    "no_fence_at_all",
			input:   "Could you please provide more details about the bug?",
			wantErr: true,
		},
		{
			name:    "two_fences_refused",
			input:   "```json\n{\"name\":\"read\",\"arguments\":{\"path\":\"a\"}}\n```\nthen\n```json\n{\"name\":\"read\",\"arguments\":{\"path\":\"b\"}}\n```",
			wantErr: true,
		},
		{
			name:    "unknown_tool_name",
			input:   "```json\n{\"name\":\"rm_rf\",\"arguments\":{}}\n```",
			wantErr: true,
		},
		{
			name:    "extra_top_level_key",
			input:   "```json\n{\"name\":\"read\",\"arguments\":{\"path\":\"a\"},\"id\":\"1\"}\n```",
			wantErr: true,
		},
		{
			name:    "arguments_not_object",
			input:   "```json\n{\"name\":\"read\",\"arguments\":\"a\"}\n```",
			wantErr: true,
		},
		{
			name:    "bad_json_in_fence",
			input:   "```json\n{name: read}\n```",
			wantErr: true,
		},
		{
			name:    "missing_arguments_key",
			input:   "```json\n{\"name\":\"read\"}\n```",
			wantErr: true,
		},
		{
			name:    "quoted_source_code_wrong_shape_refused",
			input:   "Here's the config file:\n```json\n{\"name\":\"my-package\",\"version\":\"1.0.0\"}\n```",
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, err := jsonFenceParse(tc.input, allowed)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got calls=%v", calls)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(calls) != 1 {
				t.Fatalf("expected exactly one call, got %d", len(calls))
			}
			if calls[0].Function.Name != tc.wantFn || calls[0].Function.Arguments != tc.wantArg {
				t.Fatalf("got %+v, want name=%s args=%s", calls[0], tc.wantFn, tc.wantArg)
			}
		})
	}
}

func TestJSONFenceDetect(t *testing.T) {
	if !jsonFenceDetect("```json\n{}\n```") {
		t.Error("expected detect on a json fence")
	}
	if jsonFenceDetect("plain text with no fences") {
		t.Error("unexpected detect on plain text")
	}
	if jsonFenceDetect("```js\nconsole.log(1)\n```") {
		t.Error("unexpected detect on a non-json fence")
	}
}

// TestJSONFenceViaRecoverAny confirms the new recoverer is actually wired into the
// registry, not just unit-tested in isolation.
func TestJSONFenceViaRecoverAny(t *testing.T) {
	call, name, err := recoverAny(map[string]bool{"read": true}, "```json\n{\"name\":\"read\",\"arguments\":{\"path\":\"a\"}}\n```")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if call == nil || name != "json_fence" {
		t.Fatalf("expected json_fence recovery, got call=%v name=%q", call, name)
	}
}
