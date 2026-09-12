package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSamplingDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Temperature != 0 {
			t.Errorf("temperature %v", request.Temperature)
		}
		if request.TopP != nil || request.TopK != nil || request.MinP != nil || request.Seed != nil {
			t.Error("no sampling overrides expected by default")
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
