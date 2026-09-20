package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationPlanDetectsTaskAndProjectChecks(t *testing.T) {
	root := t.TempDir()
	manifest := map[string]any{
		"dependencies":    map[string]string{"react": "latest"},
		"devDependencies": map[string]string{"typescript": "latest"},
		"scripts": map[string]string{
			"test":      "vitest run",
			"typecheck": "tsc --noEmit",
			"build":     "vite build",
			"lint":      "eslint .",
		},
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(root, "package.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package-lock.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte(`{"compilerOptions":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	plan := detectValidationPlan(root, "Bug report\n\nTest command: node scripts/check.mjs <report>")
	if _, ok := plan.Checks["test"]; !ok {
		t.Fatalf("task test command was not detected: %+v", plan)
	}
	if got := plan.Checks["test"].Args[len(plan.Checks["test"].Args)-1]; got != "scripts/check.mjs" && !strings.Contains(got, ".anvil-validation/report.json") {
		t.Fatalf("report placeholder was not preserved in command: %+v", plan.Checks["test"])
	}
	for _, kind := range []string{"typecheck", "build", "lint"} {
		if _, ok := plan.Checks[kind]; !ok {
			t.Fatalf("missing package script check %q: %+v", kind, plan.Checks)
		}
	}
	if !strings.Contains(plan.prompt(), "validate") || !strings.Contains(plan.prompt(), "React/Node") {
		t.Fatalf("validation prompt omitted automatic plan: %q", plan.prompt())
	}
}

func TestValidationCommandRejectsShellSyntax(t *testing.T) {
	if _, ok := safeValidationArgs("node --check file.js; touch injected", validationReportPath); ok {
		t.Fatal("accepted shell syntax in validation command")
	}
	if _, ok := safeValidationArgs("sh -c true", validationReportPath); ok {
		t.Fatal("accepted non-allowlisted executable")
	}
}

func TestValidationToolAndAutomaticEditFeedback(t *testing.T) {
	tools, _ := newTools(t)
	tools.Worktree = tools.Root.Name()
	tools.Validation = &ValidationPlan{Checks: map[string]ValidationCheck{
		"test": {Kind: "test", Args: []string{"node", "--check", "target.js"}, Display: "node --check target.js", Source: "test", Auto: true},
	}}
	tools.AutoValidate = true
	path := filepath.Join(tools.Root.Name(), "target.js")
	if err := os.WriteFile(path, []byte("const value = 1;\n"), 0600); err != nil {
		t.Fatal(err)
	}
	value, err := tools.Execute(context.Background(), call("validate", `{"kind":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if value.(map[string]any)["status"] != "passed" {
		t.Fatalf("manual validation failed: %+v", value)
	}
	value, err = tools.Execute(context.Background(), call("edit", `{"path":"target.js","oldText":"const value = 1;","newText":"const value = ;"}`))
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	validation, ok := result["validation"].(map[string]any)
	if !ok || validation["status"] != "failed" {
		t.Fatalf("automatic validation feedback missing: %+v", result)
	}
}
