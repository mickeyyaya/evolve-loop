#!/usr/bin/env bash
# drivers/agy.sh — driver for Antigravity CLI (agy, Gemini-backed, OAuth-authed)
#
# Promoted from v2 stub on 2026-05-21.
#
# Auth: agy is OAuth-authed (subscription path). Per user note 2026-05-21,
# GEMINI_API_KEY / GOOGLE_API_KEY in env are NOT used by agy — those would
# only matter for a direct Google API call. Bridge does not gate on them.
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

  local agy_bin
  if [[ -n "${BRIDGE_AGY_BINARY:-}" ]] && [[ "${BRIDGE_TESTING:-0}" == "1" ]]; then
    agy_bin="$BRIDGE_AGY_BINARY"
    [[ -x "$agy_bin" ]] || { echo "[agy] BRIDGE_AGY_BINARY not executable: $agy_bin" >&2; return $EC_MISSING_BINARY; }
  else
    command -v agy >/dev/null 2>&1 || { echo "[agy] agy binary not on PATH" >&2; return $EC_MISSING_BINARY; }
    agy_bin="$(command -v agy)"
  fi

  # No env-var cost-leak guard: agy is OAuth-authed; GEMINI/GOOGLE keys are
  # not on agy's billing path. Per user clarification 2026-05-21.

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
