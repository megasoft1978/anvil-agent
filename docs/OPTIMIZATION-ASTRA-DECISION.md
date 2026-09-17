# Astra decision package

Review status: preparation code is complete and the current tree may execute through the documented
runner commands. Historical model-free validation passed for the earlier prepared
revision. The approved `LLAMA-IQ2-8K-REAL` live probe was attempted and stopped at the first capability
gate under the memory stopping rule; no scored repair measurement ran. A Gemma startup was then
attempted from the restricted Codex shell and stopped before health because the shell denied the
loopback bind. Current-tree fixture checks, dynamic offline checks, and all model-dependent results
remain pending.

## Implementation checkpoint

Preparation changes applied. The following behavior was exercised on an earlier prepared revision;
the current runner/configuration additions require a fresh model-free validation pass before execution:
- Swapout growth now compares consecutive cumulative samples.
- Startup refuses an unsafe or unknown idle pressure baseline.
- Scored repair promotion requires normal-pressure samples through verification.
- llama.cpp configuration declares `/slots` for drain confirmation.
- Real 8K/16K Immer probes and paired four-task screens now exist in JSON.
- Dependencies and evidence selection are checked before server startup.

Additional preparation edits applied: input locks cover runner/grader, experiment/fixture JSON,
server binary, scenario and pilot manifests, oracle trees, and fixture source trees. Validation is
bound to that lock and selected tasks must each have a passing result. Gates require normal pressure;
scenario edits use source allowlists. The expected absent source file in the additive notify fixture
is explicit, while unexpected import failures remain infrastructure failures. Interrupted attempts
remain in the denominator and their worktrees are preserved rather than automatically overwritten on
resume.

Startup checked the memory guard while waiting for health, captured spawn errors, and waited for
process exit after termination. Server logs append across batches. The first live probe exercised
startup sampling, managed shutdown, cancellation acknowledgement, and the memory stop path.
Checkpoint recovery and cancellation/resume of a scored attempt remain untested.

Pressure now uses the macOS kernel pressure-level query (read-only inspection returned 1 on this
host); the older free-percentage heuristic is no longer used for new samples. Historical pressure
labels require reassessment and must not be treated as directly comparable to this new instrument.
Sampler requests are serialized, failures stop the batch, and phase attribution is captured before
sampling. Validation/run commands acquire an exclusive lock; managed startup checks server processes
and port occupancy. Stale locks require owner inspection rather than automatic deletion.

SIGINT/SIGTERM now mark execution interrupted, terminate owned spawned process groups, and
schedule escalation after ten seconds. Further task dispatch checks interruption. Signal behavior
remains untested, including interruptions during synchronous scenario grading.

Latest preparation changes:
- Scenario grading runs in a host-only Node subprocess so its synchronous oracle does not block
  the orchestrator's sampling loop. This is a grader, not another agent or model.
- Pilot base reproduction requires the expected fail/pass assertion report, exit 1, intact tests,
  and no timeout or missing report. Scenario bases require structured assertion failures.
- Scenario oracle-key sets must match the frozen contract.
- Native capability gates reject calls recovered from leaked text markup.
- The Go agent now has two opt-in, oracle-free harness controls for the Gemma follow-up: a dynamic
  tool-schema/forced-edit arm and one post-completion retry when no file was changed. The baseline
  remains stable-schema with no retry; these controls are only selected after a verified baseline.

The previously prepared sessions `20260914T063846193Z-b401994db269`,
`20260914T112419806Z-b401994db269`, `20260914T112822074Z-b401994db269`,
`20260914T142805574Z-b401994db269`, and `20260914T143050931Z-b401994db269` are stale because the
runner/configuration or Go recovery code changed after they were created. Their attempts remain unrun
and they must not be used for execution. The historical Qwen preparation session is
`20260914T154835852Z-b401994db269`. Offline Go tests, the selected prepared real-repository
baselines, fixture/oracle validation, dependency readiness, holdout refresh, CLI hash preflight, and
offline runner checks passed for the earlier prepared revision. The live server, sampler, warm smoke,
cancellation acknowledgement, and first capability-gate stop have now been recorded; current-tree
fixture/dynamic checks, scored repair quality, a memory-valid gate, cancellation/resume of a scored
attempt, and performance measurements remain pending.

A fresh Gemma preparation session was then created from the current static-checked CLI:
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T071413108Z-b401994db269`.
Its recorded Gemma file size and SHA-256 match the experiment configuration. The selected fit fixtures
were validated in the preceding session, while this new session's live fixture validation, inference,
gates, and repair measurement remain pending. The older session
`20260915T063334836Z-b401994db269` records the restricted-shell bind failure and must not be reused as
a model result.

A fresh local Qwen3.8 comparator session is
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T063004660Z-b401994db269`.
Its 10,094,357,632-byte model hash and input lock match the configuration, the five main fixture/oracle
contracts and the Zod holdout pass, and the runner dry-run passes. The Undici holdout remains pending
because its reference oracle opens a localhost socket denied by the restricted Codex shell. Capability
gates, model startup, warm smoke, live memory sampling, and scored repairs remain pending.

The current research update is [OPTIMIZATION-RESEARCH-2026-09-14.md](OPTIMIZATION-RESEARCH-2026-09-14.md).
It preserves the completed Qwen3.6 GGUF/llama.cpp 8K probe and closes the current TurboQuant branch.
The next repair signal is the already available local Gemma 4 attnQ4K probe, followed by Qwen3.5-9B
MLX and Gemma 4 E4B QAT Q4 if needed. The already resident dense Qwen3.8 GSQ-RCO IQ3_XXS comparator
is now prepared separately for a same-Go-CLI quality ceiling check after the Gemma path; its prior
research-suite speed is about 4.5–4.9 tok/s, so it is not expected to win the speed/memory tradeoff.
If current-model quality must be preserved and residency/I/O is the binding failure, the
later SSD lane is Hebrus Stable Affine4, Qwisp strict/bolt, then one pinned TurboQuant v0.6.x rescue.
The update also records stable slot banks, predictive expert loading, coalesced reads, and hybrid
ngram limits. The Go agent has a strict Qwen XML recovery path for parser diagnostics; recovered text
calls remain separate from native tool-call gate results.

Use the current commands and status in OPTIMIZATION-NEXT-BATCH.md. The reviewed priorities below
supersede older candidate rankings and execution orders; implementation status is recorded above.
The first inference command is historical, and no approval for a later batch is implied.

## Live first-probe result — 2026-09-14

The prepared llama.cpp server loaded the 11,522,702,304-byte Qwen3.6 UD-IQ2_M GGUF at 8K context.
Warm smoke completed one native tool call in 15.223 seconds. During startup and warm smoke, the
session recorded a peak 6.80 GiB server RSS, 13.06 GiB wired memory, 1.69 GiB compressed memory,
55.1 MiB minimum free memory, warning kernel pressure, and swap-out delta 30,716. The first
`gate-read-only` attempt was canceled after 2.514 seconds with zero tool calls, and the managed
server exited cleanly. The full rows are in
[`runs.jsonl`](../.optimization-results/20260914T154835852Z-b401994db269/runs.jsonl),
[`memory.jsonl`](../.optimization-results/20260914T154835852Z-b401994db269/memory.jsonl), and
[`server.log`](../.optimization-results/20260914T154835852Z-b401994db269/server.log).

Decision: classify Qwen3.6 UD-IQ2_M plus llama.cpp as memory-infeasible/contaminated for the
recorded host state. Stop the 16K, no-speculation screen, and ngram branches from this result. The
next evidence path is the already prepared local Gemma 4 attnQ4K repair candidate, followed by
Qwen3.5-9B 4-bit MLX if Gemma fails, or one separately prepared current-model SSD-residency candidate
if preserving Qwen3.6 is the priority.

## Reviewed decision and precedence — 2026-09-14

This section supersedes conflicting research rankings, old CHANGE lists, and old execution prompts.
The current operator has disabled conversational approval for the documented runner commands. The
checkpoint above describes implementation, not tested behavior. This review changes documentation only;
it does not certify the recorded session or binary.

### Priority

1. **Trust the measurement first.** Before spending inference budget, run the documented offline
   runner/Go behavior checks and fixture validation. Focus on memory sampling failures,
   cancellation, server drain/exit, input locks, resume without duplicate credit, source allowlists,
   oracle isolation, and native versus recovered tool calls. No model is needed for offline checks.
   The focused Go command, selected Go real-repository baselines, fixture validation, and offline Node
   runner checks have passed. The first live server startup, warm smoke, memory sampling,
   cancellation acknowledgement, and capability-gate stop are now recorded. A memory-valid gate,
   scored repair, cancellation/resume of a scored attempt, and performance screen remain pending.
2. **Close the first existing-weight feasibility probe.** `LLAMA-IQ2-8K-REAL` used pinned local
   llama.cpp, local Qwen3.6 UD-IQ2_M, actual Go tools, no speculation, and one Immer target, but
   stopped under warning pressure and swap-out growth before scoring. Preserve its denominator and
   do not retry the 16K or screen arms from it.
3. **Choose the next measured path from the failure.** The 8K probe did not pass. Prepare one smaller
   repair candidate first, or one explicitly selected SSD-residency backend when preserving Qwen3.6 is
   the priority. Only a new successful 8K baseline could justify the separately
   separately selected 16K real probe, then the full no-speculation screen. The executable graph is
   `8K-REAL -> 16K-REAL -> 16K-SCREEN -> NGRAM-SCREEN`; never run ngram before the complete screen.
   If 8K works and 16K fails, retain 8K as a candidate and prepare a separate 8K screen; do not label
   the whole engine infeasible. That fallback configuration is NOT implemented by this review.
4. **Diagnose failure, then select one fallback.** A parser failure calls for one source-backed
   parser/template correction; a deadline failure with productive tools calls for one bounded-budget
   diagnostic; a pressure failure calls for a lower-residency path. Stop the current batch on failure.
   Any changed arm needs a new configuration/session and separate explicit experiment selection. A 180-second failure is
   failure under that budget, not proof the model cannot repair. Do not feed hidden oracle details
   back into prompts or turn the initial task into a tuning target.
5. **Default fallback: smaller model.** Use the already available local Gemma 4 attnQ4K fit probe
   first, then the separately prepared local dense Qwen3.8 quality comparator when a same-harness
   quality ceiling is needed, followed by Qwen3.5-9B 4-bit MLX and Gemma 4 E4B QAT Q4 if needed;
   use Qwen3.5-4B as the low-memory control. Gemma 12B is a later escalation if the smaller quality
   result warrants it. This order is an evidence-availability and headroom hypothesis, not measured
   repair superiority. Select exact revisions, sizes, licenses, parser/templates, and engine
   capabilities before download.
6. **Alternative fallback for Qwen3.6: Hebrus, then Qwisp strict.** Choose this branch when preserving
   the model is the explicit objective or residency is the binding constraint. Do not run every
   shortlisted backend automatically. Hebrus has related-hardware evidence; Qwisp has base-M1 rows.
   Both still need an actual Go tool-loop integration and local repair evidence. Changing from IQ2 to
   Affine4 changes both quantization and engine: report a stack comparison, not an isolated engine gain.
7. **Optimize the observed bottleneck on a viable stack.** First bounded read/search output and stable
   prompt/schema reuse if traces justify them; then ngram only if decode dominates and exact hybrid
   state handling is supported. Current ngram JSON is prepared, not a mandate to run it immediately.
   Custom SSD kernels, learned predictors, and a new engine are deferred until an existing engine has
   a measured gap that a specific implementation can address.
8. **Low priority: Qwisp bolt and TurboQuant rescue.** Bolt is an approximate-quality experiment,
   never an exact-routing replacement. TurboQuant rescue requires a pinned repository and commit plus
   a concrete fix for the local failure. A version string alone is insufficient: establish whether
   local 0.25.0 and upstream 0.6.x refer to the same distribution/fork. Preserve native top-8 routing.
9. **Confirm on unused real tasks.** Freeze the selected configuration before holdouts. Report real
   repo tasks separately from synthetic scenarios, correlated seeds separately from distinct bugs,
   and repair tasks separately from new implementations. The current repair fixtures cannot support
   a claim about implementation quality; add and validate an unused implementation fixture before
   making that claim. Limit conclusions to the tested candidate set and budgets, not an absolute best.

### Corrections to evidence and decision rules

- Small repair samples cannot prove mathematical equivalence or population-wide non-inferiority.
  Exactness requires unchanged routing/state semantics and a specified numerical reference; passing
  an oracle means only that its assertions passed. Keep approximate modes visibly separate even
  when their observed repairs tie. Exactness to a quantized model does not mean equality to BF16.
- Separate raw oracle success, edit validity, execution status, and memory validity. The runner's
  `grade_pass` is a composite admissible success; retain the raw grade to distinguish a correct patch
  under pressure from an incorrect patch. Infrastructure failures remain in operational yield, but
  must not be described as model reasoning failures. Preserve interruptions and retries without
  counting the same logical task twice in quality summaries.
- Rank by verified real repairs per total elapsed hour within the allowed memory envelope. Include
  failed attempts, setup and grading; additionally show steady-state time. The 15% speed / 512 MiB
  memory thresholds are screening heuristics, not statistical proof. Report ties and Pareto tradeoffs
  rather than forcing a winner. Two seeds on four tasks are not eight independent bugs.
- Historical warning/critical labels used a free-percentage heuristic. Preserve them as historical
  labels, not kernel-pressure observations. Paging counters and incomplete repairs remain evidence;
  one low-free-memory snapshot alone does not establish sustained memory exhaustion.
- Reboot and closed apps are the controlled operator-requested baseline, not a demonstrated universal
  requirement for every candidate. A later apps-open check needs separate experiment selection. Do not suspend
  Spotlight or change OS settings to imitate an upstream benchmark. Existing swap occupancy alone is
  not active swap traffic. Never add server RSS to system wired memory.
- File units: local GGUF is 11,522,702,304 bytes = 11.52 GB = 10.73 GiB. Hebrus Affine4 is
  20,808,566,880 bytes = 20.81 GB = 19.38 GiB. Weight file size is not peak resident footprint.
  Recheck disk before downloads, including temporary files, sidecars/conversions, build artifacts,
  fixture dependencies and retained evidence. Previous free-space snapshots are not reservations.
- Artifact claims labeled BF16 for 4B/30B models with approximately 4.7/30.5 GiB are not credible as
  complete dense two-byte parameter storage without further provenance. Do not use those table
  entries for capacity planning; exact file inventories and quantization metadata are required.
- M1 Pro and base M1 results are not interchangeable. Hebrus's published run also used warm OS page
  cache and host interventions; neither its speed nor clean-start behavior is established locally.

### Storage math and implementation gate

Use an accounting model, not an asserted performance prediction:

`T_serial ≈ C + B_miss/BW + R*L + O`

`T_overlap ≈ max(C, B_prefetch/BW) + B_residual/BW + R_residual*L + P + W`

Here C, L, O, P and W are time; bytes/BW is also time. If BW already includes request overhead,
do not add R*L again. W denotes wasted-I/O/eviction delay, not bytes. Shared SSD bandwidth, queueing,
and memory bandwidth contention can invalidate ideal overlap. Measure logical requested bytes and
physical storage reads separately: a page-cache hit is not an SSD transfer. Cache hit rate must be
byte-weighted when expert sizes differ. A predictor must preserve the native router and pay for its
own memory, training/profiling, false-positive reads and CPU work. Profile only development tasks.

For an optimization affecting fraction f of end-to-end time with speedup s, the optimistic bound is
`speedup <= 1 / ((1-f) + f/s)` before added overhead. If decode is only 20% of elapsed time, even
infinite decode speed gives at most 1.25x. Use this bound to reject low-value kernel/speculation work.
No new runtime measurements were collected to populate these quantities.

### Completed model-free validation

Source inspection found these existing tests use parser functions or local `httptest` fake endpoints,
not inference. The focused command was run and passed in 0.799 seconds:

```sh
cd /Users/megasoft78/Desktop/Freelance/anvil-agent/go-agent
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOMAXPROCS=2 go test -p 1 -count=1 -timeout 90s -run '^(TestQwenXMLRecovery.*|TestNativeCallWinsOverLeakedText|TestTurnLimitAndNativeOnlyMode|TestCLIExitDeadlineAndCancellation)$' .
```

Scope: Qwen XML schema/rejection behavior, native-call precedence, native-only mode, and CLI deadline
and cancellation. It excludes live tests, real-repo fixture tests, inference and model servers. Missing
Go dependencies/toolchain cause failure rather than a download. Expected 1–5 minutes including build,
uncertain; 90s limits the test executable, not compilation. Working memory is unmeasured; allow roughly
0.5–2 GiB as a provisional planning estimate, potentially higher on a cold build. This is not a model
fit estimate. The full offline Go suite, race suite, bounded parser fuzz campaign, selected prepared
real-repository baselines, fixture/oracle validation, and offline runner checks also passed. Model
capability gates, live server drain/exit,
memory guards under runtime load, cancellation/resume during an active run, and all performance
measurements remain pending.

### Runnable status and next handoff

The existing llama configuration, local Gemma configuration/wrapper, local Qwen3.8 comparator, and
Qwen3.5 MLX preparation configuration/downloader are prepared here. Hebrus, Qwisp, TurboQuant rescue, the 8K fallback screen,
bounded-budget diagnostics, and implementation holdouts remain research/planning entries until their
adapters/configurations are prepared and statically reviewed. Missing counters must remain null with
reasons; do not invent measurements or commands.

A reboot may remove `/tmp/anvil-agent-opt`. Before execution, inspect its existence and compare all
pinned inputs. If missing or changed, rebuild during separately permitted preparation, prepare a fresh
session, and update OPTIMIZATION-NEXT-BATCH.md. Never bypass the manifest lock to reuse an old session.

Paste to the execution model:

```text
Read docs/OPTIMIZATION-ASTRA-DECISION.md first, then docs/OPTIMIZATION-NEXT-BATCH.md.
Use docs/OPTIMIZATION-CODEX-GOAL.md for the objective and consult research only for the selected arm.
Do not reapply historical completed preparation changes. Inspect current input/session availability.
Use the reviewed offline results and inspect the current session before inference. Execute only the
named documented batch and stop on its gates. Do not advance to another experiment, download weights,
close applications, change memory settings, or use external agents/models.
```

Sources rechecked for this review: [Hebrus runtime](https://github.com/hebrus-labs/hebrus),
[Hebrus M1 Pro evidence](https://github.com/hebrus-labs/hebrus/blob/main/docs/benchmarks/2026-07-29-qwen-m1-pro-16g-main.md),
[Qwisp](https://github.com/penta2himajin/qwisp),
[Qwisp results](https://github.com/penta2himajin/qwisp/blob/main/bench/RESULTS.md),
[TurboQuant upstream](https://github.com/nathannorthcutt/turboquant-mlx), and
[NeuroPrefetcher](https://arxiv.org/abs/2608.22643). These are upstream claims, not local validation.
