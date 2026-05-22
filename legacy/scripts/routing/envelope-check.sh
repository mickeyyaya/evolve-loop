#!/bin/bash
# envelope-check.sh — Validate a (cli, tier) pair against a profile's envelope.
#
# Target path: legacy/scripts/routing/envelope-check.sh
# Bash 3.2 compatible. No declare -A, no mapfile, no GNU-only flags.
#
# Usage:
#   envelope-check.sh <profile_path> <cli> <tier>
#   envelope-check.sh --cross-family <builder_cli> <auditor_cli>
#   envelope-check.sh --self-test
#
# Exit codes:
#   0 = within envelope (allowed)
#   2 = out of envelope (denied)
#   3 = invalid arguments
#   4 = cross-family violation
#   5 = tier-map.json missing or malformed

set -uo pipefail

# Tier ordering (low → high). Used for min/max comparisons.
TIER_FAST=0
TIER_BALANCED=1
TIER_DEEP=2

tier_rank() {
  case "$1" in
    fast)     echo "$TIER_FAST"     ;;
    balanced) echo "$TIER_BALANCED" ;;
    deep)     echo "$TIER_DEEP"     ;;
    *)        echo "-1"             ;;
  esac
}

# Resolve tier-map.json — search canonical locations.
find_tier_map() {
  local candidates=(
    "${EVOLVE_TIER_MAP_PATH:-}"
    "${EVOLVE_PROJECT_ROOT:-.}/legacy/scripts/routing/tier-map.json"
    "legacy/scripts/routing/tier-map.json"
  )
  for c in "${candidates[@]}"; do
    [[ -n "$c" && -f "$c" ]] && { echo "$c"; return 0; }
  done
  return 1
}

# Get a CLI's family from tier-map.json
cli_family() {
  local cli="$1"
  local map_path
  map_path=$(find_tier_map) || { echo "ERROR: tier-map.json not found" >&2; return 5; }
  jq -r --arg cli "$cli" '.families[$cli] // "unknown"' "$map_path"
}

# Get a phase's envelope from its profile JSON
read_envelope() {
  local profile_path="$1"
  local field="$2"  # min | default | max
  jq -r --arg f "$field" '.model_tier_envelope[$f] // empty' "$profile_path"
}

# Get a phase's allowed_clis
read_allowed_clis() {
  local profile_path="$1"
  jq -r '.allowed_clis[]? // empty' "$profile_path"
}

# Validate (cli, tier) ∈ profile envelope
check_envelope() {
  local profile_path="$1"
  local cli="$2"
  local tier="$3"

  if [[ ! -f "$profile_path" ]]; then
    echo "ERROR: profile not found: $profile_path" >&2
    return 3
  fi

  # Apply backward-compat defaults if envelope absent
  local min_tier default_tier max_tier
  min_tier=$(read_envelope "$profile_path" min)
  default_tier=$(read_envelope "$profile_path" default)
  max_tier=$(read_envelope "$profile_path" max)

  if [[ -z "$min_tier" ]]; then
    # Fallback: profile has no envelope field. Use the legacy model_tier_default.
    local legacy
    legacy=$(jq -r '.model_tier_default // "balanced"' "$profile_path")
    # Map opus/sonnet/haiku → deep/balanced/fast
    case "$legacy" in
      opus)   default_tier="deep"     ;;
      sonnet) default_tier="balanced" ;;
      haiku)  default_tier="fast"     ;;
      *)      default_tier="$legacy"  ;;
    esac
    min_tier="balanced"  # conservative floor for legacy profiles
    max_tier="deep"      # conservative ceiling
  fi

  # Validate tier within [min, max]
  local rank_tier rank_min rank_max
  rank_tier=$(tier_rank "$tier")
  rank_min=$(tier_rank "$min_tier")
  rank_max=$(tier_rank "$max_tier")

  if [[ "$rank_tier" -lt 0 ]]; then
    echo "DENY: unknown tier '$tier' (must be fast|balanced|deep)" >&2
    return 2
  fi

  if [[ "$rank_tier" -lt "$rank_min" ]]; then
    echo "DENY: tier '$tier' below envelope min '$min_tier' for $(basename "$profile_path" .json)" >&2
    return 2
  fi

  if [[ "$rank_tier" -gt "$rank_max" ]]; then
    echo "DENY: tier '$tier' above envelope max '$max_tier' for $(basename "$profile_path" .json)" >&2
    return 2
  fi

  # Validate cli in allowed_clis
  local allowed_clis_list
  allowed_clis_list=$(read_allowed_clis "$profile_path")
  if [[ -n "$allowed_clis_list" ]]; then
    if ! echo "$allowed_clis_list" | grep -qx "$cli"; then
      # Special case: "all" wildcard
      if ! echo "$allowed_clis_list" | grep -qx "all"; then
        echo "DENY: cli '$cli' not in allowed_clis for $(basename "$profile_path" .json)" >&2
        return 2
      fi
    fi
  fi

  return 0
}

# Cross-family invariant check: Auditor's family must differ from Builder's
check_cross_family() {
  local builder_cli="$1"
  local auditor_cli="$2"

  local builder_family auditor_family
  builder_family=$(cli_family "$builder_cli")
  auditor_family=$(cli_family "$auditor_cli")

  if [[ "$builder_family" == "$auditor_family" ]]; then
    if [[ "$builder_family" == "unknown" ]]; then
      echo "WARN: unknown family for cli '$builder_cli' or '$auditor_cli' — cross-family check inconclusive" >&2
      # WARN not DENY: degraded mode for unknown CLIs
      return 0
    fi
    echo "DENY: cross-family violation — builder.cli='$builder_cli' and auditor.cli='$auditor_cli' both family='$builder_family'" >&2
    return 4
  fi
  return 0
}

# Self-test: validate all profiles have parseable envelopes
self_test() {
  local profile_dir="${EVOLVE_PROFILE_DIR:-.evolve/profiles}"
  local pass=0
  local fail=0
  local warn=0

  if [[ ! -d "$profile_dir" ]]; then
    echo "ERROR: profile directory not found: $profile_dir" >&2
    return 3
  fi

  echo "=== envelope-check.sh --self-test ==="
  echo "Profile dir: $profile_dir"
  echo ""

  for profile in "$profile_dir"/*.json; do
    [[ -f "$profile" ]] || continue
    local name
    name=$(basename "$profile" .json)
    local min_v default_v max_v
    min_v=$(read_envelope "$profile" min)
    default_v=$(read_envelope "$profile" default)
    max_v=$(read_envelope "$profile" max)

    if [[ -z "$min_v" ]]; then
      printf "  %-15s WARN  (no envelope field — using legacy fallback)\n" "$name"
      warn=$((warn + 1))
      continue
    fi

    # Verify min <= default <= max
    local r_min r_def r_max
    r_min=$(tier_rank "$min_v")
    r_def=$(tier_rank "$default_v")
    r_max=$(tier_rank "$max_v")
    if [[ "$r_min" -le "$r_def" && "$r_def" -le "$r_max" ]]; then
      printf "  %-15s PASS  envelope=[%s, %s, %s]\n" "$name" "$min_v" "$default_v" "$max_v"
      pass=$((pass + 1))
    else
      printf "  %-15s FAIL  envelope=[%s, %s, %s] (must be min<=default<=max)\n" "$name" "$min_v" "$default_v" "$max_v"
      fail=$((fail + 1))
    fi
  done

  echo ""
  echo "PASS: $pass  WARN: $warn  FAIL: $fail"
  if [[ "$fail" -gt 0 ]]; then return 2; fi
  return 0
}

# Main argument dispatch
main() {
  if [[ $# -lt 1 ]]; then
    echo "Usage: envelope-check.sh <profile_path> <cli> <tier>" >&2
    echo "       envelope-check.sh --cross-family <builder_cli> <auditor_cli>" >&2
    echo "       envelope-check.sh --self-test" >&2
    return 3
  fi

  case "$1" in
    --self-test)
      self_test
      ;;
    --cross-family)
      [[ $# -eq 3 ]] || { echo "Usage: envelope-check.sh --cross-family <builder_cli> <auditor_cli>" >&2; return 3; }
      check_cross_family "$2" "$3"
      ;;
    *)
      [[ $# -eq 3 ]] || { echo "Usage: envelope-check.sh <profile_path> <cli> <tier>" >&2; return 3; }
      check_envelope "$1" "$2" "$3"
      ;;
  esac
}

main "$@"
