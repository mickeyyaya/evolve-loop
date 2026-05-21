#!/usr/bin/env bash
# lib/manifest-patcher.sh — append a new interactive_prompts entry to a manifest
#
# Used by `bridge add-rule` (P11.5) to ingest an escalation report and turn
# it into a permanent rule for next-run learning.

# manifest_append_rule MANIFEST_PATH NAME REGEX RESPONSE_KEYS POLICY NOTE
#   RESPONSE_KEYS: comma-separated tmux key names, or empty for policy=escalate
#   POLICY: "auto_respond" | "escalate"
manifest_append_rule() {
  local manifest_path="$1"
  local name="$2"
  local regex="$3"
  local response_keys="$4"
  local policy="$5"
  local note="${6:-Added by bridge add-rule}"

  if [[ ! -f "$manifest_path" ]]; then
    echo "[manifest-patch] manifest not found: $manifest_path" >&2
    return 1
  fi
  if [[ -z "$name" || -z "$regex" || -z "$policy" ]]; then
    echo "[manifest-patch] name, regex, policy are required" >&2
    return 1
  fi
  case "$policy" in
    auto_respond|escalate) ;;
    *) echo "[manifest-patch] invalid policy: $policy (want auto_respond|escalate)" >&2; return 1 ;;
  esac
  if [[ "$policy" == "auto_respond" && -z "$response_keys" ]]; then
    echo "[manifest-patch] policy=auto_respond requires non-empty response_keys" >&2
    return 1
  fi

  # Detect duplicates by name
  if jq -e --arg n "$name" '.interactive_prompts // [] | any(.name == $n)' "$manifest_path" >/dev/null; then
    echo "[manifest-patch] rule with name=$name already exists in $manifest_path" >&2
    return 2
  fi

  # Build the new entry (response_keys: null when empty)
  local tmp="${manifest_path}.tmp.$$"
  if [[ -n "$response_keys" ]]; then
    jq --arg name "$name" --arg regex "$regex" --arg keys "$response_keys" \
       --arg policy "$policy" --arg note "$note" \
       '.interactive_prompts |= ((. // []) + [{name: $name, regex: $regex, response_keys: $keys, policy: $policy, note: $note}])' \
       "$manifest_path" > "$tmp"
  else
    jq --arg name "$name" --arg regex "$regex" \
       --arg policy "$policy" --arg note "$note" \
       '.interactive_prompts |= ((. // []) + [{name: $name, regex: $regex, response_keys: null, policy: $policy, note: $note}])' \
       "$manifest_path" > "$tmp"
  fi

  if ! jq -e . "$tmp" >/dev/null 2>&1; then
    rm -f "$tmp"
    echo "[manifest-patch] generated JSON is invalid (jq output corrupt?)" >&2
    return 1
  fi
  mv "$tmp" "$manifest_path"
  echo "[manifest-patch] appended rule '$name' to $manifest_path" >&2
  return 0
}
