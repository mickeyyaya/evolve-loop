#!/usr/bin/env bash
#
# memo-phase-test.sh — v8.57.0 Layer P smoke tests.
# Verifies:
#   - agents/evolve-memo.md persona exists
#   - .evolve/profiles/memo.json profile is well-formed and parallel_eligible:false
#   - plugin.json registers evolve-memo
#   - phase-gate-precondition.sh recognizes 'memo' agent
#   - The orchestrator persona's PASS branch instructs invoking memo

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRATCH=$(mktemp -d)

PASS=0
FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# --- Test 1: persona file exists -------------------------------------------
header "Test 1: agents/evolve-memo.md exists"
PERSONA="$REPO_ROOT/agents/evolve-memo.md"
[ -f "$PERSONA" ] && pass "persona file present" || fail "missing $PERSONA"

# --- Test 2: profile is well-formed and parallel_eligible:false -----------
header "Test 2: memo profile is parallel_eligible:false (single-writer)"
PROFILE="$REPO_ROOT/.evolve/profiles/memo.json"
if [ ! -f "$PROFILE" ]; then
    fail "missing $PROFILE"
else
    if jq empty "$PROFILE" 2>/dev/null; then
        pass "memo.json is valid JSON"
    else
        fail "memo.json is malformed"
    fi
    PE=$(jq -r '.parallel_eligible' "$PROFILE")
    [ "$PE" = "false" ] && pass "parallel_eligible=false" || \
        fail "parallel_eligible should be false; got '$PE'"
    # Memo must NOT have permission to write anything beyond carryover-todos.json
    HAS_NARROW_WRITE=$(jq -r '.allowed_tools[]?' "$PROFILE" | grep -c "Write(.evolve/runs/cycle-\*/carryover-todos.json)")
    [ "$HAS_NARROW_WRITE" -ge 1 ] && pass "Write narrowed to carryover-todos.json" || \
        fail "Write permission for carryover-todos.json missing"
    # Should NOT have access to write retrospective-report.md or build-report.md
    if jq -r '.allowed_tools[]?' "$PROFILE" | grep -qE "retrospective-report|build-report"; then
        fail "memo profile should NOT have write access to retrospective/build reports"
    else
        pass "memo profile correctly excludes retrospective/build write access"
    fi
fi

# --- Test 3: plugin.json registers evolve-memo ----------------------------
header "Test 3: plugin.json registers evolve-memo agent"
PJ="$REPO_ROOT/.claude-plugin/plugin.json"
if jq -e '.agents | index("./agents/evolve-memo.md")' "$PJ" >/dev/null 2>&1; then
    pass "evolve-memo.md registered"
else
    fail "evolve-memo.md NOT in plugin.json:agents"
fi

# --- Test 4: phase-gate-precondition recognizes 'memo' agent --------------
header "Test 4: phase-gate-precondition.sh recognizes memo agent"
PGP="$REPO_ROOT/scripts/guards/phase-gate-precondition.sh"
grep -qE 'memo\)|\|memo\)|memo$|memo\|' "$PGP" && pass "memo in recognized agents list" || \
    fail "memo NOT in phase-gate-precondition recognized agents"
grep -q 'memo-worker-\*' "$PGP" && pass "memo-worker-* recognized" || \
    fail "memo-worker-* pattern missing"

# --- Test 5: phase-gate-precondition allows memo during phase=learn -------
header "Test 5: memo allowed during phase=learn (PASS path)"
sf="$SCRATCH/cs-learn.json"
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" init 1 /tmp/wt >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance research scout >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance discover scout >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance build builder >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance audit auditor >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance ship orchestrator >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$REPO_ROOT/scripts/lifecycle/cycle-state.sh" advance learn memo >/dev/null 2>&1
HOOK_INPUT=$(jq -nc --arg cmd "bash scripts/dispatch/subagent-run.sh memo 1 /tmp/ws" \
    '{tool_name:"Bash",tool_input:{command:$cmd}}')
RC=$(echo "$HOOK_INPUT" | EVOLVE_CYCLE_STATE_FILE="$sf" bash "$PGP" >/dev/null 2>&1; echo $?)
[ "$RC" = "0" ] && pass "phase-gate ALLOWs memo during phase=learn" || \
    fail "phase-gate REJECTED memo during phase=learn (rc=$RC)"

# --- Test 6: orchestrator persona PASS branch invokes memo ---------------
header "Test 6: orchestrator persona PASS branch references memo"
ORCH="$REPO_ROOT/agents/evolve-orchestrator.md"
grep -qi "memo" "$ORCH" && pass "orchestrator persona references memo" || \
    fail "orchestrator persona does not mention memo subagent"

# --- Summary ----------------------------------------------------------------
rm -rf "$SCRATCH"
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
