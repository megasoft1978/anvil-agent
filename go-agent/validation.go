package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	validationReportPath = ".anvil-validation/report.json"
	validationTimeout    = 2 * time.Minute
)

// ValidationCheck is a command the harness discovered from the repository or task. Args are
// executed directly with no shell, so a model can request project validation without gaining an
// arbitrary command runner.
type ValidationCheck struct {
	Kind       string   `json:"kind"`
	Args       []string `json:"args"`
	Display    string   `json:"command"`
	Source     string   `json:"source"`
	ReportPath string   `json:"report_path,omitempty"`
	Auto       bool     `json:"auto,omitempty"`
}

// ValidationPlan is derived from the worktree and the task text. It is deliberately small and
// serializable so the run trace records exactly which checks were offered to the model.
type ValidationPlan struct {
	Checks     map[string]ValidationCheck `json:"checks"`
	Frameworks []string                   `json:"frameworks,omitempty"`
}

func (p *ValidationPlan) kinds() []string {
	if p == nil {
		return nil
	}
	preferred := []string{"test", "diagnostics", "typecheck", "build", "lint"}
	seen := map[string]bool{}
	var result []string
	for _, kind := range preferred {
		if _, ok := p.Checks[kind]; ok {
			result = append(result, kind)
			seen[kind] = true
		}
	}
	var extra []string
	for kind := range p.Checks {
		if !seen[kind] {
			extra = append(extra, kind)
		}
	}
	sort.Strings(extra)
	result = append(result, extra...)
	return result
}

func (p *ValidationPlan) prompt() string {
	if p == nil || len(p.Checks) == 0 {
		return ""
	}
	kinds := p.kinds()
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		check := p.Checks[kind]
		parts = append(parts, fmt.Sprintf("%s (%s)", kind, check.Display))
	}
	frameworks := ""
	if len(p.Frameworks) > 0 {
		frameworks = " Detected project types: " + strings.Join(p.Frameworks, ", ") + "."
	}
	return "Validation is available through the native validate tool. Discovered checks: " + strings.Join(parts, "; ") + "." + frameworks + " An edit may automatically run the test check and return its result; use validate to rerun a check or inspect compiler/test diagnostics."
}

type packageManifest struct {
	Scripts         map[string]string          `json:"scripts"`
	Dependencies    map[string]json.RawMessage `json:"dependencies"`
	DevDependencies map[string]json.RawMessage `json:"devDependencies"`
	PackageManager  string                     `json:"packageManager"`
}

func detectValidationPlan(root, prompt string) *ValidationPlan {
	plan := &ValidationPlan{Checks: map[string]ValidationCheck{}}
	manifest := packageManifest{}
	packageData, packageErr := os.ReadFile(filepath.Join(root, "package.json"))
	if packageErr == nil {
		_ = json.Unmarshal(packageData, &manifest)
	}

	if packageErr == nil {
		if _, ok := manifest.Dependencies["typescript"]; ok {
			plan.Frameworks = append(plan.Frameworks, "TypeScript")
		}
		if _, ok := manifest.DevDependencies["typescript"]; ok {
			plan.Frameworks = append(plan.Frameworks, "TypeScript")
		}
		for _, name := range []string{"react", "react-dom", "next", "vite"} {
			if _, ok := manifest.Dependencies[name]; ok {
				plan.Frameworks = append(plan.Frameworks, "React/Node")
				break
			}
			if _, ok := manifest.DevDependencies[name]; ok {
				plan.Frameworks = append(plan.Frameworks, "React/Node")
				break
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
		plan.Frameworks = append(plan.Frameworks, "TypeScript")
	}
	plan.Frameworks = uniqueStrings(plan.Frameworks)

	// A task-provided command is the strongest signal. The common benchmark form uses <report>
	// as a placeholder; redirect it into a generated directory so validation does not overwrite
	// a repository file.
	if raw := declaredTestCommand(prompt); raw != "" {
		if args, ok := safeValidationArgs(raw, validationReportPath); ok {
			plan.Checks["test"] = ValidationCheck{Kind: "test", Args: args, Display: strings.Join(args, " "), Source: "task", ReportPath: validationReportPath, Auto: true}
		}
	}

	manager := packageManager(root, manifest.PackageManager)
	if manager != "" && packageErr == nil {
		for _, candidate := range []struct {
			kind  string
			names []string
		}{
			{kind: "test", names: []string{"test"}},
			{kind: "typecheck", names: []string{"typecheck", "type-check", "check-types"}},
			{kind: "build", names: []string{"build", "compile"}},
			{kind: "lint", names: []string{"lint"}},
		} {
			if _, exists := plan.Checks[candidate.kind]; exists {
				continue
			}
			if script := firstScript(manifest.Scripts, candidate.names); script != "" {
				args := []string{manager, "run", script}
				plan.Checks[candidate.kind] = ValidationCheck{Kind: candidate.kind, Args: args, Display: strings.Join(args, " "), Source: "package.json"}
				if candidate.kind == "test" {
					plan.Checks[candidate.kind] = withAuto(plan.Checks[candidate.kind])
				}
			}
		}
	}

	// A TypeScript project without a typecheck script can still expose compiler diagnostics when
	// its local compiler is installed. Adding the worktree's bin directory to PATH happens when the
	// check runs, so this remains package-manager independent and cannot invoke npx downloads.
	if _, exists := plan.Checks["typecheck"]; !exists && hasLocalBinary(root, "tsc") {
		check := ValidationCheck{Kind: "typecheck", Args: []string{"node_modules/.bin/tsc", "--noEmit"}, Display: "node_modules/.bin/tsc --noEmit", Source: "tsconfig.json"}
		plan.Checks["typecheck"] = check
	}
	if _, exists := plan.Checks["lint"]; !exists && hasLocalBinary(root, "eslint") {
		check := ValidationCheck{Kind: "lint", Args: []string{"node_modules/.bin/eslint", "."}, Display: "node_modules/.bin/eslint .", Source: "eslint installation"}
		plan.Checks["lint"] = check
	}
	if _, exists := plan.Checks["diagnostics"]; !exists {
		if check, ok := plan.Checks["typecheck"]; ok {
			check.Kind = "diagnostics"
			check.Source = check.Source + ", compiler diagnostics"
			plan.Checks["diagnostics"] = check
		}
	}

	// Keep the Go agent's own repositories useful without making this a shell tool. This also
	// makes the detection behavior generic for local tasks outside the JS benchmark.
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		if _, ok := plan.Checks["test"]; !ok && commandAvailable("go") {
			args := []string{"go", "test", "./..."}
			plan.Checks["test"] = ValidationCheck{Kind: "test", Args: args, Display: strings.Join(args, " "), Source: "go.mod", Auto: true}
		}
	}

	return plan
}

func withAuto(check ValidationCheck) ValidationCheck {
	check.Auto = true
	return check
}

func declaredTestCommand(prompt string) string {
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Test command:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Test command:"))
		}
	}
	return ""
}

func safeValidationArgs(raw, reportPath string) ([]string, bool) {
	raw = strings.ReplaceAll(raw, "<report>", reportPath)
	raw = strings.ReplaceAll(raw, "{{REPORT}}", reportPath)
	if raw == "" || strings.ContainsAny(raw, ";&|<>`$()") {
		return nil, false
	}
	args := strings.Fields(raw)
	if len(args) == 0 || !validationExecutable(args[0]) || !commandAvailable(args[0]) {
		return nil, false
	}
	for _, arg := range args {
		if strings.Contains(arg, "\x00") {
			return nil, false
		}
	}
	return args, true
}

func validationExecutable(command string) bool {
	base := filepath.Base(command)
	switch base {
	case "node", "npm", "pnpm", "yarn", "bun", "tsc", "eslint", "biome", "prettier", "vitest", "jest", "go":
		return true
	default:
		return false
	}
}

func commandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

func hasLocalBinary(root, name string) bool {
	path := filepath.Join(root, "node_modules", ".bin", name)
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func packageManager(root, declared string) string {
	if at := strings.IndexByte(declared, '@'); at > 0 {
		declared = declared[:at]
	}
	if declared == "npm" || declared == "pnpm" || declared == "yarn" || declared == "bun" {
		if commandAvailable(declared) {
			return declared
		}
	}
	for _, candidate := range []struct {
		file string
		name string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lock", "bun"},
		{"bun.lockb", "bun"},
		{"package-lock.json", "npm"},
	} {
		if _, err := os.Stat(filepath.Join(root, candidate.file)); err == nil && commandAvailable(candidate.name) {
			return candidate.name
		}
	}
	if commandAvailable("npm") {
		return "npm"
	}
	return ""
}

func firstScript(scripts map[string]string, names []string) string {
	for _, name := range names {
		if _, ok := scripts[name]; ok {
			return name
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func (t *Tools) runValidation(ctx context.Context, check ValidationCheck) (map[string]any, error) {
	if t.Worktree == "" {
		return nil, fmt.Errorf("validation worktree is not configured")
	}
	if len(check.Args) == 0 || !validationExecutable(check.Args[0]) {
		return nil, fmt.Errorf("validation command is not allowed")
	}
	if err := os.MkdirAll(filepath.Join(t.Worktree, ".anvil-validation"), 0700); err != nil {
		return nil, err
	}
	commandResult, err := runCommand(ctx, t.Worktree, check.Args, validationTimeout)
	result := map[string]any{
		"kind":             check.Kind,
		"command":          check.Display,
		"source":           check.Source,
		"duration_ms":      commandResult.WallMS,
		"output":           commandResult.Output,
		"output_truncated": commandResult.Truncated,
		"exit_code":        commandResult.ExitCode,
	}
	if commandResult.TimedOut {
		result["status"] = "timed_out"
	} else if err == nil && commandResult.ExitCode == 0 {
		result["status"] = "passed"
	} else if errors.Is(err, exec.ErrNotFound) {
		result["status"] = "unavailable"
	} else if err != nil {
		result["status"] = "error"
	} else {
		result["status"] = "failed"
	}
	if check.ReportPath != "" {
		if summary := summarizeValidationReport(filepath.Join(t.Worktree, check.ReportPath)); summary != nil {
			result["report"] = summary
		}
	}
	if t.Trace != nil {
		if traceErr := t.Trace.Event("validation", result); traceErr != nil {
			return nil, traceErr
		}
	}
	return result, nil
}

func summarizeValidationReport(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var report map[string]any
	if json.Unmarshal(data, &report) != nil {
		return nil
	}
	assertions := []any{}
	if results, ok := report["testResults"].([]any); ok {
		for _, item := range results {
			if result, ok := item.(map[string]any); ok {
				if values, ok := result["assertionResults"].([]any); ok {
					assertions = append(assertions, values...)
				}
			}
		}
	}
	if len(assertions) == 0 {
		if values, ok := report["assertionResults"].([]any); ok {
			assertions = values
		}
	}
	if len(assertions) == 0 {
		return map[string]any{"available": true, "keys": sortedMapKeys(report)}
	}
	passed, failed := 0, 0
	failedNames := []string{}
	for _, item := range assertions {
		value, _ := item.(map[string]any)
		status, _ := value["Status"].(string)
		if status == "passed" || status == "pass" {
			passed++
		} else {
			failed++
			if name, ok := value["FullName"].(string); ok {
				failedNames = append(failedNames, name)
			} else if name, ok := value["name"].(string); ok {
				failedNames = append(failedNames, name)
			}
		}
	}
	return map[string]any{"total": len(assertions), "passed": passed, "failed": failed, "failed_names": failedNames}
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
