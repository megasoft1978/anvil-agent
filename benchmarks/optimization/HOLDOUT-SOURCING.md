# Frozen holdout sourcing

These two tasks are prepared as unused post-boundary holdouts for tuning validation. Their fixture
checks remain pending until the explicit `validate` command confirms base failure, reference
success, and invalid-fixture failure.

| Task | Issue | Fix | Base → reference | Source files |
|---|---|---|---|---|
| `zod-prefault-undefined` | [#6585](https://github.com/colinhacks/zod/issues/6585) | [#6587](https://github.com/colinhacks/zod/pull/6587) | `0c483c5` → `9446b5c` | `compile.ts`, `schemas.ts`, `mini/schemas.ts` |
| `undici-mockagent-global-fetch` | [#5036](https://github.com/nodejs/undici/issues/5036) | [#5648](https://github.com/nodejs/undici/pull/5648) | `7f50f74` → `4ae29b2` | `lib/mock/mock-agent.js` |

Both fixes merged after the 2026-07-30 boundary, have human issue and fix authors, and include
focused regression tests. The task reports retain the issue symptoms and expected behavior without
including the source-file diagnosis or fix mechanism. Base/reference/invalid oracle checks passed
in session `20260913T131930072Z-b401994db269`; title-only contamination probes remain pending.
