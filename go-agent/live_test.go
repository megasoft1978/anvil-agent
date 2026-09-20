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
	"reflect"
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
func TestLiveModel(t *testing.T) {
	if os.Getenv("ANVIL_LIVE") != "1" {
		t.Skip("opt-in only: ANVIL_LIVE=1; needs an already-running local model server")
	}
	allowPaging := os.Getenv("ANVIL_ALLOW_PAGING") == "1"
	if allowPaging {
		t.Log("paging abort explicitly disabled; memory measurements remain recorded")
	}
	endpoint := os.Getenv("ANVIL_LIVE_ENDPOINT")
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
	output := os.Getenv("ANVIL_LIVE_OUTPUT")
	if output == "" {
		output = "live-results"
	}
	output, err = filepath.Abs(output)
	if err != nil {
		t.Fatal(err)
	}
	model := os.Getenv("ANVIL_LIVE_MODEL")
	if model == "" {
		model = "qwen3-coder-30b-a3b"
	}
	timeout := os.Getenv("ANVIL_LIVE_TIMEOUT")
	if timeout == "" {
		timeout = "120s"
	}
	maxTokens := os.Getenv("ANVIL_LIVE_MAX_TOKENS")
	stages := []string{"read-and-finish", "edit-and-test"}
	if stage := os.Getenv("ANVIL_REPO_STAGE"); stage != "" {
		if os.Getenv("ANVIL_PILOT") == "" {
			t.Fatal("ANVIL_REPO_STAGE requires ANVIL_PILOT")
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
			if maxTokens != "" {
				args = append(args, "--max-tokens", maxTokens)
			}
			var task repoTask
			var snapshot map[string][32]byte
			var command []string
			report := filepath.Join(t.TempDir(), "oracle.json")
			if stage == "read-and-finish" {
				args = append(args, "--prompt", "Read note.txt using the read tool, then reply with its exact contents. Do not edit any files.")
			} else if stage == "edit-and-test" {
				if err := os.WriteFile(filepath.Join(root, "sum.mjs"), []byte("export const sum = (a,b) => a-b;\n"), 0600); err != nil {
					t.Fatal(err)
				}
				command = []string{"node", "--input-type=module", "-e", `import assert from 'node:assert/strict'; import {sum} from './sum.mjs'; assert.equal(sum(2,3),5); assert.equal(sum(-2,3),1); assert.equal(sum(0,0),0);`}
				args = append(args, "--prompt", "sum.mjs returns incorrect sums. Read it, fix the source, then finish.")
			} else {
				oracleDir, err := os.MkdirTemp(output, stage+"-verification-")
				if err != nil {
					t.Fatal(err)
				}
				report = filepath.Join(oracleDir, "oracle.json")
				task, command, snapshot = prepareRepo(t, os.Getenv("ANVIL_PILOT"), stage, root, report)
				args = append(args, "--prompt", task.Report)
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
				verificationRoot := root
				verificationCommand := command
				verificationBefore := map[string][32]byte(nil)
				if stage != "edit-and-test" {
					verificationRoot, verificationCommand, verificationBefore = prepareVerificationRepo(t, root, task, stage, report)
				}
				verification, err := runCommand(context.Background(), verificationRoot, verificationCommand, 30*time.Second)
				if err != nil || verification.ExitCode != 0 || verification.TimedOut {
					t.Fatalf("host verification failed: %+v %v", verification, err)
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
					if err := checkRepoChanges(task, stage, snapshot, after); err != nil {
						t.Fatal(err)
					}
					verificationAfter, err := repoSnapshot(verificationRoot)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(verificationBefore, verificationAfter) {
						t.Fatal("test runner changed verification files")
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
