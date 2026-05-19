#!/usr/bin/env bash
# AC-ID: cycle-88-phase-registry-intent-to-discover
#
# Verifies Cycle B kernel edit on docs/architecture/phase-registry.json:
#   1. File validates as JSON.
#   2. intent phase's gate_out == "gate_intent_to_discover" (was
#      "gate_intent_to_research" pre-cycle).
#   3. scout phase's gate_in == "gate_intent_to_discover" (was
#      "gate_research_to_discover" pre-cycle).
#   4. No phase entry has `name == "research"` (no dedicated research phase in
#      the registry; intent → scout is now direct).
#   5. NO phase entry anywhere in the registry references `gate_intent_to_research`
#      or `gate_research_to_discover` as a gate (no dead pointers).
#   6. Phase ordering still includes "intent" before "scout" (intent → scout
#      adjacency preserved — non-trivial assertion that mutants reordering or
#      dropping intent will fail).
#
# Behavioral: actually parses the JSON, checks structural relationships, not
# just grep for tokens. Mutants that "rename only intent.gate_out" but leave
# scout.gate_in pointing at the retired gate fail (3); mutants that delete the
# scout entry entirely fail (6).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
REG_FILE="$REPO_ROOT/docs/architecture/phase-registry.json"

if [ ! -f "$REG_FILE" ]; then
  echo "RED cycle-88-phase-registry-intent-to-discover: phase-registry.json missing at $REG_FILE"
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "RED cycle-88-phase-registry-intent-to-discover: jq not available (required for JSON predicate)"
  exit 1
fi

fail=0
errors=""

# (1) Valid JSON.
if ! jq -e . "$REG_FILE" >/dev/null 2>&1; then
  errors="${errors}\n  phase-registry.json is NOT valid JSON"
  fail=$((fail + 1))
  # JSON-invalid → bail; the remaining checks would error noisily.
  echo "RED cycle-88-phase-registry-intent-to-discover: $fail issue(s)"
  printf "%b\n" "$errors" >&2
  exit 1
fi

# (2) intent.gate_out == gate_intent_to_discover.
intent_gate_out=$(jq -r '(.phases[] | select(.name == "intent") | .gate_out) // empty' "$REG_FILE")
if [ "$intent_gate_out" != "gate_intent_to_discover" ]; then
  errors="${errors}\n  intent.gate_out is '$intent_gate_out' (expected 'gate_intent_to_discover')"
  fail=$((fail + 1))
fi

# (3) scout.gate_in == gate_intent_to_discover.
scout_gate_in=$(jq -r '(.phases[] | select(.name == "scout") | .gate_in) // empty' "$REG_FILE")
if [ "$scout_gate_in" != "gate_intent_to_discover" ]; then
  errors="${errors}\n  scout.gate_in is '$scout_gate_in' (expected 'gate_intent_to_discover')"
  fail=$((fail + 1))
fi

# (4) No phase entry named "research".
research_count=$(jq -r '[.phases[] | select(.name == "research")] | length' "$REG_FILE")
if [ "${research_count:-0}" -ne 0 ] 2>/dev/null; then
  errors="${errors}\n  phase-registry has $research_count 'research' phase entries (must be 0)"
  fail=$((fail + 1))
fi

# (5) No dead-pointer gate references anywhere in the registry.
dead_intent=$(jq -r '[.phases[] | select((.gate_in == "gate_intent_to_research") or (.gate_out == "gate_intent_to_research"))] | length' "$REG_FILE")
dead_research=$(jq -r '[.phases[] | select((.gate_in == "gate_research_to_discover") or (.gate_out == "gate_research_to_discover"))] | length' "$REG_FILE")
if [ "${dead_intent:-0}" -gt 0 ] 2>/dev/null; then
  errors="${errors}\n  phase-registry still has $dead_intent reference(s) to retired 'gate_intent_to_research'"
  fail=$((fail + 1))
fi
if [ "${dead_research:-0}" -gt 0 ] 2>/dev/null; then
  errors="${errors}\n  phase-registry still has $dead_research reference(s) to retired 'gate_research_to_discover'"
  fail=$((fail + 1))
fi

# (6) intent → scout adjacency preserved.
intent_idx=$(jq -r '.phases | map(.name) | index("intent")' "$REG_FILE")
scout_idx=$(jq -r '.phases | map(.name) | index("scout")' "$REG_FILE")
if [ "$intent_idx" = "null" ] || [ -z "$intent_idx" ]; then
  errors="${errors}\n  phase-registry has no 'intent' phase entry"
  fail=$((fail + 1))
elif [ "$scout_idx" = "null" ] || [ -z "$scout_idx" ]; then
  errors="${errors}\n  phase-registry has no 'scout' phase entry"
  fail=$((fail + 1))
else
  if [ "$((scout_idx))" -le "$((intent_idx))" ] 2>/dev/null; then
    errors="${errors}\n  phase-registry ordering broken: intent index=$intent_idx, scout index=$scout_idx (scout must follow intent)"
    fail=$((fail + 1))
  fi
fi

if [ $fail -gt 0 ]; then
  echo "RED cycle-88-phase-registry-intent-to-discover: $fail issue(s)"
  printf "%b\n" "$errors" >&2
  exit 1
fi
echo "GREEN cycle-88-phase-registry-intent-to-discover: registry routes intent→discover directly via gate_intent_to_discover; no dead gate pointers; ordering preserved"
exit 0
