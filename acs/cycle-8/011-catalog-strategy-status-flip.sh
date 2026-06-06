#!/usr/bin/env bash
# acs-predicate: config-check — wave-status table row is an inherent doc-presence
# check; there is no subprocess that emits the catalog status. Grep waiver per
# tdd-engineer predicate-quality classification (Auditor reviews validity).
# AC-ID:         cycle-8-011
# Description:   domain-phase-catalog.md §5 wave-status table: Strategy row flipped from "⬜ queued" to ✅ done (cycle 8)
# Evidence:      docs/architecture/domain-phase-catalog.md wave status table
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#6 — wave-business-strategy-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

CATALOG="docs/architecture/domain-phase-catalog.md"
[ -f "$CATALOG" ] || { echo "RED: $CATALOG missing" >&2; exit 1; }

row=$(grep -E '^\| *Strategy *\|' "$CATALOG" | head -1)
if [ -z "$row" ]; then
  echo "RED: Strategy row missing from wave-status table" >&2; exit 1
fi
# Negative: must no longer be queued.
if echo "$row" | grep -q '⬜'; then
  echo "RED: Strategy row still queued: $row" >&2; exit 1
fi
# Positive: must be marked done.
if ! echo "$row" | grep -q '✅'; then
  echo "RED: Strategy row not marked ✅ done: $row" >&2; exit 1
fi

echo "GREEN: Strategy wave status flipped to done" >&2
exit 0
