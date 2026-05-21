#!/usr/bin/env bash
# drivers/claude-tmux.sh — driver for interactive `claude` driven via tmux
#
# Interactive `claude` via tmux. Subscription-preserving (Claude Max, OAuth).
# Bridge contract: sourced by bin/bridge; reads cmd_launch's local vars +
# bridge_profile_* + bridge_manifest_*.
#
# Auto-respond fallback (P6.5):
#   --dangerously-skip-permissions covers permission prompts (the common case).
#   auto_respond_tick polls the tmux pane between artifact checks and fires
#   manifest interactive_prompts[] rules for edge-case prompts that escape
#   the bypass (auth-recheck, rate-limit, terminal-resize, etc.).
#   Unknown prompts → escalation-report.json + rc=85.

drv_launch_claude_tmux() {
  # --- Bridge safety gate ---------------------------------------------------
  if [[ "$allow_bypass" -ne 1 ]]; then
    cat >&2 <<'BYPASS_MSG'
[claude-tmux] safety gate: --allow-bypass is required.

This driver runs claude with --dangerously-skip-permissions inside tmux
to avoid blocking on common permission dialogs. The operator must
explicitly acknowledge this by passing --allow-bypass.

Auto-respond fallback (lib/auto-respond.sh) handles edge cases that
escape the bypass; see docs/design.md.
BYPASS_MSG
    return $EC_SAFETY_GATE
  fi

  # --- Preflight ------------------------------------------------------------
  command -v tmux   >/dev/null 2>&1 || { echo "[claude-tmux] tmux missing"   >&2; return $EC_MISSING_BINARY; }
  command -v claude >/dev/null 2>&1 || { echo "[claude-tmux] claude missing" >&2; return $EC_MISSING_BINARY; }
  command -v jq     >/dev/null 2>&1 || { echo "[claude-tmux] jq missing"     >&2; return $EC_MISSING_BINARY; }

  # --- Cost-leak guards -----------------------------------------------------
  if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
    echo "[claude-tmux] ANTHROPIC_API_KEY is set — would bill API path; abort" >&2
    return $EC_COST_LEAK
  fi
  if [[ -n "${ANTHROPIC_BASE_URL:-}" ]] && [[ "${BRIDGE_ALLOW_ANTHROPIC_BASE_URL:-0}" != "1" ]]; then
    echo "[claude-tmux] ANTHROPIC_BASE_URL set without BRIDGE_ALLOW_ANTHROPIC_BASE_URL=1; abort" >&2
    return $EC_COST_LEAK
  fi
  if [[ -n "${EVOLVE_ANTHROPIC_BASE_URL:-}" ]]; then
    echo "[claude-tmux] EVOLVE_ANTHROPIC_BASE_URL set — proxy mode; abort" >&2
    return $EC_COST_LEAK
  fi

  # --- Load manifest for prompt_marker + interactive_prompts ---------------
  if ! manifest_load "claude-tmux"; then
    echo "[claude-tmux] failed to load manifest" >&2
    return $EC_BAD_FLAGS
  fi

  local working_dir="${worktree:-$PWD}"
  if [[ ! -d "$working_dir" ]]; then
    echo "[claude-tmux] working dir does not exist: $working_dir" >&2
    return $EC_BAD_FLAGS
  fi

  mkdir -p "$workspace" "$(dirname "$stdout_log")" "$(dirname "$stderr_log")" "$(dirname "$artifact")"

  # --- Resolve prompt placeholders ------------------------------------------
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

  local agent_label="${agent:-probe}"
  local session="evolve-bridge-c${cycle}-${agent_label}-pid$$-$(date +%s)"
  session="${session:0:64}"
  local scrollback_file="$workspace/tmux-final-scrollback.txt"

  echo "[claude-tmux] session=$session model=$effective_model workdir=$working_dir" >&2

  # --- Cleanup trap ---------------------------------------------------------
  _bridge_tmux_session="$session"
  _bridge_tmux_scrollback="$scrollback_file"
  trap '_bridge_tmux_cleanup' EXIT INT TERM

  # --- Spawn tmux + launch claude ------------------------------------------
  tmux new-session -d -s "$session" -x 220 -y 80
  sleep 1
  tmux send-keys -t "$session" "cd $working_dir" Enter
  sleep 1

  local claude_cmd="claude --model $effective_model --dangerously-skip-permissions"
  tmux send-keys -t "$session" "$claude_cmd" Enter
  echo "[claude-tmux] launching: $claude_cmd" >&2

  # --- Wait for REPL prompt -------------------------------------------------
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

  # --- Deliver prompt -------------------------------------------------------
  tmux load-buffer -t "$session" "$resolved_prompt_file"
  tmux paste-buffer -t "$session"
  sleep 1
  tmux send-keys -t "$session" Enter
  local prompt_bytes
  prompt_bytes=$(wc -c < "$resolved_prompt_file" | tr -d ' ')
  echo "[claude-tmux] prompt delivered (${prompt_bytes} bytes)" >&2

  # --- Wait for artifact, with auto-respond fallback ------------------------
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

    # Auto-respond fallback (P6.5): handle edge prompts that escape bypass
    auto_respond_tick "$session"
    local ar_rc=$?
    case "$ar_rc" in
      0|1)
        # 0=noop, 1=responded — keep polling
        ;;
      85)
        echo "[claude-tmux] auto-respond escalation; abandoning run" >&2
        return $EC_UNKNOWN_PROMPT
        ;;
      86)
        echo "[claude-tmux] auto-respond loop guard tripped; abandoning run" >&2
        return $EC_RESPOND_LOOP_GUARD
        ;;
    esac
  done
  if [ $artifact_seen -eq 0 ]; then
    echo "[claude-tmux] FAIL: artifact never appeared at $artifact after ${artifact_wait_timeout}s" >&2
    # Write an escalation report on plain timeout too — operator can review the pane
    local final_pane
    final_pane=$(tmux capture-pane -p -S -10000 -t "$session" 2>/dev/null || echo "")
    auto_respond_write_escalation_report "$workspace" "$final_pane" "artifact_timeout" "$session" "timeout"
    return $EC_ARTIFACT_TIMEOUT
  fi

  # --- Capture scrollback ---------------------------------------------------
  local raw
  raw=$(tmux capture-pane -p -S -10000 -t "$session" 2>/dev/null || echo "")
  echo "$raw" > "$stderr_log"
  echo "$raw" | sed -E 's/\x1b\[[0-9;]*[a-zA-Z]//g; s/\x1b\][^\x07]*\x07//g' > "$stdout_log"
  echo "[claude-tmux] scrollback captured" >&2

  tmux send-keys -t "$session" "/exit" Enter
  sleep 2

  echo "[claude-tmux] DONE: artifact-only verdict = SUCCESS" >&2
  return 0
}

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
