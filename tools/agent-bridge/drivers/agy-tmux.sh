#!/usr/bin/env bash
# drivers/agy-tmux.sh — driver for interactive `agy` (TUI) driven via tmux
#
# Drives the agy interactive REPL via tmux instead of the headless `-p` mode,
# using whatever authentication mode the operator configured for agy.
#
# agy-specific quirks (probed 2026-05-21):
#  - REPL ready marker: "? for shortcuts" (the status footer text)
#  - Trust prompt on first launch in untrusted dir: menu-select; Enter accepts default ("Yes")
#  - Alt-screen rendering; tmux capture-pane needs -S -200
#  - No CLI model selection (Gemini 3.5 Flash (High) is fixed)
#  - Exit: Ctrl+C twice, or "/quit" (varies)
#
# Test seam: BRIDGE_AGY_BINARY (gated by BRIDGE_TESTING=1).

drv_launch_agy_tmux() {
  # v0.2: permission_mode is a claude-only feature; reject early to avoid
  # silently ignoring an operator's safety-mode declaration.
  if [[ -n "${effective_permission_mode:-}" ]]; then
    echo "[agy-tmux] permission_mode='$effective_permission_mode' is not supported on this CLI" >&2
    echo "[agy-tmux] Only claude-p and claude-tmux drivers support --permission-mode." >&2
    echo "[agy-tmux] Agy exposes only --dangerously-skip-permissions; use --allow-bypass + omit permission_mode." >&2
    return $EC_BAD_FLAGS
  fi

  # v0.3: stream_output is a no-op on agy-tmux — agy CLI has no streaming
  # output flag. Log a note (not a hard reject) so operators know.
  if [[ "${effective_stream_output:-false}" == "true" ]]; then
    echo "[agy-tmux] NOTE: stream_output=true is not supported on this CLI — no-op (agy has no streaming output flag)" >&2
  fi

  # v0.5: named-session/resume support is currently claude-tmux-only.
  if [[ -n "${effective_session_name:-}" ]]; then
    echo "[agy-tmux] --session-name='$effective_session_name' is not supported on this CLI in v0.5" >&2
    echo "[agy-tmux] Only claude-tmux supports named/resumable sessions; use --cli=claude-tmux or omit --session-name." >&2
    return $EC_BAD_FLAGS
  fi

  if [[ "$allow_bypass" -ne 1 ]]; then
    echo "[agy-tmux] safety gate: --allow-bypass is required" >&2
    return $EC_SAFETY_GATE
  fi

  local agy_bin
  if [[ -n "${BRIDGE_AGY_BINARY:-}" ]] && [[ "${BRIDGE_TESTING:-0}" == "1" ]]; then
    agy_bin="$BRIDGE_AGY_BINARY"
    [[ -x "$agy_bin" ]] || { echo "[agy-tmux] BRIDGE_AGY_BINARY not executable: $agy_bin" >&2; return $EC_MISSING_BINARY; }
  else
    command -v agy >/dev/null 2>&1 || { echo "[agy-tmux] agy binary not on PATH" >&2; return $EC_MISSING_BINARY; }
    agy_bin="$(command -v agy)"
  fi
  command -v tmux >/dev/null 2>&1 || { echo "[agy-tmux] tmux missing" >&2; return $EC_MISSING_BINARY; }
  command -v jq   >/dev/null 2>&1 || { echo "[agy-tmux] jq missing"   >&2; return $EC_MISSING_BINARY; }

  if ! manifest_load "agy-tmux"; then
    echo "[agy-tmux] failed to load manifest" >&2
    return $EC_BAD_FLAGS
  fi

  local working_dir="${worktree:-$PWD}"
  [[ -d "$working_dir" ]] || { echo "[agy-tmux] working dir does not exist: $working_dir" >&2; return $EC_BAD_FLAGS; }

  mkdir -p "$workspace" "$(dirname "$stdout_log")" "$(dirname "$stderr_log")" "$(dirname "$artifact")"

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
  local session="evolve-bridge-agy-c${cycle}-${agent_label}-pid$$-$(date +%s)"
  session="${session:0:64}"
  local scrollback_file="$workspace/tmux-final-scrollback.txt"

  echo "[agy-tmux] session=$session model=gemini-3.5-flash workdir=$working_dir" >&2

  _bridge_agy_tmux_session="$session"
  _bridge_agy_tmux_scrollback="$scrollback_file"
  trap '_bridge_agy_tmux_cleanup' EXIT INT TERM

  tmux new-session -d -s "$session" -x 220 -y 80
  sleep 1
  tmux send-keys -t "$session" "cd $working_dir" Enter
  sleep 1

  # Launch agy interactively (no args = REPL).
  # --dangerously-skip-permissions auto-approves tool use inside the session.
  tmux send-keys -t "$session" "$agy_bin --dangerously-skip-permissions" Enter
  echo "[agy-tmux] launching: $agy_bin --dangerously-skip-permissions" >&2

  # Wait for REPL ready. agy renders alt-screen; need -S -200.
  local prompt_marker="${bridge_manifest_prompt_marker:-? for shortcuts}"
  local repl_boot_timeout=60
  local elapsed=0 prompt_seen=0
  while [ $elapsed -lt $repl_boot_timeout ]; do
    sleep 2
    elapsed=$((elapsed + 2))
    local pane
    pane=$(tmux capture-pane -p -S -200 -t "$session" 2>/dev/null || echo "")
    if echo "$pane" | grep -qF "$prompt_marker"; then
      prompt_seen=1
      echo "[agy-tmux] REPL ready ('$prompt_marker' detected) after ${elapsed}s" >&2
      break
    fi
    # Auto-respond fallback handles trust prompt
    auto_respond_tick "$session" || true
  done
  if [ $prompt_seen -eq 0 ]; then
    echo "[agy-tmux] FAIL: REPL never ready after ${repl_boot_timeout}s" >&2
    auto_respond_write_escalation_report "$workspace" \
      "$(tmux capture-pane -p -S -200 -t "$session" 2>/dev/null || echo "")" \
      "repl_boot_timeout" "$session" "timeout"
    return $EC_REPL_BOOT_TIMEOUT
  fi

  # Deliver prompt
  local prompt_bytes
  prompt_bytes=$(wc -c < "$resolved_prompt_file" | tr -d ' ')
  if [[ "$(bridge_human_active 2>/dev/null || echo 0)" == "1" ]]; then
    echo "[agy-tmux] human-input mode: boot pause + paste-with-review" >&2
    human_boot_pause
    human_paste_with_review "$session" "$resolved_prompt_file"
  else
    tmux load-buffer -t "$session" "$resolved_prompt_file"
    tmux paste-buffer -t "$session"
    sleep 1
    tmux send-keys -t "$session" Enter
  fi
  echo "[agy-tmux] prompt delivered (${prompt_bytes} bytes)" >&2

  # Wait for artifact
  local artifact_wait_timeout=300
  elapsed=0
  local artifact_seen=0
  while [ $elapsed -lt $artifact_wait_timeout ]; do
    sleep 2
    elapsed=$((elapsed + 2))
    if [[ -s "$artifact" ]]; then
      artifact_seen=1
      echo "[agy-tmux] artifact appeared after ${elapsed}s: $artifact" >&2
      break
    fi
    auto_respond_tick "$session"
    local ar_rc=$?
    case "$ar_rc" in
      0|1) ;;
      85) echo "[agy-tmux] auto-respond escalation" >&2; return $EC_UNKNOWN_PROMPT ;;
      86) echo "[agy-tmux] auto-respond loop guard" >&2; return $EC_RESPOND_LOOP_GUARD ;;
    esac
  done
  if [ $artifact_seen -eq 0 ]; then
    echo "[agy-tmux] FAIL: artifact never appeared at $artifact after ${artifact_wait_timeout}s" >&2
    auto_respond_write_escalation_report "$workspace" \
      "$(tmux capture-pane -p -S -10000 -t "$session" 2>/dev/null || echo "")" \
      "artifact_timeout" "$session" "timeout"
    return $EC_ARTIFACT_TIMEOUT
  fi

  local raw
  raw=$(tmux capture-pane -p -S -10000 -t "$session" 2>/dev/null || echo "")
  echo "$raw" > "$stderr_log"
  echo "$raw" | sed -E 's/\x1b\[[0-9;]*[a-zA-Z]//g; s/\x1b\][^\x07]*\x07//g' > "$stdout_log"

  # Exit: send Ctrl+C twice (agy quit pattern)
  tmux send-keys -t "$session" C-c
  sleep 1
  tmux send-keys -t "$session" C-c
  sleep 1

  echo "[agy-tmux] DONE" >&2
  return 0
}

_bridge_agy_tmux_cleanup() {
  local rc=$?
  if [[ -n "${_bridge_agy_tmux_session:-}" ]]; then
    if tmux has-session -t "$_bridge_agy_tmux_session" 2>/dev/null; then
      tmux capture-pane -p -S -10000 -t "$_bridge_agy_tmux_session" \
        > "${_bridge_agy_tmux_scrollback:-/dev/null}" 2>/dev/null || true
      tmux kill-session -t "$_bridge_agy_tmux_session" 2>/dev/null || true
      echo "[agy-tmux] session killed: $_bridge_agy_tmux_session (rc=$rc)" >&2
    fi
    _bridge_agy_tmux_session=""
  fi
}
