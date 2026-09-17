#!/bin/sh
# Run only after the Qwen3.5 model tree is downloaded, a fresh session is prepared,
# and the host memory preflight is acceptable. The runner keeps its memory and server safety guards.
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
  echo "prepared session does not exist: $SESSION (download the model and prepare a fresh session first)" >&2
  exit 2
fi
if [ ! -x "$CLI" ]; then
  echo "Go CLI is missing or not executable: $CLI (rebuild and prepare a fresh session first)" >&2
  exit 2
fi
node benchmarks/optimization/runner.mjs validate --session "$SESSION" --fetch-reference
status=0
node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" \
  --experiment MLX-QWEN35-9B-8K-REAL --start-server --max-runs 1 || status=$?
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"
exit "$status"
