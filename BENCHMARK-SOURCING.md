# Real-bug sourcing methodology

Written 2026-09-12. This is a fresh write-up, not a recovered document — an earlier version of
this methodology was designed and discussed in this project but never persisted to a file or
committed; it isn't in git history. Nothing below should be read as reconstructing that exact
earlier text, only as a defensible replacement built from the same goals.

## Why this exists

Benchmarking a coding model on "real" bugs is worthless if the model may have seen the bug (or
its fix) during training, and worthless again if the "bug" is itself AI-generated rather than a
real defect a human found and fixed. This document is the checklist a candidate fixture must pass
before it's used to measure the current Qwen3.6-35B-A3B target against Sonnet 5.

## Date boundary

A bug is eligible only if its fix was merged **after** a computed boundary date:

```
D = max(target model's weight publication date, comparison model's release date) + 30 days
```

Currently: Qwen3.6-35B-A3B publication date and Sonnet 5's release date, whichever is later, plus
a 30-day margin (training data cutoffs are rarely the same as release dates, and a margin absorbs
uncertainty about exactly when a model's pretraining corpus was frozen). Recompute this whenever
either model changes.

**Current value: D = 2026-07-30** (Qwen3.6-35B-A3B published 2026-04-17; Sonnet 5 released
2026-06-30, the later date; +30 days). Only PRs merged on or after this date are eligible.

## Sourcing method

Query merged PRs on real, popular repositories via the `gh` CLI rather than hunting manually:

```
gh search prs --merged --merged-at ">=<D>" --label bug --repo <owner/repo> \
  --json number,title,url,closingIssuesReferences,files,author
```

Candidate repos: real, widely-used, dependency-light Node/TS libraries (so `npm test` runs
offline in well under a minute) — e.g. zod, immer, ms, qs, clsx, zustand, tanstack/query. Avoid
UI-framework repos requiring a DOM test harness unless the sourcing pool is otherwise too thin.

## Eligibility checklist (all must pass)

1. PR merged on or after the boundary date `D`.
2. A linked issue exists (`closingIssuesReferences` non-empty) and is filed by a human account —
   not a bot, not obviously LLM-generated wording (repetitive scaffolding, "As an AI..." phrasing).
3. The issue describes a **symptom**, not a diagnosis — it should not name the fix or the exact
   root cause; if it does, the fixture is a diagnosis-assisted condition, not a blind one, and must
   be labeled as such rather than treated as equivalent to a symptom-only fixture.
4. The fix commit's author is a human account, not a known bot or an AI-assisted-commit pattern
   (no "Co-authored-by" trailer naming an AI tool, no auto-generated PR bodies).
5. The PR includes a real test diff (new or modified `*.test.*`/`*.spec.*` file) alongside the
   source fix — this becomes the `fail_to_pass` oracle.
6. The source diff is small and targeted (rough ceiling: 60 changed lines) — a sprawling refactor
   isn't a "bug fix" fixture and makes grading ambiguous.
7. The repo has a real usage signal (informal floor: ~2k+ GitHub stars) — an abandoned or toy repo
   raises the chance the "bug" is noise rather than a defect anyone cared about.
8. The bug is reproducible standalone: the new/modified test fails against the pre-fix commit and
   passes after, with no other unrelated commits needed in between.
9. A representative sample of previously-passing tests (`pass_to_pass`) still pass on the pre-fix
   commit — confirms the fixture isolates this one bug, not a broader breakage.
10. The fix doesn't depend on a concurrent, unrelated PR (check the merge-base and surrounding
    history) — the fixture must be extractable as a single before/after pair.
11. No AI-authorship markers anywhere in the PR: body, commit messages, or code comments.
12. The task's complexity tier (single-file/symptom-named, single-file/root-cause-hidden, or
    multi-file) is assigned from the diff and issue text alone, **before** any model is run against
    it — never re-tiered after seeing how a model performed, which would silently cherry-pick easy
    fixtures into "hard" or vice versa.

## Contamination probe (in addition to the date boundary)

Before running the real task, send the model **only the issue title** (no body, no repo access,
no fixture context) and ask what it thinks the bug and fix are. If it names the actual file or the
actual fix mechanism from the title alone, discard the fixture — that specific outcome is a much
stronger memorization signal than the date boundary alone, since a boundary date is a proxy, not a
guarantee, for what was actually in a training corpus.

## Grading separation

Two independent measurements, never conflated:

- **Oracle pass/fail** — mechanical: does `fail_to_pass` flip from failing to passing, do
  `pass_to_pass` tests still pass, after the model's actual edit. This is ground truth for
  "did it work," computed the same way regardless of which model produced the edit.
- **Quality judging** — Opus 5 grades a transcript (this model's or Sonnet 5's) blind: model
  identity redacted, tool-call formats normalized so a judge can't infer identity from harness
  quirks. Isolated single-round scoring first (rubric: hypothesis quality, edit precision,
  verification discipline, wasted turns); a paired head-to-head round is only worth adding once
  isolated scores show enough variance to make a comparison meaningful.

A fixture that fails the oracle can still be worth keeping for quality judging (e.g. "diagnosed
correctly, ran out of turns") — the two scores answer different questions and should be reported
side by side, not merged into one number.
