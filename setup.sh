#!/usr/bin/env bash
# anvil-agent — one command, no clone.
#
#   curl -fsSL https://raw.githubusercontent.com/megasoft1978/anvil-agent/main/setup.sh | bash
#
# Other modes (note the `bash -s --` needed once you pass a flag through a pipe -- without it, bash parses
# `--doctor` as its own flag rather than the script's):
#
#   curl -fsSL <raw>/setup.sh | bash -s -- --doctor        # diagnose an existing install, read-only
#   curl -fsSL <raw>/setup.sh | bash -s -- --start-only     # (re)start the server with the validated flags
#   curl -fsSL <raw>/setup.sh | bash -s -- --force-download # re-download the model even if a same-size file exists
#   curl -fsSL <raw>/setup.sh | bash -s -- --check           # compare this install against the latest release
#   curl -fsSL <raw>/setup.sh | bash -s -- --upgrade         # reapply current config + restart the server
#   curl -fsSL <raw>/setup.sh | bash -s -- --report-speed    # measure real tokens/sec on an estimate-only chip
#
# Modifiers: --yes answers yes to EVERY prompt, including "install llama.cpp now?" (brew) -- it is explicit
# consent for an unattended install, so only pass it when that is what you want.
#
# One mode needs a real git checkout, not the curl-pipe install, because it has data too large to embed here:
#
#   ./setup.sh --benchmark [scenario-id|all]   # grade a running server against the 9-scenario suite in benchmarks/
#
# Self-contained on purpose: when piped through `curl | bash`, there is no local checkout to reference sibling
# files from, so every step lives in this one file. Prompts read from /dev/tty rather than stdin, because a
# pipe consumes stdin for the script itself -- reading from stdin inside a piped script gets EOF, not a
# terminal's actual input.
#
# What this does, in order, and why each number is what it is: see README.md in this repo.
set -euo pipefail

# --help is a heredoc rather than a grep of this file's own header: when piped through `curl | bash`, $0 is
# /bin/bash itself, so there is no source file to read.
usage() {
  cat << 'EOF'
anvil-agent -- Qwen3.6-35B-A3B as a local coding assistant on a 16GB Apple Silicon Mac.

  curl -fsSL https://raw.githubusercontent.com/megasoft1978/anvil-agent/main/setup.sh | bash

Other modes (pass flags through a pipe with `bash -s --`, otherwise bash reads them as its own):

  --doctor          diagnose an existing install, read-only
  --start-only      (re)start the server with the validated flags
  --force-download  re-download the model even if a same-size file exists
  --check           compare this install against the latest release
  --upgrade         reapply the current config + restart the server
  --report-speed    measure real tokens/sec on a chip this kit only estimates for
  --benchmark [id]  grade a running server against the 9-scenario suite (needs a git checkout)

Modifiers:
  --yes             answer yes to EVERY prompt, including installing llama.cpp via brew

Details and the numbers behind every setting: README.md in the repo.
EOF
}

# ============================================================================================================
# Argument dispatch — parsed first, before anything touches hardware or disk, so hardware-untouched modes
# (--print-sig, --help) can run with zero side effects, including on Linux CI runners.
# ============================================================================================================
MODE=install
ASSUME_YES=0
FORCE_DOWNLOAD=0
BENCH_TARGET=all
while [ $# -gt 0 ]; do
  case "$1" in
    --doctor) MODE=doctor ;;
    --start-only) MODE=start-only ;;
    --print-sig) MODE=print-sig ;;
    --print-node-snippets) MODE=print-node-snippets ;;
    --selftest) MODE=selftest ;;
    --check) MODE=check ;;
    --upgrade) MODE=upgrade ;;
    --report-speed) MODE=report-speed ;;
    --benchmark)
      MODE=benchmark
      # optional positional scenario id/"all" right after the flag, e.g. `--benchmark cart-checkout`
      if [ $# -ge 2 ] && [ "${2#-}" = "$2" ]; then BENCH_TARGET="$2"; shift; fi
      ;;
    --yes) ASSUME_YES=1 ;;
    --force-download) FORCE_DOWNLOAD=1 ;;
    --help|-h) usage; exit 0 ;;
    *)
      echo "Unknown argument: $1 (see --help)" >&2
      exit 64
      ;;
  esac
  shift
done

# ============================================================================================================
# Constants — the single source of truth for the validated config. Nothing below this block should contain a
# literal that also appears above it. SERVER_FLAGS is a bash array (not a string) so it can be checked
# element-by-element (doctor's flag-drift check) and hashed as a whole (config_sig).
# ============================================================================================================
KIT_VERSION="2026.09.15"
KIT_DIR="$HOME/.anvil-agent"
MODEL_DIR="$KIT_DIR/models"
PORT=8114
MODEL_REPO="unsloth/Qwen3.6-35B-A3B-GGUF"
MODEL_FILE="Qwen3.6-35B-A3B-UD-IQ2_M.gguf"
MODEL_BYTES=11522702304
MODEL_MIN_FREE_GB=12   # model size plus headroom, checked before downloading
MODEL_ID="qwen36-35b-a3b"
CTX=24576
MAX_TOKENS=3072
# These flags are carried over from prior tuning and have not been fully re-validated against this model --
# treat them as a starting point, not a measured-correct config, until someone re-runs --benchmark against it.
# One exception, confirmed live 2026-09-12: `--ctx-checkpoints 0 --cache-ram 0` measured a real memory win on
# the old target model with "no change in generation speed" there, but on Qwen3.6-35B-A3B it causes llama-server
# to periodically discard its ENTIRE prompt-prefix cache instead of extending it incrementally -- confirmed via
# the server's own `timings.cache_n` per request: with these flags, a real 11-turn agentic run hit two full
# re-prefills (one on an 8801-token accumulated conversation, costing 81 real seconds by itself, more than a
# quarter of a 300s budget); with them removed, the identical task's cache_n grew monotonically for all 16
# turns with zero resets, and the run covered 16 turns in 245s instead of 11-12 turns in the full 300s. Real
# memory cost of removing them: server RSS grew from ~8.8GB to ~11.2GB over a longer 16-turn conversation on
# this same box -- still fits in 16GB, but leaves less headroom for other apps than the old flags did. Do not
# reintroduce these two flags without re-measuring cache_n behavior on whatever model is current at the time.
SERVER_FLAGS=(-ngl 99 -fa on -c "$CTX" --no-warmup -np 1 --spec-type ngram-simple --reasoning off -ub 256 -b 256)
# Pinned into config_sig deliberately: --spec-type ngram-simple's acceptance rate is prompt-dependent, so if
# this text ever changed without a version bump, reports collected before and after the change would silently
# describe two different measurements while claiming to be the same number.
REPORT_PROMPT="Write a small TypeScript function that debounces another function by N milliseconds, plus one example call. Explain briefly."
# Fetched by --check ONLY as a staleness beacon -- never sourced or executed, and never supplies a value this
# script acts on (every constant above is still what actually runs). A compromised or lagging beacon can tell
# you you're behind; it cannot change what your machine does.
VERSION_URL="https://raw.githubusercontent.com/megasoft1978/anvil-agent/main/VERSION"
# Substrings doctor checks for in the running server's own command line -- kept separate from SERVER_FLAGS
# because some flags take a value (`-c 24576`) and checking that as one substring is more reliable than
# checking `-c` and `24576` independently, which could each appear for unrelated reasons.
CHECK_STRINGS=("-c $CTX" "-ngl 99" "-fa on" "-np 1" "--no-warmup" "--spec-type ngram-simple" "--reasoning off")

# Only used by --benchmark, which needs a real git checkout (benchmarks/ is too large to embed in this
# self-contained script) -- every other mode ignores these.
SCRIPT_DIR="$(cd "$(dirname "$0")" 2>/dev/null && pwd)" || SCRIPT_DIR=""
BENCH_DIR="$SCRIPT_DIR/benchmarks"

DEST="$MODEL_DIR/$MODEL_FILE"
INSTALL_ENV="$KIT_DIR/install.env"

# ============================================================================================================
# Shared functions
# ============================================================================================================

ask() {  # ask "question" -> reads y/N from the real terminal, not the pipe's stdin
  if [ "$ASSUME_YES" = "1" ]; then return 0; fi
  local prompt="$1" reply="n"
  # /dev/tty can exist and pass `-r` yet still fail at actual read time ("Device not configured") in some
  # sandboxed/detached-process environments with no controlling terminal at all -- confirmed by testing, not
  # just a theoretical case. `read`'s own exit status, not just -t 0 / -r /dev/tty, decides the fallback, so a
  # failed read can never leave `reply` unset under `set -u`.
  if [ -t 0 ]; then
    read -r -p "$prompt " reply || reply="n"
  elif [ -r /dev/tty ] && read -r -p "$prompt " reply 2>/dev/null < /dev/tty; then  # 2> first: the open of /dev/tty is what fails
    :
  else
    echo "(no terminal available to ask '$prompt' -- assuming no)" >&2
    reply="n"
  fi
  case "$reply" in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

# Read one field out of a JSON blob with no jq dependency. EXPR is a JS expression over `j` (the parsed
# object), passed via env rather than string-interpolated -- interpolating a shell variable into a `node -e`
# single-quoted string can't be done safely, and this is the one channel that is safe everywhere.
# Usage: printf '%s' "$json" | EXPR='j.timings.predicted_per_second' json_field
json_field() {
  node -e '
    let s = "";
    process.stdin.on("data", d => s += d);
    process.stdin.on("end", () => {
      try {
        const j = JSON.parse(s);
        const v = (new Function("j", "return " + process.env.EXPR))(j);
        process.stdout.write(v === undefined ? "" : String(v));
      } catch (e) {
        process.exitCode = 3;
      }
    });
  '
}

# config_sig is computed from the live constants, never hand-maintained, so it cannot drift from the code that
# defines it. CI's version-sync job checks this against the repo's own VERSION file.
config_sig() {
  printf '%s\n' "$MODEL_FILE" "$MODEL_BYTES" "${SERVER_FLAGS[@]}" "$CTX" "$MAX_TOKENS" "$REPORT_PROMPT" \
    | shasum -a 256 | cut -c1-16
}

detect_hw() {  # sets CHIP, MEM_BYTES, MEM_GB, ARCH -- no exit, callers decide what to do with the result
  if [ "${ANVIL_KIT_SELFTEST:-0}" = "1" ]; then
    echo "!! ANVIL_KIT_SELFTEST=1: hardware values may be faked -- for CI and testing only !!" >&2
    CHIP="${ANVIL_KIT_FAKE_CHIP:-$(system_profiler SPHardwareDataType 2>/dev/null | awk -F': ' '/Chip/ {print $2}')}"
    MEM_BYTES="${ANVIL_KIT_FAKE_MEMBYTES:-$(sysctl -n hw.memsize 2>/dev/null || echo 0)}"
    ARCH="${ANVIL_KIT_FAKE_ARCH:-$(uname -m)}"
  else
    CHIP=$(system_profiler SPHardwareDataType 2>/dev/null | awk -F': ' '/Chip/ {print $2}')
    MEM_BYTES=$(sysctl -n hw.memsize 2>/dev/null || echo 0)
    ARCH=$(uname -m)
  fi
  MEM_GB=$(( MEM_BYTES / 1073741824 ))
}

# The two refusals. Returns 1 (does not exit) so callers -- install and selftest -- decide what "refused"
# means for them; install exits the whole script, selftest just reports the outcome.
hw_gate() {
  if [ "$ARCH" != "arm64" ] || [ -z "$CHIP" ]; then
    echo "This kit is built for Apple Silicon Macs (M1/M2/M3/M4). Detected: $ARCH, chip: '${CHIP:-none}'." >&2
    echo "It won't run on Intel Macs -- the model quant and memory tuning are Apple-Silicon-specific." >&2
    return 1
  fi
  if [ "$MEM_GB" -lt 15 ]; then
    echo "This kit needs 16GB of unified memory. Detected: ~${MEM_GB}GB." >&2
    echo "The model alone needs ~10GB resident; 8GB and 12GB Macs cannot run it at usable quality." >&2
    return 1
  fi
  return 0
}

# Published spec memory bandwidth per chip, GB/s -- used ONLY to scale a speed estimate, never presented as a
# measurement. Multi-die variants (Pro/Max/Ultra) differ a lot; unrecognised chips fall back to M1's number,
# which under-estimates anything newer rather than over-promising.
chip_bandwidth() {
  case "$1" in
    "Apple M1")        echo 68  ;; "Apple M1 Pro") echo 200 ;; "Apple M1 Max") echo 400 ;; "Apple M1 Ultra") echo 800 ;;
    "Apple M2")        echo 100 ;; "Apple M2 Pro") echo 200 ;; "Apple M2 Max") echo 400 ;; "Apple M2 Ultra") echo 800 ;;
    "Apple M3")        echo 100 ;; "Apple M3 Pro") echo 150 ;; "Apple M3 Max") echo 400 ;;
    "Apple M4")        echo 120 ;; "Apple M4 Pro") echo 273 ;; "Apple M4 Max") echo 546 ;;
    *)                 echo 68  ;;
  esac
}

# TPS_MEASURED is set (non-empty) only for chips this project has a real measurement for. Filling one in here
# plus one README row is the entire workflow for accepting a community chip report (see README "Measured
# chips" section) -- no other code path changes.
chip_measured_tps() {
  case "$1" in
    # Mac mini M1 16GB, EXP-073 (research repo), the shipped 9-scenario/44-bug suite itself, not a synthetic
    # micro-benchmark: mean tok/s across confirmed suite runs under this exact server config. The kit's own
    # levers table (README "Levers measured") uses the same metric, so this number and that table agree --
    # this was 23.6 before the -ub 256 -b 256 batch-size lever (EXP-073) raised shipped-config decode speed;
    # update this value again if a future lever changes shipped-config speed, so it never goes stale like the
    # very first version of this constant did (18.6, from a since-changed config, never reconciled here).
    "Apple M1") echo 26.6 ;;
    *)          echo ""   ;;
  esac
}

speed_line() {  # prints the measured-or-estimated speed line for $CHIP; requires detect_hw to have run
  local bw m1_bw=68 m1_tps=26.6 measured est
  bw=$(chip_bandwidth "$CHIP")
  measured=$(chip_measured_tps "$CHIP")
  if [ -n "$measured" ]; then
    echo "Speed target: ~${measured} tokens/sec -- MEASURED on this exact chip."
  else
    est=$(awk -v bw="$bw" -v m1bw="$m1_bw" -v m1tps="$m1_tps" 'BEGIN { printf "%.1f", (bw/m1bw)*m1tps }')
    echo "Speed estimate: ~${est} tokens/sec -- ESTIMATED by scaling the M1's measured speed to this chip's"
    echo "published memory bandwidth (${bw} vs M1's ${m1_bw} GB/s spec). NOT independently measured on this chip."
    echo "Capacity (what fits, what OOMs) is expected to behave the same as M1 at 16GB; decode speed is not."
  fi
}

free_disk_gb() {  # free space on the filesystem holding $1 (a directory, must already exist)
  df -g "$1" 2>/dev/null | awk 'NR==2 {print $4}'
}

# Finds the PID of a server that is genuinely ours: matches the port AND has our exact model path on its
# command line. `pgrep -f "llama-server.*port $PORT"` alone (the original check) has no right anchor and would
# also match a server on port 81145 -- confirmed by inspection, fixed here.
server_pid() {
  local pid cmd
  for pid in $(pgrep -f "llama-server.*--port ${PORT}([^0-9]|$)" 2>/dev/null || true); do
    cmd=$(ps -o command= -p "$pid" 2>/dev/null || true)
    if printf '%s' "$cmd" | grep -qF -- "--port $PORT" && printf '%s' "$cmd" | grep -qF -- "$DEST"; then
      echo "$pid"
      return 0
    fi
  done
  return 1
}

# Sets ACTUAL_BYTES and MODEL_STATUS ("ok"/"wrong-size"/"missing") directly -- must be called plainly
# (`model_status`), never via `x=$(model_status)`. A command substitution forks a subshell, and a variable a
# subshelled function sets never propagates back to the caller -- confirmed the hard way: every caller that
# did `status=$(model_status)` and then read $ACTUAL_BYTES afterward crashed under `set -u` the first time a
# model file actually existed on disk (the "missing" case never triggers the crash, which is why this survived
# earlier testing -- nothing had exercised it with a real downloaded/present file).
model_status() {
  if [ ! -f "$DEST" ]; then
    ACTUAL_BYTES=0
    MODEL_STATUS=missing
    return
  fi
  ACTUAL_BYTES=$(stat -f%z "$DEST" 2>/dev/null || stat -c%s "$DEST" 2>/dev/null)
  if [ "$ACTUAL_BYTES" = "$MODEL_BYTES" ]; then MODEL_STATUS=ok; else MODEL_STATUS=wrong-size; fi
}

server_health() {
  curl -s -m 2 "http://127.0.0.1:$PORT/health" 2>/dev/null | grep -q ok
}

# A /health-passing server can still fail every real completion -- confirmed the hard way this session.
# `|| true` on the curl matters: without it, a connection failure under `set -e` kills the whole script with
# no diagnostic, in exactly the situation this check exists to diagnose. Confirmed by inspection: the original
# had no `|| true` here.
smoke_test() {  # prints the raw response body; does not fail the script on a bad response
  curl -s -m "${1:-60}" "http://127.0.0.1:$PORT/v1/chat/completions" -H 'content-type: application/json' \
    -d '{"messages":[{"role":"user","content":"Reply with the single word: ok"}],"max_tokens":8,"temperature":0}' \
    || true
}

check_smoke() {  # prints ok/empty-reasoning/failed; takes the smoke_test() body on stdin
  local body; body=$(cat)
  if [ -z "$body" ] || ! printf '%s' "$body" | grep -qi '"content"'; then
    echo "failed:$body"
    return 1
  fi
  if printf '%s' "$body" | grep -q '"reasoning_content":"[^"]'; then
    echo "empty-reasoning"
    return 0
  fi
  echo "ok"
}

# ============================================================================================================
# Step functions -- each is the one place that step's logic lives; every mode composes these rather than
# duplicating them (this is what keeps --doctor's remediation text pointing at real, working commands).
# ============================================================================================================

step_hw_gate() {
  echo "== Checking hardware =="
  detect_hw
  hw_gate || exit 1
  echo "Detected: $CHIP, ${MEM_GB}GB unified memory."
  speed_line
}

step_prereqs() {
  echo
  echo "== Checking prerequisites =="
  WE_INSTALLED_LLAMA_SERVER=0
  if ! command -v llama-server >/dev/null 2>&1; then
    if ! command -v brew >/dev/null 2>&1; then
      echo "llama.cpp is missing, and so is Homebrew (needed to install it)." >&2
      echo 'Install Homebrew first: /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"' >&2
      exit 1
    fi
    echo "llama.cpp (llama-server) is not installed. Install with: brew install llama.cpp"
    if ask "Install it now? [y/N]"; then brew install llama.cpp; WE_INSTALLED_LLAMA_SERVER=1
    else echo "Skipped -- re-run after installing it." >&2; exit 1; fi
  fi
  if ! command -v node >/dev/null 2>&1; then
    echo "node is required by this installer's own helper scripts (JSON parsing, speed measurement)." >&2
    echo "Install it first: brew install node" >&2
    exit 1
  fi
  if ! command -v go >/dev/null 2>&1; then
    echo "warning: go is not installed -- you won't be able to build go-agent (the CLI this kit points you at)." >&2
    echo "Install it with: brew install go" >&2
  fi
  echo "Prerequisites OK: $(llama-server --version 2>&1 | head -1)"
}

step_download() {
  echo
  echo "== Downloading model =="
  mkdir -p "$MODEL_DIR"
  local free; free=$(free_disk_gb "$MODEL_DIR")
  if [ -n "$free" ] && [ "$free" -lt "$MODEL_MIN_FREE_GB" ]; then
    echo "Only ${free}GB free on the volume holding $MODEL_DIR; need at least ${MODEL_MIN_FREE_GB}GB for the model." >&2
    echo "Free up space first -- a 9.3GB download failing at 99% after 40 minutes is worse than refusing now." >&2
    exit 1
  fi

  model_status; local status="$MODEL_STATUS"
  if [ "$FORCE_DOWNLOAD" = "1" ] && [ -f "$DEST" ]; then
    echo "Forcing re-download."
    rm -f "$DEST"
    status=missing
  fi
  if [ "$status" = ok ]; then
    echo "Already downloaded and verified: $DEST"
    return
  fi
  if [ "$status" = wrong-size ]; then
    echo "Existing file is the wrong size ($ACTUAL_BYTES bytes, expected $MODEL_BYTES) -- re-downloading."
    rm -f "$DEST"
  fi

  local url="https://huggingface.co/${MODEL_REPO}/resolve/main/${MODEL_FILE}"
  local partial_bytes=0
  if [ -f "$DEST.partial" ]; then
    partial_bytes=$(stat -f%z "$DEST.partial" 2>/dev/null || stat -c%s "$DEST.partial" 2>/dev/null || echo 0)
  fi
  if [ "$partial_bytes" = "$MODEL_BYTES" ]; then
    # A previous run finished the download but was killed before the rename -- nothing left to fetch.
    echo "Found a complete download from an earlier run; verifying it instead of re-downloading."
  else
    if [ "$partial_bytes" -gt 0 ]; then
      echo "Resuming an interrupted download ($partial_bytes of $MODEL_BYTES bytes already on disk)..."
    else
      echo "Downloading $MODEL_FILE (~9.3GB, this takes a while)..."
    fi
    # -C - resumes from whatever is already in the .partial file; a 9.3GB download that dies at 80% should
    # cost 20% to finish, not 100%. Hugging Face serves byte ranges, which is what makes this work.
    curl -fL -C - --progress-bar -o "$DEST.partial" "$url"
  fi
  local actual; actual=$(stat -f%z "$DEST.partial" 2>/dev/null || stat -c%s "$DEST.partial" 2>/dev/null)
  if [ "$actual" != "$MODEL_BYTES" ]; then
    echo "Download finished but the file size is wrong: got $actual bytes, expected $MODEL_BYTES." >&2
    echo "Not moving an incomplete/corrupt file into place. Re-run this script." >&2
    rm -f "$DEST.partial"; exit 1
  fi
  mv "$DEST.partial" "$DEST"
  echo "Downloaded and verified: $DEST"
}

step_server() {  # $1: "reuse" (default) or "restart" -- restart is what --upgrade will need later
  local mode="${1:-reuse}"
  echo
  echo "== Starting server =="
  local pid; pid=$(server_pid || true)
  if [ -n "$pid" ] && [ "$mode" = "restart" ]; then
    echo "Restarting the running server (pid $pid) to apply current flags."
    kill "$pid" 2>/dev/null || true
    for _ in $(seq 1 10); do kill -0 "$pid" 2>/dev/null || break; sleep 1; done
    kill -9 "$pid" 2>/dev/null || true
    pid=""
  fi
  if [ -n "$pid" ]; then
    echo "A server is already running on port $PORT (pid $pid). Reusing it."
  else
    if [ ! -f "$DEST" ]; then
      echo "No model at $DEST -- nothing to start. Run setup.sh (without --start-only) to download it first." >&2
      exit 1
    fi
    mkdir -p "$KIT_DIR"
    nohup llama-server -m "$DEST" "${SERVER_FLAGS[@]}" --port "$PORT" \
      > "$KIT_DIR/server.log" 2>&1 &
    disown
    for _ in $(seq 1 60); do
      server_health && break
      sleep 3
    done
  fi
  if ! server_health; then
    echo "Server did not come up. Check $KIT_DIR/server.log" >&2
    exit 1
  fi
  local smoke; smoke=$(smoke_test 60)
  local verdict; verdict=$(printf '%s' "$smoke" | check_smoke) || {
    echo "Server is up but a real request failed. Response: $smoke" >&2
    exit 1
  }
  if [ "$verdict" = "empty-reasoning" ]; then
    echo "Warning: the server is emitting a non-empty reasoning channel. Thinking mode was measured this" >&2
    echo "session to never converge on coding tasks for this model -- --reasoning off should prevent it." >&2
  fi
  echo "Server ready on port $PORT."
}

manifest_get() {  # manifest_get KEY -> value from the existing install.env, or empty
  [ -f "$INSTALL_ENV" ] || return 0
  grep "^$1=" "$INSTALL_ENV" | cut -d= -f2- || true
}

write_manifest() {
  # A re-install or --upgrade over an existing install must NOT overwrite what the FIRST install recorded
  # about the world before this kit touched it -- the prerequisite it installed is still worth remembering.
  local we_llama="${WE_INSTALLED_LLAMA_SERVER:-0}"
  local first_install_at; first_install_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  if [ -f "$INSTALL_ENV" ]; then
    [ "$(manifest_get WE_INSTALLED_LLAMA_SERVER)" = "1" ] && we_llama=1
    [ -n "$(manifest_get INSTALLED_AT)" ] && first_install_at=$(manifest_get INSTALLED_AT)
  fi
  mkdir -p "$KIT_DIR"
  cat > "$INSTALL_ENV" << EOF
KIT_VERSION=$KIT_VERSION
CONFIG_SIG=$(config_sig)
INSTALLED_AT=$first_install_at
UPDATED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
MODEL_PATH=$DEST
MODEL_BYTES=$MODEL_BYTES
PORT=$PORT
MODEL_ID=$MODEL_ID
WE_INSTALLED_LLAMA_SERVER=$we_llama
EOF
  echo "wrote $INSTALL_ENV"
}

# ============================================================================================================
# --doctor -- strictly read-only. Never starts, writes, or downloads anything; every failure line names an
# existing mode (--start-only, --force-download) rather than re-implementing what it would do.
# ============================================================================================================
doctor() {
  local fails=0 warns=0 passes=0
  tag() {  # tag ok|warn|fail "message..."
    local kind="$1"; shift
    case "$kind" in
      ok)   printf '[ ok ] %s\n' "$*"; passes=$((passes+1)) ;;
      warn) printf '[warn] %s\n' "$*"; warns=$((warns+1)) ;;
      fail) printf '[fail] %s\n' "$*"; fails=$((fails+1)) ;;
      skip) printf '[skip] %s\n' "$*" ;;
    esac
  }

  echo "== anvil-agent doctor =="
  if [ -f "$INSTALL_ENV" ]; then
    echo "kit $(manifest_get KIT_VERSION) (config $(manifest_get CONFIG_SIG)), installed $(manifest_get INSTALLED_AT)"
    local installed_sig; installed_sig=$(manifest_get CONFIG_SIG)
    if [ -n "$installed_sig" ] && [ "$installed_sig" != "$(config_sig)" ]; then
      echo "(this script's config is $(config_sig) -- differs from what was installed; see --check / --upgrade)"
    fi
  else
    echo "(no install.env found -- was this installed with an older kit version, or not installed at all?)"
  fi

  echo
  echo "== Hardware =="
  detect_hw
  local hwerr; hwerr=$(mktemp)
  if hw_gate 2>"$hwerr"; then
    tag ok "$CHIP, ${MEM_GB}GB unified memory, $ARCH"
  else
    tag fail "$(cat "$hwerr")"
  fi
  rm -f "$hwerr"
  # Read-only means read-only: probe the nearest directory that already exists rather than creating MODEL_DIR.
  local probe="$MODEL_DIR"
  [ -d "$probe" ] || probe="$HOME"
  local free; free=$(free_disk_gb "$probe")
  if [ -n "$free" ] && [ "$free" -ge "$MODEL_MIN_FREE_GB" ]; then
    tag ok "free disk on $MODEL_DIR: ${free}GB (model needs ~10GB)"
  else
    tag warn "free disk on $MODEL_DIR: ${free:-unknown}GB -- may not be enough for a fresh/re- download"
  fi
  tag ok "$(speed_line | head -1)"

  echo
  echo "== Prerequisites =="
  if command -v llama-server >/dev/null 2>&1; then
    tag ok "llama-server  $(llama-server --version 2>&1 | head -1)"
  else
    tag fail "llama-server not on PATH -- install with: brew install llama.cpp"
  fi
  if command -v node >/dev/null 2>&1; then
    tag ok "node  $(node --version)"
  else
    tag fail "node not on PATH -- required by this installer's own helper scripts"
  fi
  if command -v go >/dev/null 2>&1; then
    tag ok "go  $(go version 2>&1)"
  else
    tag warn "go not on PATH -- needed to build go-agent, the CLI this kit points you at"
  fi

  echo
  echo "== Model =="
  model_status; local status="$MODEL_STATUS"
  case "$status" in
    ok) tag ok "$MODEL_FILE  $ACTUAL_BYTES bytes (expected $MODEL_BYTES)" ;;
    wrong-size) tag fail "$MODEL_FILE is $ACTUAL_BYTES bytes, expected $MODEL_BYTES -- re-download: setup.sh --force-download" ;;
    missing) tag skip "not downloaded -- run setup.sh to install" ;;
  esac

  echo
  echo "== Server =="
  local pid; pid=$(server_pid || true)
  if [ -n "$pid" ]; then
    tag ok "running on port $PORT (pid $pid)"
    if server_health; then
      tag ok "/health -> ok"
      local smoke t0 t1; t0=$(date +%s)
      smoke=$(smoke_test 120)
      t1=$(date +%s)
      local verdict; verdict=$(printf '%s' "$smoke" | check_smoke) || verdict="failed"
      case "$verdict" in
        ok) tag ok "real completion succeeded in $((t1-t0))s" ;;
        empty-reasoning) tag fail "reasoning channel is non-empty -- thinking mode never converges on this model" ;;
        *) tag fail "a real completion request failed: ${smoke:0:200}" ;;
      esac
      local cmd; cmd=$(ps -o command= -p "$pid" 2>/dev/null || true)
      local missing=""
      for chk in "${CHECK_STRINGS[@]}"; do
        printf '%s' "$cmd" | grep -qF -- "$chk" || missing="$missing $chk"
      done
      if [ -z "$missing" ]; then
        tag ok "server flags match this kit's validated set"
      else
        tag warn "server flags differ from this kit's validated set: missing:$missing"
        echo "       -> restart it: setup.sh --start-only"
      fi
      local props n_ctx
      props=$(curl -s -m 5 "http://127.0.0.1:$PORT/props" 2>/dev/null || true)
      n_ctx=$(printf '%s' "$props" | EXPR='j.default_generation_settings && j.default_generation_settings.n_ctx' json_field 2>/dev/null || true)
      if [ -n "$n_ctx" ]; then
        if [ "$n_ctx" = "$CTX" ]; then tag ok "/props n_ctx = $n_ctx (expected $CTX)"
        else tag warn "/props n_ctx = $n_ctx, expected $CTX -- the server may have clamped it"; fi
      fi
    else
      tag fail "process is running but /health doesn't respond"
    fi
  else
    tag skip "no server running on port $PORT -- run setup.sh to start one, or setup.sh --start-only"
  fi

  echo
  echo "== $fails failed, $warns warning(s), $passes passed =="
  if [ "$fails" -gt 0 ]; then return 1; elif [ "$warns" -gt 0 ]; then return 2; else return 0; fi
}

# ============================================================================================================
# --selftest -- prints detect_hw + hw_gate + speed_line output and the gate's own exit code, then returns
# WITHOUT installing anything. This is the only mode CI's hardware-detection tests exercise; it structurally
# cannot download or boot a server.
# ============================================================================================================
selftest() {
  detect_hw
  echo "chip=$CHIP mem_gb=$MEM_GB arch=$ARCH"
  if hw_gate; then
    speed_line
    exit 0
  else
    exit 1
  fi
}

# ============================================================================================================
# --check -- fetches VERSION as a staleness beacon ONLY (see VERSION_URL's comment: never sourced/executed,
# never supplies a value this script acts on). Reports three independent signals with a single exit code:
# 0 everything current, 2 something is stale (never 1 -- an update check must never look like a hard failure).
# Unreachable network is treated as success, not failure: a version check must never fail loud on a plane.
# ============================================================================================================
check() {
  echo "== anvil-agent version check =="
  echo "local script: $KIT_VERSION (config $(config_sig))"
  local remote; remote=$(curl -fs -m 5 "$VERSION_URL" 2>/dev/null || true)
  local r_ver r_sig r_model r_bytes stale=0
  r_ver=$(printf '%s\n' "$remote" | grep '^KIT_VERSION=' | cut -d= -f2- || true)
  if [ -z "$remote" ] || [ -z "$r_ver" ]; then
    echo "Could not reach or parse $VERSION_URL -- treating as up to date, not a failure."
    exit 0
  fi
  r_sig=$(printf '%s\n' "$remote" | grep '^CONFIG_SIG=' | cut -d= -f2- || true)
  r_model=$(printf '%s\n' "$remote" | grep '^MODEL_FILE=' | cut -d= -f2- || true)
  r_bytes=$(printf '%s\n' "$remote" | grep '^MODEL_BYTES=' | cut -d= -f2- || true)

  if [ -n "$r_sig" ] && [ "$r_sig" != "$(config_sig)" ]; then
    echo "[stale] this setup.sh's config differs from the latest published one ($r_ver) -- re-download it."
    stale=1
  else
    echo "[ ok ] setup.sh matches the latest published config."
  fi
  if [ -n "$r_model" ] && { [ "$r_model" != "$MODEL_FILE" ] || [ "$r_bytes" != "$MODEL_BYTES" ]; }; then
    echo "[stale] a different model is now recommended: $r_model ($r_bytes bytes)."
    stale=1
  else
    echo "[ ok ] recommended model unchanged."
  fi
  if [ -f "$INSTALL_ENV" ]; then
    local installed_sig; installed_sig=$(manifest_get CONFIG_SIG)
    if [ "$installed_sig" != "$(config_sig)" ]; then
      echo "[stale] your installed config doesn't match this script's current config -- run: setup.sh --upgrade"
      stale=1
    else
      echo "[ ok ] your installed config matches this script."
    fi
  fi
  [ "$stale" = 1 ] && exit 2 || exit 0
}

# ============================================================================================================
# --upgrade -- a flagged variant of install: restarts the server (a flag change wouldn't otherwise take
# effect) and offers to prune an old model file left behind if MODEL_FILE changed since the last install.
# Requires a prior install (install.env) -- upgrading nothing isn't a meaningful operation.
# ============================================================================================================
upgrade() {
  if [ ! -f "$INSTALL_ENV" ]; then
    echo "No existing install found ($INSTALL_ENV missing). Run setup.sh normally first." >&2
    exit 1
  fi
  local old_model; old_model=$(manifest_get MODEL_PATH)
  step_hw_gate
  step_prereqs
  step_download
  step_server restart
  write_manifest
  if [ -n "$old_model" ] && [ "$old_model" != "$DEST" ] && [ -f "$old_model" ]; then
    echo
    if ask "Old model no longer used: $old_model -- delete it? [y/N]"; then
      rm -f "$old_model"
      echo "Deleted $old_model"
    else
      echo "Left in place: $old_model"
    fi
  fi
  echo
  echo "== Upgrade complete. =="
}

# ============================================================================================================
# --benchmark -- grades an already-running, already-validated server against the 9-scenario suite in
# benchmarks/. Never boots a server itself (same division of responsibility as --doctor: measures what's
# there, doesn't set it up). Needs a real git checkout, not the curl-pipe install, because the scenario data
# and oracle test trees are too large to embed in this file -- see run_benchmark's own precondition check.
# All prompt-construction, HTTP-calling and grading logic lives in benchmarks/bench.mjs and benchmarks/
# grade.mjs, reusing the already-verified grader rather than duplicating it here in bash.
# ============================================================================================================
run_benchmark() {
  if [ ! -f "$BENCH_DIR/grade.mjs" ] || [ ! -d "$BENCH_DIR/scenarios" ] || [ ! -d "$BENCH_DIR/oracle" ]; then
    echo "--benchmark needs the full repo checkout, not the curl-pipe install." >&2
    echo "Run: git clone https://github.com/megasoft1978/anvil-agent && cd anvil-agent && ./setup.sh --benchmark" >&2
    exit 1
  fi
  local pid; pid=$(server_pid || true)
  if [ -z "$pid" ] || ! server_health; then
    echo "No validated server running on port $PORT. Start one first: setup.sh --start-only" >&2
    exit 1
  fi

  echo "== anvil-agent benchmark: $BENCH_TARGET =="
  node "$BENCH_DIR/bench.mjs" "$BENCH_TARGET" --port "$PORT" --max-tokens "$MAX_TOKENS" | node -e '
    let fails = 0, warns = 0, totalPass = 0, totalBugs = 0, totalWall = 0, n = 0, totalTok = 0, genSecs = 0, accepted = [];
    const tag = (kind, msg) => {
      const label = { ok: "[ ok ] ", warn: "[warn] ", fail: "[fail] ", skip: "[skip] " }[kind];
      console.log(label + msg);
      if (kind === "fail") fails++;
      if (kind === "warn") warns++;
    };
    process.stdin.setEncoding("utf8");
    let buf = "";
    process.stdin.on("data", (d) => {
      buf += d;
      let i;
      while ((i = buf.indexOf("\n")) >= 0) {
        const line = buf.slice(0, i); buf = buf.slice(i + 1);
        if (!line.trim()) continue;
        const r = JSON.parse(line);
        n++;
        if (r.verdict === "skip") { tag("skip", `${r.scenario}: ${r.detail}`); continue; }
        if (r.verdict === "timeout") { tag("fail", `${r.scenario}: timed out waiting for a response`); continue; }
        if (r.verdict === "empty-reasoning") { tag("warn", `${r.scenario}: reasoning channel non-empty, content empty (in ${r.wall_s}s)`); continue; }
        if (r.verdict === "error") { tag("fail", `${r.scenario}: ${r.detail}`); continue; }
        totalPass += r.pass; totalBugs += r.total; totalWall += r.wall_s;
        const pct = Math.round((r.pass / r.total) * 100);
        let speed = "";
        if (r.tps) {
          speed = `, ${r.tps.toFixed(1)} tok/s`;
          if (r.completion_tokens) { totalTok += r.completion_tokens; genSecs += r.completion_tokens / r.tps; }
          if (r.draft_accept != null) { speed += `, ${Math.round(r.draft_accept * 100)}% draft accepted`; accepted.push(r.draft_accept); }
        }
        const line2 = `${r.scenario}: ${r.pass}/${r.total} (${pct}%) in ${r.wall_s.toFixed(1)}s${speed}`;
        if (r.pass === r.total) tag("ok", line2);
        else if (r.pass > 0) tag("warn", line2);
        else tag("fail", line2);
      }
    });
    process.stdin.on("end", () => {
      if (totalBugs > 0) {
        const pct = Math.round((totalPass / totalBugs) * 100);
        let speed = "";
        if (genSecs > 0) speed = `, ${(totalTok / genSecs).toFixed(1)} tok/s generation over ${totalTok} tokens`;
        if (accepted.length) speed += `, ${Math.round(accepted.reduce((a, b) => a + b, 0) / accepted.length * 100)}% mean draft acceptance`;
        console.log(`\n== benchmark: ${totalPass}/${totalBugs} (${pct}%) across ${n} scenario(s), ${totalWall.toFixed(1)}s wall${speed} ==`);
        console.log("(vs. the published baseline table in README.md -- exact match is not expected; temperature-0 determinism only guarantees a given server build reproduces itself)");
      }
      process.exit(fails > 0 ? 1 : warns > 0 ? 2 : 0);
    });
  '
}

# ============================================================================================================
# --report-speed -- measures real tokens/sec on chips this kit currently only estimates for, and prints a
# pre-filled GitHub issue link (never opened automatically -- the report is shown in full in the terminal
# first; this kit never phones home on its own). Accepting a report is then a one-line diff: fill in
# chip_measured_tps() and one README row, no other code path changes.
# ============================================================================================================
# Prints tokens/sec for one REPORT_PROMPT completion, or "0" if it couldn't measure. The request and the
# timing both live in one node process: llama-server's own `timings.predicted_per_second` (pure generation
# rate, prompt processing excluded) is preferred, with a millisecond wall-clock fallback. Whole-second `date`
# timing was rejected -- on the fast chips this exists to measure, 256 tokens finish in ~2s, where rounding to
# a whole second is a 30-50% error.
timed_completion() {
  REPORT_PROMPT="$REPORT_PROMPT" PORT="$PORT" node -e '
    const t0 = performance.now();
    fetch("http://127.0.0.1:" + process.env.PORT + "/v1/chat/completions", {
      method: "POST", headers: { "content-type": "application/json" },
      body: JSON.stringify({ messages: [{ role: "user", content: process.env.REPORT_PROMPT }], max_tokens: 256, temperature: 0 }),
      signal: AbortSignal.timeout(120_000),
    }).then(async (r) => {
      const j = await r.json();
      const wall = (performance.now() - t0) / 1000;
      const fromServer = j && j.timings && Number(j.timings.predicted_per_second);
      const tok = j && j.usage && Number(j.usage.completion_tokens);
      const tps = fromServer > 0 ? fromServer : (tok > 0 && wall > 0 ? tok / wall : 0);
      process.stdout.write(tps > 0 ? tps.toFixed(1) : "0");
    }).catch(() => process.stdout.write("0"));
  '
}

report_speed() {
  echo "== anvil-agent chip speed report =="
  local pid; pid=$(server_pid || true)
  if [ -z "$pid" ] || ! server_health; then
    echo "No validated server running on port $PORT. Start one first: setup.sh --start-only" >&2
    exit 1
  fi
  detect_hw
  if [ -n "$(chip_measured_tps "$CHIP")" ]; then
    echo "$CHIP already has a measured number in this kit ($(chip_measured_tps "$CHIP") tokens/sec) -- no report needed."
    exit 0
  fi
  echo "Chip: $CHIP (currently only an ESTIMATE in this kit)"
  echo "Running two timed completions -- the first pays a cold mmap-page-in cost, the second is the real number."
  local tps1 tps2
  tps1=$(timed_completion)
  tps2=$(timed_completion)
  echo
  echo "First run:  ${tps1} tokens/sec"
  echo "Second run: ${tps2} tokens/sec  <- this is the number to report"
  if [ "$tps2" = "0" ]; then
    echo
    echo "Could not get a usable measurement (empty or malformed completion). Try again, or report by hand." >&2
    exit 1
  fi
  echo
  echo "To contribute this measurement, open an issue with these fields pre-filled (nothing is sent automatically):"
  REPORT_CHIP="$CHIP" REPORT_MAC="$(sw_vers -productVersion 2>/dev/null || true)" REPORT_TPS="$tps2" node -e '
    const params = new URLSearchParams({
      template: "chip-report.yml",
      chip: process.env.REPORT_CHIP || "",
      macos_version: process.env.REPORT_MAC || "",
      measured_tps: process.env.REPORT_TPS || "",
    });
    console.log("https://github.com/megasoft1978/anvil-agent/issues/new?" + params.toString());
  '
}

# ============================================================================================================
# Dispatch
# ============================================================================================================
case "$MODE" in
  print-sig)
    config_sig
    exit 0
    ;;
  print-node-snippets)
    # Extracts each `node -e '...'` program in this file between the opening quote and its matching closing
    # quote-plus-paren, so CI can run `node --check` on each without a shell interpreter seeing inside the
    # string. Relies on this file's own convention: every snippet is `node -e '` ... `'` on its own lines.
    # The closing-quote line is indented to match its snippet (e.g. "  '", or "  '"'"')" when the whole node -e
    # call is wrapped in a command substitution), not always exactly one column -- an exact-one-char version
    # silently never matched any closing line in this file, confirmed directly, so every extracted snippet used
    # to end with a stray quote (or quote-paren) line.
    # The quote character is passed in via -v rather than written as \x27, which not every awk (mawk on
    # Ubuntu, where CI's js-syntax job runs) understands inside a regex.
    # A closing line may carry shell after the quote -- `') || return 3` -- which is bash, not JS.
    case "$0" in
      *setup.sh) ;;
      *) echo "--print-node-snippets needs to read its own source; run it from a checkout, not a pipe." >&2; exit 64 ;;
    esac
    awk -v q="'" '
      /node -e .$/ { n++; print "--- snippet " n " ---"; capture=1; next }
      capture && $0 ~ ("^[ \t]*" q "\\)?( .*)?$") { capture=0; next }
      capture { print }
    ' "$0"
    exit 0
    ;;
  selftest)
    selftest
    ;;
  doctor)
    doctor
    exit $?
    ;;
  benchmark)
    run_benchmark
    exit $?
    ;;
  check)
    check
    ;;
  report-speed)
    report_speed
    ;;
  upgrade)
    upgrade
    ;;
  start-only)
    step_server restart
    ;;
  install)
    step_hw_gate
    step_prereqs
    step_download
    step_server reuse
    write_manifest
    echo
    echo "== Ready. Server running on port $PORT. =="
    echo "Point go-agent at it (needs a full checkout -- this installer is a single file, go-agent is not):"
    echo "  git clone https://github.com/megasoft1978/anvil-agent && cd anvil-agent/go-agent"
    echo "  go build -o anvil-agent ."
    echo "  ./anvil-agent --root <your-project> --endpoint http://127.0.0.1:$PORT/v1 --model $MODEL_ID \\"
    echo "      --prompt \"describe the bug\" --test-command '[\"npm\",\"test\"]'"
    echo "See go-agent/README.md for the full flag list."
    ;;
esac
