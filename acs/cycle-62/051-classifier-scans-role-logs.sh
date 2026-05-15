#!/usr/bin/env bash
# ACS predicate 051 — cycle 62
# Verifies classify_cycle_failure() in evolve-loop-dispatch.sh scans per-role
# stdout/stderr logs for infrastructure markers (not just orchestrator-report.md).
#
# Cycle 61 demonstrated the gap: memo's API 529s landed in memo-stdout.log,
# the classifier only scanned orchestrator-report.md, and the cycle was
# mis-classified as INTEGRITY-BREACH instead of `infrastructure`.
#
# AC-ID: cycle-62-051
# Description: classifier-scans-role-logs
# Evidence: code grep + behavioral fixture (memo-stdout.log with 529)
# Author: builder (manual fix, Step 3 of plan)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: plan Step 3 (B5)
#
# metadata:
#   id: 051-classifier-scans-role-logs
#   cycle: 62
#   task: classifier-per-role-scan
#   severity: MEDIUM

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
DISPATCH="$REPO_ROOT/scripts/dispatch/evolve-loop-dispatch.sh"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
rc=0

# ── Extract classify_cycle_failure into a sourceable temp file ────────────────
# The dispatcher doesn't have a `if BASH_SOURCE == 0` guard, so we can't source
# it whole. Extract just the function body.
FUNC_FILE="$TMP/classify.sh"
awk '
    /^classify_cycle_failure\(\) \{/ { capture=1 }
    capture { print }
    capture && /^\}$/ { capture=0; exit }
' "$DISPATCH" > "$FUNC_FILE"

if [ ! -s "$FUNC_FILE" ]; then
    echo "RED PRE: could not extract classify_cycle_failure from $DISPATCH"
    exit 1
fi

# Set up fixture workspace
FIXTURE_RUNS="$TMP/runs"
mkdir -p "$FIXTURE_RUNS/cycle-999"
# Orchestrator-report with NO infra markers (must rely on memo-stdout.log).
cat > "$FIXTURE_RUNS/cycle-999/orchestrator-report.md" << 'EOF'
# Orchestrator Report — Cycle 999
## Goal
Test fixture.
## Verdict
SHIPPED
EOF

# ── AC1: behavioral — memo 529 in memo-stdout.log → classifier returns 'infrastructure' ──
cat > "$FIXTURE_RUNS/cycle-999/memo-stdout.log" << 'EOF'
{"api_error_status":529,"error":"Overloaded","retry_count":3}
... rate_limit hit during memo phase
EOF

# Source the extracted function with RUNS_DIR pointing at our fixture
result=$(RUNS_DIR="$FIXTURE_RUNS" bash -c "source $FUNC_FILE; classify_cycle_failure 999" 2>&1)
if [ "$result" = "infrastructure" ]; then
    echo "GREEN AC1: classifier returns 'infrastructure' when memo-stdout.log has 529"
else
    echo "RED AC1: classifier returned '$result' (expected 'infrastructure') — per-role scan not implemented"
    rc=1
fi

# ── AC2 (anti-tautology): remove 529 from memo log → integrity-breach ─────────
echo "{}" > "$FIXTURE_RUNS/cycle-999/memo-stdout.log"
result_clean=$(RUNS_DIR="$FIXTURE_RUNS" bash -c "source $FUNC_FILE; classify_cycle_failure 999" 2>&1)
if [ "$result_clean" = "integrity-breach" ]; then
    echo "GREEN AC2 (anti-tautology): without 529, classifier correctly falls through to integrity-breach"
else
    echo "RED AC2 (anti-tautology): without 529, got '$result_clean' (expected 'integrity-breach') — classifier is too permissive"
    rc=1
fi

# ── AC3: code-level — function body references per-role logs ──────────────────
if grep -qE '(stdout|stderr)\.log' "$FUNC_FILE"; then
    echo "GREEN AC3: classifier body references *-stdout.log or *-stderr.log glob"
else
    echo "RED AC3: classifier body has no per-role log glob — only orchestrator-report.md"
    rc=1
fi

exit "$rc"
