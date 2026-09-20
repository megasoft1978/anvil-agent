# Sourced benchmark candidates

Tracked against the methodology in `BENCHMARK-SOURCING.md` (boundary D = 2026-07-30). Sourced via
`gh search prs --repo=<repo> --merged --merged-at ">=2026-07-30" --json ...`. Each entry records
the checklist evidence gathered so far; a candidate is only promoted to an actual fixture
(`manifest.json` entry with `fail_to_pass`/`pass_to_pass`) after a title-only contamination probe
against the active target fails to name the fix. The original target-specific probe transcripts
are intentionally not part of this current record.

## colinhacks/zod (real, ~35k+ stars, dependency-light, fast test suite)

### Candidate 1 — PR #6553, issue #6550 — tier T2

- **Title:** `toJSONSchema(): chaining .int() after .min()/.max() replaces explicit bounds`
- **Issue author:** Relequestual (external user, not maintainer/bot) — filed 2026, symptom + a
  runnable minimal reproduction, does not name the root cause (the fix mechanism, "format checks
  overwrite tighter min/max bounds," only appears in the PR title, not the issue).
- **Fix PR:** merged by colinhacks (maintainer, real human) after the boundary date.
- **Diff:** +36/-4 across 3 files; the actual source fix is confined to
  `packages/zod/src/v4/core/checks.ts`; test change in
  `packages/zod/src/v4/classic/tests/bigint.test.ts` and
  `packages/zod/src/v4/classic/tests/to-json-schema.test.ts` — real oracle test material.
- **Checklist status:** 1 (date) pass, 2 (human issue) pass, 3 (symptom-only) pass, 4 (human
  fix author) pass, 5 (real test diff) pass, 6 (diff size) pass, 7 (repo popularity) pass. Not yet
  done: 8/9 (standalone reproduction re-verified against the pre-fix commit), 10 (no concurrent
  dependency), 12 (tier locked before any run) — tier T2 assigned now, before any live test.
- **FIXTURE BUILT AND VERIFIED (2026-09-12).** base_commit=`e7604717801fe5cc17c525fb92abe0abeae0ae6e`,
  fix_commit(merge)=`7a00236683c79000dbab0d92f6faf0b7fba39f59`. Reproduced against the real repo:
  the 2 oracle tests (`bigint.test.ts`, `to-json-schema.test.ts`) fail on base (exactly 2 of 330,
  0 other regressions), pass after the real source fix (`checks.ts`), checklist items 8/9/10/12
  confirmed. Task key `zod-int-json-schema`. Note: fixtures now live under `go-agent/pilot/`
  (gitignored, local-only — `manifest.json` + `worktrees/<task>/`, each worktree a full pnpm
  workspace checkout with `node_modules` installed) instead of the external
  `llm-memory-wall-research` repo referenced by earlier entries below (that repo doesn't exist on
  this machine; the external-research-repo plan is superseded). Set `ANVIL_PILOT=go-agent/pilot`
  to use it. Building this fixture surfaced one harness-adjacent gotcha, not a code fix: zod's
  worktree has a handful of root-level symlinks unrelated to node_modules (`.cursorrules`,
  `README.md`, `.codex/skills`, `CLAUDE.md`) that `repoSnapshot` rejects as "unexpected fixture
  entry" (regular-file check fails on a symlink); resolved by dereferencing those 4 into real
  copies in the worktree once, no `realrepo_test.go` changes needed.

### Candidate 2 — PR #6572, issue #6557 — tier T2

- **Title:** `Excessive memory retention in v4 memoizer`
- **Issue author:** alexvoedi (external user) — real production incident report (an ESLint
  monorepo run going from 49s/5.4GB peak RSS on 4.4.3 to OOM at 181s/8.7GB+ on 4.5.4), with actual
  GC logs attached. Names zero root cause — purely a symptom + repro.
- **Fix PR:** merged by colinhacks (maintainer, real human) after the boundary date.
- **Diff:** +43/-10 across 2 files; source fix confined to
  `packages/zod/src/v4/core/memoizer.ts`; test change in
  `packages/zod/src/v4/classic/tests/cyclic-data.test.ts`.
- **Checklist status:** same as Candidate 1 — 1/2/3/4/5/6/7 pass; 8/9/10/12 not yet verified;
  tier T2 assigned now.
- **FIXTURE BUILT AND VERIFIED (2026-09-12).** base_commit=`e54716cb5676f812cefea686eb07e2df961a0cf8`,
  fix_commit(merge)=`36f17960d1defca5d0896d9424f4e1059fbbf081`. Task key `zod-memoizer-retention`. The
  1 oracle test (a GC-based retention probe, `cyclic-data.test.ts`) fails on base (1/85), passes
  after the real fix (`memoizer.ts`), 0 regressions. Note: at this base commit zod's monorepo had
  switched its declared `packageManager` to `nub@0.8.3` (a real, brew-installable TS package
  manager, not a typo — `brew install nub` gets v0.9.0, close enough to run, but its own `install`
  tried to resolve all ~1400 packages across every workspace member including `packages/docs`
  (pulls in Next.js) just to test 2 files. Abandoned `nub` and instead trimmed this fixture's
  `package.json` `workspaces` field to `["packages/zod"]` only (deleted the other workspace
  packages' directories, added a matching `pnpm-workspace.yaml`) and installed with pnpm directly —
  33s instead of several minutes, 338MB instead of 1.1GB. This is fixture-local (the copied
  worktree only), not a change to the real zod repo or to anything committed here beyond
  `pilot/manifest.json`.

### Candidate 3 — PR #6582, issue #6577 — tier T3

- **Title:** `Compatibility reconsideration: defaulted discriminator fields prevent validation of
  explicitly tagged commands`
- **Issue author:** david-gettins (external user) — real production impact report with a linked
  standalone reproduction repo, references an earlier discussion (#6545) but describes the
  compatibility symptom, not the fix.
- **Fix PR:** merged by colinhacks (maintainer, real human) after the boundary date.
- **Diff, real source only** (excluding docs, which is 2 lines and irrelevant to the fix):
  `packages/zod/src/v4/core/compile.ts` (+1/-1), `packages/zod/src/v4/core/schemas.ts` (+34/-28)
  — 2 real source files. Test oracle: `discriminated-unions.test.ts` (+112/-6).
- **Checklist status:** 1/2/3/4/5/7 pass; 6 (diff size) borderline — real source diff is ~63
  lines across 2 files, at the edge of the informal ceiling but multi-file scope is exactly what
  T3 is for. 8/9/10/12 not yet verified.
- **FIXTURE BUILT AND VERIFIED (2026-09-12).** base_commit=`1c51cbe0fe23d09f8d520b31487d50a01588fae5`,
  fix_commit(merge)=`b12aa523e7e2617c4296cccf9b24d6558ed23e95`. Task key
  `zod-discriminated-union-defaulted-tags`. 6 oracle tests fail on base (6/76), all pass after the
  real 2-file fix (`compile.ts` + `schemas.ts`), 0 regressions across the other 70. Same trimmed-
  workspace install approach as candidates #1/#2 (`workspaces: ["packages/zod"]`, pnpm directly,
  ~15s install, 338MB worktree).

## immerjs/immer (real, ~28k+ stars, dependency-free, fast test suite)

### Candidate 4 — PR #1283 — tier T1

- **Title:** `fix: key inserted array indices by name in the array-methods plugin`
- **Author:** Jaybhade (external contributor). No separate linked issue — the PR body itself is
  the bug report (a runnable repro showing a patch-stream silently drops a change under
  `enableArrayMethods()`), written before any fix reasoning; usable as a symptom-only prompt if the
  fix-mechanism sentences are excluded when constructing the task text.
- **Diff:** source fix confined to `src/plugins/arrayMethods.ts` (+6/-1); test oracle in
  `__tests__/base.js` (+17/-0). Small, single-file, exactly T1 scope.
- **Checklist status:** 1/4/5/6/7 pass; 2/3 pass with the caveat above (no separate issue, must
  extract symptom-only text from the PR body); 8/9/10/12 not yet verified.
- **FIXTURE REBUILT AND VERIFIED (2026-09-12), under `go-agent/pilot/` this time** — the original
  claim below (this entry, written in an earlier session) pointed at
  `llm-memory-wall-research/results/EXP-086/`, which doesn't exist on this machine; rebuilt from
  scratch rather than trusted as-is. base_commit=`a3be9df762c1dbe9959a011ddbab0ce838cbc468`,
  fix_commit=`d2c158f5bac7081a760bbaf501ea5c360b7856e1`. Reproduced against the real repo: the new
  oracle test fails on base (exactly 1/3224), passes after the real source fix, 0 regressions.
  Task key `immer-array-push-fix`.
  <details><summary>Original (unverified) note, kept for history</summary>
  Reproduced against the real repo: the new
  oracle test fails on base (exactly 1 failure, no other regressions), passes after the real
  source fix, checklist items 8/9/10/12 confirmed. Fixture lives at
  `llm-memory-wall-research/results/EXP-086/` (`manifest.json` +
  `worktrees/immer-array-push-fix/`), task key `immer-array-push-fix`, consumable by
  `go-agent`'s `prepareRepo`/`checkRepoReport` with zero code changes beyond the generalization
  below. This is the first fixture built under the new methodology, end to end.
  </details>

### Candidate 5 — PR #1289 — tier T3

- **Title:** `fix: preserve structural sharing for no-op array-methods calls`
- **Author:** maximilliangrand (external contributor). No separate linked issue; PR body is a
  self-contained symptom report (a change-free `produce()` call under `enableArrayMethods()`
  breaks Immer's referential-equality/structural-sharing guarantee), with a runnable repro.
- **Diff:** source fix across 2 real files — `src/plugins/arrayMethods.ts` (+87/-25),
  `src/utils/common.ts` (+11/-7); test oracle in `__tests__/base.js` and `__tests__/map-set.js`.
- **Checklist status:** 1/4/5/6/7 pass; 2/3 pass with the same no-separate-issue caveat as
  Candidate 4; 8/9/10/12 not yet verified.
- **FIXTURE BUILT AND VERIFIED (2026-09-12).** base_commit=`e3df956dca6f62c9053ab750b2c8b8a518f3e001`
  (= candidate 4's fix_commit — the two build on each other in immer's real history),
  fix_commit(merge)=`907395ad21aef22ed2c71d11f4fb4172683ab62a`. Task key
  `immer-array-methods-noop-sharing`. 10 oracle tests fail on base (10/3339 across `base.js` +
  `map-set.js`), all pass after the real 2-file fix, 0 regressions. Installed with `yarn` (immer's
  own tooling, not a monorepo, no exotic package-manager issue like zod's).

## ljharb/qs (real, extremely widely depended-upon, dependency-light, fast test suite)

### Candidate 6 — PR #595, issue #467 — tier T1

- **Title:** `Serialization for Date is not working when using filter option.`
- **Issue author:** joonseokhu (external user) — symptom + full runnable reproduction, no root
  cause named.
- **Fix PR:** deepshekhardas (external contributor, not the maintainer) — the cleanest authorship
  diversity of any candidate so far: issue filer, fixer, and maintainer are three different people.
- **Diff:** source fix confined to `lib/stringify.js` (+3/-3); test oracle in `test/stringify.js`
  (+60/-0). Small, single-file, clean T1.
- **Checklist status:** 1/2/3/4/5/6/7 all pass cleanly, no caveats. 8/9/10/12 not yet verified.
- **FIXTURE BUILT AND VERIFIED (2026-09-13).** base_commit=`8859c37470e11b42b547b275e4e9bd0bc8cc5464`,
  fix_commit=`62fd25480b0b0d9c0a667ee67e13608a363f5d0e`. Task key `qs-stringify-date-filter`. 6
  oracle tests fail on base (6/446), all pass after the real fix, 0 regressions. **TAP support,
  resolved without any go-agent code change**: qs's test suite runs on `tape`, which emits TAP
  text (`ok N description` / `not ok N description`) instead of the vitest/jest JSON shape
  `checkRepoReport` expects — but `test_command` is fully data-driven per-task, so the fix is a
  ~20-line converter script (`scripts/tap-json.mjs`) shipped INSIDE the qs worktree itself: it
  spawns the real tape test file, parses the TAP output, and writes it out in the exact
  `{testResults:[{assertionResults:[{FullName,Status}]}]}` shape the harness already reads. No
  `realrepo_test.go` change needed — the harness was already general enough.

## Sourcing note: an AI-authored false positive, caught

`pmndrs/zustand`'s two most recent post-boundary "fix" PRs (#3555, #3531) were both authored by
`app/copilot-swe-agent` — a GitHub Copilot AI coding agent, not a human. Checklist items 4/11 (no
AI-authorship markers) exist precisely for this: an increasing share of "real" merged fixes on
active repos are themselves AI-generated, which would silently reintroduce the exact contamination
problem this methodology exists to prevent (measuring one model's output against another model's
prior fix). zustand is excluded from sourcing entirely for this window as a result, not just these
two PRs — worth spot-checking any repo's recent PR authors before investing time reading further.

## Status: 6 candidates found, spanning all 3 tiers (2 each)

Sourcing and fixture construction for this first batch (2026-09-12/13) are complete. The six
fixtures are mechanically verified under `go-agent/pilot/`; any new contamination-sensitive
comparison must rerun the title-only probe against the active target before treating the pool as
fresh evidence.

**Important repo-runner split, discovered while building the first fixture:** qs uses `tape`
(TAP output), incompatible with the harness's JSON-report-based grading (built for vitest/jest).
zod and immer both use `vitest` — compatible with zero harness changes. Candidate #6 (qs) is
deprioritized until the harness gains TAP support; candidates #1/#2/#3 (zod) and #4/#5 (immer) can
proceed immediately.

**Candidate #4 (immer PR #1283, T1) is now a fully built and verified fixture** — see its entry
above for commit hashes and location (`EXP-086`). Building it surfaced and fixed a real harness
gap: `checkRepoReport` originally required the entire test file to be independently clean with
globally unique test names, which broke against immer's real, large (3223-test) suite (which
genuinely has duplicate names elsewhere, unrelated to this fixture). Fixed by scoping grading to
only the tests named in `fail_to_pass`/`pass_to_pass` — see `go-agent/realrepo_test.go`'s
`checkRepoReport`. `prepareRepo`/`checkRepoChanges` were also generalized to read `test_command`
and `source_files` from the manifest instead of hardcoding date-fns/dayjs-specific logic, with the
old hardcoded behavior kept as a fallback so the original date-fns/dayjs pilot keeps working
unchanged (re-verified: `TestPreparedRealRepoBaselines` still passes both stages).

**Update (2026-09-12): candidates #1/#2/#3/#4/#5 are all now fully built and verified fixtures**
under `go-agent/pilot/` — see each entry above for exact base/fix commit hashes. All 5 pass
`TestPreparedRealRepoBaselines` (reproduces the real bug, preserves the sampled passing tests, and
the full `go test ./...` suite (162 tests) stays green throughout). `EXP-086` (the immer fixture
this doc previously pointed at under `llm-memory-wall-research`, which turned out not to exist on
this machine) has been superseded by the rebuilt `immer-array-push-fix` under `go-agent/pilot/`.

**Update (2026-09-13): candidate #6 (qs) is also built** — see its entry above. All 6 sourced
candidates are now real, verified fixtures under `go-agent/pilot/`
(`zod-int-json-schema`, `zod-memoizer-retention`, `zod-discriminated-union-defaulted-tags`,
`immer-array-methods-noop-sharing`, `immer-array-push-fix`, `qs-stringify-date-filter`), all
passing `TestPreparedRealRepoBaselines` with the full `go test ./...` suite (162 tests) staying
green throughout. Sourcing + fixture-building for this batch is complete. Next step is running
these against a live model (see go-agent's `ANVIL_PILOT` / `TestLiveModel*`), not sourcing more
candidates.
