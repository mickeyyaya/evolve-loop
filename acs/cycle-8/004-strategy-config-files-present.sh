#!/usr/bin/env bash
# AC-ID:         cycle-8-004
# Description:   All 12 Strategy config files present on disk AND git-tracked (3 phases x phase.json + agent.md + agents/ mirror + profile.json) — dual-check per cycle-93 rule
# Evidence:      .evolve/phases/{forces-analysis,market-sizing,okr-draft}/ + .evolve/profiles/ + agents/
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#2 — wave-business-strategy-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

FILES="
.evolve/phases/forces-analysis/phase.json
.evolve/phases/forces-analysis/agent.md
agents/evolve-forces-analysis.md
.evolve/profiles/forces-analysis.json
.evolve/phases/market-sizing/phase.json
.evolve/phases/market-sizing/agent.md
agents/evolve-market-sizing.md
.evolve/profiles/market-sizing.json
.evolve/phases/okr-draft/phase.json
.evolve/phases/okr-draft/agent.md
agents/evolve-okr-draft.md
.evolve/profiles/okr-draft.json
"

fail=0
for f in $FILES; do
  # Check 1: disk presence
  if [ ! -f "$f" ]; then
    echo "RED: $f missing on disk" >&2; fail=1; continue
  fi
  # Check 2: git tracking — catches gitignored worktree files (cycle-92 defect)
  if ! git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
    echo "RED: $f untracked — may be gitignored or not staged" >&2; fail=1
  fi
done
[ "$fail" -eq 0 ] || exit 1

echo "GREEN: all 12 Strategy config files present and git-tracked" >&2
exit 0
