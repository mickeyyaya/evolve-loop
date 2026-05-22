#!/bin/bash
# decide-cycle-routing-test.sh — Test harness for decide-cycle-routing.sh.
#
# Target path: legacy/scripts/routing/decide-cycle-routing-test.sh
# Bash 3.2 compatible.
#
# Runs 6 canned scenarios and asserts cycle-routing.json output matches expectations.
#
# Usage: decide-cycle-routing-test.sh

set -uo pipefail

# Locate decide script
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DECIDE_SCRIPT="$SCRIPT_DIR/decide-cycle-routing.sh"

if [[ ! -x "$DECIDE_SCRIPT" ]]; then
  echo "ERROR: decide-cycle-routing.sh not found or not executable at $DECIDE_SCRIPT" >&2
  exit 3
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

PASS=0
FAIL=0

# Helper: build a synthetic intent.md
make_intent() {
  local out="$1" risk="$2" awn="$3" premises="$4" constraints="$5" interfaces="$6"
  {
    echo "<!-- challenge-token: test -->"
    echo "---"
    echo "awn_class: $awn"
    echo "risk_level: $risk"
    echo "goal: test goal"
    echo "non_goals:"
    echo "  - test non-goal"
    echo "constraints:"
    for i in $(seq 1 "$constraints"); do echo "  - constraint $i"; done
    echo "interfaces:"
    for i in $(seq 1 "$interfaces"); do echo "  - interface $i"; done
    echo "acceptance_checks:"
    echo "  - check: test check"
    echo "    how_verified: programmatic"
    echo "assumptions:"
    echo "  - test assumption"
    echo "challenged_premises:"
    for i in $(seq 1 "$premises"); do
      echo "  - premise: premise $i"
      echo "    challenge: test challenge"
      echo "    proposed_alternative: test alt"
    done
  } > "$out"
}

# Helper: build a synthetic state.json
make_state() {
  local out="$1" failed="$2" fitness_reg="$3" mastery="$4" carryover_high="$5"
  local failed_arr="[]"
  if [[ "$failed" -gt 0 ]]; then
    failed_arr=$(jq -n --arg n "$failed" '[range(0; ($n|tonumber))] | map({fingerprint: "test", phase: "build"})')
  fi
  local carryover_arr="[]"
  if [[ "$carryover_high" -gt 0 ]]; then
    carryover_arr=$(jq -n --arg n "$carryover_high" '[range(0; ($n|tonumber))] | map({id: "T", priority: "HIGH", action: "test"})')
  fi
  jq -n \
    --argjson fa "$failed_arr" \
    --argjson co "$carryover_arr" \
    --arg fr "$fitness_reg" \
    --argjson ms "$mastery" \
    '{
      failedApproaches: $fa,
      fitnessRegression: ($fr == "true"),
      mastery: { level: "novice", consecutiveSuccesses: $ms },
      carryoverTodos: $co
    }' > "$out"
}

# Helper: assert phase has expected tier
assert_phase_tier() {
  local label="$1" routing_file="$2" phase="$3" expected_tier="$4"
  local got
  got=$(jq -r --arg p "$phase" '.phases[$p].tier' "$routing_file")
  if [[ "$got" == "$expected_tier" ]]; then
    printf "  ✓ [%s] %s tier=%s\n" "$label" "$phase" "$got"
    PASS=$((PASS + 1))
  else
    printf "  ✗ [%s] %s expected=%s got=%s\n" "$label" "$phase" "$expected_tier" "$got"
    FAIL=$((FAIL + 1))
  fi
}

run_scenario() {
  local name="$1" intent="$2" state="$3"
  local routing="$TMP_DIR/routing-$name.json"
  EVOLVE_PROFILE_DIR="$TMP_DIR/no-profiles" bash "$DECIDE_SCRIPT" --dry-run "$intent" "$state" > /dev/null 2>&1
  # Re-run capturing routing output via generate-style
  # Since --dry-run writes to /dev/stdout, we need a different path. Use a synthetic workspace.
  mkdir -p "$TMP_DIR/workspace-$name"
  cp "$intent" "$TMP_DIR/workspace-$name/intent.md"
  EVOLVE_PROJECT_ROOT="$TMP_DIR" EVOLVE_PROFILE_DIR="$TMP_DIR/no-profiles" bash "$DECIDE_SCRIPT" 99 "$TMP_DIR/workspace-$name" > /dev/null 2>&1
  # Override state path for test isolation
  mkdir -p "$TMP_DIR/.evolve"
  cp "$state" "$TMP_DIR/.evolve/state.json"
  EVOLVE_PROJECT_ROOT="$TMP_DIR" EVOLVE_PROFILE_DIR="$TMP_DIR/no-profiles" bash "$DECIDE_SCRIPT" 99 "$TMP_DIR/workspace-$name"
  mv "$TMP_DIR/workspace-$name/cycle-routing.json" "$routing"
  echo "$routing"
}

echo "=== decide-cycle-routing.sh test harness ==="
echo ""

# Scenario 1: low-risk CLEAR goal, no failures, high mastery → scout/triage at fast
echo "Scenario 1: low-risk CLEAR + mastery streak → downshift scout/triage to fast"
make_intent "$TMP_DIR/i1.md" low CLEAR 0 1 1
make_state  "$TMP_DIR/s1.json" 0 false 6 0
r=$(run_scenario s1 "$TMP_DIR/i1.md" "$TMP_DIR/s1.json")
assert_phase_tier S1 "$r" scout    fast
assert_phase_tier S1 "$r" triage   fast
assert_phase_tier S1 "$r" builder  balanced
assert_phase_tier S1 "$r" auditor  deep
echo ""

# Scenario 2: critical risk → builder + tdd-engineer + auditor at deep
echo "Scenario 2: critical risk → safety phases at deep"
make_intent "$TMP_DIR/i2.md" critical IMR 5 8 3
make_state  "$TMP_DIR/s2.json" 0 false 0 0
r=$(run_scenario s2 "$TMP_DIR/i2.md" "$TMP_DIR/s2.json")
assert_phase_tier S2 "$r" builder       deep
assert_phase_tier S2 "$r" tdd-engineer  deep
assert_phase_tier S2 "$r" auditor       deep
assert_phase_tier S2 "$r" memo          fast
echo ""

# Scenario 3: high uncertainty AwN → plan-reviewer at deep
echo "Scenario 3: AwN IMKI → plan-reviewer at deep"
make_intent "$TMP_DIR/i3.md" medium IMKI 1 2 2
make_state  "$TMP_DIR/s3.json" 0 false 0 0
r=$(run_scenario s3 "$TMP_DIR/i3.md" "$TMP_DIR/s3.json")
assert_phase_tier S3 "$r" plan-reviewer deep
echo ""

# Scenario 4: many challenged premises → retrospective pre-bumped
echo "Scenario 4: 3+ premises → retrospective at deep"
make_intent "$TMP_DIR/i4.md" medium CLEAR 4 2 2
make_state  "$TMP_DIR/s4.json" 0 false 0 0
r=$(run_scenario s4 "$TMP_DIR/i4.md" "$TMP_DIR/s4.json")
assert_phase_tier S4 "$r" retrospective deep
echo ""

# Scenario 5: many interfaces → builder at deep
echo "Scenario 5: 4+ interfaces → builder at deep"
make_intent "$TMP_DIR/i5.md" medium CLEAR 1 2 6
make_state  "$TMP_DIR/s5.json" 0 false 0 0
r=$(run_scenario s5 "$TMP_DIR/i5.md" "$TMP_DIR/s5.json")
assert_phase_tier S5 "$r" builder deep
echo ""

# Scenario 6: prior failures → builder + tdd-engineer at deep
echo "Scenario 6: failedApproaches[] non-empty → builder + tdd at deep"
make_intent "$TMP_DIR/i6.md" medium CLEAR 1 2 2
make_state  "$TMP_DIR/s6.json" 3 false 0 0
r=$(run_scenario s6 "$TMP_DIR/i6.md" "$TMP_DIR/s6.json")
assert_phase_tier S6 "$r" builder      deep
assert_phase_tier S6 "$r" tdd-engineer deep
echo ""

echo "============================================"
echo "PASS: $PASS  FAIL: $FAIL"
if [[ "$FAIL" -gt 0 ]]; then exit 1; fi
exit 0
