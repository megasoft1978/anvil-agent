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
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// The pilot is an explicit, trusted local fixture input, not model context.
//
// TestCommand and SourceFiles are optional for compatibility with the original date-fns/dayjs
// pilot. New real-repository fixtures should set both explicitly.
type repoTask struct {
	Report      string            `json:"report"`
	Tests       map[string]string `json:"test_files"`
	Fail        []string          `json:"fail_to_pass"`
	Pass        []string          `json:"pass_to_pass"`
	TestCommand []string          `json:"test_command,omitempty"`
	SourceFiles []string          `json:"source_files,omitempty"`
}

var repoTestFileArgPattern = regexp.MustCompile(`\.(test|spec)\.[cm]?[jt]sx?$`)

// repoTestNameVariants accepts both the names emitted by a test runner that includes its test
// file and the shorter names recorded in the curated oracle. Vitest can prefix assertion names
// with the test-file argument when several projects are run together; the Node grader applies the
// same normalization for the benchmark path.
func repoTestNameVariants(name string, testCommand []string) []string {
	trimmed := strings.TrimSpace(name)
	variants := []string{trimmed}
	seen := map[string]bool{trimmed: true}
	for _, arg := range testCommand {
		prefix := strings.TrimSpace(arg)
		if !repoTestFileArgPattern.MatchString(prefix) {
			continue
		}
		candidate := prefix + " " + trimmed
		if strings.HasPrefix(trimmed, prefix+" ") {
			candidate = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix+" "))
		}
		if !seen[candidate] {
			seen[candidate] = true
			variants = append(variants, candidate)
		}
	}
	return variants
}

// isSource reports whether path is one of this task's fixable source files: the set the model is
// allowed to change, used both to decide what to preload as context and to catch a run that
// edited something out of scope (a test, a config file) instead of the real source.
func (task repoTask) isSource(stage, path string) bool {
	if len(task.SourceFiles) > 0 {
		for _, candidate := range task.SourceFiles {
			if candidate == path {
				return true
			}
		}
		return false
	}
	return legacySource(stage, path)
}

func legacySource(stage, path string) bool {
	if strings.HasPrefix(stage, "date-fns-") {
		return path == "src/isWithinInterval/index.ts"
	}
	return strings.HasPrefix(stage, "dayjs-") && (path == "src/plugin/utc/index.js" || path == "src/index.js")
}

// legacyCommand reproduces the original hardcoded per-stage test invocation, kept only so the
// original date-fns/dayjs pilot manifest (which has no test_command field) keeps running.
func legacyCommand(stage, report string) []string {
	if strings.HasPrefix(stage, "date-fns-") {
		return []string{"node", "node_modules/vitest/vitest.mjs", "run", "src/isWithinInterval/test.ts", "--reporter=json", "--outputFile=" + report}
	}
	return []string{"node", "node_modules/jest/bin/jest.js", "--runInBand", "--json", "--outputFile=" + report, "--coverageDirectory=" + filepath.Join(filepath.Dir(report), "coverage"), "--", "test/plugin/utc-utcOffset.test.js"}
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
		// Skip node_modules/.git wherever they occur, not just at the fixture root: a pnpm/yarn
		// workspace (observed on zod) has a per-package node_modules too. Each one can be either
		// a real directory or a symlink (root node_modules is always symlinked in by prepareRepo;
		// pnpm also symlinks individual workspace packages into each other's node_modules) — a
		// symlink is not IsDir() under WalkDir's Lstat semantics, so it must be accepted here
		// rather than falling through to the "must be a regular file" check below, which would
		// otherwise reject it.
		if name := entry.Name(); name == "node_modules" || name == ".git" {
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

func copyPreparedRepo(t *testing.T, source, root string) {
	t.Helper()
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
	// Dependencies are prepared outside the model-visible source snapshot and linked only into
	// the disposable model/verification roots. Hidden evaluator files are installed later in the
	// verification root, after the model process has finished.
	if info, err := os.Stat(filepath.Join(source, "node_modules")); err != nil || !info.IsDir() {
		t.Fatal("prepared dependencies missing")
	}
	if err := os.Symlink(filepath.Join(source, "node_modules"), filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
}

func installTaskTests(t *testing.T, root string, task repoTask) {
	t.Helper()
	for rel, oracle := range task.Tests {
		if !filepath.IsLocal(rel) {
			t.Fatal("oracle path escapes fixture")
		}
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(oracle), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func taskCommand(task repoTask, stage, report string) []string {
	if len(task.TestCommand) > 0 {
		command := make([]string, len(task.TestCommand))
		for i, arg := range task.TestCommand {
			command[i] = strings.ReplaceAll(arg, "{{REPORT}}", report)
		}
		return command
	}
	return legacyCommand(stage, report)
}

func prepareVerificationRepo(t *testing.T, source string, task repoTask, stage, report string) (string, []string, map[string][32]byte) {
	t.Helper()
	root := t.TempDir()
	copyPreparedRepo(t, source, root)
	installTaskTests(t, root, task)
	before, err := repoSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, taskCommand(task, stage, report), before
}

func prepareRepo(t *testing.T, pilot, stage, root, report string) (repoTask, []string, map[string][32]byte) {
	t.Helper()
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
	copyPreparedRepo(t, source, root)
	command := taskCommand(task, stage, report)
	before, err := repoSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	// Source hints vary by stage. Shell invocation hints are removed from model context; the host
	// test runner executes the trusted verification command after the model finishes.
	if index := strings.Index(task.Report, "Test command:"); index >= 0 {
		task.Report = task.Report[:index]
	}
	task.Report += "\nFix the source, then finish. Do not change tests, configuration, or dependencies."
	if os.Getenv("ANVIL_PRELOAD_SOURCES") == "1" {
		var paths []string
		for path := range before {
			if task.isSource(stage, path) {
				if selected := os.Getenv("ANVIL_PRELOAD_FILE"); selected != "" && selected != path {
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
	if hint := os.Getenv("ANVIL_DIAGNOSIS_HINT"); hint != "" {
		if os.Getenv("ANVIL_PRELOAD_SOURCES") != "1" {
			t.Fatal("ANVIL_DIAGNOSIS_HINT requires ANVIL_PRELOAD_SOURCES=1 (diagnosis-assisted is a source-supplied condition)")
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
	// Only tests actually named in fail_to_pass/pass_to_pass are graded; everything else in the
	// report is ignored. The original design graded every test in the file, which works when
	// Pass+Fail enumerate the whole (small, curated) suite, as date-fns/dayjs's fixtures do — but
	// a real repo's own test file can be large and can contain unrelated tests, including (observed
	// on immer's real suite) two structurally distinct tests that happen to render the same
	// fullName. pass_to_pass is meant to be a representative regression sample, not a requirement
	// that the entire file be independently known-clean; scoping the grade to the tracked set is
	// what makes that true.
	//
	// A tracked name can legitimately appear more than once in one report: some real suites
	// (observed on zod) run every test file across multiple vitest "projects" (type-check, esm,
	// cjs, ...), so the same logical test executes several times per run. Rather than treat a
	// repeat as ambiguous, require every occurrence of a tracked name to agree on status — that
	// still catches genuine flakiness (one project passing, another failing) as a hard error.
	testCommand := task.TestCommand
	tracked := map[string]bool{}
	for _, name := range task.Pass {
		for _, variant := range repoTestNameVariants(name, testCommand) {
			tracked[variant] = true
		}
	}
	for _, name := range task.Fail {
		for _, variant := range repoTestNameVariants(name, testCommand) {
			tracked[variant] = true
		}
	}
	actual := map[string][]string{}
	for _, suite := range report.TestResults {
		for _, test := range suite.AssertionResults {
			name := strings.TrimSpace(test.FullName)
			if !tracked[name] {
				continue
			}
			actual[name] = append(actual[name], test.Status)
		}
	}
	agrees := func(name, want string) bool {
		var statuses []string
		for _, variant := range repoTestNameVariants(name, testCommand) {
			statuses = append(statuses, actual[variant]...)
		}
		if len(statuses) == 0 {
			return false
		}
		for _, status := range statuses {
			if status != want {
				return false
			}
		}
		return true
	}
	for _, name := range task.Pass {
		if !agrees(name, "passed") {
			return fmt.Errorf("regression/missing test: %s", name)
		}
	}
	want := "failed"
	if fixed {
		want = "passed"
	}
	for _, name := range task.Fail {
		if !agrees(name, want) {
			return fmt.Errorf("expected %s: %s", want, name)
		}
	}
	return nil
}

func checkRepoChanges(task repoTask, stage string, before, after map[string][32]byte) error {
	changed := false
	for path, hash := range before {
		if next, exists := after[path]; !exists || next != hash {
			if !exists || !task.isSource(stage, path) {
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

func selectedPilotStages(t *testing.T, pilot string) []string {
	t.Helper()
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
	if raw := strings.TrimSpace(os.Getenv("ANVIL_BASELINE_TASKS")); raw != "" {
		stages := strings.Split(raw, ",")
		for i := range stages {
			stages[i] = strings.TrimSpace(stages[i])
			if _, ok := manifest.Tasks[stages[i]]; !ok || stages[i] == "" {
				t.Fatalf("ANVIL_BASELINE_TASKS names unknown pilot task %q", stages[i])
			}
		}
		return stages
	}
	stages := make([]string, 0, len(manifest.Tasks))
	for stage := range manifest.Tasks {
		stages = append(stages, stage)
	}
	sort.Strings(stages)
	return stages
}

// Run before loading the model. This proves the prepared reports reproduce the original bugs.
func TestPreparedRealRepoBaselines(t *testing.T) {
	pilot := os.Getenv("ANVIL_PILOT")
	if pilot == "" {
		t.Skip("requires ANVIL_PILOT prepared pilot directory")
	}
	for _, stage := range selectedPilotStages(t, pilot) {
		t.Run(stage, func(t *testing.T) {
			modelRoot := t.TempDir()
			report := filepath.Join(t.TempDir(), "baseline.json")
			task, _, _ := prepareRepo(t, pilot, stage, modelRoot, report)
			verificationRoot, command, before := prepareVerificationRepo(t, modelRoot, task, stage, report)
			result, err := runCommand(context.Background(), verificationRoot, command, 30*time.Second)
			if err != nil || result.TimedOut || result.ExitCode != 1 {
				t.Fatalf("baseline: %+v %v", result, err)
			}
			if err := checkRepoReport(report, task, false); err != nil {
				t.Fatalf("%v\n%s", err, result.Output)
			}
			after, err := repoSnapshot(verificationRoot)
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

func TestRepoReportNamePrefixes(t *testing.T) {
	const prefix = "packages/example/tests/example.test.ts"
	testCommand := []string{"node", "node_modules/vitest/vitest.mjs", "run", prefix, "--reporter=json"}
	if got, want := repoTestNameVariants("bug", testCommand), []string{"bug", prefix + " bug"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unprefixed variants = %#v, want %#v", got, want)
	}
	if got, want := repoTestNameVariants(prefix+" bug", testCommand), []string{prefix + " bug", "bug"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("prefixed variants = %#v, want %#v", got, want)
	}

	path := filepath.Join(t.TempDir(), "report.json")
	report := fmt.Sprintf(`{"testResults":[{"assertionResults":[{"fullName":%q,"status":"failed"},{"fullName":%q,"status":"passed"}]}]}`, prefix+" bug", prefix+" stable")
	if err := os.WriteFile(path, []byte(report), 0600); err != nil {
		t.Fatal(err)
	}
	task := repoTask{Fail: []string{"bug"}, Pass: []string{"stable"}, TestCommand: testCommand}
	if err := checkRepoReport(path, task, false); err != nil {
		t.Fatalf("prefixed report was rejected: %v", err)
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
	if err := checkRepoChanges(repoTask{}, "date-fns-guided", before, after); err != nil {
		t.Fatal(err)
	}
	after["test.ts"] = sha256.Sum256([]byte("cheat"))
	if err := checkRepoChanges(repoTask{}, "date-fns-guided", before, after); err == nil {
		t.Fatal("accepted test tampering")
	}
}
