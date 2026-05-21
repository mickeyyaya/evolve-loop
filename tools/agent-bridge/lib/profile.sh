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
  bridge_profile_auto_respond_destructive_ops="$(jq -r '.auto_respond.destructive_ops // false' "$path")"
  bridge_profile_auto_respond_timeout_s="$(jq -r '.auto_respond.timeout_s // 600' "$path")"
  bridge_profile_prompt_overrides_count="$(jq -r '.prompt_overrides // [] | length' "$path")"

  if [[ -z "$bridge_profile_name" ]]; then
    echo "[bridge:profile] missing required field: name (in $path)" >&2
    return 1
  fi

  export bridge_profile_name bridge_profile_model bridge_profile_allowed_tools_csv \
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
  printf 'name=%s\nmodel=%s\nallowed_tools=%s\nauto_respond.destructive_ops=%s\nauto_respond.timeout_s=%s\nprompt_overrides_count=%s\n' \
    "$bridge_profile_name" \
    "$bridge_profile_model" \
    "$bridge_profile_allowed_tools_csv" \
    "$bridge_profile_auto_respond_destructive_ops" \
    "$bridge_profile_auto_respond_timeout_s" \
    "$bridge_profile_prompt_overrides_count"
fi
