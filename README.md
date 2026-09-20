<p align="center"><img src="assets/logo.png" width="140" alt="anvil-agent logo"></p>

# anvil-agent

**A local coding model, sized to fit your Mac, driven by this repo's own Go harness. One command to get a
server running; go-agent does the actual read/edit work.**

```
curl -fsSL https://raw.githubusercontent.com/megasoft1978/anvil-agent/main/setup.sh | bash
```

Checks your hardware, downloads the current target model (once), and starts a `llama-server` tuned for it.
No cloud API, no account, nothing leaves your machine once the model's on disk.

The [Go agent harness](go-agent/README.md) is what actually reads and edits against that server —
a small read/edit loop with configurable deadlines and JSONL traces. See its
[offline verification results](go-agent/TESTING.md) for what's been checked so far.

This project is currently tuned for the verified local winner, Qwen3-Coder-30B-A3B Q2_K (see
`go-agent/profiles.go`): its sampling defaults and server flags are kept together so the shipped
configuration stays reproducible. The fresh-install script uses the same 8K-context, low-reasoning
configuration that passed the strongest local repair screen.

## Benchmarks

The local screen uses the same Go harness, native Read/Search/Edit/Write tools, and external
fixture oracles on an Apple M1 Mac mini with 16 GiB unified memory. Quality is reported as
verified oracle behavior, alongside wall time, llama.cpp throughput, and peak memory. The
complete ledger and immutable artifact identities are in the [local model summary](docs/LOCAL-MODEL-SUMMARY-2026-09-20.md)
and [audit](docs/LOCAL-MODEL-AUDIT-2026-09-19.md).

### Current local model screen — paused 2026-09-20

The current local screen uses the same Go harness and external oracle on the
same Apple M1/16 GiB machine. Prefill/decode are llama.cpp-reported tokens per
second; quality is the oracle result; memory is peak server RSS and minimum
free memory. The measurements are representative runs rather than a perfectly
controlled benchmark because reasoning modes and tasks differ. The complete
ledger, including excluded models and immutable artifact identities, is in the
[local model summary](docs/LOCAL-MODEL-SUMMARY-2026-09-20.md) and [audit](docs/LOCAL-MODEL-AUDIT-2026-09-19.md).

| Model | GGUF size | Quality result | Prefill / decode | Wall | Peak RSS / min free |
|---|---:|---|---:|---:|---:|
| **Qwen3-Coder 30B A3B** | 11.259 GB | QS + job queue passed; guarded Bash passed | 120.5 / 17.0 tok/s | 148 s | 8.77 GiB / 55 MB |
| **Qwen3.6 27B A3B Coder** | 9.058 GB | Job queue passed 4/4; second task pending | 115.8 / 9.6 tok/s | 552 s | 8.43 GiB / 56 MB |
| **Granite 4.2 3B** | 2.244 GB | 2/3 native repair tasks passed | 187.1 / 18.9 tok/s | 476 s | 3.86 GiB / 58 MB |
| **Qwen3.5 4B** | 2.741 GB | Job queue passed; QS near-pass; Immer no edit | 175.6 / 12.6 tok/s | 202 s | 3.39 GiB / 64 MB |
| **Ministral 3 14B** | 8.240 GB | QS passed; job queue failed | 58.7 / 5.9 tok/s | 230 s | 9.21 GiB / 56 MB |
| **Ministral 3 8B** | 5.199 GB | QS near-pass; queue priority failed | 89.4 / 8.9 tok/s | 284 s | 6.20 GiB / 54 MB |
| **Bonsai 2 27B** | 7.206 GB | Medium QS repair verified; low/xhigh too slow | ~30–31 / 3.3–4.6 tok/s | 1,800 s cap | ~8.7–8.8 GiB / 54–55 MB |
| **Gemma 4 26B A4B** | 10.015 GB | Gates passed; scored repair pending | — | — | ~10.1 GiB / 57–61 MB |

The current practical baseline is Qwen3-Coder 30B A3B: it is the fastest model
with multiple verified repairs. The separate Qwen3.6-27B comparison also demonstrated
a complete multi-file repair, but at substantially higher latency. There are eight actual
GGUF weights remaining, totaling 55,960,893,536 bytes (52.118 GiB).

### Direct assessment — GPT-5.6 Luna Max (2026-09-20)

I evaluated the recorded traces, native tool calls, external oracle outcomes, and memory logs
directly. The evidence supports this assessment:

- **Qwen3-Coder 30B A3B Q2_K is the current winner for this machine.** It completed two
  independent repairs—`qs-stringify-date-filter` and the multi-file `job-queue` task—with
  eight native tool calls per task and every external oracle check passing.
- **The repairs were functionally precise, not merely plausible edits.** The Date repair covered
  nested values, filter-produced Dates, comma-separated arrays, custom serialization, and invalid
  Dates. The queue repair passed priority ordering, terminal-job idempotency, exponential backoff,
  completion-time retention, and the remaining queue checks.
- **Native tools should remain primary.** A guarded Bash retest also passed the queue oracle, but
  Bash-only mode exhausted its turn budget. Bash is useful as a constrained supplement, not a
  replacement for the structured read/search/edit loop.
- **The practical limit is memory headroom.** The winner used roughly 8.4–8.8 GiB RSS and left
  only about 55 MB of minimum free memory on this 16 GiB host. The low-reasoning, 8K profile is
  usable, but cold-start conditions and everyday applications can change the result.
- **Confidence is strong for the current baseline, not universal.** The winner has the best
  combination of verified repair breadth and latency in the recorded screen, but the models were
  not all run on an identical task/mode matrix. A common-task follow-up is still required before
  claiming a statistically general ranking.

The recommendation is therefore to keep Qwen3-Coder 30B A3B as the shipped default, keep Bash
guarded and opt-in, and treat the remaining models as comparison or fallback candidates rather
than replacing the current baseline.

<details>
<summary><strong>Advanced options</strong></summary>

Passing a flag through a pipe needs `bash -s --` (otherwise bash reads the flag itself):

```
curl -fsSL <raw-url>/setup.sh | bash -s -- --doctor          # diagnose an existing install, read-only
curl -fsSL <raw-url>/setup.sh | bash -s -- --start-only       # (re)start the server with the validated flags
curl -fsSL <raw-url>/setup.sh | bash -s -- --upgrade          # reapply the current config + restart the server
curl -fsSL <raw-url>/uninstall.sh | bash                      # remove everything
```

`--doctor` is the first thing to run if something's wrong. `setup.sh --help` and `uninstall.sh --help` list
every other flag.

**Requirements:** Apple Silicon Mac (M1/M2/M3/M4), 16GB unified memory or more, ~13GB free disk, and
[Homebrew](https://brew.sh) + [Node.js](https://nodejs.org) if you don't already have them. Building go-agent
also needs [Go](https://go.dev). Rather not pipe a script into `bash`? Read `setup.sh` first — one
self-contained file, every command visible.

</details>
