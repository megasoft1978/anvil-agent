# Disk cleanup plan

Read-only snapshot after the conservative cleanup on 2026-09-14:

- Data volume: 460 GiB total, about 405 GiB used, about 32 GiB available, 93% full.
- Local model store: about 27 GiB (`Qwen3.6-35B-A3B-UD-IQ2_M.gguf` and the historical TQ3 model).
- Optimization evidence: about 87 MiB after removing regenerable fixture/worktree trees from stale sessions; compact manifests, decisions, JSONL rows, and server logs were retained.
- Separate research checkout: about 34 GiB, including 28 GiB of candidate GGUFs and 3.9 GiB of results.
- Regenerable caches: about 442 MiB Go build, 237 MiB Homebrew, and 100 MiB pip, plus larger app-owned caches.

The next smaller-model candidate is the Qwen3.5-9B MLX tree, observed at about 5.98 GB. The prepared
downloader requires that estimate plus a 4 GiB reserve, so the current snapshot has enough space for
one isolated download if no other process consumes the headroom. Disk cleanup does not reduce unified-
memory pressure or a model's resident allocation.

The cleanup already removed stale generated fixture/worktree trees under the retained optimization
sessions after compact evidence was preserved. The separate research checkout still contains
generated dependency trees under:

```text
~/Desktop/Freelance/llm-memory-wall-research/results/EXP-085/**/node_modules
~/Desktop/Freelance/llm-memory-wall-research/results/EXP-086/**/node_modules
```

They account for about 3.0 GiB, are ignored and untracked, and their lockfiles, manifests, source
trees, reports, and experiment notes remain outside those directories. They can be regenerated if
those historical experiments are reopened. Before removing them, confirm that no process has an open
file below either result root and retain the experiment manifests and reports.

The next candidates are old result-session generated worktrees after their compact summaries have
been retained. Keep the current preparation session and rejected-run evidence until the optimization
report is complete. The 16 GiB local TQ3 model and the 28 GiB research candidate models are the
largest possible recoveries, but they should be removed only after their branches are explicitly
closed or the artifacts are copied to another disk. The current Qwen3.6 GGUF must remain until its
8K probe is complete.

Regenerable caches can be cleared after their owning applications and build processes are stopped:

```text
~/Library/Caches/go-build       ~442 MiB
~/Library/Caches/Homebrew       ~237 MiB
~/Library/Caches/pip            ~100 MiB
```

Clear one target at a time, recheck `df -h /System/Volumes/Data` after each target, and stop if free
space decreases unexpectedly or an application reports a cache error. Do not clear active application
caches, the current optimization session, fixture dependencies, model files, or research results as
part of a generic cache purge. The read-only inventory command is:

```sh
cd /Users/megasoft78/Desktop/Freelance/anvil-agent
node benchmarks/optimization/disk-report.mjs
```

The current sandbox cannot remove the research-checkout files because that directory is outside the
writable workspace. No further cleanup is required for the first Qwen3.5 download; if more disk is
needed, remove one explicitly reviewed research cache or candidate after preserving its report and
checking that no process has it open. The batch still requires a reboot and closed applications for
memory reasons, while the runner itself never closes applications or changes system settings.
