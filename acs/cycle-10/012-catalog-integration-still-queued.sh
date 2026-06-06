#!/usr/bin/env bash
# acs-predicate: config-check — wave-status table rows are an inherent
# doc-presence check; there is no subprocess that emits the catalog status.
# Grep waiver per tdd-engineer predicate-quality classification.
# AC-ID:         cycle-10-012
# Description:   Negative/scope guard — Integration wave row remains "⬜ queued" (cycle 10 ships ONLY the Product wave; one-wave-per-cycle discipline). NOTE: Ops is ✅ done since cycle 5 — do NOT check it for queued (cycle-9 eval C8 defect).
# Evidence:      docs/architecture/domain-phase-catalog.md wave status table
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#8 — wave-product-discovery-tdd-and-phases
# NOTE: negative invariant — expected GREEN at RED baseline AND after build.

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

CATALOG="docs/architecture/domain-phase-catalog.md"
[ -f "$CATALOG" ] || { echo "RED: $CATALOG missing" >&2; exit 1; }

row=$(grep -E '^\| *Integration *\|' "$CATALOG" | head -1)
if [ -z "$row" ]; then
  echo "RED: Integration row missing from wave-status table" >&2; exit 1
fi
if ! echo "$row" | grep -q '⬜'; then
  echo "RED: Integration row no longer queued (unexpected flip): $row" >&2; exit 1
fi
if echo "$row" | grep -q '✅'; then
  echo "RED: Integration row incorrectly marked done: $row" >&2; exit 1
fi

echo "GREEN: Integration remains queued" >&2
exit 0
