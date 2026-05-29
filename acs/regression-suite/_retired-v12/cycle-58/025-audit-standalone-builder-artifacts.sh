#!/usr/bin/env bash
# ACS predicate 025 — cycle 58
# Verifies that init-standalone-cycle.sh + check-phase-inputs.sh correctly
# handle audit phase with pre-populated builder artifacts. check-phase-inputs
# exits 0 when both build-report.md + tester-report.md exist, and exits 1
# when tester-report.md is absent. No orchestrator ledger entries for cycle 9999.
#
# AC-ID: cycle-58-025
# Description: audit phase init and input-check succeed with builder artifacts present
# Evidence: scripts/utility/init-standalone-cycle.sh, scripts/utility/check-phase-inputs.sh
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-58-3 (audit standalone flow)
#
# metadata:
#   id: 025-audit-runs-standalone-given-builder-artifacts
#   cycle: 58
#   task: adr5-standalone-phase-runners
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
INIT_SCRIPT="$REPO_ROOT/scripts/utility/init-standalone-cycle.sh"
CHECK_INPUTS="$REPO_ROOT/scripts/utility/check-phase-inputs.sh"

for _f in "$INIT_SCRIPT" "$CHECK_INPUTS"; do
    if [ ! -f "$_f" ]; then
        echo "RED: required file not found: $_f"
        exit 1
    fi
done

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

FIXTURE_CYCLE=9999
WS="$TMP/.evolve/runs/cycle-$FIXTURE_CYCLE"
rc=0

# Set up: workspace with audit inputs (build-report.md + tester-report.md)
mkdir -p "$WS"
printf '# Build Report — fixture\n**Status:** PASS\nFixture for predicate 025.\n' \
    > "$WS/build-report.md"
printf '# Tester Report — fixture\nPredicates verified by tester.\n' \
    > "$WS/tester-report.md"

# ── AC1: init-standalone-cycle.sh for audit exits 0 ──────────────────────────
set +e
init_out=$(EVOLVE_PROJECT_ROOT="$TMP" bash "$INIT_SCRIPT" \
    --cycle "$FIXTURE_CYCLE" --phase audit --force-overwrite 2>&1)
init_rc=$?
set -e

if [ "$init_rc" -eq 0 ]; then
    echo "GREEN AC1: init-standalone-cycle.sh --phase audit exits 0"
else
    echo "RED AC1: init-standalone-cycle.sh --phase audit failed (rc=$init_rc): $init_out"
    rc=1
fi

# ── AC2: cycle-state.json has phase=audit ────────────────────────────────────
CYCLE_STATE="$TMP/.evolve/cycle-state.json"
if [ ! -f "$CYCLE_STATE" ]; then
    echo "RED AC2: cycle-state.json not created"
    rc=1
else
    got_phase=$(jq -r '.phase // empty' "$CYCLE_STATE" 2>/dev/null || true)
    if [ "$got_phase" = "audit" ]; then
        echo "GREEN AC2: cycle-state.json has phase=audit"
    else
        echo "RED AC2: expected phase=audit, got phase=$got_phase"
        rc=1
    fi
fi

# ── AC3: check-phase-inputs.sh audit exits 0 with artifacts present ──────────
# Audit phase inputs: build-report.md + tester-report.md (no state fields)
set +e
check_out=$(EVOLVE_PROJECT_ROOT="$TMP" bash "$CHECK_INPUTS" audit "$FIXTURE_CYCLE" 2>&1)
check_rc=$?
set -e

if [ "$check_rc" -eq 0 ]; then
    echo "GREEN AC3: check-phase-inputs.sh audit exits 0 when artifacts present"
else
    echo "RED AC3: check-phase-inputs.sh audit exits $check_rc. Output: $check_out"
    rc=1
fi

# ── AC4 (anti-tautology): exits 1 when tester-report.md is absent ────────────
TMP3=$(mktemp -d)
trap 'rm -rf "$TMP3"' EXIT
WS3="$TMP3/.evolve/runs/cycle-$FIXTURE_CYCLE"
mkdir -p "$WS3"
# Only build-report.md, no tester-report.md
printf '# Build Report — fixture\n**Status:** PASS\n' > "$WS3/build-report.md"

set +e
EVOLVE_PROJECT_ROOT="$TMP3" bash "$CHECK_INPUTS" audit "$FIXTURE_CYCLE" > /dev/null 2>&1
ac4_rc=$?
set -e

if [ "$ac4_rc" -eq 1 ]; then
    echo "GREEN AC4 (anti-tautology): check-phase-inputs exits 1 when tester-report.md absent"
else
    echo "RED AC4 (anti-tautology): expected exit 1 without tester-report.md, got $ac4_rc"
    rc=1
fi

# ── AC5: no orchestrator ledger entries for cycle 9999 ────────────────────────
# The standalone init path must not create ledger entries.
LEDGER="$TMP/.evolve/ledger.jsonl"
if [ -f "$LEDGER" ]; then
    # Check for any entries referencing cycle 9999
    found_entries=0
    while IFS= read -r line; do
        case "$line" in
            *'"cycle_id":9999'*|*'"cycle":9999'*) found_entries=$((found_entries + 1)) ;;
        esac
    done < "$LEDGER"
    if [ "$found_entries" -eq 0 ]; then
        echo "GREEN AC5: no ledger entries for cycle 9999"
    else
        echo "RED AC5: found $found_entries ledger entries for cycle 9999 (should be 0)"
        rc=1
    fi
else
    echo "GREEN AC5: no ledger file created (expected for standalone init)"
fi

exit "$rc"
