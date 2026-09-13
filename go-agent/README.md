# go-agent

A small Go agent for controlled coding experiments against an already-running local llama-server.
No external Go dependencies, model downloads, server management, or ambient project instructions.
Go 1.24+, macOS or Linux.

Sampling defaults and prompt profile are selected per `--model` from `profiles.go` (a small registry,
one entry per model this harness has been tuned against; unknown models fall back to a conservative
default) rather than hardcoded in `main.go` -- see that file for how to add a model. Tool-call recovery
selects a parser by the shape of
a malformed response, not by which model is declared.

The loop sends OpenAI-compatible chat requests with `tool_choice: "auto"` and
`parallel_tool_calls: false`. It executes one tool at a time and finishes on an ordinary text reply.
There is no `done` tool and no forced grammar. Requests are non-streaming in this version; recovery
cannot stop a model's generation loop early. Live quality and speed are not yet established.

## Build and try one fixture

From this directory:

```sh
go build -o /tmp/anvil-agent .
work=$(mktemp -d)
cp testdata/tiny-edit/sum.mjs testdata/tiny-edit/test.mjs "$work/"
/tmp/anvil-agent --root "$work" \
  --prompt 'Fix sum.mjs so it adds its arguments, then finish.' --output ./runs
```

The fixture is intentionally broken. The server must already be healthy at `127.0.0.1:8114`.
Do not load it until the machine has enough RAM. The harness does not quit applications or change
server flags. Dependencies for real-repo tasks must be installed before model residency.

Default limits: 120 seconds **total**, including model requests and tools;
16 model requests; 8,192 output tokens per request; sampling per the
target model's own profile in `profiles.go` (not greedy decoding by default).
`--help` lists overrides. Supply `ANVIL_API_KEY` only if the endpoint requires authentication.

Three behaviors that used to be opt-in flags are now permanently on, because live testing showed
each one measurably helps and none regress the offline suite (see TESTING.md for the runs):

- **Rich edit feedback**: a mismatched `edit` call gets the current source location and a reason
  ("not found", "occurs N times", "no-op") instead of a bare error, so the model can actually see
  what to fix instead of blindly retrying.
- **Ledger**: the harness remembers every path confirmed absent and every edit already tried
  (applied-then-reverted, or failed to apply) in the current run, and refuses an exact repeat before
  it executes rather than letting the model burn a turn re-discovering the same dead end.
- **Host-side verification**: the agent loop never executes repository commands. Benchmark and
  integration tests run trusted verification commands outside the model-facing tool surface.

Everything else that was previously a matrix of experimental flags (prompt profiles, read-format
variants, dedup-reads, search, task-reminder, sampling profiles, preserve-tool-reasoning, per-edit
leaked-markup recovery, seed, temperature, max-turns, max-history-bytes, tool-timeout)
has been removed from the CLI — most were single-session diagnostics never adjusted in practice; a
few (temperature, max-turns) turned out to never need changing in normal runs, so a fixed sane
default replaced the flag. The remaining CLI surface is: `--root`, `--endpoint`,
`--prompt`/`--prompt-file`/`--task-file`, `--instructions`, `--output`, `--timeout`, `--model`,
`--max-tokens`, `--tui`.

After a timeout with confirmed edits, use `ANVIL_REPLAY_TRACE=/path/to/trace.jsonl` plus `ANVIL_PILOT`
and `ANVIL_REPO_STAGE` with `go test -run '^TestReplayRealRepoOracle$' -v -count=1`. This replays only
successful source edits whose before/after content and hashes match, then grades pristine tests in a
fresh fixture. Postmortem reports persist beside the trace; a postmortem pass is not an in-budget run.

## Tools and completion

| Tool | Arguments | Behavior |
|---|---|---|
| `read` | `path`, optional 1-based `offset` | Reads a file or lists a directory; up to 200 lines/entries and 32 KiB per response. Use `next_offset` to continue. |
| `edit` | `path`, `oldText`, `newText` | Replaces exactly one non-empty match in an existing file. Saves before/after evidence before writing. |
| `write` | `path`, `content` | Creates a NEW file (parent directories created as needed). Refuses if the path already exists — `edit` is required to modify an existing file. Enabled by default; disable with `--write=false`. |
| `search` | `text`, optional `path` | Literal-text grep across the worktree (or one file/directory), skipping `.git`/`node_modules`/build output and binary files. Enabled by default. |

File tools use `os.Root` to reject paths and symlinks outside the selected worktree. They accept UTF-8
regular files up to 1 MiB. There is deliberately no tool for arbitrary shell commands: this harness
is scoped to controlled coding experiments, not an open-ended shell. Use disposable worktrees and
run trusted verification commands outside the model loop. Test stdout/stderr is capped at 32 KiB;
process groups are killed at the command deadline so a watch process or child cannot keep the run alive.

On text completion, `completed` means only that the model supplied a final reply. It does not claim
that a repository test passed. For measured real-repo results, run the trusted verifier separately
against pristine oracle tests; preserve guided and independent results as different conditions.

Qwen3.6-35B-A3B emits well-formed native tool calls, so the leaked-markup recovery path
(`<|tool_call>call:name{...}<tool_call|>` and the missing-`call:` variant) exists but is normally
unused; it stays available in `recovery.go` for any model that leaks this specific format instead of
a real `tool_calls` response.
Token-limit responses execute no tools. Three identical consecutive tool calls stop as `stalled`,
with a corrective warning after the second identical call. A model emitting more than one tool call
in a single turn has only the first executed; the rest are dropped and traced as
`dropped_parallel_calls` rather than failing the run. File-read responses send literal source text
to the model; traces retain structured metadata and content.

History has a 64 KiB request-byte ceiling and is never silently trimmed. This is not a tokenizer;
the server remains responsible for its actual context-token limit.

## Task input and artifacts

Choose exactly one of `--prompt`, `--prompt-file`, or `--task-file`. The latter accepts the research
repository's task JSON and sends **only `report`**, never its fix commit or oracle contents. It does
not prepare a worktree, install dependencies, or perform independent oracle grading. Add explicit
kit instructions with `--instructions /path/to/AGENTS.md` if required for a comparison.

Example for an already-prepared, disposable date-fns worktree:

```sh
/tmp/anvil-agent --root /path/to/date-fns-worktree \
  --task-file /path/to/date-fns-0d1a2239.json \
  --output ./runs
```

Each attempt creates a unique directory containing `trace.jsonl` and `summary.json`. By default the
artifact parent is the worktree's sibling `.<worktree-name>-anvil-runs`; `--output` overrides it but
must remain outside the worktree. Traces and directories are private to the current user. Traces
contain prompts, raw responses, server usage/timings when returned, tool results, and edit backups;
do not share them without reviewing their repository contents. API authorization headers are not logged.
Raw responses include malformed output; recovered spans are not repeated in subsequent requests.

Summary statuses distinguish `completed`, `truncated`,
`malformed_tool_call`, `stalled`, `turn_limit`, `context_limit`, `empty_completion`, `protocol_error`,
`timeout`, `cancelled`, and transport/trace errors. `completed` means only that the model supplied a
final reply; verification is external to the agent loop. CLI exit codes: 0 completed, 1 unsuccessful run, 2 CLI/artifact
setup error, 124 total deadline, 130 cancellation. No files are rolled back automatically; the trace's
`edit_backup` entries preserve the before and after text for inspection and recovery.

## Terminal UI (first version)

```sh
/tmp/anvil-agent --root "$work" --endpoint http://127.0.0.1:8114/v1 \
  --prompt 'Fix sum.mjs so it adds its arguments, then finish.' --tui --timeout 120s
```

`--tui` launches a live Bubble Tea view of the same run `runAgent` would otherwise execute
headlessly: a header (model/repo/elapsed/turns/tool calls/edits), a status line (`waiting for
model` / `running: <tool>` / `done: <status>`), a scrolling transcript of read/edit/test/
correction/recovery events, and the final answer. `q` or `ctrl+c` cancels the in-flight run and
exits; the session otherwise stays open after completion so you can read the result. It falls back
silently to the normal headless JSON output whenever stdout is not a real terminal (redirected,
piped, or under test), so scripts and CI are unaffected.

This is a first version: no multiline input composer, `/diff`/`/tests`/`/context`/`/status`/`/resume`
commands, mid-run steering, session resume, undo, expandable panels, or token streaming yet. The
transcript is in-memory only (capped at 2,000 lines), not paged from disk. It has been verified
rendering correctly end-to-end against a real PTY with a throwaway fixture server; resize, paste,
cancel-mid-tool-call, and terminal-restoration-on-error still need manual verification by a human
at a real terminal before you rely on those specifically.

## Regression suite

```sh
go vet ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go test -run '^$' -fuzz '^FuzzLeakedMarkupRecovery$' -fuzztime=15s -parallel=2
```

All default tests are offline and use fake local HTTP endpoints. They cover:

- Real CLI read → edit → final reply, with linked tool results and replayable traces; host-side verification
  remains covered by the oracle and process-runner tests.
- Observed EXP-077/081 leaked-markup failures, both opener formats, repeated/cross-channel spans, native precedence,
  every byte truncation of a tool call, nested values, invalid separators, and fuzz-generated input.
- Malformed JSON, unknown tools/arguments, token truncation, empty responses, context/turn limits,
  repeated calls, one-correction limits, HTTP failures, cancellation, and host-side verification outcomes.
- Traversal/symlink escapes, file and output limits, ambiguous edits, backup failures, binary/special
  files, fixed command dispatch, process-group deadlines and child cleanup.
- CLI input contracts, isolated prompts, credential exclusion, private and unique artifacts, and redirects.

CI runs race detection, tests, vet, a bounded fuzz campaign, and a build on macOS and Linux.
Fuzz seeds always run in the ordinary suite. Regression fixtures document their provenance in
`testdata/leaked-markup-regressions.json`.

## Opt-in live model smoke tests

Once the local server is running with sufficient memory:

```sh
ANVIL_LIVE=1 go test -run '^TestLiveModel$' -v -count=1 -timeout=5m
```

Runs **read-and-finish**, then **edit-and-test**, sequentially, each with a 120-second total deadline.
The second stage requires Node. It stops on the first failure. On macOS it monitors pageouts every
second and cancels if they increase; it never stops a server it does not own. This monitoring cannot
replace checking memory before starting the server. On Linux pageout monitoring is unavailable here.
Traces persist in `live-results/` even though fixture directories are temporary. Optional variables:
`ANVIL_LIVE_ENDPOINT` (loopback only), `ANVIL_LIVE_MODEL`, and `ANVIL_LIVE_OUTPUT`.
`ANVIL_ALLOW_PAGING=1` explicitly disables the paging abort while retaining measurements. Use only
when accepting possible system slowdown; results with paging are not clean performance measurements.

These smoke tests are not a real-repo benchmark. After they pass, use the prepared guided date-fns
and dayjs tasks, then independent prompts, one run at a time. Pin the same server configuration,
sampling settings, instructions, verification command, and deadline across any comparison.

## Progressive real-repo checks (Go only)

Use the existing EXP-085 prepared pilot directory as `ANVIL_PILOT`. Before loading the model, verify
that both original bugs still reproduce through the Go runner:

```sh
ANVIL_PILOT=/path/to/prepared/pilot go test -run '^TestPreparedRealRepoBaselines$' -v -count=1
```

Then, with a healthy server and enough RAM, run the two smoke stages followed by **one** real issue:

```sh
ANVIL_LIVE=1 ANVIL_PILOT=/path/to/prepared/pilot ANVIL_REPO_STAGE=date-fns-guided \
  go test -run '^TestLiveModel$' -v -count=1 -timeout=8m
```

Advance manually to `dayjs-guided`, then `date-fns-independent` and `dayjs-independent`, only after
inspecting the preceding result. Each invocation starts with fresh disposable fixtures and stops on
the first failed stage; it does not reuse a previous model patch. Real-repo preparation copies files
without Git history, reuses preinstalled dependencies, and installs the manifest's pristine oracle
tests. Only the issue report goes into the prompt; the host test runner executes verification after
the model run.
The runner verifies each expected test by name, rejects missing/skipped tests and regressions, and
rejects all file changes except the issue's designated source files. Jest coverage stays enabled but
writes outside the worktree. This is a controlled regression experiment, not a hostile-code sandbox:
trusted repository tests still execute with your user permissions and shared dependency caches.
