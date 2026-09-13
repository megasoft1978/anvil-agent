package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Grade confirmed source edits even when generation timed out before the final test command.
// Replay only successful edit results, never an unconfirmed write-ahead backup.
func TestReplayRealRepoOracle(t *testing.T) {
	trace := os.Getenv("ANVIL_REPLAY_TRACE")
	if trace == "" {
		t.Skip("requires ANVIL_REPLAY_TRACE and ANVIL_PILOT")
	}
	pilot := os.Getenv("ANVIL_PILOT")
	if pilot == "" {
		t.Fatal("ANVIL_PILOT required")
	}
	stage := os.Getenv("ANVIL_REPO_STAGE")
	if stage == "" {
		stage = "dayjs-guided"
	}
	dir, err := os.MkdirTemp(filepath.Dir(trace), "postmortem-")
	if err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(dir, "oracle.json")
	root := t.TempDir()
	task, command, before := prepareRepo(t, pilot, stage, root, report)
	file, err := os.Open(trace)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	type backup struct{ Path, Before, After string }
	var pending *backup
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	edits := 0
	for scanner.Scan() {
		var event struct {
			Type string
			Data json.RawMessage
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		switch event.Type {
		case "edit_backup":
			if err := json.Unmarshal(event.Data, &pending); err != nil {
				t.Fatal(err)
			}
		case "tool_result":
			var result struct {
				Name   string
				Output struct {
					Error  string
					Result struct {
						AfterSHA string `json:"after_sha256"`
					}
				}
			}
			if err := json.Unmarshal(event.Data, &result); err != nil {
				t.Fatal(err)
			}
			if result.Name != "edit" {
				continue
			}
			if result.Output.Error != "" {
				pending = nil
				continue
			}
			if pending == nil || !task.isSource(stage, pending.Path) || fmt.Sprintf("%x", sha256.Sum256([]byte(pending.After))) != result.Output.Result.AfterSHA {
				t.Fatal("unconfirmed or out-of-scope edit")
			}
			path := filepath.Join(root, pending.Path)
			current, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(current) != pending.Before {
				t.Fatal("replay baseline does not match recorded source")
			}
			if err := os.WriteFile(path, []byte(pending.After), 0600); err != nil {
				t.Fatal(err)
			}
			edits++
			pending = nil
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if edits == 0 {
		t.Fatal("no confirmed source edits")
	}
	after, err := repoSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkRepoChanges(task, stage, before, after); err != nil {
		t.Fatal(err)
	}
	result, err := runCommand(context.Background(), root, command, 30*time.Second)
	log := fmt.Sprintf("replayed %d edits; exit=%d error=%v\n%s", edits, result.ExitCode, err, result.Output)
	if writeErr := os.WriteFile(filepath.Join(dir, "result.txt"), []byte(log), 0600); writeErr != nil {
		t.Fatal(writeErr)
	}
	t.Logf("Postmortem artifacts: %s\n%s", dir, log)
	if err != nil || result.ExitCode != 0 || result.TimedOut {
		t.Fatal("replayed patch does not pass verification")
	}
	if err := checkRepoReport(report, task, true); err != nil {
		t.Fatal(err)
	}
}
