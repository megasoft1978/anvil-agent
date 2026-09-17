#!/bin/sh
# Run after a fresh Qwen3.8 session is prepared. The runner keeps its memory and server safety guards.
set -eu
if [ "$#" -eq 1 ]; then
  SESSION=$1
elif [ "$#" -eq 2 ] && [ "$1" = "--approved" ]; then
  SESSION=$2
else
  echo "Usage: $0 /absolute/prepared/session" >&2
  exit 2
fi
CLI=${CLI:-/tmp/anvil-agent-opt}
cd /Users/megasoft78/Desktop/Freelance/anvil-agent
if [ ! -f "$SESSION/session.json" ]; then
  echo "prepared session does not exist: $SESSION (prepare a fresh session first)" >&2
  exit 2
fi
if [ ! -x "$CLI" ]; then
  echo "Go CLI is missing or not executable: $CLI (rebuild and prepare a fresh session first)" >&2
  exit 2
fi
node benchmarks/optimization/runner.mjs validate --session "$SESSION" \
  --task immer-array-push-fix,notify-channel
status=0
node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" \
  --experiment QWEN38-LOCAL-8K-REAL --start-server --max-runs 1 || status=$?
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"
exit "$status"
