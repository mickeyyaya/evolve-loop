#!/usr/bin/env bash
# drivers/claude-tmux.sh — driver for interactive `claude` driven via tmux
#
# Vendored from evolve-loop's scripts/cli_adapters/claude-tmux.sh (PROTOTYPE,
# GO-verified 2026-05-21 — see docs/research/tmux-claude-driver-prototype.md
# in the parent repo). Trimmed of EVOLVE_TMUX_PROTOTYPE_ALLOW_BYPASS gate
# (bridge uses its own --allow-bypass flag); cost-leak guards retained;
# REPL-detection fix retained.
#
# Contract: sourced by bin/bridge; reads cmd_launch's local vars +
# bridge_profile_* + bridge_manifest_*. Honors $allow_bypass (--allow-bypass)
# as the bridge-level safety gate.
#
# Auto-respond fallback (P6.5) will hook into the wait-artifact loop:
# the manifest's interactive_prompts[] declares regex→response patterns the
# loop can fire when an unexpected prompt appears mid-run.

drv_launch_claude_tmux() {
  # --- Bridge safety gate: --allow-bypass must be set ------------------------
  if [[ "$allow_bypass" -ne 1 ]]; then
    cat >&2 <<'BYPASS_MSG'
[claude-tmux] safety gate: --allow-bypass is required.

This driver runs claude with --dangerously-skip-permissions inside tmux
to avoid blocking on permission dialogs. The operator must explicitly
acknowledge this by passing --allow-bypass (or env BRIDGE_ALLOW_BYPASS=1).

If you want permission prompts handled programmatically without bypass,
that path is P6.5 (lib/auto-respond.sh) — not yet wired in v0.1.0.
BYPASS_MSG
    return $EC_SAFETY_GATE
  fi

  # --- Preflight binary checks -----------------------------------------------
  command -v tmux   >/dev/null 2>&1 || { echo "[claude-tmux] tmux missing"   >&2; return $EC_MISSING_BINARY; }
  command -v claude >/dev/null 2>&1 || { echo "[claude-tmux] claude missing" >&2; return $EC_MISSING_BINARY; }
  command -v jq     >/dev/null 2>&1 || { echo "[claude-tmux] jq missing"     >&2; return $EC_MISSING_BINARY; }

  # --- Cost-leak guards (refuse if env would route to API or a proxy) --------
  if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
    echo "[claude-tmux] ANTHROPIC_API_KEY is set — would bill API path, not subscription; abort" >&2
    return $EC_COST_LEAK
  fi
  if [[ -n "${ANTHROPIC_BASE_URL:-}" ]] && [[ "${BRIDGE_ALLOW_ANTHROPIC_BASE_URL:-0}" != "1" ]]; then
    echo "[claude-tmux] ANTHROPIC_BASE_URL is set — proxy mode would invalidate the billing test; abort" >&2
    return $EC_COST_LEAK
  fi
  if [[ -n "${EVOLVE_ANTHROPIC_BASE_URL:-}" ]]; then
    echo "[claude-tmux] EVOLVE_ANTHROPIC_BASE_URL is set — proxy mode would invalidate the billing test; abort" >&2
    return $EC_COST_LEAK
  fi

  # --- Resolve working directory ---------------------------------------------
  local working_dir="${worktree:-$PWD}"
  if [[ ! -d "$working_dir" ]]; then
    echo "[claude-tmux] working dir does not exist: $working_dir" >&2
    return $EC_BAD_FLAGS
  fi

  # --- Prepare workspace -----------------------------------------------------
  mkdir -p "$workspace" "$(dirname "$stdout_log")" "$(dirname "$stderr_log")" "$(dirname "$artifact")"

  # --- Substitute placeholders in the prompt ---------------------------------
  local resolved_prompt_file="$workspace/resolved-prompt.txt"
  local prompt_content
  prompt_content="$(cat "$prompt_file")"
  if [[ "$prompt_content" == *'$CHALLENGE_TOKEN'* ]]; then
    local challenge_token
    challenge_token="$(openssl rand -hex 8 2>/dev/null || date +%s | tr -d '\n')"
    echo "$challenge_token" > "$workspace/challenge-token.txt"
    prompt_content="${prompt_content//\$CHALLENGE_TOKEN/$challenge_token}"
  fi
  prompt_content="${prompt_content//\$ARTIFACT_PATH/$artifact}"
  echo "$prompt_content" > "$resolved_prompt_file"

  # --- Build session name ----------------------------------------------------
  local agent_label="${agent:-probe}"
  local session="evolve-bridge-c${cycle}-${agent_label}-pid$$-$(date +%s)"
  session="${session:0:64}"
  local scrollback_file="$workspace/tmux-final-scrollback.txt"

  echo "[claude-tmux] session=$session model=$effective_model workdir=$working_dir" >&2

  # --- Trap cleanup ----------------------------------------------------------
  _bridge_tmux_session="$session"
  _bridge_tmux_scrollback="$scrollback_file"
  trap '_bridge_tmux_cleanup' EXIT INT TERM

  # --- Spawn tmux session ----------------------------------------------------
  tmux new-session -d -s "$session" -x 220 -y 80
  sleep 1
  echo "[claude-tmux] tmux session started" >&2

  tmux send-keys -t "$session" "cd $working_dir" Enter
  sleep 1

  # --- Launch claude interactively (NO -p) -----------------------------------
  local claude_cmd="claude --model $effective_model --dangerously-skip-permissions"
  tmux send-keys -t "$session" "$claude_cmd" Enter
  echo "[claude-tmux] launching: $claude_cmd" >&2

  # --- Wait for REPL prompt --------------------------------------------------
  # Prototype found: search FULL pane for ❯ (mid-pane render — tail -N misses it)
  local prompt_marker="${bridge_manifest_prompt_marker:-❯}"
  local repl_boot_timeout=60
  local elapsed=0
  local prompt_seen=0
  while [ $elapsed -lt $repl_boot_timeout ]; do
    sleep 1
    elapsed=$((elapsed + 1))
    local pane
    pane=$(tmux capture-pane -p -t "$session" 2>/dev/null || echo "")
    if echo "$pane" | grep -qF "$prompt_marker"; then
      prompt_seen=1
      echo "[claude-tmux] REPL prompt ($prompt_marker) detected after ${elapsed}s" >&2
      break
    fi
  done
  if [ $prompt_seen -eq 0 ]; then
    echo "[claude-tmux] FAIL: REPL prompt never appeared after ${repl_boot_timeout}s" >&2
    return $EC_REPL_BOOT_TIMEOUT
  fi

  # --- Deliver prompt via tmux buffer ----------------------------------------
  tmux load-buffer -t "$session" "$resolved_prompt_file"
  tmux paste-buffer -t "$session"
  sleep 1
  tmux send-keys -t "$session" Enter
  local prompt_bytes
  prompt_bytes=$(wc -c < "$resolved_prompt_file" | tr -d ' ')
  echo "[claude-tmux] prompt delivered (${prompt_bytes} bytes)" >&2

  # --- Wait for artifact -----------------------------------------------------
  # P6.5 hook point: BRIDGE_AUTO_RESPOND_ENABLED=1 would poll the pane between
  # artifact checks and fire interactive_prompts[] rules. For P6 we just wait.
  local artifact_wait_timeout=300
  elapsed=0
  local artifact_seen=0
  while [ $elapsed -lt $artifact_wait_timeout ]; do
    sleep 2
    elapsed=$((elapsed + 2))
    if [[ -s "$artifact" ]]; then
      artifact_seen=1
      echo "[claude-tmux] artifact appeared after ${elapsed}s: $artifact" >&2
      break
    fi
    # P6.5 will insert: bridge_auto_respond_tick "$session" || ...
  done
  if [ $artifact_seen -eq 0 ]; then
    echo "[claude-tmux] FAIL: artifact never appeared at $artifact after ${artifact_wait_timeout}s" >&2
    return $EC_ARTIFACT_TIMEOUT
  fi

  # --- Capture scrollback ----------------------------------------------------
  local raw
  raw=$(tmux capture-pane -p -S -10000 -t "$session" 2>/dev/null || echo "")
  echo "$raw" > "$stderr_log"
  echo "$raw" | sed -E 's/\x1b\[[0-9;]*[a-zA-Z]//g; s/\x1b\][^\x07]*\x07//g' > "$stdout_log"
  echo "[claude-tmux] scrollback captured: stdout=$stdout_log stderr=$stderr_log" >&2

  # --- Graceful REPL exit ----------------------------------------------------
  tmux send-keys -t "$session" "/exit" Enter
  sleep 2

  echo "[claude-tmux] DONE: artifact-only verdict = SUCCESS" >&2
  return 0
}

# Cleanup helper used by the trap inside drv_launch_claude_tmux.
_bridge_tmux_cleanup() {
  local rc=$?
  if [[ -n "${_bridge_tmux_session:-}" ]]; then
    if tmux has-session -t "$_bridge_tmux_session" 2>/dev/null; then
      tmux capture-pane -p -S -10000 -t "$_bridge_tmux_session" \
        > "${_bridge_tmux_scrollback:-/dev/null}" 2>/dev/null || true
      tmux kill-session -t "$_bridge_tmux_session" 2>/dev/null || true
      echo "[claude-tmux] session killed: $_bridge_tmux_session (rc=$rc)" >&2
    fi
    _bridge_tmux_session=""
  fi
}
