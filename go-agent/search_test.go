package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	tools, _ := newTools(t)
	tools.SearchEnabled = true
	if err := os.WriteFile(filepath.Join(tools.Root.Name(), "a"), []byte("header\nclone() {\n return this\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	value, err := tools.Execute(context.Background(), call("search", `{"path":"a","text":"clone("}`))
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	if result["matches"] != 1 || !strings.Contains(result["content"].(string), "2: clone() {") || result["truncated"] != false {
		t.Fatalf("%+v", result)
	}
	for _, args := range []string{`{"path":"../a","text":"x"}`, `{"path":".git/config","text":"x"}`, `{"path":"a","text":""}`, `{"path":"a","text":"x","offset":1}`} {
		if _, err := tools.Execute(context.Background(), call("search", args)); err == nil {
			t.Errorf("accepted %s", args)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tools.Root.Name(), "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := tools.Execute(context.Background(), call("search", `{"path":"link","text":"secret"}`)); err == nil {
		t.Fatal("followed escaping symlink")
	}
	tools.SearchEnabled = false
	if _, err := tools.Execute(context.Background(), call("search", `{"path":"a","text":"clone"}`)); err == nil {
		t.Fatal("search ran when disabled")
	}
}

func TestSearchRepoWide(t *testing.T) {
	tools, _ := newTools(t)
	tools.SearchEnabled = true
	root := tools.Root.Name()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("needle in a\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.txt"), []byte("needle in b\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "pkg", "c.txt"), []byte("needle in node_modules\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// No path: defaults to the whole worktree.
	value, err := tools.Execute(context.Background(), call("search", `{"text":"needle"}`))
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	if result["matches"] != 2 {
		t.Fatalf("expected 2 matches (a.txt, sub/b.txt), skipping node_modules: %+v", result)
	}
	content := result["content"].(string)
	if !strings.Contains(content, "a.txt:") || !strings.Contains(content, "sub/b.txt:") || strings.Contains(content, "node_modules") {
		t.Fatalf("unexpected content: %q", content)
	}

	// Explicit "." behaves the same as omitting path.
	value, err = tools.Execute(context.Background(), call("search", `{"path":".","text":"needle"}`))
	if err != nil {
		t.Fatal(err)
	}
	if value.(map[string]any)["matches"] != 2 {
		t.Fatalf("explicit . should match repo-wide default: %+v", value)
	}

	// A subdirectory path restricts the search to that subtree.
	value, err = tools.Execute(context.Background(), call("search", `{"path":"sub","text":"needle"}`))
	if err != nil {
		t.Fatal(err)
	}
	result = value.(map[string]any)
	if result["matches"] != 1 || !strings.Contains(result["content"].(string), "sub/b.txt:") {
		t.Fatalf("subtree restriction failed: %+v", result)
	}
}

func TestSearchLimitsAndSchema(t *testing.T) {
	tools, _ := newTools(t)
	tools.SearchEnabled = true
	if err := os.WriteFile(filepath.Join(tools.Root.Name(), "a"), []byte(strings.Repeat("needle\n", 50)), 0600); err != nil {
		t.Fatal(err)
	}
	value, err := tools.Execute(context.Background(), call("search", `{"path":"a","text":"needle"}`))
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	if result["matches"] != 20 || result["truncated"] != true || len(result["content"].(string)) > maxOutputBytes {
		t.Fatalf("%+v", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tools.Execute(ctx, call("search", `{"path":"a","text":"needle"}`)); err == nil {
		t.Fatal("ignored cancellation")
	}
	for _, enabled := range []bool{false, true} {
		data, err := json.Marshal(toolDefinitions(true, enabled))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"name":"search"`) != enabled {
			t.Fatal("wrong search schema exposure")
		}
	}
}
