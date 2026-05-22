#!/usr/bin/env bash
# lib/profile.sh — load and validate agent profile JSON
#
# Usage (sourced by bin/bridge):
#   . lib/profile.sh
#   profile_load "$profile_path" || exit $EC_BAD_FLAGS
#   echo "$bridge_profile_name / $bridge_profile_model / $bridge_profile_allowed_tools_csv"
#
# Profile schema (v1):
#   {
#     "name": string                  (required)
#     "model": "haiku" | ...           (optional; used when --model=auto)
#     "allowed_tools": [string, ...]   (default [])
#     "permission_mode": "plan" | "default" | "acceptEdits" | "bypassPermissions" | "auto" | "dontAsk"
#                                      (optional; default "" = let driver/CLI decide)
#     "stream_output": bool            (optional; default false; v0.3+)
#                                      claude-p only: append --output-format=stream-json
#                                      --include-partial-messages so claude emits realtime
#                                      JSONL events. Solves phase-observer false-positive
#                                      stall kills on long orchestrator sessions where
#                                      claude -p's default text output stays silent until
#                                      the final response. Other drivers log a no-op note.
#     "session_name": string           (optional; default ""; v0.5+)
#                                      *-tmux drivers only. Uses STABLE tmux session name
#                                      `evolve-bridge-named-<session_name>` instead of the
#                                      auto-generated pid-and-timestamp form. Named sessions
#                                      persist across bridge process exits (trap doesn't kill),
#                                      are EXEMPT from orphan-sweep, and AUTO-RESUME on the
#                                      next launch — claude REPL context is preserved.
#                                      Explicit cleanup via `bridge sessions kill <name>`.
#                                      Valid chars: [a-zA-Z0-9._-]+, max 32 chars.
#     "auto_respond": {
#       "destructive_ops": bool        (default false)
#       "timeout_s": int               (default 600)
#     }
#     "prompt_overrides": [...]        (default [])
#   }
#
# Exports on success (bash 3.2 — no associative arrays; allowed_tools is CSV):
#   bridge_profile_name
#   bridge_profile_model
#   bridge_profile_allowed_tools_csv
#   bridge_profile_permission_mode
#   bridge_profile_stream_output                 ("true" | "false")
#   bridge_profile_session_name                  (string; default "")
#   bridge_profile_auto_respond_destructive_ops
#   bridge_profile_auto_respond_timeout_s
#   bridge_profile_prompt_overrides_count
#
# Exit codes (return):
#   0  loaded
#   1  bad file or bad JSON or missing required field
#   2  jq not available

profile_load() {
  local path="$1"
  if [[ -z "$path" ]]; then
    echo "[bridge:profile] empty path" >&2
    return 1
  fi
  if [[ ! -f "$path" ]]; then
    echo "[bridge:profile] file not found: $path" >&2
    return 1
  fi
  if ! command -v jq >/dev/null 2>&1; then
    echo "[bridge:profile] jq not on PATH (install: brew install jq)" >&2
    return 2
  fi
  if ! jq -e . "$path" >/dev/null 2>&1; then
    echo "[bridge:profile] invalid JSON: $path" >&2
    return 1
  fi

  bridge_profile_name="$(jq -r '.name // ""' "$path")"
  bridge_profile_model="$(jq -r '.model // ""' "$path")"
  bridge_profile_allowed_tools_csv="$(jq -r '.allowed_tools // [] | join(",")' "$path")"
  bridge_profile_permission_mode="$(jq -r '.permission_mode // ""' "$path")"
  bridge_profile_stream_output="$(jq -r '.stream_output // false | tostring' "$path")"
  bridge_profile_session_name="$(jq -r '.session_name // ""' "$path")"
  bridge_profile_auto_respond_destructive_ops="$(jq -r '.auto_respond.destructive_ops // false' "$path")"
  bridge_profile_auto_respond_timeout_s="$(jq -r '.auto_respond.timeout_s // 600' "$path")"
  bridge_profile_prompt_overrides_count="$(jq -r '.prompt_overrides // [] | length' "$path")"

  if [[ -z "$bridge_profile_name" ]]; then
    echo "[bridge:profile] missing required field: name (in $path)" >&2
    return 1
  fi

  # Validate permission_mode against claude CLI's accepted choices when set.
  # Empty string means "let driver/CLI decide" (back-compat with v1 profiles).
  case "$bridge_profile_permission_mode" in
    ""|plan|default|acceptEdits|bypassPermissions|auto|dontAsk) ;;
    *)
      echo "[bridge:profile] invalid permission_mode '$bridge_profile_permission_mode' (in $path)" >&2
      echo "[bridge:profile] valid: plan, default, acceptEdits, bypassPermissions, auto, dontAsk" >&2
      return 1
      ;;
  esac

  # v0.3: Validate stream_output is boolean (true | false). jq's `tostring`
  # converts JSON true/false → "true"/"false". A non-bool value (e.g. "not-a-bool"
  # in JSON) would round-trip as that exact string, which we reject here.
  # JSON null is handled by the `// false` default — never reaches this check.
  case "$bridge_profile_stream_output" in
    true|false) ;;
    *)
      echo "[bridge:profile] invalid stream_output '$bridge_profile_stream_output' (in $path) — must be boolean (true | false)" >&2
      return 1
      ;;
  esac

  # v0.5: Validate session_name. Empty is valid (default). When set, must match
  # [a-zA-Z0-9._-]+ (safe for tmux session names, no shell metachars), max 32 chars.
  if [[ -n "$bridge_profile_session_name" ]]; then
    if [[ ${#bridge_profile_session_name} -gt 32 ]]; then
      echo "[bridge:profile] invalid session_name (in $path) — max 32 chars (got ${#bridge_profile_session_name})" >&2
      return 1
    fi
    if [[ ! "$bridge_profile_session_name" =~ ^[a-zA-Z0-9._-]+$ ]]; then
      echo "[bridge:profile] invalid session_name '$bridge_profile_session_name' (in $path) — must match [a-zA-Z0-9._-]+ (no shell metachars)" >&2
      return 1
    fi
  fi

  export bridge_profile_name bridge_profile_model bridge_profile_allowed_tools_csv \
         bridge_profile_permission_mode \
         bridge_profile_stream_output \
         bridge_profile_session_name \
         bridge_profile_auto_respond_destructive_ops \
         bridge_profile_auto_respond_timeout_s \
         bridge_profile_prompt_overrides_count
  return 0
}

# When sourced, no-op. When executed directly, dump the loaded fields (debug):
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 PROFILE_JSON_PATH" >&2
    exit 10
  fi
  profile_load "$1" || exit $?
  printf 'name=%s\nmodel=%s\nallowed_tools=%s\npermission_mode=%s\nstream_output=%s\nsession_name=%s\nauto_respond.destructive_ops=%s\nauto_respond.timeout_s=%s\nprompt_overrides_count=%s\n' \
    "$bridge_profile_name" \
    "$bridge_profile_model" \
    "$bridge_profile_allowed_tools_csv" \
    "$bridge_profile_permission_mode" \
    "$bridge_profile_stream_output" \
    "$bridge_profile_session_name" \
    "$bridge_profile_auto_respond_destructive_ops" \
    "$bridge_profile_auto_respond_timeout_s" \
    "$bridge_profile_prompt_overrides_count"
fi
