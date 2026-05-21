#!/usr/bin/env bash
# drivers/codex-tmux.sh — driver for interactive `codex` (TUI) driven via tmux
#
# Purpose: keep codex on ChatGPT-account subscription billing by driving its
# interactive REPL (NOT `codex exec`). Mirrors claude-tmux's architecture.
#
# Notable codex-specific quirks (probed 2026-05-21):
#  - REPL marker: `›` (U+203A single right-pointing angle quotation mark)
#  - codex uses alt-screen rendering — tmux capture-pane MUST use -S -200
#    (history scrollback) to see content. Bare capture-pane returns blank.
#  - First-launch trust prompt in untrusted dirs: "Do you trust the contents
#    of this directory?" — handled by auto-respond manifest entry.
#  - Quit: type `/quit` or Ctrl+C twice
#
# Test seam: BRIDGE_CODEX_BINARY (gated by BRIDGE_TESTING=1).

drv_launch_codex_tmux() {
  # v0.2: permission_mode is a claude-only feature; reject early to avoid
  # silently ignoring an operator's safety-mode declaration.
  if [[ -n "${effective_permission_mode:-}" ]]; then
    echo "[codex-tmux] permission_mode='$effective_permission_mode' is not supported on this CLI" >&2
    echo "[codex-tmux] Only claude-p and claude-tmux drivers support --permission-mode." >&2
    echo "[codex-tmux] For codex, use --sandbox <mode> via the prompt or omit permission_mode." >&2
    return $EC_BAD_FLAGS
  fi

  # v0.3: stream_output is a no-op on codex-tmux — codex has no streaming
  # output flag. Log a note (not a hard reject) so operators know.
  if [[ "${effective_stream_output:-false}" == "true" ]]; then
    echo "[codex-tmux] NOTE: stream_output=true is not supported on this CLI — no-op (codex has no streaming output flag)" >&2
  fi

  if [[ "$allow_bypass" -ne 1 ]]; then
    echo "[codex-tmux] safety gate: --allow-bypass is required (running interactive codex with bypass-like semantics)" >&2
    return $EC_SAFETY_GATE
  fi

  local codex_bin
  if [[ -n "${BRIDGE_CODEX_BINARY:-}" ]] && [[ "${BRIDGE_TESTING:-0}" == "1" ]]; then
    codex_bin="$BRIDGE_CODEX_BINARY"
    [[ -x "$codex_bin" ]] || { echo "[codex-tmux] BRIDGE_CODEX_BINARY not executable: $codex_bin" >&2; return $EC_MISSING_BINARY; }
  else
    command -v codex >/dev/null 2>&1 || { echo "[codex-tmux] codex binary not on PATH" >&2; return $EC_MISSING_BINARY; }
    codex_bin="$(command -v codex)"
  fi
  command -v tmux >/dev/null 2>&1 || { echo "[codex-tmux] tmux missing" >&2; return $EC_MISSING_BINARY; }
  command -v jq   >/dev/null 2>&1 || { echo "[codex-tmux] jq missing"   >&2; return $EC_MISSING_BINARY; }

  if [[ -n "${OPENAI_API_KEY:-}" ]] && [[ "${BRIDGE_ALLOW_OPENAI_API_KEY:-0}" != "1" ]]; then
    echo "[codex-tmux] cost-leak guard: OPENAI_API_KEY set without BRIDGE_ALLOW_OPENAI_API_KEY=1" >&2
    return $EC_COST_LEAK
  fi

  if ! manifest_load "codex-tmux"; then
    echo "[codex-tmux] failed to load manifest" >&2
    return $EC_BAD_FLAGS
  fi

  local working_dir="${worktree:-$PWD}"
  [[ -d "$working_dir" ]] || { echo "[codex-tmux] working dir does not exist: $working_dir" >&2; return $EC_BAD_FLAGS; }

  mkdir -p "$workspace" "$(dirname "$stdout_log")" "$(dirname "$stderr_log")" "$(dirname "$artifact")"

  # Map Claude tier aliases (same as headless codex driver)
  local resolved_model="$effective_model"
  case "$effective_model" in
    haiku)  resolved_model="gpt-5.4-mini" ;;
    sonnet) resolved_model="gpt-5.4" ;;
    opus)   resolved_model="gpt-5.5" ;;
  esac

  # Substitute placeholders
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
  local session="evolve-bridge-codex-c${cycle}-${agent_label}-pid$$-$(date +%s)"
  session="${session:0:64}"
  local scrollback_file="$workspace/tmux-final-scrollback.txt"

  echo "[codex-tmux] session=$session model=$resolved_model workdir=$working_dir" >&2

  _bridge_codex_tmux_session="$session"
  _bridge_codex_tmux_scrollback="$scrollback_file"
  trap '_bridge_codex_tmux_cleanup' EXIT INT TERM

  tmux new-session -d -s "$session" -x 220 -y 80
  sleep 1
  tmux send-keys -t "$session" "cd $working_dir" Enter
  sleep 1

  # Launch codex interactively. -m only if a real codex model name; otherwise
  # let codex pick its account default (gpt-5.5 for ChatGPT auth).
  local launch_cmd="$codex_bin"
  case "$resolved_model" in
    gpt-*|o-*|o1*|o3*|o4*|codex*) launch_cmd="$codex_bin -m $resolved_model" ;;
  esac
  tmux send-keys -t "$session" "$launch_cmd" Enter
  echo "[codex-tmux] launching: $launch_cmd" >&2

  # codex uses alt-screen; capture-pane needs -S -200
  local prompt_marker="${bridge_manifest_prompt_marker:-›}"
  local repl_boot_timeout=60
  local elapsed=0 prompt_seen=0
  while [ $elapsed -lt $repl_boot_timeout ]; do
    sleep 2
    elapsed=$((elapsed + 2))
    local pane
    pane=$(tmux capture-pane -p -S -200 -t "$session" 2>/dev/null || echo "")
    if echo "$pane" | grep -qF "$prompt_marker"; then
      prompt_seen=1
      echo "[codex-tmux] REPL prompt ($prompt_marker) detected after ${elapsed}s" >&2
      break
    fi
    # Auto-respond fallback handles trust prompt etc.
    auto_respond_tick "$session" || true
  done
  if [ $prompt_seen -eq 0 ]; then
    echo "[codex-tmux] FAIL: REPL prompt never appeared after ${repl_boot_timeout}s" >&2
    auto_respond_write_escalation_report "$workspace" \
      "$(tmux capture-pane -p -S -200 -t "$session" 2>/dev/null || echo "")" \
      "repl_boot_timeout" "$session" "timeout"
    return $EC_REPL_BOOT_TIMEOUT
  fi

  # Deliver prompt
  local prompt_bytes
  prompt_bytes=$(wc -c < "$resolved_prompt_file" | tr -d ' ')
  if [[ "$(bridge_human_active 2>/dev/null || echo 0)" == "1" ]]; then
    echo "[codex-tmux] human-input mode: boot pause + paste-with-review" >&2
    human_boot_pause
    human_paste_with_review "$session" "$resolved_prompt_file"
  else
    tmux load-buffer -t "$session" "$resolved_prompt_file"
    tmux paste-buffer -t "$session"
    sleep 1
    tmux send-keys -t "$session" Enter
  fi
  echo "[codex-tmux] prompt delivered (${prompt_bytes} bytes)" >&2

  # Wait for artifact, with auto-respond fallback
  local artifact_wait_timeout=300
  elapsed=0
  local artifact_seen=0
  while [ $elapsed -lt $artifact_wait_timeout ]; do
    sleep 2
    elapsed=$((elapsed + 2))
    if [[ -s "$artifact" ]]; then
      artifact_seen=1
      echo "[codex-tmux] artifact appeared after ${elapsed}s: $artifact" >&2
      break
    fi
    auto_respond_tick "$session"
    local ar_rc=$?
    case "$ar_rc" in
      0|1) ;;
      85) echo "[codex-tmux] auto-respond escalation" >&2; return $EC_UNKNOWN_PROMPT ;;
      86) echo "[codex-tmux] auto-respond loop guard" >&2; return $EC_RESPOND_LOOP_GUARD ;;
    esac
  done
  if [ $artifact_seen -eq 0 ]; then
    echo "[codex-tmux] FAIL: artifact never appeared at $artifact after ${artifact_wait_timeout}s" >&2
    auto_respond_write_escalation_report "$workspace" \
      "$(tmux capture-pane -p -S -10000 -t "$session" 2>/dev/null || echo "")" \
      "artifact_timeout" "$session" "timeout"
    return $EC_ARTIFACT_TIMEOUT
  fi

  # Capture scrollback (deep, for forensic)
  local raw
  raw=$(tmux capture-pane -p -S -10000 -t "$session" 2>/dev/null || echo "")
  echo "$raw" > "$stderr_log"
  echo "$raw" | sed -E 's/\x1b\[[0-9;]*[a-zA-Z]//g; s/\x1b\][^\x07]*\x07//g' > "$stdout_log"
  echo "[codex-tmux] scrollback captured" >&2

  # Clean exit: codex uses /quit
  tmux send-keys -t "$session" "/quit" Enter
  sleep 2

  echo "[codex-tmux] DONE" >&2
  return 0
}

_bridge_codex_tmux_cleanup() {
  local rc=$?
  if [[ -n "${_bridge_codex_tmux_session:-}" ]]; then
    if tmux has-session -t "$_bridge_codex_tmux_session" 2>/dev/null; then
      tmux capture-pane -p -S -10000 -t "$_bridge_codex_tmux_session" \
        > "${_bridge_codex_tmux_scrollback:-/dev/null}" 2>/dev/null || true
      tmux kill-session -t "$_bridge_codex_tmux_session" 2>/dev/null || true
      echo "[codex-tmux] session killed: $_bridge_codex_tmux_session (rc=$rc)" >&2
    fi
    _bridge_codex_tmux_session=""
  fi
}
