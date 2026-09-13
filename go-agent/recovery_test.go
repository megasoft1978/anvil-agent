package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type regression struct {
	Name          string `json:"name"`
	Content       string `json:"content"`
	Reasoning     string `json:"reasoning_content"`
	Finish        string `json:"finish_reason"`
	RecoveredName string `json:"recovered_name"`
}

func regressions(t testing.TB) []regression {
	t.Helper()
	data, err := os.ReadFile("testdata/leaked-markup-regressions.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []regression
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestObservedLeakedMarkupParserRegressions(t *testing.T) {
	for _, fixture := range regressions(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			content := fixture.Content
			if fixture.Finish == "length" {
				content = strings.Repeat(content, 60)
			}
			got, err := recoverCall(content, fixture.Reasoning)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.RecoveredName == "" {
				if got != nil {
					t.Fatal("invented missing call")
				}
				return
			}
			if got == nil || got.Function.Name != fixture.RecoveredName {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestRecoveryRejectsEveryTruncation(t *testing.T) {
	span := `<|tool_call>:edit{path:<|"|>a.ts<|"|>,oldText:<|"|>old<|"|>,newText:<|"|>new<|"|>}<tool_call|>`
	for i := 0; i < len(span); i++ {
		got, _ := recoverCall(span[:i])
		if got != nil {
			t.Fatalf("recovered partial span at byte %d", i)
		}
	}
}

func TestRecoveryDepthAndSyntaxLimits(t *testing.T) {
	for _, body := range []string{
		`{x:` + strings.Repeat("[", 70) + `0` + strings.Repeat("]", 70) + `}`,
		`{x:[1,]}`, `{x:[1 2]}`, `{x:1,}`, `{x:1 y:2}`, `{:1}`, `{x:}`, `{x:[`,
	} {
		if got, err := recoverCall(callAnchor + ":read" + body + callClose); got != nil || err == nil {
			t.Fatalf("accepted %s: %+v %v", body, got, err)
		}
	}
}

// Run seed cases in ordinary CI; -fuzz=FuzzLeakedMarkupRecovery extends the corpus without loading a model.
func FuzzLeakedMarkupRecovery(f *testing.F) {
	for _, fixture := range regressions(f) {
		f.Add(fixture.Content + fixture.Reasoning)
	}
	for _, seed := range []string{"", "<|tool_call>", `<|tool_call>:read{path:<|"|>a<|"|>}<tool_call|>`, strings.Repeat("[", 100)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 64<<10 {
			t.Skip()
		}
		got, err := recoverCall(text)
		if err != nil && got != nil {
			t.Fatal("error must not expose an executable call")
		}
		if got == nil {
			return
		}
		if !namePattern.MatchString(got.Function.Name) || !json.Valid([]byte(got.Function.Arguments)) {
			t.Fatalf("invalid recovered call: %+v", got)
		}
		decoder := json.NewDecoder(bytes.NewBufferString(got.Function.Arguments))
		var value any
		if err := decoder.Decode(&value); err != nil {
			t.Fatal(err)
		}
		if _, ok := value.(map[string]any); !ok {
			t.Fatalf("arguments are not an object: %T", value)
		}
	})
}
