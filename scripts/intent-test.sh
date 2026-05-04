#!/usr/bin/env bash
#
# intent-test.sh — Unit tests for the evolve-intent skill v0.1 (v8.19.0).
#
# Covers:
#   - cycle-state.sh: 'intent' is a valid phase; intent_required field at init
#   - cycle-state.sh accept-intent operator command
#   - phase-gate.sh: gate_calibrate_to_intent + gate_intent_to_research
#   - phase-gate-precondition.sh: scout denied when intent.md missing under
#     EVOLVE_REQUIRE_INTENT=1; allowed when intent ledger entry present
#
# Tests use temp state files (EVOLVE_CYCLE_STATE_FILE) and temp ledgers so
# the real .evolve/* tree is never touched.
#
# Usage: bash scripts/intent-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

unset EVOLVE_BYPASS_PHASE_GATE
unset EVOLVE_REQUIRE_INTENT

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CYCLE_STATE="$REPO_ROOT/scripts/cycle-state.sh"
PHASE_GATE="$REPO_ROOT/scripts/phase-gate.sh"
PRECONDITION="$REPO_ROOT/scripts/guards/phase-gate-precondition.sh"

TEST_DIR=$(mktemp -d -t intent-test.XXXXXX)
TEST_STATE="$TEST_DIR/cycle-state.json"
TEST_LEDGER="$TEST_DIR/ledger.jsonl"
TEST_WORKSPACE="$TEST_DIR/workspace/cycle-99"
mkdir -p "$TEST_WORKSPACE"
trap 'rm -rf "$TEST_DIR"' EXIT
export EVOLVE_CYCLE_STATE_FILE="$TEST_STATE"
export EVOLVE_LEDGER_OVERRIDE="$TEST_LEDGER"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# Reset state between tests
reset_state() {
    rm -f "$TEST_STATE"
    : > "$TEST_LEDGER"
    rm -rf "$TEST_WORKSPACE"
    mkdir -p "$TEST_WORKSPACE"
}

# Compute SHA256 for a file (BSD-shasum compatible)
sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
    else shasum -a 256 "$1" | awk '{print $1}'; fi
}

# Write a sample valid intent.md
write_valid_intent() {
    local path="$1"
    local awn="${2:-IMKI}"
    cat > "$path" <<EOF
---
awn_class: $awn
goal: Add a force-directed graph to visualize node clusters.
non_goals:
  - "do not add zoom/pan"
constraints:
  - "must render under 3 seconds for 100 nodes"
interfaces:
  - "src/components/graph.tsx"
acceptance_checks:
  - check: "user can identify clusters within 3s"
    how_verified: "manual"
assumptions:
  - "data is already structured as nodes/edges"
challenged_premises:
  - premise: "user wants force-directed layout"
    challenge: "tree layout may be clearer for hierarchical data"
    proposed_alternative: "ask user about data shape before deciding"
risk_level: medium
---

# Restated Intent
The user wants to visualize relationships between nodes. They said force-directed,
but tree layout may serve them better depending on data shape.
EOF
}

# Write a ledger entry binding intent.md
write_intent_ledger() {
    local cycle="$1"
    local artifact="$2"
    local sha
    sha=$(sha256_file "$artifact")
    local ts
    ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    cat >> "$TEST_LEDGER" <<EOF
{"ts":"${ts}","cycle":${cycle},"role":"intent","kind":"agent_subprocess","model":"opus","exit_code":0,"duration_s":"60","artifact_path":"${artifact}","artifact_sha256":"${sha}","challenge_token":"test-token","git_head":"none","tree_state_sha":"none"}
EOF
}

[ -f "$CYCLE_STATE" ] || { echo "FATAL: $CYCLE_STATE missing"; exit 1; }
[ -f "$PHASE_GATE" ] || { echo "FATAL: $PHASE_GATE missing"; exit 1; }
[ -f "$PRECONDITION" ] || { echo "FATAL: $PRECONDITION missing"; exit 1; }

# === Test 1: 'intent' is a valid phase value ==================================
header "Test 1: cycle_state_advance accepts 'intent' phase"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
set +e
bash "$CYCLE_STATE" advance intent intent >/dev/null 2>&1
rc=$?
set -e
phase=$(jq -r '.phase' "$TEST_STATE" 2>/dev/null)
[ "$rc" = "0" ] && [ "$phase" = "intent" ] && pass "advance to phase=intent worked (rc=0, phase=$phase)" || fail_ "rc=$rc phase=$phase"

# === Test 2: 'intent' completes between calibrate and research ================
header "Test 2: completed_phases tracks intent → research transition"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
bash "$CYCLE_STATE" advance intent intent >/dev/null 2>&1
bash "$CYCLE_STATE" advance research scout >/dev/null 2>&1
completed=$(jq -r '.completed_phases | join(",")' "$TEST_STATE" 2>/dev/null)
echo "$completed" | grep -q "intent" && pass "intent recorded in completed_phases ($completed)" || fail_ "intent missing from completed_phases ($completed)"

# === Test 3: cycle_state_init records intent_required from env ================
header "Test 3: cycle_state_init records intent_required:true when env=1"
reset_state
EVOLVE_REQUIRE_INTENT=1 bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
ir=$(jq -r '.intent_required' "$TEST_STATE" 2>/dev/null)
[ "$ir" = "true" ] && pass "intent_required=true recorded" || fail_ "intent_required=$ir (expected true)"

# === Test 4: cycle_state_init defaults intent_required to false ==============
header "Test 4: cycle_state_init defaults intent_required:false when env unset"
reset_state
unset EVOLVE_REQUIRE_INTENT
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
ir=$(jq -r '.intent_required' "$TEST_STATE" 2>/dev/null)
[ "$ir" = "false" ] && pass "intent_required=false default" || fail_ "intent_required=$ir (expected false)"

# === Test 5: re-running /intent replaces intent.md (autonomy: no checkpoint) ==
header "Test 5: re-running intent persona replaces prior intent.md (no human approval)"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
# First intent.md (CLEAR class)
write_valid_intent "$TEST_WORKSPACE/intent.md" CLEAR
sha1=$(sha256_file "$TEST_WORKSPACE/intent.md")
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
# Re-run: replace with a different intent.md (IMKI class)
write_valid_intent "$TEST_WORKSPACE/intent.md" IMKI
sha2=$(sha256_file "$TEST_WORKSPACE/intent.md")
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
# Gate should accept the latest ledger entry (sha2), not the first
set +e
bash "$PHASE_GATE" intent-to-research 99 "$TEST_WORKSPACE" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "0" ] && [ "$sha1" != "$sha2" ] && pass "latest intent.md accepted (sha changed: $sha1 → $sha2)" || fail_ "rc=$rc sha1=$sha1 sha2=$sha2"

# === Test 6: gate_intent_to_research denies when intent.md missing ===========
header "Test 6: gate_intent_to_research denies missing intent.md"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
set +e
bash "$PHASE_GATE" intent-to-research 99 "$TEST_WORKSPACE" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" != "0" ] && pass "missing intent.md denied (rc=$rc)" || fail_ "rc=$rc (expected non-zero)"

# === Test 7: gate_intent_to_research denies when challenged_premises < 1 =====
header "Test 7: gate_intent_to_research denies zero-premise intent.md"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
cat > "$TEST_WORKSPACE/intent.md" <<'EOF'
---
awn_class: CLEAR
goal: Build the thing.
challenged_premises: []
---
EOF
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
set +e
out=$(bash "$PHASE_GATE" intent-to-research 99 "$TEST_WORKSPACE" 2>&1)
rc=$?
set -e
echo "$out" | grep -q "challenged_premises" && matched=0 || matched=1
[ "$rc" != "0" ] && [ "$matched" = "0" ] && pass "zero-premise denied with clear message (rc=$rc)" || fail_ "rc=$rc matched=$matched out=[$out]"

# === Test 8: gate_intent_to_research denies awn_class=IBTC ===================
header "Test 8: gate_intent_to_research denies awn_class=IBTC"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
write_valid_intent "$TEST_WORKSPACE/intent.md" IBTC
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
set +e
out=$(bash "$PHASE_GATE" intent-to-research 99 "$TEST_WORKSPACE" 2>&1)
rc=$?
set -e
echo "$out" | grep -qi "ibtc\|out of scope\|scope rejection" && matched=0 || matched=1
[ "$rc" != "0" ] && [ "$matched" = "0" ] && pass "IBTC short-circuit denied (rc=$rc)" || fail_ "rc=$rc matched=$matched out=[$out]"

# === Test 9: gate_intent_to_research passes valid intent.md + ledger =========
header "Test 9: gate_intent_to_research allows valid intent.md"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
write_valid_intent "$TEST_WORKSPACE/intent.md"
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
set +e
bash "$PHASE_GATE" intent-to-research 99 "$TEST_WORKSPACE" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "0" ] && pass "valid intent.md allowed (rc=0)" || fail_ "rc=$rc"

# === Test 10: precondition denies scout when intent_required=true + no intent ledger
header "Test 10: precondition denies scout when intent_required=true and no intent ledger entry"
reset_state
EVOLVE_REQUIRE_INTENT=1 bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
# Initial phase=calibrate, intent_required=true, no intent ledger entry yet
payload='{"tool_input":{"command":"bash scripts/subagent-run.sh scout 99 .evolve/runs/cycle-99"}}'
set +e
echo "$payload" | bash "$PRECONDITION" >/dev/null 2>&1
rc=$?
set -e
# rc=2 = explicit deny; we want the precondition to deny scout under these conditions
[ "$rc" = "2" ] && pass "scout denied when intent.md missing (rc=2)" || fail_ "rc=$rc (expected 2 for deny)"

# === Test 11: precondition allows scout when intent ledger entry present =====
header "Test 11: precondition allows scout when intent.md ledger entry present"
reset_state
EVOLVE_REQUIRE_INTENT=1 bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
bash "$CYCLE_STATE" advance intent intent >/dev/null 2>&1
bash "$CYCLE_STATE" advance research scout >/dev/null 2>&1
write_valid_intent "$TEST_WORKSPACE/intent.md"
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
payload='{"tool_input":{"command":"bash scripts/subagent-run.sh scout 99 .evolve/runs/cycle-99"}}'
set +e
echo "$payload" | bash "$PRECONDITION" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "0" ] && pass "scout allowed when intent.md ledger present (rc=0)" || fail_ "rc=$rc (expected 0)"

# === Test 12: default behavior unchanged (intent_required=false) =============
header "Test 12: default flow (intent_required=false) — scout allowed at calibrate"
reset_state
unset EVOLVE_REQUIRE_INTENT
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
payload='{"tool_input":{"command":"bash scripts/subagent-run.sh scout 99 .evolve/runs/cycle-99"}}'
set +e
echo "$payload" | bash "$PRECONDITION" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "0" ] && pass "default flow unchanged: scout allowed (rc=0)" || fail_ "rc=$rc (expected 0)"

# === Test 13: gate_calibrate_to_intent only fires when intent_required=true ==
header "Test 13: gate_calibrate_to_intent passes when intent_required=true"
reset_state
EVOLVE_REQUIRE_INTENT=1 bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
set +e
bash "$PHASE_GATE" calibrate-to-intent 99 "$TEST_WORKSPACE" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "0" ] && pass "gate passes when intent_required=true (rc=0)" || fail_ "rc=$rc (expected 0)"

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
