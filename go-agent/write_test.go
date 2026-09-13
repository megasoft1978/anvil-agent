package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteDisabledByDefault(t *testing.T) {
	tools, _ := newTools(t)
	if _, err := tools.Execute(context.Background(), call("write", `{"path":"a","content":"x"}`)); err == nil {
		t.Fatal("write should be disabled unless WriteEnabled is set")
	}
}

func TestWriteCreatesFile(t *testing.T) {
	tools, _ := newTools(t)
	tools.WriteEnabled = true
	result, err := tools.Execute(context.Background(), call("write", `{"path":"new.txt","content":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["created"] != true || m["bytes"] != 5 {
		t.Fatalf("%+v", m)
	}
	data, err := os.ReadFile(filepath.Join(tools.Root.Name(), "new.txt"))
	if err != nil || string(data) != "hello" {
		t.Fatalf("%s %v", data, err)
	}
	if !tools.Edited["new.txt"] {
		t.Fatal("write should mark the file as edited")
	}
}

func TestWriteRefusesExistingFile(t *testing.T) {
	tools, _ := newTools(t)
	tools.WriteEnabled = true
	if err := os.WriteFile(filepath.Join(tools.Root.Name(), "already.txt"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := tools.Execute(context.Background(), call("write", `{"path":"already.txt","content":"new"}`)); err == nil {
		t.Fatal("write should refuse to overwrite an existing file")
	}
	data, err := os.ReadFile(filepath.Join(tools.Root.Name(), "already.txt"))
	if err != nil || string(data) != "old" {
		t.Fatalf("existing file must be untouched: %s %v", data, err)
	}
}

func TestWriteRefusesEscape(t *testing.T) {
	tools, _ := newTools(t)
	tools.WriteEnabled = true
	for _, path := range []string{"../escape.txt", "/etc/escape.txt", "../../x"} {
		if _, err := tools.Execute(context.Background(), call("write", `{"path":"`+path+`","content":"x"}`)); err == nil {
			t.Fatalf("write should refuse escaping path %q", path)
		}
	}
}

func TestWriteCreatesNestedDirs(t *testing.T) {
	tools, _ := newTools(t)
	tools.WriteEnabled = true
	if _, err := tools.Execute(context.Background(), call("write", `{"path":"a/b/c/new.txt","content":"nested"}`)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tools.Root.Name(), "a/b/c/new.txt"))
	if err != nil || string(data) != "nested" {
		t.Fatalf("%s %v", data, err)
	}
}

func TestToolDefinitionsIncludeWriteOnlyWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		defs := toolDefinitions(false, false, enabled)
		found := false
		for _, d := range defs {
			if d.Function.Name == "write" {
				found = true
			}
		}
		if found != enabled {
			t.Fatalf("write tool presence should match enabled=%v, got %v", enabled, found)
		}
	}
}
