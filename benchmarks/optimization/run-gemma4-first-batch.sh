#!/bin/sh
# Run after a fresh Gemma session is prepared. The runner keeps its memory and server safety guards.
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
if [ "${EXTERNAL_SERVER:-0}" = "1" ]; then
  if [ -z "${SERVER_PID:-}" ]; then
    echo "EXTERNAL_SERVER=1 requires SERVER_PID to identify the externally started server" >&2
    exit 2
  fi
  node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" \
    --experiment GEMMA4-LOCAL-QUICK-REAL --external-server --server-pid "$SERVER_PID" --max-runs 1 || status=$?
else
  node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" \
    --experiment GEMMA4-LOCAL-QUICK-REAL --start-server --max-runs 1 || status=$?
fi
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"
exit "$status"
