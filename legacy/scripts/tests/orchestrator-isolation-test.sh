#!/usr/bin/env bash
#
# orchestrator-isolation-test.sh — Per-cycle orchestrator isolation contract.
#
# Plan reference: ~/.claude/plans/linked-meandering-lobster.md
#
# Verifies the per-cycle isolation contract.
#
# Architectural note on enforcement layers (refined in Step 2 implementation):
#   - .evolve/ledger.jsonl cannot be in sandbox.deny_subpaths because it's in
#     write_subpaths (orchestrator's child subagent-run.sh appends ledger
#     entries from inside the sandbox; denying writes breaks the loop).
#     Read denial therefore lives at the Claude Code tool-perm layer
#     (disallowed_tools: Read(...) + Bash(cat/head/tail/grep ...) patterns).
#   - .evolve/runs/cycle-* cannot be broadly denied at the write layer either
#     (current-cycle workspace must be writable). The .attempt-* quarantine
#     paths can — they're never legitimate write targets.
#   - Historical-cycle-dir read denial (other than .attempt-*) is DEFERRED:
#     it requires narrowing allowed_tools from Read (unrestricted) to explicit
#     current-cycle-only patterns + verifying CC allow-override-deny semantics.
#     Tracked in docs/architecture/cycle-isolation.md (Step 8).
#
# Contract verified:
#   1. .evolve/ledger.jsonl reads denied via disallowed_tools
#   2. .evolve/runs/cycle-*/.attempt-* reads denied via disallowed_tools
#   3. .evolve/runs/cycle-*/.attempt-* writes denied via sandbox.deny_subpaths
#   4. build-invocation-context.sh filters same-cycle ledger entries
#   5. resume-cycle.sh moves prior-attempt artifacts into .attempt-K/ before
#      re-spawning orchestrator
#   6. cycle-release.sh exists and is invoked from run-cycle.sh's EXIT trap
#
# Usage: bash scripts/tests/orchestrator-isolation-test.sh
# Exit 0 = all asserts pass; exit 1 = at least one failure.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
PROFILE="$PROJECT_ROOT/.evolve/profiles/orchestrator.json"
# Note: the plan named build-invocation-context.sh as the recentLedgerEntries
# site, but the actual injection lives in run-cycle.sh:build_context().
# build-invocation-context.sh is the static bedrock prefix (no dynamic data).
INVOCATION_CTX="$PROJECT_ROOT/scripts/dispatch/run-cycle.sh"
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

# --- Test 2: ledger.jsonl reads denied via tool-perm layer -------------------
header "Test 2: .evolve/ledger.jsonl reads denied (disallowed_tools)"
# Read tool deny + at least one Bash cat/head/tail/grep deny pattern.
read_deny="0"
bash_deny="0"
if jq -e '.disallowed_tools | index("Read(.evolve/ledger.jsonl)")' "$PROFILE" >/dev/null 2>&1; then
    read_deny="1"
fi
while IFS= read -r entry; do
    [ -z "$entry" ] && continue
    case "$entry" in
        "Bash(cat .evolve/ledger.jsonl"*|\
        "Bash(head .evolve/ledger.jsonl"*|\
        "Bash(tail .evolve/ledger.jsonl"*|\
        "Bash(grep:* .evolve/ledger.jsonl"*)
            bash_deny="1"; break ;;
    esac
done < <(jq -r '.disallowed_tools[]?' "$PROFILE" 2>/dev/null)
if [ "$read_deny" = "1" ] && [ "$bash_deny" = "1" ]; then
    pass ".evolve/ledger.jsonl denied via Read() + Bash() patterns"
elif [ "$read_deny" = "1" ]; then
    fail_ "Read(.evolve/ledger.jsonl) denied but Bash cat/head/tail/grep paths NOT denied"
    echo "    Fix: add \"Bash(cat .evolve/ledger.jsonl*)\" + head/tail/grep variants to disallowed_tools"
elif [ "$bash_deny" = "1" ]; then
    fail_ "Bash ledger paths denied but Read(.evolve/ledger.jsonl) NOT denied"
    echo "    Fix: add \"Read(.evolve/ledger.jsonl)\" to disallowed_tools"
else
    fail_ "neither Read(.evolve/ledger.jsonl) nor Bash ledger paths in disallowed_tools"
    echo "    Fix: add \"Read(.evolve/ledger.jsonl)\" + \"Bash(cat .evolve/ledger.jsonl*)\" etc."
fi

# --- Test 3: resume-quarantine reads denied via tool-perm --------------------
header "Test 3: .evolve/runs/cycle-*/.attempt-* reads denied (disallowed_tools)"
q_read_deny="0"
q_bash_deny="0"
while IFS= read -r entry; do
    [ -z "$entry" ] && continue
    case "$entry" in
        "Read(.evolve/runs/cycle-"*".attempt-"*) q_read_deny="1" ;;
        "Bash(cat .evolve/runs/cycle-"*".attempt-"*) q_bash_deny="1" ;;
        "Bash(head .evolve/runs/cycle-"*".attempt-"*) q_bash_deny="1" ;;
        "Bash(tail .evolve/runs/cycle-"*".attempt-"*) q_bash_deny="1" ;;
        "Bash(ls .evolve/runs/cycle-"*".attempt-"*) q_bash_deny="1" ;;
    esac
done < <(jq -r '.disallowed_tools[]?' "$PROFILE" 2>/dev/null)
if [ "$q_read_deny" = "1" ] && [ "$q_bash_deny" = "1" ]; then
    pass "quarantine .attempt-* reads denied via Read() + Bash() patterns"
else
    fail_ "quarantine .attempt-* reads not fully denied (read=$q_read_deny bash=$q_bash_deny)"
    echo "    Fix: ensure both \"Read(.evolve/runs/cycle-*/.attempt-*/**)\""
    echo "         and \"Bash(cat .evolve/runs/cycle-*/.attempt-*/**)\" (plus"
    echo "         head/tail/ls variants) are in disallowed_tools"
fi

# --- Test 4: resume-quarantine writes denied via sandbox.deny_subpaths --------
header "Test 4: .evolve/runs/cycle-*/.attempt-* writes denied (deny_subpaths)"
attempt_denied="0"
while IFS= read -r entry; do
    [ -z "$entry" ] && continue
    case "$entry" in
        *.attempt-*|*attempt-*) attempt_denied="1"; break ;;
    esac
done < <(jq -r '.sandbox.deny_subpaths[]?' "$PROFILE" 2>/dev/null)
if [ "$attempt_denied" = "1" ]; then
    pass "resume quarantine write path present in sandbox.deny_subpaths"
else
    fail_ "no .attempt-* deny entry in sandbox.deny_subpaths"
    echo "    Fix: add \".evolve/runs/cycle-*/.attempt-*\" to .sandbox.deny_subpaths"
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
