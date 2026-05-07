#!/usr/bin/env bash
#
# orchestrator-sandbox-coverage-test.sh — Drift-detection for orchestrator
# sandbox classification (v8.13.7).
#
# WHY THIS EXISTS
#
# The cycle 8215 sandbox fix uses a *broaden-then-deny-by-enumeration* model:
#  1. write_subpaths includes ".evolve/cycle-state*" which the generator
#     dirname()s to ".evolve/" — broadening writes to the whole .evolve/ dir.
#  2. deny_subpaths then re-restricts every top-level .evolve/ entry that is
#     NOT one of the three intended write targets (cycle-state*, ledger.jsonl,
#     runs/cycle-*).
#
# This is correct at the snapshot in time of cycle 8215 — every on-disk
# top-level .evolve/ entry is either intended-writable or in deny_subpaths.
# But the model inverts the safe default: any FUTURE top-level addition
# becomes OS-writable by default unless someone updates the deny list.
#
# Cycle 8215 audit (rc3) MEDIUM-1: "There is no test that fails when .evolve/
# gains an entry not in deny_subpaths and not in write_subpaths."
#
# This script IS that test. It reads the live `.evolve/` top-level layout and
# the orchestrator profile's classification lists, and fails loud if any
# on-disk entry is unclassified.
#
# Usage: bash scripts/orchestrator-sandbox-coverage-test.sh
# Exit 0 = every entry classified; exit 1 = drift detected.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROFILE="$REPO_ROOT/.evolve/profiles/orchestrator.json"
EVOLVE_DIR="${EVOLVE_DIR_OVERRIDE:-$REPO_ROOT/.evolve}"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# === Test 1: prerequisites =================================================
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

# === Test 2: classification coverage =======================================
header "Test 2: every on-disk .evolve/ top-level entry is classified"

# Collect on-disk entries (skip . and ..).
on_disk=()
while IFS= read -r entry; do
    [ -n "$entry" ] && on_disk+=("$entry")
done < <(ls -A "$EVOLVE_DIR" 2>/dev/null | sort)

if [ "${#on_disk[@]}" = "0" ]; then
    fail_ "could not enumerate $EVOLVE_DIR (empty or inaccessible)"
else
    write_subpaths=$(jq -r '.sandbox.write_subpaths[]?' "$PROFILE")
    deny_subpaths=$(jq -r '.sandbox.deny_subpaths[]?' "$PROFILE")

    # Build expected-classification table.
    classify() {
        local entry="$1"
        local full=".evolve/$entry"

        # Check write_subpaths — supports three match modes:
        #   1. exact            (wp == full)
        #   2. prefix-of-glob   (full starts with wp's literal prefix; handles
        #                        ".evolve/cycle-state*" matching ".evolve/cycle-state.json")
        #   3. parent-of-glob   (full == wp's dirname; handles ".evolve/runs/cycle-*"
        #                        matching ".evolve/runs" — the parent dir is
        #                        intended-writable as the container for the glob targets)
        #   4. dir prefix       (full is under wp/; handles literal dir entries)
        local wp
        while IFS= read -r wp; do
            [ -z "$wp" ] && continue
            if [ "$full" = "$wp" ]; then
                echo "WRITE"; return 0
            fi
            if [[ "$wp" == *"*"* ]]; then
                # Glob: check the no-trailing-* prefix AND the parent dir.
                local prefix="${wp%\*}"
                prefix="${prefix%/}"
                if [[ "$full" == "$prefix"* ]] || [ "$full" = "$(dirname "$wp")" ]; then
                    echo "WRITE"; return 0
                fi
            else
                # Literal dir: anything under it is also a write target.
                if [[ "$full" == "$wp"/* ]]; then
                    echo "WRITE"; return 0
                fi
            fi
        done <<< "$write_subpaths"

        # Check deny_subpaths — supports literal and dir prefix.
        local dp
        while IFS= read -r dp; do
            [ -z "$dp" ] && continue
            if [ "$full" = "$dp" ] \
               || [[ "$full" == "$dp"/* ]]; then
                echo "DENY"
                return 0
            fi
        done <<< "$deny_subpaths"

        echo "UNCLASSIFIED"
        return 1
    }

    unclassified=()
    classified=()
    for entry in "${on_disk[@]}"; do
        c=$(classify "$entry")
        if [ "$c" = "UNCLASSIFIED" ]; then
            unclassified+=("$entry")
        else
            classified+=("$entry:$c")
        fi
    done

    if [ "${#unclassified[@]}" = "0" ]; then
        pass "all ${#on_disk[@]} top-level entries classified (write or deny)"
        echo "  classified: ${classified[*]}"
    else
        fail_ "${#unclassified[@]} unclassified entries — drift detected"
        echo "  UNCLASSIFIED:"
        for e in "${unclassified[@]}"; do
            echo "    .evolve/$e"
        done
        echo
        echo "  Add each to either:"
        echo "    .sandbox.write_subpaths (intentional write target)"
        echo "    .sandbox.deny_subpaths (forbidden — blocks orchestrator writes)"
        echo "  in $PROFILE"
    fi
fi

# === Test 3: write_subpaths intentions match design ========================
# Sanity: there must be at least 3 distinct write_subpaths (cycle-state,
# ledger, runs). If someone collapses or removes one, the structural
# guarantee of the dispatcher breaks.
header "Test 3: write_subpaths contains the three required entries"
required=(".evolve/runs/cycle-*" ".evolve/cycle-state*" ".evolve/ledger.jsonl")
missing_required=()
for r in "${required[@]}"; do
    if ! jq -e --arg r "$r" '.sandbox.write_subpaths | index($r)' "$PROFILE" >/dev/null 2>&1; then
        missing_required+=("$r")
    fi
done
if [ "${#missing_required[@]}" = "0" ]; then
    pass "all 3 required write targets present"
else
    fail_ "missing required write targets: ${missing_required[*]}"
    echo "  These are essential for the orchestrator to function:"
    echo "    .evolve/runs/cycle-*       — orchestrator-report.md + handoff"
    echo "    .evolve/cycle-state*       — cycle-state.sh advance() + .tmp.\$\$"
    echo "    .evolve/ledger.jsonl       — subagent-run.sh appends per-invocation"
fi

# === Test 4: state.json must be denied =====================================
# state.json carries the TOFU pin for ship.sh; orchestrator must never write it.
header "Test 4: .evolve/state.json is in deny_subpaths"
if jq -e '.sandbox.deny_subpaths | index(".evolve/state.json")' "$PROFILE" >/dev/null 2>&1; then
    pass ".evolve/state.json correctly denied"
else
    fail_ ".evolve/state.json is NOT denied — orchestrator could overwrite TOFU pin"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
