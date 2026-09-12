package main

import "testing"

func TestXMLAttrRecovery(t *testing.T) {
	allowed := map[string]bool{"read": true, "edit": true}
	for _, tc := range []struct {
		name    string
		input   string
		wantErr bool
		wantFn  string
		wantArg string
	}{
		{
			// Verbatim capture from Qwen2.5-Coder-7B-Instruct this session.
			name:    "captured_fenced",
			input:   "```xml\n<function name=\"read\" arguments='{\"path\": \"test.txt\"}'/>\n```",
			wantFn:  "read",
			wantArg: `{"path":"test.txt"}`,
		},
		{
			name:    "bare_no_fence",
			input:   `<function name="read" arguments='{"path": "test.txt"}'/>`,
			wantFn:  "read",
			wantArg: `{"path":"test.txt"}`,
		},
		{
			name:    "double_quoted_args",
			input:   `<function name="read" arguments="{&quot;path&quot;: &quot;test.txt&quot;}"/>`,
			wantErr: true, // entity-escaped: refused, not unescaped
		},
		{
			name:    "conflicting_tool_names_still_single_call",
			input:   `<function name="edit" arguments='{"path":"a"}'/>`,
			wantFn:  "edit",
			wantArg: `{"path":"a"}`,
		},
		{
			name:    "missing_close_slash",
			input:   `<function name="read" arguments='{}'>`,
			wantErr: true,
		},
		{
			name:    "unknown_tool_name",
			input:   `<function name="rm_rf" arguments='{}'/>`,
			wantErr: true,
		},
		{
			name:    "args_not_object_array",
			input:   `<function name="read" arguments='["a"]'/>`,
			wantErr: true,
		},
		{
			name:    "args_not_object_scalar",
			input:   `<function name="read" arguments='5'/>`,
			wantErr: true,
		},
		{
			name:    "args_bad_json",
			input:   `<function name="read" arguments='{path: test}'/>`,
			wantErr: true,
		},
		{
			name: "narration_plus_call_refused",
			input: "I'll read the file now.\n" +
				`<function name="read" arguments='{"path":"a"}'/>`,
			wantErr: true, // whole-message-match required; narration disqualifies it
		},
		{
			name:    "quoted_source_code_refused",
			input:   "```go\nfunc x() { }\n// <function name=\"read\" arguments='{}'/> is just a comment here\n```",
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, err := xmlAttrParse(tc.input, allowed)
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

// TestXMLAttrTruncationSweep mirrors TestRecoveryRejectsEveryTruncation: every strict
// prefix of a valid capture must never produce a call.
func TestXMLAttrTruncationSweep(t *testing.T) {
	allowed := map[string]bool{"read": true}
	full := `<function name="read" arguments='{"path": "test.txt"}'/>`
	for i := 1; i < len(full); i++ {
		prefix := full[:i]
		if calls, err := xmlAttrParse(prefix, allowed); err == nil && len(calls) > 0 {
			t.Fatalf("truncated prefix %q recovered a call: %+v", prefix, calls)
		}
	}
}

func TestXMLAttrDetect(t *testing.T) {
	if !xmlAttrDetect(`<function name="read" arguments='{}'/>`) {
		t.Error("expected detect on function tag")
	}
	if !xmlAttrDetect(`<tool_call name="read" arguments='{}'/>`) {
		t.Error("expected detect on tool_call tag")
	}
	if xmlAttrDetect("plain text with no tags") {
		t.Error("unexpected detect on plain text")
	}
}
