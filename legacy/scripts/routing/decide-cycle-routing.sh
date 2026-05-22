#!/bin/bash
# decide-cycle-routing.sh — Deterministic per-cycle model routing decision.
#
# Target path: legacy/scripts/routing/decide-cycle-routing.sh
# Bash 3.2 compatible. Reads intent.md + state.json + recent retros.
# Writes cycle-routing.json constrained by profile envelopes.
#
# Usage:
#   decide-cycle-routing.sh <cycle> <workspace>
#   decide-cycle-routing.sh --dry-run <intent_path> <state_path>
#   decide-cycle-routing.sh --validate <cycle-routing.json>
#
# Exit codes:
#   0 = success
#   2 = inputs missing or malformed
#   3 = profile envelope violation (should not happen — we clamp)
#   4 = invalid arguments

set -uo pipefail

# Tier ordering
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

tier_name() {
  case "$1" in
    0) echo "fast"     ;;
    1) echo "balanced" ;;
    2) echo "deep"     ;;
    *) echo ""         ;;
  esac
}

# Max of two tier names
tier_max() {
  local r1 r2
  r1=$(tier_rank "$1")
  r2=$(tier_rank "$2")
  if [[ "$r1" -ge "$r2" ]]; then echo "$1"; else echo "$2"; fi
}

# Min of two tier names
tier_min() {
  local r1 r2
  r1=$(tier_rank "$1")
  r2=$(tier_rank "$2")
  if [[ "$r1" -le "$r2" ]]; then echo "$1"; else echo "$2"; fi
}

# Clamp tier to envelope [min, max]
tier_clamp() {
  local tier="$1"
  local env_min="$2"
  local env_max="$3"
  tier=$(tier_max "$tier" "$env_min")
  tier=$(tier_min "$tier" "$env_max")
  echo "$tier"
}

# Read intent.md frontmatter as JSON via embedded YAML→JSON conversion using jq.
# intent.md format: YAML frontmatter delimited by --- markers.
parse_intent() {
  local intent_path="$1"
  if [[ ! -f "$intent_path" ]]; then
    echo "{}"
    return
  fi
  # Extract YAML frontmatter (between --- markers); convert simple keys to JSON via jq.
  # Since bash 3.2 + jq cannot parse arbitrary YAML, we extract specific fields via grep.
  local risk_level awn_class
  risk_level=$(grep -m1 '^risk_level:' "$intent_path" 2>/dev/null | sed 's/^risk_level:[[:space:]]*//; s/[[:space:]]*$//')
  awn_class=$(grep -m1 '^awn_class:' "$intent_path" 2>/dev/null | sed 's/^awn_class:[[:space:]]*//; s/[[:space:]]*$//')

  # Count list-style fields by counting lines starting with "  - " under each section.
  local n_premises n_constraints n_interfaces n_non_goals
  n_premises=$(awk '/^challenged_premises:/{p=1;next} /^[a-z_]+:/{p=0} p && /^  - /{c++} END{print c+0}' "$intent_path" 2>/dev/null)
  n_constraints=$(awk '/^constraints:/{p=1;next} /^[a-z_]+:/{p=0} p && /^  - /{c++} END{print c+0}' "$intent_path" 2>/dev/null)
  n_interfaces=$(awk '/^interfaces:/{p=1;next} /^[a-z_]+:/{p=0} p && /^  - /{c++} END{print c+0}' "$intent_path" 2>/dev/null)
  n_non_goals=$(awk '/^non_goals:/{p=1;next} /^[a-z_]+:/{p=0} p && /^  - /{c++} END{print c+0}' "$intent_path" 2>/dev/null)

  cat <<EOF
{
  "risk_level": "${risk_level:-medium}",
  "awn_class": "${awn_class:-CLEAR}",
  "challenged_premises_count": ${n_premises:-0},
  "constraints_count": ${n_constraints:-0},
  "interfaces_count": ${n_interfaces:-0},
  "non_goals_count": ${n_non_goals:-0}
}
EOF
}

# Read routing-relevant signals from state.json
parse_state() {
  local state_path="$1"
  if [[ ! -f "$state_path" ]]; then
    echo "{}"
    return
  fi
  jq -c '{
    failed_approaches_count: (.failedApproaches // [] | length),
    fitness_regression: (.fitnessRegression // false),
    mastery_streak: (.mastery.consecutiveSuccesses // 0),
    carryover_high_count: (.carryoverTodos // [] | map(select(.priority == "HIGH")) | length)
  }' "$state_path"
}

# Default routing per phase (capability tier)
default_tier_for_phase() {
  case "$1" in
    memo|evaluator)                            echo "fast"     ;;
    triage)                                    echo "balanced" ;;  # default within envelope [fast, balanced]
    orchestrator|scout|builder|tester|inspirer) echo "balanced" ;;
    intent|plan-reviewer|tdd-engineer|auditor|retrospective) echo "deep" ;;
    *) echo "balanced" ;;
  esac
}

# Read envelope min/max for a phase from profile JSON
phase_envelope_min() {
  local phase="$1"
  local profile_dir="${EVOLVE_PROFILE_DIR:-.evolve/profiles}"
  local profile="$profile_dir/$phase.json"
  if [[ -f "$profile" ]]; then
    jq -r '.model_tier_envelope.min // "balanced"' "$profile"
  else
    echo "balanced"
  fi
}

phase_envelope_max() {
  local phase="$1"
  local profile_dir="${EVOLVE_PROFILE_DIR:-.evolve/profiles}"
  local profile="$profile_dir/$phase.json"
  if [[ -f "$profile" ]]; then
    jq -r '.model_tier_envelope.max // "deep"' "$profile"
  else
    echo "deep"
  fi
}

# Pick CLI for a phase, honoring cross_family_with constraint
pick_cli_for_phase() {
  local phase="$1"
  local builder_cli="${2:-}"
  local profile_dir="${EVOLVE_PROFILE_DIR:-.evolve/profiles}"
  local profile="$profile_dir/$phase.json"

  # Get allowed_clis list
  local allowed
  if [[ -f "$profile" ]]; then
    allowed=$(jq -r '.allowed_clis[]? // empty' "$profile")
  fi

  # Default: prefer claude (most-supported); fall back to first allowed.
  local preferred="claude"

  # If allowed_clis is restricted (not "all"), pick first in list (other than builder's family for auditor)
  if [[ -n "$allowed" && "$(echo "$allowed" | head -1)" != "all" ]]; then
    preferred=$(echo "$allowed" | head -1)
  fi

  # Cross-family enforcement: if this phase has cross_family_with constraint, exclude builder's family
  local cf_with=""
  if [[ -f "$profile" ]]; then
    cf_with=$(jq -r '.cross_family_with // empty' "$profile")
  fi
  if [[ -n "$cf_with" && -n "$builder_cli" ]]; then
    # If our preferred CLI is same family as builder, try alternatives
    local builder_family
    builder_family=$(map_cli_to_family "$builder_cli")
    local preferred_family
    preferred_family=$(map_cli_to_family "$preferred")
    if [[ "$builder_family" == "$preferred_family" ]]; then
      # Try alternatives in allowed list
      local alt
      if [[ -n "$allowed" ]]; then
        for alt in $allowed; do
          [[ "$alt" == "all" ]] && continue
          local alt_family
          alt_family=$(map_cli_to_family "$alt")
          if [[ "$alt_family" != "$builder_family" ]]; then
            preferred="$alt"
            break
          fi
        done
      fi
      # If still same family, fall back to gemini (most-supported 3P)
      if [[ "$(map_cli_to_family "$preferred")" == "$builder_family" ]]; then
        preferred="gemini"
      fi
    fi
  fi

  echo "$preferred"
}

map_cli_to_family() {
  local map_path="${EVOLVE_PROJECT_ROOT:-.}/legacy/scripts/routing/tier-map.json"
  if [[ -f "$map_path" ]]; then
    jq -r --arg cli "$1" '.families[$cli] // "unknown"' "$map_path"
  else
    case "$1" in
      claude) echo "anthropic" ;;
      gemini) echo "google"    ;;
      codex)  echo "openai"    ;;
      grok)   echo "xai"       ;;
      *)      echo "unknown"   ;;
    esac
  fi
}

# Apply signal-driven adjustments to a phase's tier
adjust_tier_for_phase() {
  local phase="$1"
  local base_tier="$2"
  local intent_json="$3"
  local state_json="$4"

  local result="$base_tier"

  local risk awn n_premises n_constraints n_interfaces
  risk=$(echo "$intent_json" | jq -r '.risk_level // "medium"')
  awn=$(echo "$intent_json" | jq -r '.awn_class // "CLEAR"')
  n_premises=$(echo "$intent_json" | jq -r '.challenged_premises_count // 0')
  n_constraints=$(echo "$intent_json" | jq -r '.constraints_count // 0')
  n_interfaces=$(echo "$intent_json" | jq -r '.interfaces_count // 0')

  local n_failed fitness_reg mastery_streak carryover_high
  n_failed=$(echo "$state_json" | jq -r '.failed_approaches_count // 0')
  fitness_reg=$(echo "$state_json" | jq -r '.fitness_regression // false')
  mastery_streak=$(echo "$state_json" | jq -r '.mastery_streak // 0')
  carryover_high=$(echo "$state_json" | jq -r '.carryover_high_count // 0')

  # Apply rules per phase
  case "$phase" in
    builder|tdd-engineer)
      # Bump on critical risk, prior failures, high carryover, many interfaces
      [[ "$risk" == "critical" || "$risk" == "high" ]] && result=$(tier_max "$result" "deep")
      [[ "$n_interfaces" -ge 4 ]] && result=$(tier_max "$result" "deep")
      [[ "$carryover_high" -ge 2 ]] && result=$(tier_max "$result" "deep")
      [[ "$n_failed" -gt 0 ]] && result=$(tier_max "$result" "deep")
      ;;
    auditor)
      # Already at deep by default; bump if fitness regression
      [[ "$fitness_reg" == "true" ]] && result=$(tier_max "$result" "deep")
      ;;
    plan-reviewer)
      # Bump on high-uncertainty intent
      case "$awn" in
        IMKI|IMR|IBTC|PMU) result=$(tier_max "$result" "deep") ;;
      esac
      ;;
    retrospective)
      # Pre-bump on high challenged-premises (failure-likely)
      [[ "$n_premises" -ge 3 ]] && result=$(tier_max "$result" "deep")
      ;;
    scout|triage)
      # Downshift on easy + CLEAR + no failures + mastery
      if [[ "$risk" == "low" && "$awn" == "CLEAR" && "$n_failed" -eq 0 && "$mastery_streak" -ge 5 ]]; then
        result=$(tier_min "$result" "fast")
      fi
      ;;
    memo|evaluator)
      # Always at min (fast); no upward pressure
      ;;
  esac

  echo "$result"
}

# Generate cycle-routing.json
generate_routing() {
  local cycle="$1"
  local intent_path="$2"
  local state_path="$3"
  local output_path="$4"

  local intent_json state_json
  intent_json=$(parse_intent "$intent_path")
  state_json=$(parse_state "$state_path")

  local decided_at
  decided_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  # Build phase decisions in dependency order: builder first (so auditor can read its cli)
  local phases="memo evaluator triage orchestrator scout intent builder tester inspirer plan-reviewer tdd-engineer auditor retrospective"

  local builder_cli=""
  local result='{"cycle":'"$cycle"',"decided_at":"'"$decided_at"'","decision_source":"deterministic","phases":{}}'

  local phase
  for phase in $phases; do
    local base_tier env_min env_max chosen_tier rationale chosen_cli
    base_tier=$(default_tier_for_phase "$phase")
    env_min=$(phase_envelope_min "$phase")
    env_max=$(phase_envelope_max "$phase")
    chosen_tier=$(adjust_tier_for_phase "$phase" "$base_tier" "$intent_json" "$state_json")
    chosen_tier=$(tier_clamp "$chosen_tier" "$env_min" "$env_max")

    # Pick CLI (auditor sees builder_cli for cross-family enforcement)
    if [[ "$phase" == "auditor" ]]; then
      chosen_cli=$(pick_cli_for_phase "$phase" "$builder_cli")
    else
      chosen_cli=$(pick_cli_for_phase "$phase")
    fi

    # Capture builder's CLI for later cross-family check
    [[ "$phase" == "builder" ]] && builder_cli="$chosen_cli"

    # Build rationale string
    rationale="default"
    if [[ "$chosen_tier" != "$base_tier" ]]; then
      rationale="adjusted by signals"
    fi
    if [[ "$phase" == "auditor" && -n "$builder_cli" ]]; then
      if [[ "$chosen_cli" != "claude" ]]; then
        rationale="cross_family_with=builder forced different CLI"
      fi
    fi

    # Append to result JSON via jq
    result=$(echo "$result" | jq --arg p "$phase" --arg c "$chosen_cli" --arg t "$chosen_tier" --arg r "$rationale" \
      '.phases[$p] = {cli: $c, tier: $t, rationale: $r}')
  done

  # Atomic write
  local tmp="${output_path}.tmp.$$"
  echo "$result" | jq '.' > "$tmp"
  mv "$tmp" "$output_path"
  echo "[decide-cycle-routing] wrote $output_path" >&2
}

# Main argument dispatch
main() {
  if [[ $# -lt 1 ]]; then
    cat <<EOF >&2
Usage:
  decide-cycle-routing.sh <cycle> <workspace>
  decide-cycle-routing.sh --dry-run <intent_path> <state_path>
  decide-cycle-routing.sh --validate <cycle-routing.json>
EOF
    return 4
  fi

  case "$1" in
    --dry-run)
      [[ $# -eq 3 ]] || { echo "Usage: --dry-run <intent_path> <state_path>" >&2; return 4; }
      local intent_json state_json
      intent_json=$(parse_intent "$2")
      state_json=$(parse_state "$3")
      echo "intent_signals:"
      echo "$intent_json" | jq '.'
      echo "state_signals:"
      echo "$state_json" | jq '.'
      generate_routing 0 "$2" "$3" "/dev/stdout"
      ;;
    --validate)
      [[ $# -eq 2 ]] || { echo "Usage: --validate <cycle-routing.json>" >&2; return 4; }
      jq -e '.phases | to_entries | map(.value | has("cli") and has("tier"))| all' "$2" >/dev/null && \
        echo "VALID" || { echo "INVALID" >&2; return 2; }
      ;;
    *)
      [[ $# -eq 2 ]] || { echo "Usage: decide-cycle-routing.sh <cycle> <workspace>" >&2; return 4; }
      local cycle="$1"
      local workspace="$2"
      local intent_path="$workspace/intent.md"
      local state_path="${EVOLVE_PROJECT_ROOT:-.}/.evolve/state.json"
      local output_path="$workspace/cycle-routing.json"
      generate_routing "$cycle" "$intent_path" "$state_path" "$output_path"
      ;;
  esac
}

main "$@"
