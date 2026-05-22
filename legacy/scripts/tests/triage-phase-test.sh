#!/usr/bin/env bash
#
# triage-phase-test.sh — v8.56.0 Layer C smoke tests.
# Verifies cycle-state.sh recognizes 'triage' as a valid phase, phase-gate
# accepts/rejects triage transitions correctly, and the agent profile is
# well-formed.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRATCH=$(mktemp -d)

PASS=0
FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# --- Test 1: triage profile JSON is valid + has parallel_eligible:false ----
header "Test 1: triage profile is well-formed and parallel_eligible:false"
PROFILE="$REPO_ROOT/.evolve/profiles/triage.json"
if [ ! -f "$PROFILE" ]; then
    fail "profile missing: $PROFILE"
else
    if jq empty "$PROFILE" 2>/dev/null; then
        pass "triage.json is valid JSON"
    else
        fail "triage.json is malformed"
    fi
    PE=$(jq -r '.parallel_eligible' "$PROFILE")
    [ "$PE" = "false" ] && pass "parallel_eligible=false (single-writer)" || \
        fail "parallel_eligible should be false, got '$PE'"
    OUT=$(jq -r '.output_artifact' "$PROFILE")
    [ "$OUT" = ".evolve/runs/cycle-{cycle}/triage-decision.md" ] && \
        pass "output_artifact correct" || fail "wrong output_artifact: $OUT"
fi

# --- Test 2: triage persona file exists -------------------------------------
header "Test 2: agents/evolve-triage.md exists"
PERSONA="$REPO_ROOT/agents/evolve-triage.md"
[ -f "$PERSONA" ] && pass "persona file exists" || fail "missing $PERSONA"

# --- Test 3: cycle-state.sh accepts 'triage' phase --------------------------
header "Test 3: cycle-state.sh advance triage works"
sf="$SCRATCH/cs-test.json"
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" init 1 /tmp/wt >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance research scout >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance discover scout >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance triage triage >/dev/null 2>&1
PHASE=$(jq -r '.phase' "$sf" 2>/dev/null)
[ "$PHASE" = "triage" ] && pass "phase advanced to triage" || fail "expected phase=triage, got '$PHASE'"
COMPLETED=$(jq -r '.completed_phases | join(",")' "$sf")
echo "$COMPLETED" | grep -q "discover" && pass "discover recorded as completed" || \
    fail "discover not in completed_phases (got '$COMPLETED')"

# Continue to plan-review — verifies triage is recognized as a known phase
# (so the completed-phase dedup logic runs through it without errors)
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance plan-review plan-reviewer >/dev/null 2>&1
COMPLETED=$(jq -r '.completed_phases | join(",")' "$sf")
echo "$COMPLETED" | grep -q "triage" && pass "triage recorded as completed (known phase)" || \
    fail "triage NOT in completed_phases (means it's not in the \$known list)"

# --- Test 4: phase-gate-precondition recognizes triage as agent + phase ----
header "Test 4: phase-gate-precondition.sh recognizes triage agent + phase"
# Simulate a Bash hook invocation requesting the triage agent during phase=discover
sf2="$SCRATCH/cs-pgp.json"
EVOLVE_CYCLE_STATE_FILE="$sf2" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" init 2 /tmp/wt2 >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf2" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance research scout >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf2" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance discover scout >/dev/null 2>&1

# Build a tool_input JSON that requests triage via subagent-run.sh
HOOK_INPUT=$(jq -nc --arg cmd "bash scripts/dispatch/subagent-run.sh triage 2 /tmp/ws" \
    '{tool_name:"Bash",tool_input:{command:$cmd}}')
# Override CYCLE_STATE_FILE the hook reads
RC=$(echo "$HOOK_INPUT" | EVOLVE_CYCLE_STATE_FILE="$sf2" bash "$REPO_ROOT/scripts/guards/phase-gate-precondition.sh" >/dev/null 2>&1; echo $?)
[ "$RC" = "0" ] && pass "phase-gate-precondition ALLOWs triage during phase=discover" || \
    fail "phase-gate-precondition REJECTED triage (rc=$RC); expected ALLOW"

# Negative case: requesting triage during phase=audit must be DENIED
EVOLVE_CYCLE_STATE_FILE="$sf2" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance triage triage >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf2" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance plan-review plan-reviewer >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf2" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance tdd tdd-engineer >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf2" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance build builder >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf2" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance audit auditor >/dev/null 2>&1
RC=$(echo "$HOOK_INPUT" | EVOLVE_CYCLE_STATE_FILE="$sf2" bash "$REPO_ROOT/scripts/guards/phase-gate-precondition.sh" >/dev/null 2>&1; echo $?)
[ "$RC" != "0" ] && pass "phase-gate-precondition DENIES triage during phase=audit" || \
    fail "phase-gate-precondition ALLOWED triage during phase=audit; expected DENY"

# --- Test 5: phase-gate.sh has triage gate functions ------------------------
header "Test 5: phase-gate.sh exposes discover-to-triage and triage-to-plan-review"
PG="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
grep -q "discover-to-triage)" "$PG" && pass "discover-to-triage gate registered" || \
    fail "discover-to-triage gate missing from dispatch"
grep -q "triage-to-plan-review)" "$PG" && pass "triage-to-plan-review gate registered" || \
    fail "triage-to-plan-review gate missing"
grep -q "gate_discover_to_triage()" "$PG" && pass "gate_discover_to_triage function defined" || \
    fail "gate_discover_to_triage function missing"
grep -q "gate_triage_to_plan_review()" "$PG" && pass "gate_triage_to_plan_review function defined" || \
    fail "gate_triage_to_plan_review function missing"

# --- Test 6: gate_triage_to_plan_review blocks on cycle_size_estimate=large -
header "Test 6: triage→plan-review gate blocks on cycle_size_estimate=large"
# Build a workspace with a large-flagged triage decision
ws="$SCRATCH/ws-large"
mkdir -p "$ws"
cat > "$ws/triage-decision.md" <<TDEOF
<!-- challenge-token: test -->
# Triage Decision — Cycle 99
cycle_size_estimate: large

## top_n
- todo-1: big thing
TDEOF
# Append a fake triage ledger entry so the gate finds it
LEDGER="$SCRATCH/.evolve/ledger.jsonl"
mkdir -p "$(dirname "$LEDGER")"
echo '{"role":"triage","cycle":99,"ts":"2026-05-09T00:00:00Z"}' > "$LEDGER"
# phase-gate.sh expects EVOLVE_DIR / WORKSPACE / CYCLE / LEDGER vars
set +e
EVOLVE_PROJECT_ROOT="$SCRATCH" \
WORKSPACE="$ws" \
CYCLE="99" \
bash "$PG" triage-to-plan-review 99 "$ws" >/dev/null 2>&1
RC=$?
set -e
[ "$RC" != "0" ] && pass "gate blocks on cycle_size_estimate=large" || \
    fail "gate did NOT block on large (rc=$RC)"

# Positive case: small should pass (with the ledger entry present)
# NOTE: substance check requires >=50 words, so the fixture is intentionally verbose.
cat > "$ws/triage-decision.md" <<TDEOF
<!-- challenge-token: test -->
# Triage Decision — Cycle 99

cycle_size_estimate: small

## top_n (commit to THIS cycle)
- todo-1: small thing — priority=H, evidence=scout-report.md, source=scout

## deferred (carry to NEXT cycle's carryoverTodos)
- todo-2: medium thing — priority=M, defer_reason=out-of-scope-for-this-cycle

## dropped (rejected with reason)
- todo-3: stale thing — reason=duplicate-of-todo-1

## Rationale
This cycle stays small to preserve velocity. Three items would dilute focus given
the recent v8.55 release window. The dropped item duplicates work covered by
todo-1; the deferred item is medium-scope work not blocking the cycle goal.
TDEOF
set +e
EVOLVE_PROJECT_ROOT="$SCRATCH" \
WORKSPACE="$ws" \
CYCLE="99" \
bash "$PG" triage-to-plan-review 99 "$ws" >/dev/null 2>&1
RC=$?
set -e
[ "$RC" = "0" ] && pass "gate accepts cycle_size_estimate=small" || \
    fail "gate rejected small (rc=$RC); expected accept"

# --- Test 7: plugin.json registers evolve-triage agent ----------------------
header "Test 7: plugin.json registers evolve-triage agent"
PJ="$REPO_ROOT/.claude-plugin/plugin.json"
if jq -e '.agents | index("./agents/evolve-triage.md")' "$PJ" >/dev/null 2>&1; then
    pass "evolve-triage.md registered in plugin.json"
else
    fail "evolve-triage.md NOT registered in plugin.json"
fi

# --- Summary ----------------------------------------------------------------
rm -rf "$SCRATCH"
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
