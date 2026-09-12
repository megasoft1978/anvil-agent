package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSamplingProfiles(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		temperature float64
		gemma       bool
		seed        bool
	}{
		{"baseline", nil, 0, false, false},
		{"gemma", []string{"--sampling-profile", "gemma"}, 1, true, false},
		{"override", []string{"--sampling-profile", "gemma", "--temperature", "0.3", "--seed", "0"}, .3, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request Request
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if request.Temperature != tc.temperature {
					t.Errorf("temperature %v", request.Temperature)
				}
				if tc.gemma {
					if request.TopP == nil || *request.TopP != .95 || request.TopK == nil || *request.TopK != 64 || request.MinP == nil || *request.MinP != 0 {
						t.Error("missing explicit Gemma sampling parameters")
					}
				} else if request.TopP != nil || request.TopK != nil || request.MinP != nil {
					t.Error("baseline sampling changed")
				}
				if tc.seed {
					if request.Seed == nil || *request.Seed != 0 {
						t.Error("zero seed omitted")
					}
				} else if request.Seed != nil {
					t.Error("unexpected seed")
				}
				reply(w, Message{Content: "done"}, "stop")
			}))
			defer server.Close()
			args := append([]string{"--root", t.TempDir(), "--output", t.TempDir(), "--endpoint", server.URL, "--prompt", "finish"}, tc.args...)
			var out, stderr bytes.Buffer
			if code := cli(context.Background(), args, &out, &stderr); code != 0 {
				t.Fatalf("exit %d: %s %s", code, out.String(), stderr.String())
			}
		})
	}
}
