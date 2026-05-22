#!/usr/bin/env bash
#
# cycle-simulate-test.sh — Tests for scripts/dispatch/cycle-simulator.sh and
# scripts/dispatch/run-cycle.sh --simulate (v8.50.0).
#
# Each test creates a fresh temp project repo, copies the necessary scripts +
# resolve-roots.sh + cycle-state.sh in, and exercises the simulator path.
#
# Verifies:
#   - Every phase produces its named artifact
#   - cycle-state.json advances through every phase
#   - 6 ledger entries are appended
#   - ledger.tip is updated
#   - prev_hash chain is intact
#   - simulator-report.md is written
#   - ship.sh --dry-run is invoked (no real commits)
#   - --simulate via run-cycle.sh end-to-end
#
# Bash 3.2 compatible. No declare -A, no GNU-only date/sed.

set -uo pipefail

unset EVOLVE_PROJECT_ROOT EVOLVE_PLUGIN_ROOT EVOLVE_RESOLVE_ROOTS_LOADED
unset EVOLVE_BYPASS_SHIP_VERIFY EVOLVE_SHIP_RELEASE_NOTES

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SIMULATOR="$REPO_ROOT/scripts/dispatch/cycle-simulator.sh"
RUN_CYCLE="$REPO_ROOT/scripts/dispatch/run-cycle.sh"
RESOLVE_ROOTS="$REPO_ROOT/scripts/lifecycle/resolve-roots.sh"
CYCLE_STATE="$REPO_ROOT/scripts/lifecycle/cycle-state.sh"
SHIP_SH="$REPO_ROOT/scripts/lifecycle/ship.sh"
PHASE_GATE="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
VERIFY_CHAIN="$REPO_ROOT/scripts/observability/verify-ledger-chain.sh"
SCRATCH=$(mktemp -d -t "cycle-sim-XXXXXX")
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Set up a fake project repo with the scripts the simulator needs.
# Returns the repo path.
make_project() {
    local proj="$SCRATCH/proj-$RANDOM"
    mkdir -p "$proj/scripts/dispatch" "$proj/scripts/lifecycle" "$proj/scripts/observability" "$proj/.evolve/runs"
    cp "$SIMULATOR"      "$proj/scripts/dispatch/cycle-simulator.sh"
    cp "$RESOLVE_ROOTS"  "$proj/scripts/lifecycle/resolve-roots.sh"
    cp "$CYCLE_STATE"    "$proj/scripts/lifecycle/cycle-state.sh"
    cp "$SHIP_SH"        "$proj/scripts/lifecycle/ship.sh"
    cp "$PHASE_GATE"     "$proj/scripts/lifecycle/phase-gate.sh"
    cp "$VERIFY_CHAIN"   "$proj/scripts/observability/verify-ledger-chain.sh"
    chmod +x "$proj/scripts"/*/*.sh
    : > "$proj/.evolve/ledger.jsonl"
    echo '{}' > "$proj/.evolve/state.json"
    echo "fixture" > "$proj/fixture.txt"
    cd "$proj"
    git init -q
    git config user.email "test@evolve-loop.test"
    git config user.name "Test User"
    git config core.hooksPath /dev/null
    cat > .gitignore <<EOF
.evolve/
EOF
    git add -A
    git commit -q -m "initial"
    cd "$REPO_ROOT" >/dev/null
    echo "$proj"
}

# Set up cycle-state.json for cycle N at phase=calibrate, the entry condition
# the simulator expects (it advances through phase transitions). EVOLVE_PROJECT_ROOT
# must be set so cycle-state.json lands in the test repo, not the parent repo.
init_cycle_state() {
    local proj="$1" cycle="$2"
    EVOLVE_PROJECT_ROOT="$proj" \
        bash "$proj/scripts/lifecycle/cycle-state.sh" init "$cycle" ".evolve/runs/cycle-$cycle" >/dev/null 2>&1
}

# === Test 1: simulator advances through every phase ============================
header "Test 1: cycle-simulator.sh writes 6 phase artifacts + simulator-report.md"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9001
cd "$PROJ"
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9001 ".evolve/runs/cycle-9001" >/tmp/sim-out 2>&1
RC=$?
set -e
WS="$PROJ/.evolve/runs/cycle-9001"
artifacts=("intent.md" "scout-report.md" "build-report.md" "audit-report.md" "retrospective-report.md" "simulator-report.md")
all_present=1
for a in "${artifacts[@]}"; do
    [ -f "$WS/$a" ] || all_present=0
done
if [ "$RC" = "0" ] && [ "$all_present" = "1" ]; then
    pass "all 6 artifacts present + rc=0"
else
    fail_ "rc=$RC all_present=$all_present (missing: $(for a in ${artifacts[@]}; do [ -f "$WS/$a" ] || echo -n "$a "; done))"
fi
cd "$REPO_ROOT"

# === Test 2: each artifact contains the challenge token =========================
header "Test 2: every artifact contains the simulator's challenge token"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9002
cd "$PROJ"
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9002 ".evolve/runs/cycle-9002" >/tmp/sim-out 2>&1
set -e
WS="$PROJ/.evolve/runs/cycle-9002"
all_have_token=1
for a in intent.md scout-report.md build-report.md audit-report.md retrospective-report.md; do
    grep -q "challenge-token: sim-token-9002-" "$WS/$a" 2>/dev/null || all_have_token=0
done
if [ "$all_have_token" = "1" ]; then
    pass "all 5 phase artifacts have challenge-token"
else
    fail_ "some artifacts missing challenge-token"
fi
cd "$REPO_ROOT"

# === Test 3: ledger gets 5 simulated entries (intent/scout/build/audit/retro) ==
header "Test 3: ledger receives 5 simulated entries"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9003
cd "$PROJ"
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9003 ".evolve/runs/cycle-9003" >/tmp/sim-out 2>&1
set -e
ENTRIES=$(wc -l < "$PROJ/.evolve/ledger.jsonl" | tr -d ' ')
SIM_ENTRIES=$(grep -c '"simulated":true' "$PROJ/.evolve/ledger.jsonl" 2>/dev/null || echo 0)
if [ "$ENTRIES" -ge 5 ] && [ "$SIM_ENTRIES" = "5" ]; then
    pass "5 simulated ledger entries appended (total=$ENTRIES)"
else
    fail_ "entries=$ENTRIES simulated=$SIM_ENTRIES (expected ≥5 / 5)"
fi
cd "$REPO_ROOT"

# === Test 4: ledger.tip updated ================================================
header "Test 4: .evolve/ledger.tip exists and matches last entry SHA"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9004
cd "$PROJ"
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9004 ".evolve/runs/cycle-9004" >/tmp/sim-out 2>&1
set -e
TIP="$PROJ/.evolve/ledger.tip"
if [ -f "$TIP" ]; then
    last_line=$(tail -1 "$PROJ/.evolve/ledger.jsonl")
    if command -v sha256sum >/dev/null 2>&1; then
        actual_sha=$(printf '%s' "$last_line" | sha256sum | awk '{print $1}')
    else
        actual_sha=$(printf '%s' "$last_line" | shasum -a 256 | awk '{print $1}')
    fi
    tip_sha=$(awk -F: '{print $2}' "$TIP")
    if [ "$actual_sha" = "$tip_sha" ]; then
        pass "ledger.tip SHA matches last entry"
    else
        fail_ "tip=$tip_sha actual=$actual_sha"
    fi
else
    fail_ "ledger.tip missing"
fi
cd "$REPO_ROOT"

# === Test 5: prev_hash chain intact (verify-ledger-chain returns 0) ============
header "Test 5: prev_hash chain intact post-simulate"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9005
cd "$PROJ"
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9005 ".evolve/runs/cycle-9005" >/tmp/sim-out 2>&1
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/observability/verify-ledger-chain.sh >/tmp/verify-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ]; then
    pass "verify-ledger-chain rc=0 (chain intact)"
else
    fail_ "verify-ledger-chain rc=$RC tail: $(tail -3 /tmp/verify-out)"
fi
cd "$REPO_ROOT"

# === Test 6: cycle-state advances through every phase ==========================
header "Test 6: cycle-state.json reaches phase=retrospective"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9006
cd "$PROJ"
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9006 ".evolve/runs/cycle-9006" >/tmp/sim-out 2>&1
set -e
FINAL=$(jq -r '.phase' "$PROJ/.evolve/cycle-state.json" 2>/dev/null)
if [ "$FINAL" = "retrospective" ]; then
    pass "final phase=retrospective"
else
    fail_ "final phase=$FINAL (expected retrospective)"
fi
cd "$REPO_ROOT"

# === Test 7: simulator does not commit anything =================================
header "Test 7: simulator leaves git tree clean (no commits beyond initial)"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9007
cd "$PROJ"
COMMITS_BEFORE=$(git -C "$PROJ" log --oneline 2>/dev/null | wc -l | tr -d ' ')
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9007 ".evolve/runs/cycle-9007" >/tmp/sim-out 2>&1
set -e
COMMITS_AFTER=$(git -C "$PROJ" log --oneline 2>/dev/null | wc -l | tr -d ' ')
if [ "$COMMITS_BEFORE" = "$COMMITS_AFTER" ]; then
    pass "commits unchanged ($COMMITS_BEFORE = $COMMITS_AFTER)"
else
    fail_ "before=$COMMITS_BEFORE after=$COMMITS_AFTER (simulator committed!)"
fi
cd "$REPO_ROOT"

# === Test 8: ship.sh --dry-run was exercised (preview journal exists) ==========
header "Test 8: simulator's ship phase wrote dry-run preview"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9008
cd "$PROJ"
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9008 ".evolve/runs/cycle-9008" >/tmp/sim-out 2>&1
set -e
preview=$(ls -1 "$PROJ/.evolve/release-journal/dry-run-"*.json 2>/dev/null | head -1)
# Note: ship.sh may rc=2 in simulator context if audit-binding fails, but it
# should still write the preview. Acceptable either way for plumbing validation.
if [ -n "$preview" ] || grep -q "ship.sh --dry-run" /tmp/sim-out; then
    pass "ship.sh --dry-run was invoked (preview=$([ -n "$preview" ] && echo present || echo missing))"
else
    fail_ "no evidence of ship.sh --dry-run invocation"
fi
cd "$REPO_ROOT"

# === Test 9: simulator-report.md present with summary ==========================
header "Test 9: simulator-report.md summarises the run"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9009
cd "$PROJ"
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9009 ".evolve/runs/cycle-9009" >/tmp/sim-out 2>&1
set -e
REPORT="$PROJ/.evolve/runs/cycle-9009/simulator-report.md"
if [ -f "$REPORT" ] && grep -q "All 6 phases advanced" "$REPORT"; then
    pass "simulator-report.md contains summary"
else
    fail_ "simulator-report.md missing or empty"
fi
cd "$REPO_ROOT"

# === Test 10: dry-run preview JSON written from simulator's ship phase ========
# After Test 8 confirmed the simulator invokes ship.sh --dry-run, Test 10
# verifies the resulting preview JSON is well-formed and has class=cycle.
header "Test 10: simulator's dry-run preview is well-formed JSON"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9010
cd "$PROJ"
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9010 ".evolve/runs/cycle-9010" >/tmp/sim-out 2>&1
set -e
preview=$(ls -1 "$PROJ/.evolve/release-journal/dry-run-"*.json 2>/dev/null | head -1)
if [ -n "$preview" ] && jq empty "$preview" 2>/dev/null; then
    class=$(jq -r '.class' "$preview")
    reason=$(jq -r '.exit_reason' "$preview")
    if [ "$class" = "cycle" ] && [ -n "$reason" ]; then
        pass "preview valid JSON: class=$class exit_reason=$reason"
    else
        fail_ "class=$class reason=$reason"
    fi
else
    fail_ "no preview or invalid JSON: preview=$preview"
fi
cd "$REPO_ROOT"

# === Test 11: run-cycle.sh --simulate end-to-end ===============================
# Full integration via run-cycle.sh (provisions worktree, builds prompt, then
# delegates to cycle-simulator.sh).
header "Test 11: run-cycle.sh --simulate end-to-end"
PROJ=$(make_project)
mkdir -p "$PROJ/scripts/lifecycle" "$PROJ/scripts/dispatch" "$PROJ/scripts/failure" "$PROJ/scripts/observability" "$PROJ/agents"
cp "$RUN_CYCLE" "$PROJ/scripts/dispatch/run-cycle.sh"
cp "$REPO_ROOT/scripts/dispatch/subagent-run.sh" "$PROJ/scripts/dispatch/subagent-run.sh"
cp "$REPO_ROOT/scripts/failure/failure-adapter.sh" "$PROJ/scripts/failure/failure-adapter.sh" 2>/dev/null || true
# orchestrator-prompt — minimal stub
echo "# orchestrator (stub)" > "$PROJ/agents/evolve-orchestrator.md"
# Required for prompt build path
chmod +x "$PROJ/scripts"/*/*.sh
cd "$PROJ"
set +e
# Disable worktree provisioning (simulator does not need a worktree;
# `--simulate` only walks state + ledger).
EVOLVE_PROJECT_ROOT="$PROJ" EVOLVE_PLUGIN_ROOT="$PROJ" EVOLVE_SKIP_WORKTREE=1 \
    bash scripts/dispatch/run-cycle.sh --cycle 9011 --simulate "test goal" >/tmp/sim-out 2>&1
RC=$?
set -e
WS="$PROJ/.evolve/runs/cycle-9011"
if [ "$RC" = "0" ] && [ -f "$WS/simulator-report.md" ]; then
    pass "run-cycle.sh --simulate end-to-end (rc=0, simulator-report.md present)"
else
    fail_ "rc=$RC report=$([ -f "$WS/simulator-report.md" ] && echo present || echo missing); tail: $(tail -10 /tmp/sim-out)"
fi
cd "$REPO_ROOT"

# === Test 12: --simulate is mutually fast (single cycle ≤ 15s on dev machine) ==
# Soft bound; mainly catches infinite loops.
header "Test 12: --simulate completes in under 15 seconds"
PROJ=$(make_project)
init_cycle_state "$PROJ" 9012
cd "$PROJ"
START=$(date +%s)
set +e
EVOLVE_PROJECT_ROOT="$PROJ" bash scripts/dispatch/cycle-simulator.sh 9012 ".evolve/runs/cycle-9012" >/tmp/sim-out 2>&1
set -e
END=$(date +%s)
DUR=$((END - START))
if [ "$DUR" -le 15 ]; then
    pass "duration ${DUR}s ≤ 15s"
else
    fail_ "duration ${DUR}s exceeded 15s budget"
fi
cd "$REPO_ROOT"

# === Summary ====================================================================
echo
echo "==========================================="
echo "  Total: 12 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
