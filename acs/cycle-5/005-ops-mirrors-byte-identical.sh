#!/usr/bin/env bash
# AC-ID:         cycle-5-005
# Description:   Ops agents/evolve-<name>.md mirrors are byte-identical to .evolve/phases/<name>/agent.md (diff exits 0 for all 3)
# Evidence:      agents/evolve-{incident-postmortem,runbook-draft,capacity-plan}.md vs .evolve/phases/*/agent.md
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#5 — wave-ops-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

fail=0
for name in incident-postmortem runbook-draft capacity-plan; do
  src=".evolve/phases/$name/agent.md"
  mirror="agents/evolve-$name.md"
  if [ ! -f "$src" ]; then
    echo "RED: $src missing — phase dir not authored" >&2; fail=1; continue
  fi
  if [ ! -f "$mirror" ]; then
    echo "RED: $mirror missing — mirror not written" >&2; fail=1; continue
  fi
  # Behavioral: diff is the byte-identity oracle (exit code, not grep).
  if ! diff -q "$src" "$mirror" >/dev/null 2>&1; then
    echo "RED: $mirror differs from $src — mirrors must be byte-identical" >&2; fail=1
  fi
done
[ "$fail" -eq 0 ] || exit 1

echo "GREEN: all 3 Ops mirrors byte-identical to phase-dir agent.md" >&2
exit 0
