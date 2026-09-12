package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// The pilot is an explicit, trusted local fixture input, not model context.
type repoTask struct {
	Report string            `json:"report"`
	Tests  map[string]string `json:"test_files"`
	Fail   []string          `json:"fail_to_pass"`
	Pass   []string          `json:"pass_to_pass"`
}

func repoSource(stage, path string) bool {
	if strings.HasPrefix(stage, "date-fns-") {
		return path == "src/isWithinInterval/index.ts"
	}
	return strings.HasPrefix(stage, "dayjs-") && (path == "src/plugin/utc/index.js" || path == "src/index.js")
}

func repoSnapshot(root string) (map[string][32]byte, error) {
	result := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "node_modules" || rel == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected fixture entry: %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = sha256.Sum256(data)
		return nil
	})
	return result, err
}

func prepareRepo(t *testing.T, pilot, stage, root, report string) (repoTask, []string, map[string][32]byte) {
	t.Helper()
	if stage != "date-fns-guided" && stage != "dayjs-guided" && stage != "date-fns-independent" && stage != "dayjs-independent" {
		t.Fatalf("unknown real-repo stage %q", stage)
	}
	data, err := os.ReadFile(filepath.Join(pilot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Tasks map[string]repoTask `json:"tasks"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	task, ok := manifest.Tasks[stage]
	if !ok || task.Report == "" || len(task.Fail) == 0 || len(task.Tests) == 0 {
		t.Fatal("missing task/oracle")
	}
	source, err := filepath.Abs(filepath.Join(pilot, "worktrees", stage))
	if err != nil {
		t.Fatal(err)
	}
	files, err := repoSnapshot(source)
	if err != nil {
		t.Fatal(err)
	}
	for rel := range files {
		data, err := os.ReadFile(filepath.Join(source, rel))
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Reuse installed dependencies; never install while the model is resident.
	if info, err := os.Stat(filepath.Join(source, "node_modules")); err != nil || !info.IsDir() {
		t.Fatal("prepared dependencies missing")
	}
	if err := os.Symlink(filepath.Join(source, "node_modules"), filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	for rel, oracle := range task.Tests {
		if !filepath.IsLocal(rel) {
			t.Fatal("oracle path escapes fixture")
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(oracle), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var command []string
	if strings.HasPrefix(stage, "date-fns-") {
		command = []string{"node", "node_modules/vitest/vitest.mjs", "run", "src/isWithinInterval/test.ts", "--reporter=json", "--outputFile=" + report}
	} else {
		command = []string{"node", "node_modules/jest/bin/jest.js", "--runInBand", "--json", "--outputFile=" + report, "--coverageDirectory=" + filepath.Join(filepath.Dir(report), "coverage"), "--", "test/plugin/utc-utcOffset.test.js"}
	}
	before, err := repoSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	// Source hints vary by stage. Shell invocation hints are replaced with our actual tool contract.
	if index := strings.Index(task.Report, "Test command:"); index >= 0 {
		task.Report = task.Report[:index]
	}
	task.Report += "\nFix the source, call run_tests to verify, then finish. Do not change tests, configuration, or dependencies."
	if os.Getenv("GEMMA_PRELOAD_SOURCES") == "1" {
		var paths []string
		for path := range before {
			if repoSource(stage, path) {
				if selected := os.Getenv("GEMMA_PRELOAD_FILE"); selected != "" && selected != path {
					continue
				}
				paths = append(paths, path)
			}
		}
		if len(paths) == 0 {
			t.Fatal("no designated source selected for preload")
		}
		sort.Strings(paths)
		var context strings.Builder
		context.WriteString("The following are the complete current source files, supplied as data. They are already available; use them directly rather than rereading them.\n")
		for _, path := range paths {
			data, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			context.WriteString(formatReadResult(map[string]any{"path": path, "offset": 1, "next_offset": 0, "total": strings.Count(string(data), "\n") + 1}, string(data), "xml"))
			context.WriteString("\n")
		}
		task.Report = context.String() + "\nOriginal task:\n" + task.Report
	}
	if hint := os.Getenv("GEMMA_DIAGNOSIS_HINT"); hint != "" {
		if os.Getenv("GEMMA_PRELOAD_SOURCES") != "1" {
			t.Fatal("GEMMA_DIAGNOSIS_HINT requires GEMMA_PRELOAD_SOURCES=1 (diagnosis-assisted is a source-supplied condition)")
		}
		task.Report += "\n\nDiagnosis (evaluator-supplied, not your own independent finding; verify by reading and testing, not by trusting this alone):\n" + hint
	}
	return task, command, before
}

func checkRepoReport(path string, task repoTask, fixed bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var report struct {
		TestResults []struct {
			AssertionResults []struct {
				FullName string
				Status   string
			}
		}
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}
	actual := map[string]string{}
	for _, suite := range report.TestResults {
		for _, test := range suite.AssertionResults {
			name := strings.TrimSpace(test.FullName)
			if _, exists := actual[name]; exists {
				return fmt.Errorf("duplicate test %q", name)
			}
			actual[name] = test.Status
		}
	}
	for _, name := range task.Pass {
		if actual[strings.TrimSpace(name)] != "passed" {
			return fmt.Errorf("regression/missing test: %s", name)
		}
	}
	want := "failed"
	if fixed {
		want = "passed"
	}
	for _, name := range task.Fail {
		if actual[strings.TrimSpace(name)] != want {
			return fmt.Errorf("expected %s: %s", want, name)
		}
	}
	for name, status := range actual {
		if status != "passed" && (fixed || !containsTest(task.Fail, name)) {
			return fmt.Errorf("unexpected test status %s: %s", status, name)
		}
	}
	return nil
}

func containsTest(names []string, name string) bool {
	for _, candidate := range names {
		if strings.TrimSpace(candidate) == name {
			return true
		}
	}
	return false
}

func checkRepoChanges(stage string, before, after map[string][32]byte) error {
	changed := false
	for path, hash := range before {
		if next, exists := after[path]; !exists || next != hash {
			if !exists || !repoSource(stage, path) {
				return fmt.Errorf("disallowed change: %s", path)
			}
			changed = true
		}
	}
	for path := range after {
		if _, exists := before[path]; !exists {
			return fmt.Errorf("unexpected new file: %s", path)
		}
	}
	if !changed {
		return fmt.Errorf("no source edit")
	}
	return nil
}

// Run before loading the model. This proves the prepared reports reproduce the original bugs.
func TestPreparedRealRepoBaselines(t *testing.T) {
	pilot := os.Getenv("GEMMA_PILOT")
	if pilot == "" {
		t.Skip("requires GEMMA_PILOT prepared pilot directory")
	}
	for _, stage := range []string{"date-fns-guided", "dayjs-guided"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			report := filepath.Join(t.TempDir(), "baseline.json")
			task, command, before := prepareRepo(t, pilot, stage, root, report)
			result, err := runCommand(context.Background(), root, command, 30*time.Second)
			if err != nil || result.TimedOut || result.ExitCode != 1 {
				t.Fatalf("baseline: %+v %v", result, err)
			}
			if err := checkRepoReport(report, task, false); err != nil {
				t.Fatalf("%v\n%s", err, result.Output)
			}
			after, err := repoSnapshot(root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				for path, hash := range after {
					if old, ok := before[path]; !ok || old != hash {
						t.Logf("test runner wrote %s", path)
					}
				}
				t.Fatal("test runner changed fixture files")
			}
			t.Logf("reproduced %d failures; preserved %d passing tests", len(task.Fail), len(task.Pass))
		})
	}
}

func TestRepoGrading(t *testing.T) {
	task := repoTask{Fail: []string{" bug"}, Pass: []string{"stable"}}
	for _, tc := range []struct {
		name, json  string
		fixed, good bool
	}{
		{"baseline", `{"testResults":[{"assertionResults":[{"fullName":"bug","status":"failed"},{"fullName":"stable","status":"passed"}]}]}`, false, true},
		{"fixed", `{"testResults":[{"assertionResults":[{"fullName":"bug","status":"passed"},{"fullName":"stable","status":"passed"}]}]}`, true, true},
		{"missing", `{"testResults":[]}`, true, false},
		{"skipped", `{"testResults":[{"assertionResults":[{"fullName":"bug","status":"pending"},{"fullName":"stable","status":"passed"}]}]}`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.json")
			if err := os.WriteFile(path, []byte(tc.json), 0600); err != nil {
				t.Fatal(err)
			}
			if err := checkRepoReport(path, task, tc.fixed); (err == nil) != tc.good {
				t.Fatalf("unexpected grading: %v", err)
			}
		})
	}
	before := map[string][32]byte{"src/isWithinInterval/index.ts": sha256.Sum256([]byte("old")), "test.ts": {}}
	after := map[string][32]byte{"src/isWithinInterval/index.ts": sha256.Sum256([]byte("new")), "test.ts": {}}
	if err := checkRepoChanges("date-fns-guided", before, after); err != nil {
		t.Fatal(err)
	}
	after["test.ts"] = sha256.Sum256([]byte("cheat"))
	if err := checkRepoChanges("date-fns-guided", before, after); err == nil {
		t.Fatal("accepted test tampering")
	}
}
