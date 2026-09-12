package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

var pageoutPattern = regexp.MustCompile(`(?m)^Pageouts:\s+(\d+)\.`)

func livePageouts() (int64, error) {
	if runtime.GOOS != "darwin" {
		return -1, nil
	}
	output, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, err
	}
	match := pageoutPattern.FindSubmatch(output)
	if len(match) != 2 {
		return 0, fmt.Errorf("vm_stat pageout count missing")
	}
	return strconv.ParseInt(string(match[1]), 10, 64)
}

// This suite never starts a server. Opt in only after arranging enough RAM for the local model.
// Run sequentially; preserve traces outside testing.T's temporary fixture directory.
func TestLiveGemma4(t *testing.T) {
	if os.Getenv("GEMMA_LIVE") != "1" {
		t.Skip("opt-in only: GEMMA_LIVE=1; needs an already-running local Gemma server")
	}
	if os.Getenv("GEMMA_EDIT_ONLY") == "1" && os.Getenv("GEMMA_PRELOAD_SOURCES") != "1" {
		t.Fatal("edit-only real-repo diagnostic requires GEMMA_PRELOAD_SOURCES=1")
	}
	allowPaging := os.Getenv("GEMMA_ALLOW_PAGING") == "1"
	if allowPaging {
		t.Log("paging abort explicitly disabled; memory measurements remain recorded")
	}
	endpoint := os.Getenv("GEMMA_LIVE_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8114/v1"
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if host := u.Hostname(); host != "127.0.0.1" && host != "localhost" && host != "::1" {
		t.Fatal("live suite requires a loopback endpoint")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	request, err := http.NewRequestWithContext(ctx, "GET", u.Scheme+"://"+u.Host+"/health", nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		cancel()
		t.Fatalf("local server not healthy: %v", err)
	}
	response.Body.Close()
	cancel()
	if response.StatusCode != 200 {
		t.Fatalf("health HTTP %d", response.StatusCode)
	}
	output := os.Getenv("GEMMA_LIVE_OUTPUT")
	if output == "" {
		output = "live-results"
	}
	output, err = filepath.Abs(output)
	if err != nil {
		t.Fatal(err)
	}
	model := os.Getenv("GEMMA_LIVE_MODEL")
	if model == "" {
		model = "gemma4"
	}
	timeout := os.Getenv("GEMMA_LIVE_TIMEOUT")
	if timeout == "" {
		timeout = "120s"
	}
	stages := []string{"read-and-finish", "edit-and-test"}
	if stage := os.Getenv("GEMMA_REPO_STAGE"); stage != "" {
		if os.Getenv("GEMMA_PILOT") == "" {
			t.Fatal("GEMMA_REPO_STAGE requires GEMMA_PILOT")
		}
		stages = append(stages, stage)
	}
	for _, stage := range stages {
		if !t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			var random [12]byte
			if _, err := rand.Read(random[:]); err != nil {
				t.Fatal(err)
			}
			marker := "marker-" + hex.EncodeToString(random[:])
			if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte(marker+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"--root", root, "--output", output, "--endpoint", endpoint, "--model", model, "--timeout", timeout}
			if os.Getenv("GEMMA_PRESERVE_TOOL_REASONING") == "1" {
				args = append(args, "--preserve-tool-reasoning")
			}
			if os.Getenv("GEMMA_TASK_REMINDER") == "1" {
				args = append(args, "--task-reminder")
			}
			if profile := os.Getenv("GEMMA_SAMPLING_PROFILE"); profile != "" {
				args = append(args, "--sampling-profile", profile)
			}
			if seed := os.Getenv("GEMMA_SEED"); seed != "" {
				args = append(args, "--seed", seed)
			}
			if os.Getenv("GEMMA_ENABLE_SEARCH") == "1" {
				args = append(args, "--enable-search")
			}
			if os.Getenv("GEMMA_DEDUP_READS") == "1" {
				args = append(args, "--dedup-reads")
			}
			if profile := os.Getenv("GEMMA_PROMPT_PROFILE"); profile != "" {
				args = append(args, "--prompt-profile", profile)
			}
			if format := os.Getenv("GEMMA_READ_FORMAT"); format != "" {
				args = append(args, "--read-format", format)
			}
			if os.Getenv("GEMMA_RICH_EDIT_FEEDBACK") == "1" {
				args = append(args, "--rich-edit-feedback")
			}
			if os.Getenv("GEMMA_AUTO_TEST_AFTER_EDIT") == "1" {
				args = append(args, "--auto-test-after-edit")
			}
			if os.Getenv("GEMMA_DETECT_REPEATED_EDITS") == "1" {
				args = append(args, "--detect-repeated-edits")
			}
			var task repoTask
			var snapshot map[string][32]byte
			report := filepath.Join(t.TempDir(), "oracle.json")
			if stage == "read-and-finish" {
				args = append(args, "--prompt", "Read note.txt using the read tool, then reply with its exact contents. Do not edit any files.")
			} else if stage == "edit-and-test" {
				if err := os.WriteFile(filepath.Join(root, "sum.mjs"), []byte("export const sum = (a,b) => a-b;\n"), 0600); err != nil {
					t.Fatal(err)
				}
				command, _ := json.Marshal([]string{"node", "--input-type=module", "-e", `import assert from 'node:assert/strict'; import {sum} from './sum.mjs'; assert.equal(sum(2,3),5); assert.equal(sum(-2,3),1); assert.equal(sum(0,0),0);`})
				args = append(args, "--prompt", "sum.mjs returns incorrect sums. Read it, fix the source, use run_tests to verify, then finish.", "--test-command", string(command))
			} else {
				var command []string
				oracleDir, err := os.MkdirTemp(output, stage+"-verification-")
				if err != nil {
					t.Fatal(err)
				}
				report = filepath.Join(oracleDir, "oracle.json")
				task, command, snapshot = prepareRepo(t, os.Getenv("GEMMA_PILOT"), stage, root, report)
				if os.Getenv("GEMMA_EDIT_ONLY") == "1" {
					args = append(args, "--edit-only")
				}
				encoded, err := json.Marshal(command)
				if err != nil {
					t.Fatal(err)
				}
				args = append(args, "--prompt", task.Report, "--test-command", string(encoded))
			}
			before, err := livePageouts()
			if err != nil {
				t.Fatal(err)
			}
			ctx, stop := context.WithCancel(context.Background())
			monitor := make(chan error, 1)
			go func() {
				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						monitor <- nil
						return
					case <-ticker.C:
						after, err := livePageouts()
						if err != nil || (!allowPaging && before >= 0 && after > before) {
							if err == nil {
								err = fmt.Errorf("pageouts increased by %d; stopping live suite", after-before)
							}
							stop()
							monitor <- err
							return
						}
					}
				}
			}()
			var stdout, stderr bytes.Buffer
			code := cli(ctx, args, &stdout, &stderr)
			stop()
			monitorErr := <-monitor
			after, memoryErr := livePageouts()
			t.Logf("%s\n%s\npageouts before=%d after=%d", stderr.String(), stdout.String(), before, after)
			if monitorErr != nil {
				t.Fatal(monitorErr)
			}
			if memoryErr != nil || (!allowPaging && before >= 0 && after > before) {
				t.Fatalf("memory contamination: before=%d after=%d error=%v", before, after, memoryErr)
			}
			var summary Summary
			if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
				t.Fatal(err)
			}
			if code != 0 || summary.Status != "completed" {
				t.Fatalf("stage failed: exit=%d %+v", code, summary)
			}
			if summary.ToolCalls < 1 {
				t.Fatal("no tool executed")
			}
			if stage == "read-and-finish" {
				if !strings.Contains(summary.Answer, marker) || len(summary.EditedFiles) != 0 {
					t.Fatal("read/finish did not satisfy fixture")
				}
			} else {
				if summary.Verification == nil || summary.Verification.ExitCode != 0 {
					t.Fatal("no successful final verification")
				}
				if stage == "edit-and-test" {
					if len(summary.EditedFiles) != 1 || summary.EditedFiles[0] != "sum.mjs" {
						t.Fatal("edit stage did not produce a verified source edit")
					}
				} else {
					after, err := repoSnapshot(root)
					if err != nil {
						t.Fatal(err)
					}
					if err := checkRepoChanges(stage, snapshot, after); err != nil {
						t.Fatal(err)
					}
					if err := checkRepoReport(report, task, true); err != nil {
						t.Fatal(err)
					}
					t.Logf("real repo verified: %d fail-to-pass, %d pass-to-pass", len(task.Fail), len(task.Pass))
				}
			}
		}) {
			return
		}
	}
}
