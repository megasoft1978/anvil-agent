# Verification snapshot — 2026-09-10

Environment: Go 1.24.0, macOS/arm64. No model was loaded for this verification.

| Check | Result |
|---|---|
| Offline suite with race detection | 107 passing tests/subtests, zero failures |
| Statement coverage | 88.4% |
| Parser fuzz campaign | 395,912 executions with two workers; no failures |
| Static analysis | `go vet ./...` passed |
| Build | `/tmp/gemma-agent` built; CLI help verified |
| Live Gemma suite | Read, tiny edit, guided date-fns passed; see below |
| Linux runtime | CI configured; not executed on this Mac |

The fake-endpoint integration test exercises the real CLI and worktree: read, exact edit, configured
test command, ordinary text completion, final verification, and saved before/after evidence. Archived
EXP-077/081 Gemma output is exercised both through the parser and through the agent loop. No claim
about live Gemma success rate or speed follows from those tests.

The suite also covers every byte truncation of a representative Gemma call, duplicate/cross-channel
spans, conflicting calls, nested values, malformed arguments, file boundaries, special files,
subprocess descendants, capped output, deadlines, trace failures, and isolated task input.

During development the tests exposed a JSON-escaping assumption in a test (corrected to compare
decoded values) and a possible named-pipe read that could block the tool loop (file tools now open
nonblocking and reject non-regular files). Argument handling rejects duplicate and incorrectly cased
keys rather than inheriting `encoding/json`'s permissive behavior.

The Go progressive runner now also reproduces the original real-repo failures in fresh fixtures:
date-fns has 1 expected failure and 10 passing tests; dayjs has 1 expected failure and 12 passing tests.
These are baseline checks, not model-generated fixes. The first dayjs baseline exposed generated
coverage files inside the fixture; coverage output is now directed outside it, retaining coverage.
The full race-enabled suite passes with both real-repo baseline checks enabled.

## First live real-repo success

Full-context optimized `gemma-4-26B-A4B-it-UD-IQ2_M-attnQ4K.gguf`, 24,576 context,
kit server flags, temperature 0, 120-second per-stage deadlines. Paging was explicitly allowed by
the user and did increase; elapsed times below are observations, not clean performance benchmarks.

The initial JSON-wrapped file results stalled on three identical reads in guided date-fns. A repeat
warning alone also stalled. After changing file results to literal source text (retaining that warning),
all three stages passed in `live-results/guarded-20260910T230934`:

| Stage | Result | Agent elapsed |
|---|---|---|
| Read and finish | Correct random marker; 1 native tool call | 7.9s |
| Tiny edit and test | Fixed addition; final verification passed | 43.8s |
| Guided date-fns | Numeric sort comparator; 1 fail-to-pass, 10 pass-to-pass; only source changed | 55.5s |

This is one successful real-repo attempt, not an established success rate. XML follow-up results are below.
Raw requests, responses, and source edit backups persist in that run's traces. Its oracle report was
temporary; later runs now preserve real-repo oracle JSON outside the disposable worktree as well.
The server's rendered prompt was inspected: OpenAI-style tool results map to native Gemma
`tool_response` blocks. The Go loop uses its own bounded recovery and repeat warning, not Pi extensions.
The full race-enabled offline suite passes after these changes.

The next baseline guided-dayjs attempt (`guarded-20260910T231206`) timed out at 120s with
10 tool calls and no edits, including malformed output and incorrect pagination. Both smoke stages
passed. A selectable focused initial prompt is being evaluated against this failure; the baseline
remains the default until there is evidence to promote the candidate.

Focused prompt attempt `guarded-20260910T232155`: read passed; tiny edit passed with 4 tool calls
(baseline used 6); guided dayjs still timed out at 120s with 7 tool calls and no edits. The candidate
is not promoted. Paging increased, so elapsed-time comparisons are not controlled speed evidence.

XML attempt `guarded-20260910T232741` passed both smoke tests but timed out on dayjs with 8 tool calls
and no edits. XML has not demonstrated an improvement; literal text remains the default.

On September 11, enabling reasoning with a 128-token budget (`guarded-20260911T065644`) also timed out
on dayjs: 5 tool calls, no edits. The tiny edit passed in 63s. This changes the kit's reasoning-off
condition and is not promoted. Its read response included the correct marker plus metadata, which
the current smoke assertion permits; it was not an exact-content-only response.

Test-first prompt (`guarded-20260911T070029`) exposed the concrete dayjs failure to the model but
still timed out with 8 calls and no edits. Enabling equivalent-offset normalization and unchanged-read
references (`guarded-20260911T070414`) caught duplicate requests correctly; dayjs stopped as stalled
after 104.4s, 10 calls, no edits. Neither option is promoted as a solution.

File search plus test-first (`guarded-20260911T070719`) timed out with 8 calls and no edits; the model
did not use the optional search tool. Google-recommended sampling (temperature 1, top-p .95, top-k 64,
with llama.cpp min-p disabled and seed 42) plus baseline prompt (`guarded-20260911T071118`) timed out
with 7 calls and no edits. Repeating that sampling/prompt condition on the original pre-requantization
IQ2_M file (`guarded-20260911T071438`) timed out with 10 calls and no edits. These single attempts do
not establish comparative success rates or isolate all sources of variability; they do not support
claiming any of these options solves dayjs.

Task reminders (`guarded-20260911T071752`) reduced the tiny fixture to 3 calls, but dayjs still timed
out with 6 calls and no edits. Supplying both complete source files up front, with normal tools
(`guarded-20260911T072205`), also timed out: 4 calls, no edits; the model reread supplied source.
These source-provided trials are a more guided diagnostic condition, not independent navigation.

Source-provided edit/test-only mode (`guarded-20260911T072554`) produced two actual edits but timed
out before verification. Postmortem replay of confirmed edits preserved the original failure
(1 failed, 12 passed); it did not fix the bug. With only the UTC plugin supplied
(`guarded-20260911T073104`), the model reached a failing test after an edit, then repeated invalid/no-op
edits and stalled after 51s. Restricting reads enables patch attempts, but has not yet yielded a correct fix.

With the UTC plugin supplied, edit/test-only tools, a 512-token reasoning budget, and
`preserve_tool_reasoning=true` (`guarded-20260911T073629`), the model ran a test then attempted an edit
before timing out at 120s. The edit itself removed the function-wide `let ins = this` and declared
`const ins` inside one branch, leaving every other branch with `ReferenceError: ins is not defined`
— a scope-narrowing mistake, not a semantics mistake. Postmortem replay of that confirmed edit
(`live-results/guarded-20260911T073629/traces/20260911T053805Z-816722671/postmortem-3018661895/`)
is 12 failed/1 passed: worse than baseline. Pageouts rose during this run. This is the concrete case
the 2026-09-11 `-rich-edit-feedback` change targets: keeping enough surrounding source context that a
scope-changing edit is visible as such, rather than silently accepting a narrowed declaration.

## 2026-09-11 offline changes (no live model; see SESSION-2026-09-11/NEXT.md for detail)

Four opt-in changes, all off by default, full suite 138/138 (was 135), `go vet` clean, both
`TestPreparedRealRepoBaselines` fixtures still reproduce their original failures unchanged:

- `-rich-edit-feedback`: edit outcomes (not-found, ambiguous, no-op, success) return a bounded
  current-source snippet and a reason instead of a bare Go error. Fixes a concrete interface
  contradiction: the old no-match error said "read the file and retry," which is impossible when
  `-edit-only` disables `read`. Not yet tested live.
- `-auto-test-after-edit`: runs the configured test command immediately after a successful edit and
  feeds the result back, instead of requiring the model to remember to call `run_tests`. Not yet
  tested live.
- `search` is now repository-wide by default (was single-file-only, and unused in every dayjs attempt
  above). Skips `.git`/`node_modules`/`dist`/`build`/`.next`/`vendor`/`target`/`__pycache__`/`.cache`
  and unreadable/binary files; bounded to 20 matches / 4000 files scanned. A single-file `path` still
  works as before.
- `--tui`: a first-version Bubble Tea terminal UI (`github.com/charmbracelet/bubbletea` v1.3.4),
  auto-falling back to headless JSON when stdout is not a real terminal. One `Trace.Sink` hook
  delivers every event to both the JSONL logger and the TUI unchanged — no duplicated `runAgent`.
  Verified end-to-end with a real PTY (`script`) against a throwaway local fixture: header, live
  status transitions, and final answer all render correctly. NOT verified: resize, paste,
  cancel-mid-tool-call, terminal restoration on error — needs a human at a real terminal.
  Driving `tea.Program.Run()` through piped (non-PTY) I/O hangs indefinitely, so the repo test
  suite (`tui_test.go`) covers the model/Sink logic only, not a full `Program.Run()`.

None of these four changes have a live-Gemma result yet. They are the plumbing the next guided
dayjs attempt needs, not evidence themselves.
