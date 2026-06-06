#!/usr/bin/env bash
# AC-ID:         cycle-6-004
# Description:   All 12 PM config files present on disk AND git-tracked (3 phases x phase.json + agent.md + agents/ mirror + profile.json) — dual-check per cycle-93 rule
# Evidence:      .evolve/phases/{risk-register,scope-baseline,dependency-map}/ + .evolve/profiles/ + agents/
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#4 — wave-pm-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

FILES="
.evolve/phases/risk-register/phase.json
.evolve/phases/risk-register/agent.md
agents/evolve-risk-register.md
.evolve/profiles/risk-register.json
.evolve/phases/scope-baseline/phase.json
.evolve/phases/scope-baseline/agent.md
agents/evolve-scope-baseline.md
.evolve/profiles/scope-baseline.json
.evolve/phases/dependency-map/phase.json
.evolve/phases/dependency-map/agent.md
agents/evolve-dependency-map.md
.evolve/profiles/dependency-map.json
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

echo "GREEN: all 12 PM config files present and git-tracked" >&2
exit 0
