#!/usr/bin/env bash
# lib/manifest-loader.sh — load and validate a per-CLI capability manifest
#
# Usage (sourced):
#   . lib/manifest-loader.sh
#   manifest_load "$cli" || return $?
#   echo "$bridge_manifest_binary  $bridge_manifest_prompt_marker"
#
# Expects BRIDGE_LIB_DIR to be set by the caller (bin/bridge sets it).
#
# Manifest schema (v1):
#   {
#     "schema_version": int,
#     "cli": string                          (required)
#     "binary": string                       (required)
#     "binary_min_version": string           (optional)
#     "default_tier": "full|hybrid|degraded|none"
#     "tier_dependencies": { tier: [bin,…] }
#     "prompt_marker": string or null        (REPL marker for capture-pane)
#     "default_model": string
#     "default_args": [string, …]            (passed to underlying CLI)
#     "interactive_prompts": [               (used by lib/auto-respond.sh)
#       {
#         "name": string,
#         "regex": string,                   (extended-regex pattern)
#         "response_keys": string or null,   (null → escalate; string → send-keys)
#         "policy": "auto_respond" | "escalate",
#         "note": string
#       }
#     ]
#     "stub": bool                           (v2-deferred CLIs)
#   }
#
# Exports on success:
#   bridge_manifest_cli
#   bridge_manifest_binary
#   bridge_manifest_binary_min_version
#   bridge_manifest_default_tier
#   bridge_manifest_prompt_marker
#   bridge_manifest_default_model
#   bridge_manifest_stub
#   bridge_manifest_interactive_prompts_count
#   bridge_manifest_path                     (the JSON path itself, for jq drilling)
#
# Return codes:
#   0  loaded
#   1  manifest not found / invalid JSON / missing required fields
#   2  jq missing

manifest_load() {
  local cli="$1"
  if [[ -z "$cli" ]]; then
    echo "[bridge:manifest] empty cli name" >&2
    return 1
  fi
  if [[ -z "${BRIDGE_LIB_DIR:-}" ]]; then
    echo "[bridge:manifest] BRIDGE_LIB_DIR not set (must be exported by caller)" >&2
    return 1
  fi
  local path="${BRIDGE_LIB_DIR}/manifests/${cli}.json"
  if [[ ! -f "$path" ]]; then
    echo "[bridge:manifest] no manifest for cli=$cli (looked at $path)" >&2
    return 1
  fi
  if ! command -v jq >/dev/null 2>&1; then
    echo "[bridge:manifest] jq not on PATH" >&2
    return 2
  fi
  if ! jq -e . "$path" >/dev/null 2>&1; then
    echo "[bridge:manifest] invalid JSON: $path" >&2
    return 1
  fi

  bridge_manifest_cli="$(jq -r '.cli // ""' "$path")"
  bridge_manifest_binary="$(jq -r '.binary // ""' "$path")"
  bridge_manifest_binary_min_version="$(jq -r '.binary_min_version // ""' "$path")"
  bridge_manifest_default_tier="$(jq -r '.default_tier // "none"' "$path")"
  bridge_manifest_prompt_marker="$(jq -r '.prompt_marker // ""' "$path")"
  bridge_manifest_default_model="$(jq -r '.default_model // ""' "$path")"
  bridge_manifest_stub="$(jq -r '.stub // false' "$path")"
  bridge_manifest_interactive_prompts_count="$(jq -r '.interactive_prompts // [] | length' "$path")"
  bridge_manifest_path="$path"

  if [[ -z "$bridge_manifest_cli" || -z "$bridge_manifest_binary" ]]; then
    echo "[bridge:manifest] missing required fields (cli, binary) in $path" >&2
    return 1
  fi

  export bridge_manifest_cli bridge_manifest_binary bridge_manifest_binary_min_version \
         bridge_manifest_default_tier bridge_manifest_prompt_marker \
         bridge_manifest_default_model bridge_manifest_stub \
         bridge_manifest_interactive_prompts_count bridge_manifest_path
  return 0
}

# Standalone debug: bash lib/manifest-loader.sh CLI_NAME
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  if [[ $# -ne 1 ]]; then
    echo "usage: $0 CLI_NAME" >&2
    exit 10
  fi
  # When invoked directly, derive BRIDGE_LIB_DIR from this script's location
  BRIDGE_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  export BRIDGE_LIB_DIR
  manifest_load "$1" || exit $?
  printf 'cli=%s\nbinary=%s\nbinary_min_version=%s\ndefault_tier=%s\nprompt_marker=%s\ndefault_model=%s\nstub=%s\ninteractive_prompts_count=%s\n' \
    "$bridge_manifest_cli" "$bridge_manifest_binary" \
    "$bridge_manifest_binary_min_version" "$bridge_manifest_default_tier" \
    "$bridge_manifest_prompt_marker" "$bridge_manifest_default_model" \
    "$bridge_manifest_stub" "$bridge_manifest_interactive_prompts_count"
fi
