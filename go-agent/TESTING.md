# Testing go-agent

This project targets Qwen3.6-35B-A3B through an already-running local
llama-server. The agent loop defaults to repository inspection and edits with
no shell tool. Verification commands run from Go test code outside the
model-facing tool surface. Bash is available only when an experiment explicitly
sets `bash_mode` to `guarded` or `only`; those runs still require disposable
worktrees and are not a security sandbox.

## Offline checks

From `go-agent/`:

```sh
go vet ./...
go test ./...
go test -race ./...
```

The default suite uses fake local HTTP endpoints and temporary worktrees. It
covers request/response protocol handling, tool-call recovery, read/edit/search/
write contracts, path and file limits, cancellation, artifact isolation, TUI
event handling, and the host-side process runner.

The process runner remains because offline and live oracle tests need to execute
trusted fixture commands after an agent run. It is not exposed as a model tool.

## Local model smoke test

Start a healthy Qwen3.6-35B-A3B server with enough free memory, then run:

```sh
ANVIL_LIVE=1 go test -run '^TestLiveModel$' -v -count=1 -timeout=5m
```

The suite runs `read-and-finish` and `edit-and-test` sequentially. The second
stage verifies the edited fixture with a host-side command after the model
returns. It never starts or stops a server it does not own.

Optional variables:

- `ANVIL_LIVE_ENDPOINT`: loopback OpenAI-compatible endpoint.
- `ANVIL_LIVE_MODEL`: model ID sent to the server; defaults to `qwen36-35b-a3b`.
- `ANVIL_LIVE_OUTPUT`: directory for traces.
- `ANVIL_LIVE_TIMEOUT`: per-stage agent deadline; defaults to `120s`.
- `ANVIL_LIVE_MAX_TOKENS`: per-request completion limit.
- `ANVIL_ALLOW_PAGING=1`: keep running on macOS pageouts while recording them.

On macOS, the live suite checks pageouts once per second. Verify free memory
before loading the server; pageout monitoring is only a cancellation guard, not
a substitute for that check.

## Prepared real-repository checks

Use a prepared pilot directory in `ANVIL_PILOT`. Before loading the model,
confirm that the pristine fixtures still reproduce their recorded failures:

```sh
ANVIL_PILOT=/path/to/prepared/pilot \
  ANVIL_BASELINE_TASKS=immer-array-push-fix,zod-int-json-schema \
  go test -run '^TestPreparedRealRepoBaselines$' -v -count=1
```

Then run one stage at a time with a healthy server:

```sh
ANVIL_LIVE=1 \
ANVIL_PILOT=/path/to/prepared/pilot \
ANVIL_REPO_STAGE=immer-array-push-fix \
go test -run '^TestLiveModel$' -v -count=1 -timeout=8m
```

Advance to another stage only after inspecting the preceding trace. Each stage
uses a fresh disposable worktree and runs its trusted oracle command after the
agent finishes. The grader requires the expected failures to pass, preserves
tracked regressions, rejects changes outside designated source files, and keeps
coverage/report artifacts outside the model worktree.

The two-step experiment (`TestLiveModelTwoStep`) separates diagnosis from
implementation. The diagnosis call is read/search-only; the implementation
call receives preloaded source and can edit it. Host-side oracle execution
checks the resulting source after the implementation call.

## Replaying timed-out edits

When an agent run times out after making edits, the trace contains durable
before/after records. With `ANVIL_PILOT` and `ANVIL_REPO_STAGE` set, replay a
trace against a fresh fixture:

```sh
ANVIL_REPLAY_TRACE=/path/to/trace.jsonl \
  go test -run '^TestReplayRealRepoOracle$' -v -count=1
```

Replay is a postmortem check. It does not turn an unfinished model run into an
in-budget completion.
