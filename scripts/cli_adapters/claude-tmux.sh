#!/usr/bin/env bash
#
# claude-tmux.sh — PROTOTYPE adapter driving interactive `claude` via tmux.
#
# Purpose: a one-shot test of whether interactive `claude` driven from tmux
# bills to subscription quota instead of the post-2026-06-15 Agent SDK
# credit pool. NOT production-ready. NOT integrated into the pipeline.
#
# Inputs (env, contract matches scripts/cli_adapters/claude.sh:24-30):
#   PROFILE_PATH        Absolute path to agent profile JSON
#   RESOLVED_MODEL      Model tier: haiku / sonnet / opus
#   PROMPT_FILE         Path to injected prompt
#   CYCLE               Integer cycle number (or 0 for probes)
#   WORKSPACE_PATH      Absolute path to workspace dir
#   STDOUT_LOG          Where to write ANSI-stripped scrollback
#   STDERR_LOG          Where to write raw scrollback (forensic)
#   ARTIFACT_PATH       Where the agent must write its final report
#   AGENT (opt)         Agent role; defaults to "probe"
#   WORKTREE_PATH (opt) Absolute path to git worktree; defaults to PWD
#
# Hard-coded safety gates:
#   EVOLVE_TMUX_PROTOTYPE_ALLOW_BYPASS=1 must be set or adapter refuses
#   ANTHROPIC_API_KEY must be unset (would invalidate the billing test)
#   ANTHROPIC_BASE_URL / EVOLVE_ANTHROPIC_BASE_URL must be unset (proxy mode)
#
# Exit codes:
#   0    adapter ran, artifact appeared
#   2    prototype safety gate not set
#   3    environment looks like API or proxy mode (would invalidate test)
#   80   REPL boot timeout (60s)
#   81   artifact never appeared within 5 min
#   127  required binary not found
#

set -uo pipefail

# --- Mandatory env -----------------------------------------------------------
: "${PROFILE_PATH:?claude-tmux.sh: PROFILE_PATH unset}"
: "${RESOLVED_MODEL:?claude-tmux.sh: RESOLVED_MODEL unset}"
: "${PROMPT_FILE:?claude-tmux.sh: PROMPT_FILE unset}"
: "${CYCLE:?claude-tmux.sh: CYCLE unset}"
: "${WORKSPACE_PATH:?claude-tmux.sh: WORKSPACE_PATH unset}"
: "${STDOUT_LOG:?claude-tmux.sh: STDOUT_LOG unset}"
: "${STDERR_LOG:?claude-tmux.sh: STDERR_LOG unset}"
: "${ARTIFACT_PATH:?claude-tmux.sh: ARTIFACT_PATH unset}"

# --- Preflight binary checks -------------------------------------------------
command -v tmux   >/dev/null 2>&1 || { echo "[claude-tmux] tmux missing"   >&2; exit 127; }
command -v claude >/dev/null 2>&1 || { echo "[claude-tmux] claude missing" >&2; exit 127; }
command -v jq     >/dev/null 2>&1 || { echo "[claude-tmux] jq missing"     >&2; exit 127; }

# --- Prototype safety gate ---------------------------------------------------
if [ "${EVOLVE_TMUX_PROTOTYPE_ALLOW_BYPASS:-0}" != "1" ]; then
    echo "[claude-tmux] requires EVOLVE_TMUX_PROTOTYPE_ALLOW_BYPASS=1 (prototype only)" >&2
    exit 2
fi

# --- Cost-leak guards (refuse if env would route to API or a proxy) ----------
if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
    echo "[claude-tmux] ANTHROPIC_API_KEY is set — would bill to API, not subscription; abort" >&2
    exit 3
fi
if [ -n "${ANTHROPIC_BASE_URL:-}" ]; then
    echo "[claude-tmux] ANTHROPIC_BASE_URL is set — proxy mode would invalidate the billing test; abort" >&2
    exit 3
fi
if [ -n "${EVOLVE_ANTHROPIC_BASE_URL:-}" ]; then
    echo "[claude-tmux] EVOLVE_ANTHROPIC_BASE_URL is set — proxy mode would invalidate the billing test; abort" >&2
    exit 3
fi

# --- Resolve working dir -----------------------------------------------------
WORKING_DIR="${WORKTREE_PATH:-$PWD}"
if [ ! -d "$WORKING_DIR" ]; then
    echo "[claude-tmux] working dir does not exist: $WORKING_DIR" >&2
    exit 1
fi

# --- Build session name ------------------------------------------------------
AGENT_LABEL="${AGENT:-probe}"
SESSION="evolve-claude-tmux-c${CYCLE}-${AGENT_LABEL}-pid$$-$(date +%s)"
SESSION="${SESSION:0:64}"

echo "[claude-tmux] session=$SESSION model=$RESOLVED_MODEL workdir=$WORKING_DIR" >&2

# --- Cleanup trap ------------------------------------------------------------
SCROLLBACK_FILE="$WORKSPACE_PATH/tmux-final-scrollback.txt"
mkdir -p "$WORKSPACE_PATH"

cleanup() {
    local rc=$?
    if tmux has-session -t "$SESSION" 2>/dev/null; then
        # Capture final scrollback before kill (forensic record)
        tmux capture-pane -p -S -10000 -t "$SESSION" > "$SCROLLBACK_FILE" 2>/dev/null || true
        tmux kill-session -t "$SESSION" 2>/dev/null || true
        echo "[claude-tmux] session killed: $SESSION (rc=$rc)" >&2
    fi
    exit "$rc"
}
trap cleanup EXIT INT TERM

# --- Spawn tmux session ------------------------------------------------------
tmux new-session -d -s "$SESSION" -x 220 -y 80
sleep 1
echo "[claude-tmux] tmux session started" >&2

# Change to working dir inside the session
tmux send-keys -t "$SESSION" "cd $WORKING_DIR" Enter
sleep 1

# --- Launch claude interactively (NO -p) -------------------------------------
# --dangerously-skip-permissions grants Write/Bash without prompting; without
# this the REPL would block on permission confirmations we can't reliably
# auto-confirm. Production would replace with --tools "Read,Write,..." whitelist.
CLAUDE_CMD="claude --model $RESOLVED_MODEL --dangerously-skip-permissions"
tmux send-keys -t "$SESSION" "$CLAUDE_CMD" Enter
echo "[claude-tmux] launching: $CLAUDE_CMD" >&2

# --- Wait for REPL prompt ----------------------------------------------------
# We poll for the REPL prompt character (❯) anywhere in the visible pane.
# Note: the Ink-based UI renders the prompt mid-pane (with horizontal-rule
# separators above/below and empty trailing rows), so a tail-N restriction
# would miss it. We search the full capture for the ❯ glyph.
REPL_BOOT_TIMEOUT=60
elapsed=0
prompt_seen=0
while [ $elapsed -lt $REPL_BOOT_TIMEOUT ]; do
    sleep 1
    elapsed=$((elapsed + 1))
    pane=$(tmux capture-pane -p -t "$SESSION" 2>/dev/null || echo "")
    if echo "$pane" | grep -qE '❯'; then
        prompt_seen=1
        echo "[claude-tmux] REPL prompt (❯) detected after ${elapsed}s" >&2
        break
    fi
done

if [ $prompt_seen -eq 0 ]; then
    echo "[claude-tmux] FAIL: REPL prompt never appeared after ${REPL_BOOT_TIMEOUT}s" >&2
    exit 80
fi

# --- Deliver prompt via tmux buffer ------------------------------------------
# load-buffer reads a file into tmux's internal paste buffer, paste-buffer
# injects it as if typed. This handles multi-line / special-char prompts
# safely without shell-quoting hell.
tmux load-buffer -t "$SESSION" "$PROMPT_FILE"
tmux paste-buffer -t "$SESSION"
sleep 1
tmux send-keys -t "$SESSION" Enter
PROMPT_BYTES=$(wc -c < "$PROMPT_FILE" | tr -d ' ')
echo "[claude-tmux] prompt delivered (${PROMPT_BYTES} bytes)" >&2

# --- Wait for artifact to appear ---------------------------------------------
# This is the BILLING SIGNAL: the agent inside the REPL must write its
# artifact to disk using its own Write tool. If the artifact appears with
# the right token, the round-trip succeeded.
ARTIFACT_WAIT_TIMEOUT=300
elapsed=0
artifact_seen=0
while [ $elapsed -lt $ARTIFACT_WAIT_TIMEOUT ]; do
    sleep 2
    elapsed=$((elapsed + 2))
    if [ -s "$ARTIFACT_PATH" ]; then
        artifact_seen=1
        echo "[claude-tmux] artifact appeared after ${elapsed}s: $ARTIFACT_PATH" >&2
        break
    fi
done

if [ $artifact_seen -eq 0 ]; then
    echo "[claude-tmux] FAIL: artifact never appeared at $ARTIFACT_PATH after ${ARTIFACT_WAIT_TIMEOUT}s" >&2
    exit 81
fi

# --- Capture scrollback for forensic record ----------------------------------
# stdout = ANSI-stripped, stderr = raw. Downstream consumers can grep either.
RAW=$(tmux capture-pane -p -S -10000 -t "$SESSION" 2>/dev/null || echo "")
echo "$RAW" > "$STDERR_LOG"
echo "$RAW" | sed -E 's/\x1b\[[0-9;]*[a-zA-Z]//g; s/\x1b\][^\x07]*\x07//g' > "$STDOUT_LOG"
echo "[claude-tmux] scrollback captured: stdout=$STDOUT_LOG stderr=$STDERR_LOG" >&2

# --- Graceful REPL exit ------------------------------------------------------
# /exit cleanly terminates the REPL; trap fallback kills the session anyway.
tmux send-keys -t "$SESSION" "/exit" Enter
sleep 2

echo "[claude-tmux] DONE: artifact-only verdict = SUCCESS" >&2
exit 0
