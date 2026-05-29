#!/usr/bin/env bash
# ACS predicate 052 — cycle 62
# Verifies scout-grounding-check.sh correctly distinguishes grounded vs
# fabricated working-tree claims in scout-report.md.
#
# AC-ID: cycle-62-052
# Description: scout-findings-grounded
# Evidence: 4 ACs — fabricated path RED, real path GREEN, empty section GREEN,
#           cycle-61 regression replay
# Author: builder (manual fix, Step 4 of plan)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: plan Step 4 (B1)
#
# metadata:
#   id: 052-scout-findings-grounded
#   cycle: 62
#   task: scout-grounding-check
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
CHECK="$REPO_ROOT/scripts/lifecycle/scout-grounding-check.sh"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
rc=0

if [ ! -x "$CHECK" ]; then
    echo "RED PRE: scout-grounding-check.sh missing or not executable at $CHECK"
    exit 1
fi

# ── AC1: fabricated path claim must FAIL the check ─────────────────────────────
FAB="$TMP/fabricated-scout.md"
cat > "$FAB" << 'EOF'
# Scout Report — Test

## Discovery Summary
Test fixture.

## Key Findings

| ID | Area | Finding | Severity |
|----|------|---------|----------|
| F-1 | Working tree | `scripts/this_path_does_not_exist_anywhere.sh` +90 lines: fabricated claim | HIGH |
EOF

if cd "$REPO_ROOT" && bash "$CHECK" "$FAB" >/dev/null 2>&1; then
    echo "RED AC1: check passed for a fabricated path (false negative — check ineffective)"
    rc=1
else
    echo "GREEN AC1: check correctly RED'd a fabricated path claim"
fi

# ── AC2: real path with claim must PASS the check ─────────────────────────────
REAL="$TMP/real-scout.md"
cat > "$REAL" << 'EOF'
# Scout Report — Test

## Key Findings

| ID | Area | Finding | Severity |
|----|------|---------|----------|
| F-1 | Working tree | `scripts/cli_adapters/gemini.sh` modified | HIGH |
EOF

if cd "$REPO_ROOT" && bash "$CHECK" "$REAL" >/dev/null 2>&1; then
    echo "GREEN AC2: check passed for a real path with claim"
else
    echo "RED AC2: check RED'd a real path that exists in git state"
    rc=1
fi

# ── AC3 (anti-tautology): empty Key Findings section → GREEN ─────────────────
EMPTY="$TMP/empty-scout.md"
cat > "$EMPTY" << 'EOF'
# Scout Report — Test

## Discovery Summary
Nothing to report.

## Carryover Decisions
None.
EOF

if cd "$REPO_ROOT" && bash "$CHECK" "$EMPTY" >/dev/null 2>&1; then
    echo "GREEN AC3 (anti-tautology): empty/missing Key Findings section is vacuously grounded"
else
    echo "RED AC3 (anti-tautology): empty Key Findings caused unexpected RED — check is over-strict"
    rc=1
fi

# ── AC4: rows with path-but-no-claim are SKIPPED (not falsely RED'd) ───────────
NOCLAIM="$TMP/noclaim-scout.md"
cat > "$NOCLAIM" << 'EOF'
# Scout Report — Test

## Key Findings

| ID | Area | Finding | Severity |
|----|------|---------|----------|
| F-1 | Reference | The file `scripts/this_path_does_not_exist.sh` is described in some context | LOW |
EOF

# This row has a path but NO claim marker (+N lines, untracked, modified, etc.)
# The check should skip it (not RED).
if cd "$REPO_ROOT" && bash "$CHECK" "$NOCLAIM" >/dev/null 2>&1; then
    echo "GREEN AC4: rows with no quantitative claim are correctly skipped"
else
    echo "RED AC4: check RED'd a path-only reference with no claim — over-grounding"
    rc=1
fi

exit "$rc"
