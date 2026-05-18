#!/usr/bin/env bash
#
# orchestrator-isolation-test.sh — Per-cycle orchestrator isolation contract.
#
# Plan reference: ~/.claude/plans/linked-meandering-lobster.md
#
# Verifies the four-point isolation contract:
#   1. orchestrator profile denies reads of .evolve/ledger.jsonl
#   2. orchestrator profile denies reads of historical .evolve/runs/cycle-* dirs
#      (with a runtime carve-out for the current cycle — see Step 2)
#   3. orchestrator profile denies reads of resume-quarantine .attempt-* dirs
#   4. build-invocation-context.sh filters same-cycle ledger entries
#   5. resume-cycle.sh moves prior-attempt artifacts into .attempt-K/ before
#      re-spawning orchestrator
#   6. cycle-release.sh exists and is invoked from run-cycle.sh's EXIT trap
#
# Profile-static checks (Tests 1-4) introspect orchestrator.json with jq —
# no subagent spawn needed. The other tests grep the relevant scripts for
# marker tokens that the implementation must include.
#
# Usage: bash scripts/tests/orchestrator-isolation-test.sh
# Exit 0 = all asserts pass; exit 1 = at least one failure.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
PROFILE="$PROJECT_ROOT/.evolve/profiles/orchestrator.json"
INVOCATION_CTX="$PROJECT_ROOT/scripts/dispatch/build-invocation-context.sh"
RESUME_CYCLE="$PROJECT_ROOT/scripts/dispatch/resume-cycle.sh"
RUN_CYCLE="$PROJECT_ROOT/scripts/dispatch/run-cycle.sh"
CYCLE_RELEASE="$PROJECT_ROOT/scripts/lifecycle/cycle-release.sh"

PASS=0
FAIL=0
TESTS_TOTAL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*" >&2; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# --- Test 1: prerequisites ---------------------------------------------------
header "Test 1: orchestrator profile + jq present"
if [ ! -f "$PROFILE" ]; then
    fail_ "missing $PROFILE"
elif ! command -v jq >/dev/null 2>&1; then
    fail_ "jq required but not installed"
elif ! jq -e . "$PROFILE" >/dev/null 2>&1; then
    fail_ "$PROFILE is not valid JSON"
else
    pass "profile readable, jq available"
fi

# --- Test 2: ledger.jsonl is in deny_subpaths --------------------------------
header "Test 2: .evolve/ledger.jsonl is denied for orchestrator reads"
if jq -e '.sandbox.deny_subpaths | index(".evolve/ledger.jsonl")' "$PROFILE" >/dev/null 2>&1; then
    pass ".evolve/ledger.jsonl present in deny_subpaths"
else
    fail_ ".evolve/ledger.jsonl missing from deny_subpaths — orchestrator can Read/cat raw ledger"
    echo "    Fix: add \".evolve/ledger.jsonl\" to .sandbox.deny_subpaths in $PROFILE"
fi

# --- Test 3: historical .evolve/runs is denied --------------------------------
header "Test 3: historical .evolve/runs entries are denied"
# Accept either pattern:
#   - ".evolve/runs"           (whole tree denied; current cycle write_subpaths
#                                glob acts as the carve-out)
#   - ".evolve/runs/cycle-*"   (literal glob denying every cycle dir; current
#                                cycle still writable via write_subpaths)
runs_denied="0"
if jq -e '.sandbox.deny_subpaths | index(".evolve/runs")' "$PROFILE" >/dev/null 2>&1; then
    runs_denied="1"
fi
if jq -e '.sandbox.deny_subpaths | index(".evolve/runs/cycle-*")' "$PROFILE" >/dev/null 2>&1; then
    runs_denied="1"
fi
if [ "$runs_denied" = "1" ]; then
    pass "historical .evolve/runs reads denied (cross-cycle isolation enforced)"
else
    fail_ "historical .evolve/runs entries are NOT in deny_subpaths"
    echo "    Fix: add \".evolve/runs\" to .sandbox.deny_subpaths (current cycle"
    echo "         is carved out via .sandbox.write_subpaths \".evolve/runs/cycle-*\")"
fi

# --- Test 4: resume-quarantine attempt dirs are denied ------------------------
header "Test 4: .evolve/runs/cycle-*/.attempt-* (resume quarantine) is denied"
attempt_denied="0"
while IFS= read -r entry; do
    [ -z "$entry" ] && continue
    case "$entry" in
        *.attempt-*|*attempt-*) attempt_denied="1"; break ;;
    esac
done < <(jq -r '.sandbox.deny_subpaths[]?' "$PROFILE" 2>/dev/null)
if [ "$attempt_denied" = "1" ]; then
    pass "resume quarantine path present in deny_subpaths"
else
    fail_ "no .attempt-* deny entry found — orchestrator could opportunistically Read prior-attempt artifacts on resume"
    echo "    Fix: add \".evolve/runs/cycle-*/.attempt-*\" (or equivalent) to .sandbox.deny_subpaths"
fi

# --- Test 5: build-invocation-context.sh filters same-cycle ledger entries ---
header "Test 5: build-invocation-context.sh filters same-cycle ledger entries"
if [ ! -f "$INVOCATION_CTX" ]; then
    fail_ "missing $INVOCATION_CTX"
else
    if grep -E -q 'recentLedgerEntries.*cycle.*!=|cycle !=.*current|select\(.cycle ?!=|same-cycle.*filter|filter.*same.*cycle|EVOLVE_LEDGER_FILTER_SAME_CYCLE' "$INVOCATION_CTX"; then
        pass "build-invocation-context.sh contains same-cycle filter"
    else
        fail_ "build-invocation-context.sh does NOT filter same-cycle ledger entries"
        echo "    Fix: in the recentLedgerEntries pipeline, drop entries where"
        echo "         .cycle == \$CURRENT_CYCLE. Same-cycle entries are stale-"
        echo "         attempt noise on --resume."
    fi
fi

# --- Test 6: resume-cycle.sh quarantines prior-attempt artifacts --------------
header "Test 6: resume-cycle.sh quarantines stale artifacts into .attempt-K/"
if [ ! -f "$RESUME_CYCLE" ]; then
    fail_ "missing $RESUME_CYCLE"
else
    if grep -E -q '\.attempt-|attempt_quarantine|quarantine_prior_attempt|resume_quarantine|EVOLVE_QUARANTINE_PRIOR_ATTEMPT' "$RESUME_CYCLE"; then
        pass "resume-cycle.sh contains attempt-quarantine logic"
    else
        fail_ "resume-cycle.sh does NOT quarantine prior-attempt artifacts"
        echo "    Fix: before re-spawning orchestrator, move \$WORKSPACE/* into"
        echo "         \$WORKSPACE/.attempt-\$K/ (K from cycle-state.json:"
        echo "         autoResumeAttempts, or counted from existing .attempt-* dirs)."
    fi
fi

# --- Test 7: cycle-release.sh exists and is wired into run-cycle.sh EXIT ------
header "Test 7: cycle-release.sh exists and run-cycle.sh invokes it on terminal exit"
if [ ! -f "$CYCLE_RELEASE" ]; then
    fail_ "missing $CYCLE_RELEASE — Step 6 not yet implemented"
elif [ ! -x "$CYCLE_RELEASE" ]; then
    fail_ "$CYCLE_RELEASE not executable"
else
    pass "cycle-release.sh exists and is executable"
    if grep -E -q 'cycle-release\.sh|cycle_release' "$RUN_CYCLE"; then
        pass "run-cycle.sh invokes cycle-release.sh"
    else
        fail_ "run-cycle.sh does NOT invoke cycle-release.sh in EXIT trap"
        echo "    Fix: in run-cycle.sh, add cycle-release.sh call to the EXIT trap"
        echo "         so it fires on any terminal state (SHIP, FAIL, watchdog-kill,"
        echo "         quota-pause), not just clean exit."
    fi
fi

# --- Summary -----------------------------------------------------------------
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
