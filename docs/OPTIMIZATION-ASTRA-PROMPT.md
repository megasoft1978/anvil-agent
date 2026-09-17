# Prompt for GPT Astra

> Review precedence: [the revised decision package](OPTIMIZATION-ASTRA-DECISION.md#reviewed-decision-and-precedence--2026-09-14) controls priorities and interpretation where this document differs. The first live probe is recorded in the attached decision package; later model-dependent validation remains pending.

Attach these four files with this message:

- `docs/OPTIMIZATION-ASTRA-DECISION.md`
- `docs/OPTIMIZATION-CODEX-GOAL.md`
- `docs/OPTIMIZATION-ASTRA-REVIEW.md`
- `docs/OPTIMIZATION-RESEARCH-2026-09-14.md`

Then paste:

```text
Act as a rigorous reviewer of the attached local coding-agent optimization goal and research brief.
The target is an Apple M1 Mac with 16 GiB unified memory, the existing Go tool-calling agent harness,
and real repository bug repair through native read/search/edit/write calls.

Review the evidence and sources, check every requirement against the proposed execution order, and
improve the plan so GPT 5.6 Luna Max can execute it economically and safely. Preserve the hard
constraints: one local server and one attempt at a time, no external agents or models, no automatic
model downloads, no automatic application closing or memory-setting changes, external verification of
the resulting worktree, complete JSONL instrumentation, resumable execution, and persistent runner
safety guards before fixture validation or any model-dependent inference, warmup, or benchmarks.

Treat the dated research brief as the source for the current engine shortlist. It separates two
decision lanes: (a) the headroom-based repair hypothesis using a smaller clean model, and (b) the
current-model SSD lane using a Qwen3.6-compatible Metal engine. The current research ranking first
tested the local Qwen3.6 GGUF. Its 8K probe made one native warm-smoke call but stopped at the first
capability gate under warning pressure and swap-out growth, before a scored repair. The next
repair-quality probes are Qwen3.5-9B MLX and Gemma 4 E4B; for preserving
Qwen3.6, evaluate Hebrus first, Qwisp strict and then bolt only with the repair oracle, and a pinned
TurboQuant-MLX v0.6.x rescue only if its native routing and tool protocol remain exact. Treat
published M1 Pro, M4, M5, GPU, and changed-quality results as directional evidence rather than local
performance or repair proof. Treat top-k reduction, fixed hotlists, learned sparsification and dropped tokens as approximate
model variants. Repair tests cannot prove model equivalence. Exact speculative decoding requires
correct target verification and hybrid-state rollback; passing sampled tasks is not a proof.

Pay special attention to:

1. whether the recorded Qwen3.6 GGUF/llama.cpp 8K stop is correctly classified as a memory failure,
   and whether a smaller repair model or a Qwen3.6 SSD backend has the higher expected value;
2. whether the 8K fit probe, 16K repair, smaller Qwen3.5/Gemma candidates, and harness ablations are
   ordered with useful stopping rules;
3. whether the proposed memory estimates and published model claims are clearly separated from local
   measurements;
4. whether the custom Go harness measures verified repair quality, native tool parsing, time, context,
   cache behavior, and unified-memory pressure without introducing confounders;
5. whether disk cleanup, model provenance, parser/template compatibility, and the 16 GiB host safety
   conditions are explicit and reversible.
6. whether the storage math is measured at the right boundary: compute time, bytes per token, physical
   read count and size, residual misses, prediction precision/recall, wasted reads, queue depth, and
   overlap. Check that any learned expert predictor preserves the native router as final authority and
   is rejected when prediction overhead or wasted I/O exceeds the measured benefit.

Return a compact decision package for Codex:

- `KEEP`: statements and experiments that are supported;
- `CHANGE`: concrete corrections, with replacement wording or JSON fields;
- `ORDER`: the final ordered batches, each with its exact command, gate, stop rule, expected runtime,
  and uncertain memory requirement;
- `LUNA HANDOFF`: a paste-ready execution prompt that reads the revised goal and plan, performs only
  the named batch, resumes from JSONL checkpoints, and reports pending validations honestly.

Do not run commands, download weights, start a server, perform inference, or invent local results.
Do not ask for raw traces unless a specific cited artifact is insufficient; the attached brief links to
the compact result records. Keep the returned text concise enough for Codex to apply without rereading
the entire repository.
```

After Astra returns its corrections, give the response back to Codex and ask it to apply only the
accepted documentation/configuration changes and run static checks. Codex may then execute the named
batch directly after the clean-host preflight, while retaining the memory and stopping rules.
