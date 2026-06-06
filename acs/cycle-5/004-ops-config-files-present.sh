#!/usr/bin/env bash
# AC-ID:         cycle-5-004
# Description:   All 12 Ops config files present on disk AND git-tracked (3 phases x phase.json + agent.md + agents/ mirror + profile.json) — dual-check per cycle-93 rule
# Evidence:      .evolve/phases/{incident-postmortem,runbook-draft,capacity-plan}/ + .evolve/profiles/ + agents/
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#4 — wave-ops-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

FILES="
.evolve/phases/incident-postmortem/phase.json
.evolve/phases/incident-postmortem/agent.md
agents/evolve-incident-postmortem.md
.evolve/profiles/incident-postmortem.json
.evolve/phases/runbook-draft/phase.json
.evolve/phases/runbook-draft/agent.md
agents/evolve-runbook-draft.md
.evolve/profiles/runbook-draft.json
.evolve/phases/capacity-plan/phase.json
.evolve/phases/capacity-plan/agent.md
agents/evolve-capacity-plan.md
.evolve/profiles/capacity-plan.json
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

echo "GREEN: all 12 Ops config files present and git-tracked" >&2
exit 0
