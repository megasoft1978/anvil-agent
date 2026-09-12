package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSamplingDefaults locks in Qwen's own documented sampling recommendation for this
// model in instruct/coding use, not greedy decoding: the model card explicitly warns
// greedy decoding can cause endless repetition, which is the exact failure this harness
// otherwise fights structurally (the ledger, read-dedup, the per-turn tool ban). A fixed
// seed keeps runs reproducible for the regression suite despite non-zero temperature.
func TestSamplingDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Temperature != 0.7 {
			t.Errorf("temperature %v, want 0.7", request.Temperature)
		}
		if request.TopP == nil || *request.TopP != 0.8 {
			t.Errorf("top_p %v, want 0.8", request.TopP)
		}
		if request.TopK == nil || *request.TopK != 20 {
			t.Errorf("top_k %v, want 20", request.TopK)
		}
		if request.PresencePenalty == nil || *request.PresencePenalty != 1.0 {
			t.Errorf("presence_penalty %v, want 1.0", request.PresencePenalty)
		}
		if request.RepeatPenalty == nil || *request.RepeatPenalty != 1.05 {
			t.Errorf("repeat_penalty %v, want 1.05", request.RepeatPenalty)
		}
		if request.Seed == nil || *request.Seed != 42 {
			t.Errorf("seed %v, want a fixed default for reproducibility", request.Seed)
		}
		if request.MinP != nil {
			t.Error("min_p is not part of the documented recommendation; expected unset")
		}
		reply(w, Message{Content: "done"}, "stop")
	}))
	defer server.Close()
	args := []string{"--root", t.TempDir(), "--output", t.TempDir(), "--endpoint", server.URL, "--prompt", "finish"}
	var out, stderr bytes.Buffer
	if code := cli(context.Background(), args, &out, &stderr); code != 0 {
		t.Fatalf("exit %d: %s %s", code, out.String(), stderr.String())
	}
}
