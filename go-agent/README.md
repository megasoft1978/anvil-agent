# Gemma coding harness (experimental)

A small Go agent for controlled coding experiments against an already-running Gemma 4 llama-server.
No external Go dependencies, Pi extensions, model downloads, server management, UI, or ambient project
instructions. Go 1.24+, macOS or Linux. Existing kit installation and Pi defaults are unchanged.

The loop sends OpenAI-compatible chat requests with `tool_choice: "auto"` and
`parallel_tool_calls: false`. It executes one tool at a time and finishes on an ordinary text reply.
There is no `done` tool and no forced grammar. Requests are non-streaming in this version; recovery
cannot stop a model's generation loop early. Live quality and speed are not yet established.

## Build and try one fixture

From this directory:

```sh
go build -o /tmp/gemma-agent .
gemma_work=$(mktemp -d)
cp testdata/tiny-edit/sum.mjs testdata/tiny-edit/test.mjs "$gemma_work/"
/tmp/gemma-agent --root "$gemma_work" \
  --prompt 'Fix sum.mjs so it adds its arguments. Run the tests, then finish.' \
  --test-command '["node","test.mjs"]' --output ./runs
```

The fixture is intentionally broken. The server must already be healthy at `127.0.0.1:8114`.
Do not load it until the machine has enough RAM. The harness does not quit applications or change
server flags. Dependencies for real-repo tasks must be installed before model residency.

Default limits: 120 seconds **total**, including model requests, tools, and final verification;
30 seconds per test command; 16 model requests; 3,072 output tokens per request; temperature 0.
`--help` lists overrides. Supply `GEMMA_API_KEY` only if the endpoint requires authentication.

`--prompt-profile baseline` (default) retains the original concise prompt. `--prompt-profile focused`
is an experimental Gemma-oriented workflow prompt: source-first navigation, exact pagination,
small edits, configured tests, and evidence-based completion. Both use the same native tools.
For a controlled live comparison set `GEMMA_PROMPT_PROFILE=focused`; the selected initial prompt
and configuration are retained in the trace. Do not infer superiority from a single successful run.
`--prompt-profile test-first` is another candidate: reproduce the failing test before inspecting source.
`--read-format text` is the default source representation; `json` restores the original wrapped result,
and `xml` uses escaped metadata with CDATA source content. `GEMMA_READ_FORMAT` selects the same option
in live tests. These change tool-result content only, not Gemma's native tool-call protocol.

Two further opt-in experiments are `--dedup-reads` (normalize equivalent default offsets and reference
earlier unchanged pages; the raw trace still stores each actual read) and `--enable-search` (a fourth
tool finding literal text within one file, up to 20 matches with line numbers and nearby source).
Search uses the same rooted UTF-8 regular-file restrictions, not a shell or recursive repository scan.
Live equivalents are `GEMMA_DEDUP_READS=1` and `GEMMA_ENABLE_SEARCH=1`. Neither is enabled by default.

`--sampling-profile gemma` uses Google's recommended temperature 1.0, top-p .95, and top-k 64,
and disables llama.cpp's additional min-p filter. `--temperature` explicitly overrides the profile's
temperature; `--seed` pins a non-negative server sampling seed. Baseline sampling remains unchanged.
Live equivalents: `GEMMA_SAMPLING_PROFILE=gemma` and `GEMMA_SEED=42`.
See the [Gemma 4 model card](https://ai.google.dev/gemma/docs/core/model_card_4).

`--task-reminder` restates only the original task after successful reads, retaining the tool result.
Use `GEMMA_TASK_REMINDER=1` for live trials. `GEMMA_PRELOAD_SOURCES=1` is a separate, live-test-only
diagnostic that supplies the complete designated source files up front. Label those results
**source-provided**, not independent navigation or the earlier guided condition. It supplies buggy
worktree source, never the reference fix, and keeps the same source-change and oracle checks.
`GEMMA_PRELOAD_FILE=src/plugin/utc/index.js` narrows that diagnostic to one designated source file.
`GEMMA_EDIT_ONLY=1` additionally uses a source-repair prompt and disables read/search for the real-repo
stage; it requires source preloading. The equivalent CLI switch is `--edit-only`, for caller-supplied
source tasks. These restricted diagnostic results must not be reported as normal agent navigation.

Two 2026-09-11 opt-in experiments target the observed dayjs no-op/stale-edit failure mode (see
TESTING.md): `--rich-edit-feedback` returns a bounded current-source snippet and a reason on every
edit outcome (not-found, ambiguous, no-op, success) instead of a bare error — the old no-op/no-match
errors told the model to "read the file and retry," which is impossible under `--edit-only`.
`--auto-test-after-edit` runs the configured test command immediately after a successful edit and
feeds the result back, instead of requiring the model to remember to call `run_tests`. Neither has a
live-Gemma result yet; both default to off and change nothing else.

After a timeout with confirmed edits, use `GEMMA_REPLAY_TRACE=/path/to/trace.jsonl` plus `GEMMA_PILOT`
and `GEMMA_REPO_STAGE` with `go test -run '^TestReplayRealRepoOracle$' -v -count=1`. This replays only
successful source edits whose before/after content and hashes match, then grades pristine tests in a
fresh fixture. Postmortem reports persist beside the trace; a postmortem pass is not an in-budget run.

## Tools and completion

| Tool | Arguments | Behavior |
|---|---|---|
| `read` | `path`, optional 1-based `offset` | Reads a file or lists a directory; up to 200 lines/entries and 32 KiB per response. Use `next_offset` to continue. |
| `edit` | `path`, `oldText`, `newText` | Replaces exactly one non-empty match in an existing file. Saves before/after evidence before writing. |
| `run_tests` | `{}` | Runs the caller's fixed JSON argument list, without shell parsing. Available only with `--test-command`. |
| `search` | optional `path` (default: whole worktree), `text` | Literal (non-regex) text search, repository-wide by default; a `path` restricts it to one file or subtree. Skips `.git`, `node_modules`, `dist`, `build`, `.next`, `vendor`, `target`, `__pycache__`, `.cache`, and unreadable/binary files. Bounded to 20 matches and 4,000 files scanned. Enable with `--enable-search`. |

File tools use `os.Root` to reject paths and symlinks outside the selected worktree. They accept UTF-8
regular files up to 1 MiB. They cannot create files or run arbitrary shell commands. The configured
test command executes repository code with your user permissions: **this is not a security sandbox**.
Use disposable worktrees and trusted test commands. Test stdout/stderr is capped at 32 KiB; process
groups are killed at the command deadline so a watch process or child cannot keep the run alive.

On text completion, the configured test command is run again under the remaining deadline. A zero
exit means that command passed, not that an independent oracle proved the bug fixed. For measured
real-repo results, grade separately using pristine oracle tests; the model can edit repository tests
or configuration. Preserve guided and independent results as different conditions.

Gemma recovery recognizes both `<|tool_call>call:name{...}<tool_call|>` and the observed missing-`call:`
variant. Identical repetitions across text/reasoning recover once. Distinct calls, incomplete spans,
or missing closers are never guessed. Token-limit responses execute no tools. Native calls take
precedence. Unrecoverable leaked output gets at most one correction per run; three identical
consecutive tool calls stop as `stalled`, with a corrective warning after the second identical call.
File-read responses send literal source text to the model; traces retain structured metadata and content.
Use `--recover-gemma=false` for a native-only comparison.

History has a 64 KiB request-byte ceiling and is never silently trimmed. This is not a tokenizer;
the server remains responsible for its actual context-token limit.

## Task input and artifacts

Choose exactly one of `--prompt`, `--prompt-file`, or `--task-file`. The latter accepts the research
repository's task JSON and sends **only `report`**, never its fix commit or oracle contents. It does
not prepare a worktree, install dependencies, or perform independent oracle grading. Add explicit
kit instructions with `--instructions /path/to/AGENTS.md` if required for a comparison.

Example for an already-prepared, disposable date-fns worktree:

```sh
/tmp/gemma-agent --root /path/to/date-fns-worktree \
  --task-file /path/to/date-fns-0d1a2239.json \
  --test-command '["npx","--no-install","vitest","run","src/isWithinInterval/test.ts"]' \
  --output ./runs
```

Each attempt creates a unique directory containing `trace.jsonl` and `summary.json`. By default the
artifact parent is the worktree's sibling `.<worktree-name>-gemma-runs`; `--output` overrides it but
must remain outside the worktree. Traces and directories are private to the current user. Traces
contain prompts, raw responses, server usage/timings when returned, tool results, and edit backups;
do not share them without reviewing their repository contents. API authorization headers are not logged.
Raw responses include malformed output; recovered spans are not repeated in subsequent requests.

Summary statuses distinguish `completed`, `verification_failed`, `verification_error`, `truncated`,
`malformed_tool_call`, `stalled`, `turn_limit`, `context_limit`, `empty_completion`, `protocol_error`,
`timeout`, `cancelled`, and transport/trace errors. `completed` without a test command means only
that the model supplied a final reply. CLI exit codes: 0 completed, 1 unsuccessful run, 2 CLI/artifact
setup error, 124 total deadline, 130 cancellation. No files are rolled back automatically; the trace's
`edit_backup` entries preserve the before and after text for inspection and recovery.

## Terminal UI (first version)

```sh
/tmp/gemma-agent --root "$gemma_work" --endpoint http://127.0.0.1:8114/v1 \
  --prompt 'Fix sum.mjs so it adds its arguments. Run the tests, then finish.' \
  --test-command '["node","test.mjs"]' --tui --timeout 120s
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
go test -run '^$' -fuzz '^FuzzGemmaRecovery$' -fuzztime=15s -parallel=2
```

All default tests are offline and use fake local HTTP endpoints. They cover:

- Real CLI read → edit → test → final verification, with linked tool results and replayable traces.
- Observed EXP-077/081 Gemma failures, both opener formats, repeated/cross-channel spans, native precedence,
  every byte truncation of a tool call, nested values, invalid separators, and fuzz-generated input.
- Malformed JSON, unknown tools/arguments, token truncation, empty responses, context/turn limits,
  repeated calls, one-correction limits, HTTP failures, cancellation, and verification failures.
- Traversal/symlink escapes, file and output limits, ambiguous edits, backup failures, binary/special
  files, fixed command dispatch, process-group deadlines and child cleanup.
- CLI input contracts, isolated prompts, credential exclusion, private and unique artifacts, and redirects.

CI runs race detection, tests, vet, a bounded fuzz campaign, and a build on macOS and Linux.
Fuzz seeds always run in the ordinary suite. Regression fixtures document their provenance in
`testdata/gemma4-regressions.json`.

## Opt-in live Gemma smoke tests

Once the local server is running with sufficient memory:

```sh
GEMMA_LIVE=1 go test -run '^TestLiveGemma4$' -v -count=1 -timeout=5m
```

Runs **read-and-finish**, then **edit-and-test**, sequentially, each with a 120-second total deadline.
The second stage requires Node. It stops on the first failure. On macOS it monitors pageouts every
second and cancels if they increase; it never stops a server it does not own. This monitoring cannot
replace checking memory before starting the server. On Linux pageout monitoring is unavailable here.
Traces persist in `live-results/` even though fixture directories are temporary. Optional variables:
`GEMMA_LIVE_ENDPOINT` (loopback only), `GEMMA_LIVE_MODEL`, and `GEMMA_LIVE_OUTPUT`.
`GEMMA_ALLOW_PAGING=1` explicitly disables the paging abort while retaining measurements. Use only
when accepting possible system slowdown; results with paging are not clean performance measurements.

These smoke tests are not a real-repo benchmark. After they pass, use the prepared guided date-fns
and dayjs tasks, then independent prompts, one run at a time. Pin the same server configuration,
sampling settings, instructions, test command, and deadline when comparing against Pi.

## Progressive real-repo checks (Go only)

Use the existing EXP-085 prepared pilot directory as `GEMMA_PILOT`. Before loading the model, verify
that both original bugs still reproduce through the Go runner:

```sh
GEMMA_PILOT=/path/to/prepared/pilot go test -run '^TestPreparedRealRepoBaselines$' -v -count=1
```

Then, with a healthy server and enough RAM, run the two smoke stages followed by **one** real issue:

```sh
GEMMA_LIVE=1 GEMMA_PILOT=/path/to/prepared/pilot GEMMA_REPO_STAGE=date-fns-guided \
  go test -run '^TestLiveGemma4$' -v -count=1 -timeout=8m
```

Advance manually to `dayjs-guided`, then `date-fns-independent` and `dayjs-independent`, only after
inspecting the preceding result. Each invocation starts with fresh disposable fixtures and stops on
the first failed stage; it does not reuse a previous model patch. Real-repo preparation copies files
without Git history, reuses preinstalled dependencies, and installs the manifest's pristine oracle
tests. Only the issue report (with test instructions adapted to `run_tests`) goes into the prompt.
The runner verifies each expected test by name, rejects missing/skipped tests and regressions, and
rejects all file changes except the issue's designated source files. Jest coverage stays enabled but
writes outside the worktree. This is a controlled regression experiment, not a hostile-code sandbox:
trusted repository tests still execute with your user permissions and shared dependency caches.
