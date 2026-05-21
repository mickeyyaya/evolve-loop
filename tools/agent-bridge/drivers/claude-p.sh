#!/usr/bin/env bash
# drivers/claude-p.sh — driver for headless `claude -p` invocation
#
# Headless `claude -p` driver. Trimmed to the bridge contract; orchestrator-
# specific budgeting and policy stay out of this codebase.
# See docs/design.md §3.1 for the full driver contract.
#
# Contract: sourced by bin/bridge. Defines drv_launch_claude_p which
# operates on local vars from cmd_launch's scope and exported
# bridge_profile_* from profile_load.

drv_launch_claude_p() {
  # Required: claude binary (with BRIDGE_CLAUDE_BINARY test seam)
  local claude_bin
  if [[ -n "${BRIDGE_CLAUDE_BINARY:-}" ]] && [[ "${BRIDGE_TESTING:-0}" == "1" ]]; then
    claude_bin="$BRIDGE_CLAUDE_BINARY"
    [[ -x "$claude_bin" ]] || { echo "[claude-p] BRIDGE_CLAUDE_BINARY not executable: $claude_bin" >&2; return $EC_MISSING_BINARY; }
  else
    command -v claude >/dev/null 2>&1 || { echo "[claude-p] claude binary not on PATH" >&2; return $EC_MISSING_BINARY; }
    claude_bin="$(command -v claude)"
  fi

  # Cost-leak guard: bridge expects subscription auth. If ANTHROPIC_API_KEY
  # is set, that overrides OAuth and bills the API path — fail loudly.
  if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
    echo "[claude-p] cost-leak guard: ANTHROPIC_API_KEY is set; refusing to run (would bill API path, not subscription)" >&2
    echo "[claude-p] unset the variable, or use a different shell, then retry." >&2
    return $EC_COST_LEAK
  fi
  if [[ -n "${ANTHROPIC_BASE_URL:-}" ]] && [[ "${BRIDGE_ALLOW_ANTHROPIC_BASE_URL:-0}" != "1" ]]; then
    echo "[claude-p] cost-leak guard: ANTHROPIC_BASE_URL set without BRIDGE_ALLOW_ANTHROPIC_BASE_URL=1" >&2
    return $EC_COST_LEAK
  fi

  mkdir -p "$workspace"
  mkdir -p "$(dirname "$stdout_log")"
  mkdir -p "$(dirname "$stderr_log")"
  mkdir -p "$(dirname "$artifact")"

  # Read prompt + substitute $CHALLENGE_TOKEN / $ARTIFACT_PATH
  local prompt_content
  prompt_content="$(cat "$prompt_file")"
  if [[ "$prompt_content" == *'$CHALLENGE_TOKEN'* ]]; then
    local challenge_token
    challenge_token="$(openssl rand -hex 8 2>/dev/null || date +%s | tr -d '\n')"
    echo "$challenge_token" > "$workspace/challenge-token.txt"
    prompt_content="${prompt_content//\$CHALLENGE_TOKEN/$challenge_token}"
  fi
  prompt_content="${prompt_content//\$ARTIFACT_PATH/$artifact}"

  # Build claude args
  local claude_args=()
  claude_args+=(-p "$prompt_content")
  claude_args+=(--model "$effective_model")

  # v0.2: permission-mode pass-through. Driver layer is unopinionated;
  # validation already happened in bin/bridge cmd_launch.
  if [[ -n "${effective_permission_mode:-}" ]]; then
    claude_args+=(--permission-mode "$effective_permission_mode")
  fi

  # v0.3: stream-output — when enabled, append claude streaming flags so
  # the parent's stdout log gets continuous JSONL writes. This solves the
  # phase-observer false-positive stall kill (parent text output stays
  # silent until final response, exceeding the 600s observer threshold
  # for long orchestrator sessions that dispatch subagents).
  # --verbose is required by claude when combining --output-format=stream-json
  # with --print (-p); omitting it produces a runtime error.
  if [[ "${effective_stream_output:-false}" == "true" ]]; then
    claude_args+=(--output-format stream-json --include-partial-messages --verbose)
  fi

  # Allowed-tools from profile: bash 3.2-safe array split on comma.
  if [[ -n "${bridge_profile_allowed_tools_csv:-}" ]]; then
    local saved_ifs="$IFS"
    IFS=','
    local tool_list=()
    read -ra tool_list <<<"$bridge_profile_allowed_tools_csv"
    IFS="$saved_ifs"
    if [[ ${#tool_list[@]} -gt 0 ]]; then
      claude_args+=(--allowedTools)
      claude_args+=("${tool_list[@]}")
    fi
  fi

  echo "[claude-p] cycle=$cycle agent=$agent model=$effective_model artifact=$artifact permission_mode=${effective_permission_mode:-(default)}" >&2
  echo "[claude-p] invoking: claude -p <prompt> --model $effective_model ${effective_permission_mode:+--permission-mode $effective_permission_mode} --allowedTools ${bridge_profile_allowed_tools_csv:-}" >&2

  set +e
  "$claude_bin" "${claude_args[@]}" > "$stdout_log" 2> "$stderr_log"
  local rc=$?
  set -e

  echo "[claude-p] claude exited rc=$rc" >&2
  return $rc
}
