#!/usr/bin/env bash
# ACS predicate 009 — cycle 50
# evolve-tester.md uses dual-var WORKTREE fallback pattern
#
# AC-ID: cycle-50-009
# Description: evolve-tester.md uses EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:- dual-var pattern
# Evidence: agents/evolve-tester.md:94
# Author: builder (evolve-builder)
# Created: 2026-05-14T13:55:00Z
# Acceptance-of: build-report.md AC-9
#
# metadata:
#   id: 009-tester-dual-var-worktree-pattern
#   cycle: 50
#   task: research-cache-phase-b
#   severity: MEDIUM
set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
TESTER="$REPO_ROOT/agents/evolve-tester.md"
[ -f "$TESTER" ] || { echo "RED: $TESTER not found"; exit 1; }

rc=0

# AC1: dual-var pattern present — EVOLVE_WORKTREE_PATH primary, WORKTREE_PATH fallback
# This verifies the Phase B worktree fix (predicate WORKTREE fix from cycle-50 commit b386d8a)
if ! grep -q 'EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-' "$TESTER"; then
    echo "RED AC1: dual-var pattern 'EVOLVE_WORKTREE_PATH:-\${WORKTREE_PATH:-' not found in evolve-tester.md"
    echo "       (predicate WORKTREE fix from cycle-50 missing)"
    rc=1
else
    echo "GREEN AC1: dual-var WORKTREE fallback pattern found in evolve-tester.md"
fi

# AC2: the pattern includes the git fallback (defense-in-depth)
if ! grep -q 'git rev-parse --show-toplevel' "$TESTER"; then
    echo "RED AC2: git rev-parse fallback not found in evolve-tester.md WORKTREE pattern"
    rc=1
else
    echo "GREEN AC2: git rev-parse fallback present in evolve-tester.md WORKTREE pattern"
fi

exit $rc
