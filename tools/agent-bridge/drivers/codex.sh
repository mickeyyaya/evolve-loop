#!/usr/bin/env bash
# drivers/codex.sh — driver for OpenAI Codex CLI (codex exec non-interactive)
#
# Promoted from v2 stub on 2026-05-21.
#
# Model tier mapping (researched 2026-05-21 via OpenAI Codex docs):
#   haiku  → gpt-5.4-mini   (fast, cheap; matches Claude Haiku tier)
#   sonnet → gpt-5.4        (balanced; matches Claude Sonnet tier)
#   opus   → gpt-5.5        (flagship; matches Claude Opus tier)
#   gpt-*  → passed through unchanged
#   auto / empty → omit -m; codex uses ChatGPT-account default (gpt-5.5)
#
# All three mapped names verified working with ChatGPT-account auth on 2026-05-21.
#
# Test seam: BRIDGE_CODEX_BINARY (gated by BRIDGE_TESTING=1).

drv_launch_codex() {
  # v0.2: permission_mode is a claude-only feature (claude --permission-mode).
  # Codex has its own permission model (--sandbox <MODE>, -c sandbox_permissions=[...])
  # but no plan-mode equivalent. Fail loudly rather than silently ignore an
  # operator's safety-mode declaration.
  if [[ -n "${effective_permission_mode:-}" ]]; then
    echo "[codex] permission_mode='$effective_permission_mode' is not supported on this CLI" >&2
    echo "[codex] Only claude-p and claude-tmux drivers support --permission-mode." >&2
    echo "[codex] For codex, use --sandbox <mode> via the prompt or omit permission_mode." >&2
    return $EC_BAD_FLAGS
  fi

  local codex_bin
  if [[ -n "${BRIDGE_CODEX_BINARY:-}" ]] && [[ "${BRIDGE_TESTING:-0}" == "1" ]]; then
    codex_bin="$BRIDGE_CODEX_BINARY"
    [[ -x "$codex_bin" ]] || { echo "[codex] BRIDGE_CODEX_BINARY not executable: $codex_bin" >&2; return $EC_MISSING_BINARY; }
  else
    command -v codex >/dev/null 2>&1 || { echo "[codex] codex binary not on PATH" >&2; return $EC_MISSING_BINARY; }
    codex_bin="$(command -v codex)"
  fi

  if [[ -n "${OPENAI_API_KEY:-}" ]] && [[ "${BRIDGE_ALLOW_OPENAI_API_KEY:-0}" != "1" ]]; then
    echo "[codex] cost-leak guard: OPENAI_API_KEY set without BRIDGE_ALLOW_OPENAI_API_KEY=1" >&2
    return $EC_COST_LEAK
  fi

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

  # Map Claude tier aliases → codex model names
  local resolved_model="$effective_model"
  case "$effective_model" in
    haiku)  resolved_model="gpt-5.4-mini" ;;
    sonnet) resolved_model="gpt-5.4" ;;
    opus)   resolved_model="gpt-5.5" ;;
  esac

  local codex_args=(exec --output-last-message "$artifact")
  case "$resolved_model" in
    ""|auto)
      echo "[codex] effective_model='$effective_model' → omitting -m (codex picks default)" >&2
      ;;
    gpt-*|o-*|o1*|o3*|o4*|codex*)
      codex_args=(exec -m "$resolved_model" --output-last-message "$artifact")
      echo "[codex] model: $effective_model → $resolved_model (via -m)" >&2
      ;;
    *)
      echo "[codex] WARN: unrecognized model '$resolved_model' — omitting -m" >&2
      ;;
  esac

  echo "[codex] cycle=$cycle agent=$agent artifact=$artifact" >&2
  echo "[codex] invoking: $codex_bin ${codex_args[*]}" >&2

  set +e
  echo "$prompt_content" | "$codex_bin" "${codex_args[@]}" \
    > "$stdout_log" 2> "$stderr_log"
  local rc=$?
  set -e

  echo "[codex] codex exited rc=$rc" >&2
  return $rc
}
