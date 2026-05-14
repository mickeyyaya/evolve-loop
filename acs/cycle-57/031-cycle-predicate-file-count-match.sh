#!/usr/bin/env bash
# ACS predicate 031 — cycle 57
# Verifies that the count of .sh files in acs/cycle-N/ matches
# acs-verdict.json:predicate_suite.this_cycle_count. Prevents silent scope
# omissions where a predicate file exists on disk but was not included in the
# ACS suite run (EVOLVE_PROJECT_ROOT mismatch, wrong CYCLE var, etc.)
#
# AC-ID: cycle-57-031
# Description: acs/cycle-57/ file count matches acs-verdict.json this_cycle_count
# Evidence: acs/cycle-57/*.sh, .evolve/runs/cycle-57/acs-verdict.json
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-57-5
#
# metadata:
#   id: 031-cycle-predicate-file-count-match
#   cycle: 57
#   task: egps-integrity-regression
#   severity: HIGH

set -uo pipefail

CYCLE="${CYCLE:-57}"
WORKSPACE="${WORKSPACE:-${EVOLVE_PROJECT_ROOT:-.}/.evolve/runs/cycle-${CYCLE}}"
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"

ACS_VERDICT="$WORKSPACE/acs-verdict.json"
CYCLE_ACS_DIR="$REPO_ROOT/acs/cycle-${CYCLE}"

# ── Pre-flight ────────────────────────────────────────────────────────────────
if [ ! -f "$ACS_VERDICT" ]; then
    echo "RED: acs-verdict.json not found at $ACS_VERDICT — run-acs-suite.sh must run first"
    exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
    echo "RED: jq not available — required for acs-verdict.json parsing"
    exit 1
fi

rc=0

# ── AC1: acs/cycle-N/ directory exists ───────────────────────────────────────
if [ -d "$CYCLE_ACS_DIR" ]; then
    echo "GREEN AC1: acs/cycle-${CYCLE}/ directory exists at $CYCLE_ACS_DIR"
else
    echo "RED AC1: acs/cycle-${CYCLE}/ directory not found at $CYCLE_ACS_DIR"
    rc=1
fi

# ── AC2: acs-verdict.json has this_cycle_count field ─────────────────────────
this_cycle_count=$(jq -r '.predicate_suite.this_cycle_count // empty' "$ACS_VERDICT" 2>/dev/null || echo "")
if [ -n "$this_cycle_count" ]; then
    echo "GREEN AC2: acs-verdict.json has predicate_suite.this_cycle_count=$this_cycle_count"
else
    echo "RED AC2: acs-verdict.json missing predicate_suite.this_cycle_count field"
    rc=1
fi

# ── AC3: file count on disk matches this_cycle_count ─────────────────────────
if [ -d "$CYCLE_ACS_DIR" ]; then
    disk_count=$(find "$CYCLE_ACS_DIR" -maxdepth 1 -name "*.sh" -type f 2>/dev/null | wc -l | tr -d ' ')
else
    disk_count=0
fi

if [ -n "$this_cycle_count" ] && [ "$disk_count" = "$this_cycle_count" ]; then
    echo "GREEN AC3: disk file count ($disk_count) matches acs-verdict.json this_cycle_count ($this_cycle_count)"
elif [ -z "$this_cycle_count" ]; then
    echo "GREEN AC3: this_cycle_count not available — skipping count match (AC2 already failed)"
else
    echo "RED AC3: disk file count ($disk_count) does not match acs-verdict.json this_cycle_count ($this_cycle_count)"
    echo "  disk files: $(find "$CYCLE_ACS_DIR" -maxdepth 1 -name '*.sh' -type f 2>/dev/null | sort | tr '\n' ' ')"
    rc=1
fi

# ── AC4: this_cycle_count > 0 (non-empty predicate suite) ─────────────────────
if [ "${this_cycle_count:-0}" -gt 0 ]; then
    echo "GREEN AC4: this_cycle_count=$this_cycle_count > 0 — non-empty predicate suite for this cycle"
else
    echo "RED AC4: this_cycle_count=0 — this cycle produced no predicates; EGPS integrity gap"
    rc=1
fi

# ── AC5 (anti-tautology): all cycle predicate files are executable ────────────
not_exec_count=0
if [ -d "$CYCLE_ACS_DIR" ]; then
    while IFS= read -r pred_file; do
        [ -f "$pred_file" ] || continue
        if [ ! -x "$pred_file" ]; then
            echo "RED AC5 (anti-tautology): predicate not executable: $pred_file"
            not_exec_count=$((not_exec_count + 1))
        fi
    done < <(find "$CYCLE_ACS_DIR" -maxdepth 1 -name "*.sh" -type f 2>/dev/null | sort)
fi
if [ "$not_exec_count" -eq 0 ]; then
    echo "GREEN AC5 (anti-tautology): all cycle-${CYCLE} predicates are executable"
else
    echo "RED AC5 (anti-tautology): $not_exec_count predicate(s) not executable — chmod +x required"
    rc=1
fi

exit "$rc"
