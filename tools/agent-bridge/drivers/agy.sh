#!/usr/bin/env bash
# drivers/agy.sh — driver for Antigravity CLI (agy, Gemini-backed)
#
# Promoted from v2 stub on 2026-05-21.
#
# Auth: agy authenticates via whatever method the operator has configured.
# Per user note 2026-05-21, GEMINI_API_KEY / GOOGLE_API_KEY in env are
# not used by agy directly. Bridge does not gate on them.
#
# Model: agy does NOT expose model selection via CLI flag (-c is --continue).
# All tier aliases (haiku/sonnet/opus/auto) map to agy's default (gemini-3.5-flash).
#
# Test seam: BRIDGE_AGY_BINARY (gated by BRIDGE_TESTING=1).

drv_launch_agy() {
  # v0.2: permission_mode is a claude-only feature (claude --permission-mode).
  # Agy has only --dangerously-skip-permissions, no plan-mode equivalent.
  # Fail loudly rather than silently ignore the operator's safety-mode declaration.
  if [[ -n "${effective_permission_mode:-}" ]]; then
    echo "[agy] permission_mode='$effective_permission_mode' is not supported on this CLI" >&2
    echo "[agy] Only claude-p and claude-tmux drivers support --permission-mode." >&2
    echo "[agy] Agy exposes only --dangerously-skip-permissions; use --allow-bypass + omit permission_mode." >&2
    return $EC_BAD_FLAGS
  fi

  # v0.3: stream_output is a no-op on agy — agy CLI has no streaming output
  # equivalent to claude --output-format=stream-json. Log a note (not a hard
  # reject) so operators know their stream_output config has no effect here.
  if [[ "${effective_stream_output:-false}" == "true" ]]; then
    echo "[agy] NOTE: stream_output=true is not supported on this CLI — no-op (agy has no streaming output flag)" >&2
  fi

  # v0.5: agy is single-shot — --session-name has no semantic effect here.
  if [[ -n "${effective_session_name:-}" ]]; then
    echo "[agy] NOTE: --session-name='$effective_session_name' is no-op for this driver (single-shot process). Use --cli=claude-tmux for named/resumable sessions." >&2
  fi

  local agy_bin
  if [[ -n "${BRIDGE_AGY_BINARY:-}" ]] && [[ "${BRIDGE_TESTING:-0}" == "1" ]]; then
    agy_bin="$BRIDGE_AGY_BINARY"
    [[ -x "$agy_bin" ]] || { echo "[agy] BRIDGE_AGY_BINARY not executable: $agy_bin" >&2; return $EC_MISSING_BINARY; }
  else
    command -v agy >/dev/null 2>&1 || { echo "[agy] agy binary not on PATH" >&2; return $EC_MISSING_BINARY; }
    agy_bin="$(command -v agy)"
  fi

  # No env-var credential-isolation guard for agy: those env vars are not
  # on agy's authentication path. Per user clarification 2026-05-21.

  mkdir -p "$workspace" "$(dirname "$stdout_log")" "$(dirname "$stderr_log")" "$(dirname "$artifact")"

  local prompt_content
  prompt_content="$(cat "$prompt_file")"
  if [[ "$prompt_content" == *'$CHALLENGE_TOKEN'* ]]; then
    local challenge_token
    challenge_token="$(openssl rand -hex 8 2>/dev/null || date +%s | tr -d '\n')"
    echo "$challenge_token" > "$workspace/challenge-token.txt"
    prompt_content="${prompt_content//\$CHALLENGE_TOKEN/$challenge_token}"
  fi
  prompt_content="${prompt_content//\$ARTIFACT_PATH/$artifact}"

  case "$effective_model" in
    haiku|sonnet|opus|auto|"")
      echo "[agy] tier '$effective_model' → agy default (gemini-3.5-flash); agy has no -m flag" >&2
      ;;
    *)
      echo "[agy] WARN: model '$effective_model' is not a Claude tier alias — agy ignores it anyway" >&2
      ;;
  esac

  echo "[agy] cycle=$cycle agent=$agent artifact=$artifact" >&2
  echo "[agy] invoking: $agy_bin -p <prompt> --dangerously-skip-permissions" >&2

  set +e
  "$agy_bin" -p "$prompt_content" --dangerously-skip-permissions \
    > "$stdout_log" 2> "$stderr_log"
  local rc=$?
  set -e

  echo "[agy] agy exited rc=$rc" >&2
  return $rc
}
