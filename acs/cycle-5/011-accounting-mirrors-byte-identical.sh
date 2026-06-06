#!/usr/bin/env bash
# AC-ID:         cycle-5-011
# Description:   3 accounting mirror files (agents/evolve-{account-reconcile,variance-analysis,close-checklist}.md) present, git-tracked, and byte-identical to their phase-dir agent.md — root-cause fix for cycles 3-4 ACS 002
# Evidence:      agents/evolve-*.md vs .evolve/phases/*/agent.md (diff exit code) + git ls-files dual-check
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#11 — accounting-carry-forward-mirrors

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

fail=0
for name in account-reconcile variance-analysis close-checklist; do
  src=".evolve/phases/$name/agent.md"
  mirror="agents/evolve-$name.md"
  if [ ! -f "$src" ]; then
    echo "RED: $src missing — accounting phase dir not in this tree (carry-forward not landed)" >&2; fail=1; continue
  fi
  if [ ! -f "$mirror" ]; then
    echo "RED: $mirror missing — mirror not written" >&2; fail=1; continue
  fi
  # Behavioral: diff is the byte-identity oracle (exit code, not grep).
  if ! diff -q "$src" "$mirror" >/dev/null 2>&1; then
    echo "RED: $mirror differs from $src — mirrors must be byte-identical" >&2; fail=1; continue
  fi
  # Dual-check (cycle-93 rule): BOTH sides must be git-tracked — the cycles 3-4
  # failure mode was exactly "exists on disk, never committed".
  for f in "$src" "$mirror"; do
    if ! git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
      echo "RED: $f untracked — exists on disk but not committed (cycle 3-4 defect mode)" >&2; fail=1
    fi
  done
done
[ "$fail" -eq 0 ] || exit 1

echo "GREEN: all 3 accounting mirrors present, tracked, byte-identical" >&2
exit 0
