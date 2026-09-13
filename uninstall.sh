#!/usr/bin/env bash
# Reverses anvil-agent's setup.sh, driven by the install manifest setup.sh writes at
# $HOME/.anvil-agent/install.env. Self-contained on purpose (same reason as setup.sh: no local checkout
# to source a shared file from over a pipe) -- it duplicates exactly one thing from setup.sh, the `ask()`
# helper, and nothing else. Everything it needs to know about what was installed, it reads from the manifest.
#
#   curl -fsSL https://raw.githubusercontent.com/megasoft1978/anvil-agent/main/uninstall.sh | bash
#
# Flags (remember `bash -s --` when piping one through curl):
#   --dry-run      print the plan and exit, touch nothing
#   --yes          skip confirmation prompts (the model file is still kept by default -- see --purge-model)
#   --purge-model  also delete the downloaded model file (the only irreversible, expensive-to-redo step here)
#   --keep-model   explicitly keep it without being asked (useful with --yes)
#   --all          also reverse a llama.cpp install, but ONLY if this kit's own install actually did it
set -euo pipefail

DRY_RUN=0
ASSUME_YES=0
PURGE_MODEL=0
KEEP_MODEL=0
DO_ALL=0
while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --yes) ASSUME_YES=1 ;;
    --purge-model) PURGE_MODEL=1 ;;
    --keep-model) KEEP_MODEL=1 ;;
    --all) DO_ALL=1 ;;
    --help|-h)
      # A heredoc, not a grep of this file: when piped through `curl | bash`, $0 is /bin/bash, not this script.
      cat << 'EOF'
anvil-agent uninstall -- reverses setup.sh using the manifest it wrote at ~/.anvil-agent/install.env

  curl -fsSL https://raw.githubusercontent.com/megasoft1978/anvil-agent/main/uninstall.sh | bash

Flags (pass them through a pipe with `bash -s --`):
  --dry-run      print the plan and exit, touch nothing
  --yes          skip confirmation prompts (the model file is still kept -- see --purge-model)
  --purge-model  also delete the downloaded model file (the only irreversible, expensive-to-redo step)
  --keep-model   explicitly keep it without being asked (useful with --yes)
  --all          also offer to reverse a llama.cpp install, but ONLY if this kit's own install did it
EOF
      exit 0
      ;;
    *) echo "Unknown argument: $1 (see --help)" >&2; exit 64 ;;
  esac
  shift
done

ask() {  # duplicated from setup.sh deliberately -- see header comment for why
  if [ "$DRY_RUN" = "1" ]; then echo "  (dry-run) would ask: $1"; return 0; fi  # so run() can show the full plan
  if [ "$ASSUME_YES" = "1" ]; then return 0; fi
  local prompt="$1" reply="n"
  # /dev/tty can exist and pass `-r` yet still fail at actual read time ("Device not configured") in some
  # sandboxed/detached-process environments with no controlling terminal at all -- confirmed by testing, not
  # just a theoretical case. `read`'s own exit status decides the fallback, so a failed read can never leave
  # `reply` unset under `set -u`.
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

run() {  # run <description> -- <command...>  ; honors --dry-run uniformly
  local desc="$1"; shift
  if [ "$DRY_RUN" = "1" ]; then
    echo "  (dry-run) would: $desc"
    return 0
  fi
  "$@"
}

KIT_DIR="$HOME/.anvil-agent"
INSTALL_ENV="$KIT_DIR/install.env"

# Fallback defaults for when install.env is missing (older kit, or install never completed) -- deliberately
# the most conservative values: never delete the model without an explicit flag, never assume we installed a
# prerequisite.
PORT=8114
MODEL_PATH=""
WE_INSTALLED_LLAMA_SERVER=0

MANIFEST_FOUND=0
if [ -f "$INSTALL_ENV" ]; then
  MANIFEST_FOUND=1
  # Read with grep/cut, never `source` -- this file is only ever written by setup.sh itself, but the same
  # habit that protects against a remote VERSION file (see the version-check design) costs nothing here either.
  PORT=$(grep '^PORT=' "$INSTALL_ENV" | cut -d= -f2- || echo "$PORT")
  MODEL_PATH=$(grep '^MODEL_PATH=' "$INSTALL_ENV" | cut -d= -f2- || echo "")
  WE_INSTALLED_LLAMA_SERVER=$(grep '^WE_INSTALLED_LLAMA_SERVER=' "$INSTALL_ENV" | cut -d= -f2- || echo 0)
else
  echo "no install manifest found at $INSTALL_ENV -- falling back to this uninstaller's defaults." >&2
  echo "anything an older/newer setup.sh did differently will be left in place." >&2
fi

echo "== anvil-agent uninstall $([ "$DRY_RUN" = "1" ] && echo "(dry run)") =="
echo

# ---------- 1. stop the server, but only if it's genuinely ours --------------------------------------------
echo "== Server =="
MINE=""
for pid in $(pgrep -f "llama-server.*--port ${PORT}([^0-9]|$)" 2>/dev/null || true); do
  cmd=$(ps -o command= -p "$pid" 2>/dev/null || true)
  if [ -n "$MODEL_PATH" ] && printf '%s' "$cmd" | grep -qF -- "$MODEL_PATH"; then
    MINE="$MINE $pid"
  elif [ -z "$MODEL_PATH" ] && printf '%s' "$cmd" | grep -qF -- "--port $PORT"; then
    # No manifest to confirm the model path -- still port-matched, ask before touching it.
    MINE="$MINE $pid"
  fi
done
if [ -z "$MINE" ]; then
  echo "no server on port $PORT matching this kit's model -- nothing to stop"
else
  for pid in $MINE; do
    echo "found our server: pid $pid, model $MODEL_PATH"
    if ask "Stop it? [y/N]"; then
      run "kill pid $pid" bash -c "kill '$pid' 2>/dev/null || true"
      if [ "$DRY_RUN" != "1" ]; then
        for _ in $(seq 1 10); do kill -0 "$pid" 2>/dev/null || break; sleep 1; done
        kill -9 "$pid" 2>/dev/null || true
      fi
    else
      echo "left running -- port $PORT and $MODEL_PATH stay in use"
    fi
  done
fi
# Also report (never touch) a same-port process that is NOT ours.
for pid in $(pgrep -f "llama-server.*--port ${PORT}([^0-9]|$)" 2>/dev/null || true); do
  case " $MINE " in *" $pid "*) continue ;; esac
  echo "note: pid $pid is also on port $PORT but doesn't match our model path -- left alone"
done

# ---------- 2. bookkeeping files, always safe to remove ------------------------------------------------------
echo
echo "== Kit bookkeeping =="
if [ "$MANIFEST_FOUND" = "1" ] || [ -d "$KIT_DIR" ]; then
  # An unmatched glob is passed through literally and rm -f ignores it, so no shell -c re-quoting is needed.
  run "remove $KIT_DIR/server.log, install.env, *.partial" \
    rm -f "$KIT_DIR/server.log" "$INSTALL_ENV" "$KIT_DIR/models/"*.partial
  [ "$DRY_RUN" = "1" ] || echo "removed"
else
  echo "nothing to do"
fi

# ---------- 3. the model file -- report, ask once, default no ------------------------------------------------
echo
echo "== Model file =="
if [ -z "$MODEL_PATH" ] || [ ! -f "$MODEL_PATH" ]; then
  echo "no model file on record (or already gone) -- nothing to do"
else
  size_bytes=$(stat -f%z "$MODEL_PATH" 2>/dev/null || stat -c%s "$MODEL_PATH" 2>/dev/null || echo 0)
  size_gb=$(awk -v b="$size_bytes" 'BEGIN { printf "%.1f", b/1073741824 }')
  echo "Model file:  $MODEL_PATH"
  echo "Size:        ${size_gb}GB ($size_bytes bytes)"
  echo "Re-download: the same amount again from Hugging Face if you reinstall"
  if [ "$KEEP_MODEL" = "1" ]; then
    echo "kept (--keep-model)"
  elif [ "$PURGE_MODEL" = "1" ]; then
    run "delete $MODEL_PATH" rm -f "$MODEL_PATH"
    [ "$DRY_RUN" = "1" ] || echo "deleted"
  elif [ "$DRY_RUN" = "1" ]; then
    echo "  (dry-run) would ask whether to delete it -- default no; --purge-model deletes without asking"
  elif [ "$ASSUME_YES" = "1" ]; then
    # ask() returns yes unconditionally under --yes, which would make plain --yes delete the model file --
    # exactly the outcome the header comment promises will NOT happen. This is the one step --yes must never
    # answer on its own; only an explicit --purge-model may delete it, confirmed as a real bug by testing.
    echo "kept (--yes alone never deletes this -- pass --purge-model too if you want it gone)"
  elif ask "Delete it? [y/N]"; then
    run "delete $MODEL_PATH" rm -f "$MODEL_PATH"
    echo "deleted"
  else
    echo "kept -- delete it yourself later with: rm '$MODEL_PATH'"
  fi
fi
if [ "$DRY_RUN" != "1" ]; then
  rmdir "$HOME/.anvil-agent/models" 2>/dev/null || true
  rmdir "$HOME/.anvil-agent" 2>/dev/null || true
fi

# ---------- 4. prerequisites -- only what we installed, only under --all -------------------------------------
echo
echo "== Prerequisites (llama.cpp) =="
if [ "$WE_INSTALLED_LLAMA_SERVER" = "1" ]; then
  # Plain if/then here, not `[ ... ] && { ask && run; }`: under `set -e`, a braces group that ends with a
  # failed `ask` (the user answering "no") is the final command of the && list, so the whole script would
  # exit right there -- before "Done".
  echo "this kit installed llama.cpp -- reverse with: brew uninstall llama.cpp"
  if [ "$DO_ALL" = "1" ]; then
    if ask "Uninstall llama.cpp now? [y/N]"; then run "brew uninstall llama.cpp" brew uninstall llama.cpp; else echo "kept llama.cpp"; fi
  else
    echo "(pass --all to be offered removal, since this is a general-purpose tool)"
  fi
else
  echo "not installed by this kit (or that isn't recorded) -- nothing offered"
fi

echo
echo "== Done. =="
