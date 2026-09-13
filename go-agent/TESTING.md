# Verification snapshot — 2026-09-10

Environment: Go 1.24.0, macOS/arm64. No model was loaded for this verification.

| Check | Result |
|---|---|
| Offline suite with race detection | 107 passing tests/subtests, zero failures |
| Statement coverage | 88.4% |
| Parser fuzz campaign | 395,912 executions with two workers; no failures |
| Static analysis | `go vet ./...` passed |
| Build | `/tmp/anvil-agent` built; CLI help verified |
| Live model suite | Read, tiny edit, guided date-fns passed; see go-agent live-results/ for current runs |
| Linux runtime | CI configured; not executed on this Mac |

The fake-endpoint integration test exercises the real CLI and worktree: read, exact edit, configured
test command, ordinary text completion, final verification, and saved before/after evidence. Archived
leaked-markup-format output is exercised both through the parser and through the agent loop. No claim
about live model success rate or speed follows from those tests.

The suite also covers every byte truncation of a representative leaked-markup call, duplicate/cross-channel
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
- `search` is now repository-wide by default (was single-file-only). Skips `.git`/`node_modules`/`dist`/`build`/`.next`/`vendor`/`target`/`__pycache__`/`.cache`
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

These changes have not yet been live-tested against the current target model (Qwen3.6-35B-A3B).

## Planned validation — Qwen3.6-35B-A3B (current target, only model this project tracks)

Every `SERVER_FLAGS` choice in `setup.sh` and every rejected lever in this project's history was tuned
or tested against a different, now-abandoned target model. None of it is assumed to still hold. This
is the open punch list, ordered by expected value, each with what would settle it:

1. **Thinking mode (done, negative result, 2026-09-12).** A/B'd the model's own documented
   thinking-mode-for-code sampling (temp 0.6, top_p 0.95, presence_penalty 0.0) against the
   non-thinking default on a real blind repo-navigation-then-fix task (dayjs). Same outcome as
   non-thinking: 6 turns, 0 edits, truncated at 300s — actual reasoning-token output was near-zero
   on 4 of 6 turns. Single run (stochastic, n=1); doesn't rule out thinking mode, but gives no
   support for it either. Not adopted as the default.
2. **`--cache-reuse` (done, confirmed still disabled, 2026-09-12, different reason than before).**
   The old "disabled" rationale was Gemma-4-specific (sliding-window-attention) and doesn't describe
   this model. Checked the one real risk on record — llama.cpp issue ggml-org/llama.cpp#23589, a
   KV-cache-drop regression specific to Qwen3.6 — and confirmed it was fixed by PR #24110 as of
   build b9518 (installed `llama-server` here is b10809, well past the fix). Tried enabling it
   anyway (`--cache-reuse 256`), including with `--kv-unified` forced on (in case slot-count
   auto-detection was the blocker, since the flag's default depends on that) — server logs
   `cache_reuse is not supported by this context, it will be disabled` in both cases. Root cause is
   architectural, not the old SWA reason nor a KV-unified setting: Qwen3.6-35B-A3B's hybrid Gated
   DeltaNet (linear-attention/recurrent-state) layers don't support the KV-shifting mechanism
   cache-reuse depends on. Confirmed no-op; not worth revisiting unless the model architecture or
   llama.cpp's DeltaNet support changes.
3. **`--ctx-checkpoints 0 --cache-ram 0` — done, real regression found and fixed (2026-09-12).**
   These measured a 4.87→1.02GB memory win with "no change in generation speed" on Gemma-4. On
   Qwen3.6-35B-A3B they cause something much worse than "the memory win evaporates": confirmed via
   `llama-server`'s own `timings.cache_n` per request that with these flags set, the SAME identical
   agentic task (immer fixture, blind, seed-fixed, byte-identical trajectory reproduced twice)
   periodically discards its entire prompt-prefix cache and re-prefills the whole accumulated
   conversation from scratch, instead of extending it incrementally — two full resets in one
   11-turn run, one of them an 8801-token re-prefill costing 81 real seconds by itself (more than a
   quarter of a 300s budget). Removing both flags: `cache_n` grew monotonically for all 16 turns of
   the identical task with zero resets, and the run covered 16 turns in 245s instead of 11-12 turns
   in the full 300s — a large, direct, measured speed win. Real memory cost: server RSS grew from
   ~8.8GB to ~11.2GB over a longer 16-turn conversation on the same box — still fits in 16GB, less
   headroom for other apps. `setup.sh`'s `SERVER_FLAGS`/`CHECK_STRINGS` updated to drop both flags.
   First tested with `--cache_prompt: true` added explicitly to the harness's own request body as a
   cheaper candidate fix — zero effect, trace was byte-identical before and after, ruling that out
   before finding the real cause.
4. **MTP speculative decoding.** `--spec-type draft-mtp --spec-draft-n-max 3`, fully merged in
   llama.cpp (PR #22673), needs a separate `unsloth/Qwen3.6-35B-A3B-MTP-GGUF` download (~11.5GB) and
   verifying its quant is comparable to the currently-used UD-IQ2_M. Real-world reports: ~75%
   acceptance, 2x+ generation speedup, no accuracy regression. Settle with: download, then a repeat
   of the same live real-repo scenario, comparing tok/s and pass/fail against the non-MTP baseline.
   Do this only after the dayjs finding below is understood — faster wrong output is still wrong.
5. **`-c 24576` sizing and `max_tokens` 3072→4096.** Cheap headroom check now that KV cost is small;
   settle with a memory-log read at a larger context, no benchmark re-run needed unless memory is
   tight.
6. **Rejected-lever re-checks specific to this model, not inherited from Gemma-4's suite:** a
   stronger prompt demanding every file get touched, a few-shot prompt example — both were
   model/prompt-specific findings on Gemma-4, unverified for Qwen3.6-35B-A3B. Quantizing MoE experts
   further and alternative quants are architecture-general findings (unlikely to flip) and lower
   priority to re-run.
7. **Headline finding, well-replicated (2026-09-12): explores well, essentially never commits to
   an edit within a ~300s budget — on two different real bugs, four different conditions.** dayjs
   (blind@180s, blind@300s, full-source-preload@300s, narrow-preload@300s, thinking-mode@300s): 0
   edits in 4 of 5 trials; preloaded, 1 edit attempt (turn 6) but ran out of time before
   verification. immer (a much simpler, single-file, 6-line real fix — not dayjs's cross-file
   subtlety), blind@300s, twice: 0 edits both times, 11 tool calls each, all reads. The second
   immer trial had the harness's `search` tool newly enabled (see below) plus a system-prompt nudge
   to prefer it over full-file reads — no behavior change, the model never called it.
   Navigation itself is good: on both immer trials the model found the actual fix-relevant files
   unprompted within the first 4-6 calls (`arrayMethods.ts`, the real fix location, plus
   `patches.ts`/`proxy.ts`, both cited in the real PR's own root-cause explanation) — it just kept
   reading past that point instead of acting.
   **Real per-turn timing data (not estimated) shows the bottleneck is prefill, not decode/
   "thinking too long":** decode is fast and small every turn (2-4s, 24-44 completion tokens);
   one single turn's 8050-token prompt took 77.5 real seconds to prefill (~100 tok/s prefill
   throughput at this quant) vs 3.8s to generate that turn's short reply. Across ~11 turns of
   reads, cumulative prefill cost of a growing transcript is enough on its own to consume the whole
   300s budget, independent of how much the model "deliberates." This reframes the MTP lever
   (item 4): MTP speeds up decode, and decode is not the bottleneck here, so it may not address
   this specific problem even though it's still worth testing for the tasks where it is generation-
   bound.
   **Update after fixing item 3 (the cache-reset bug):** re-ran the identical immer task with
   `--ctx-checkpoints 0 --cache-ram 0` removed. It got noticeably further — 16 turns in 245s instead
   of 11-12 turns in a full 300s, hitting `turn_limit` (not the time deadline) for the first time —
   and STILL made zero edits, still just read 16 times. This cleanly isolates the finding: it was
   never purely a time-budget artifact of the cache bug. Given genuinely more usable turns in the
   same wall-clock budget, the model still doesn't act. The "explores well, never commits" pattern
   is real model/prompt behavior, not (or not only) a harness inefficiency.
   Consulted Opus 5, who recommended a turn-count-triggered (not time-triggered) forced edit-only
   window once N reads have happened with no edit, mirroring the exact `closeOutReserve` pattern.
   **Implemented and live-tested (2026-09-12): it works.** After 5 reads with no edit, `read`/
   `search` are stripped from the declared tools for that turn (reapplied every turn until an edit
   happens, not a permanent ban) — see `forceEditAfterReads`/`readsSinceEdit` in `agent.go`. Re-ran
   the identical immer task: the window fired 3 times; the model tried `run_tests` twice (a
   legitimate action, not a workaround) before finally making a real edit to the correct file
   (`arrayMethods.ts`) on the third forced turn — the first edit of any kind across 8 live trials
   today. The specific fix was **wrong** (it invented a different mechanism — marking `"length"` as
   reassigned on `pop` — instead of the real fix, stringifying the inserted index) and `run_tests`
   correctly reported `exit_code=1` three times; the model then went back to reading
   (`src/internal.ts`) to reinvestigate, and ran out of time mid-reinvestigation
   (`status:"truncated"`, "insufficient time remaining for another turn").
   This is a qualitatively different, more ordinary failure mode than before: a real
   edit-test-diagnose loop now runs, and it failed on *fix correctness*, not on *ever attempting a
   fix*. `readCache`/read-dedup and the ledger already prevent the forced window from just
   repeating a past failed edit; this wasn't observed to loop.
   **Follow-up at 600s budget (2026-09-12):** hit `turn_limit` (16 turns) at 467s — well under the
   600s budget, so `MaxTurns` is now the binding constraint, not time. Made a SECOND, different
   edit attempt after the first one's `run_tests` came back failing again (targeting the
   pop-then-push overlap case differently, still not the real fix — stringifying the inserted
   index — which needs noticing a string-vs-number key mismatch in `patches.ts`, a file this run
   didn't revisit after its first pass, unlike the very first blind trial which did read it). Two
   genuine edit-test-reinvestigate cycles in one run, still no convergence in 16 turns. Next lever
   for this specific question is turn budget (`MaxTurns`), not time or the force-edit mechanism
   itself, which is doing its job.
8. **`search` tool enabled by default (2026-09-12).** Was fully implemented and tested
   (`search_test.go`) but silently left off after an earlier CLI simplification — `Tools.SearchEnabled`
   defaulted to `false` and nothing in `main.go` set it. Now on by default, with a system-prompt line
   encouraging it over full-file reads for large files. One live trial so far (immer, above): the
   model didn't use it. Real bug fixed regardless of whether it changes behavior — a working,
   tested capability should not be silently unavailable.
