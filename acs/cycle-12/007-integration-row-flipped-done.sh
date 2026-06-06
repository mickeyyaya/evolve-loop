#!/usr/bin/env bash
# acs-predicate: config-check — wave-status table rows are an inherent
# doc-presence check; there is no subprocess that emits the catalog status.
# Grep waiver per tdd-engineer predicate-quality classification.
# AC-ID:         cycle-12-007
# Description:   Integration wave row in domain-phase-catalog.md §5 flipped to
#                "✅ done (cycle 12)" — and the 5 neighbouring wave rows keep
#                their ✅ done status (cycle-9 eval C8 neighbour-clobber class).
# Evidence:      docs/architecture/domain-phase-catalog.md wave status table
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md — extend-phase-core-values AC-2
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

CATALOG="docs/architecture/domain-phase-catalog.md"
[ -f "$CATALOG" ] || { echo "RED: $CATALOG missing" >&2; exit 1; }

row=$(grep -E '^\| *Integration *\|' "$CATALOG" | head -1)
[ -n "$row" ] || { echo "RED: Integration row missing from wave-status table" >&2; exit 1; }

if ! echo "$row" | grep -q '✅'; then
  echo "RED: Integration row not marked ✅ done: $row" >&2; exit 1
fi
if ! echo "$row" | grep -q 'cycle 12'; then
  echo "RED: Integration row does not record 'cycle 12' (BA-H2 traceability): $row" >&2; exit 1
fi

# Neighbour-row integrity: the flip must not clobber the other 5 waves.
for wave in PM Strategy Accounting Product Ops; do
  wrow=$(grep -E "^\| *${wave} *\|" "$CATALOG" | head -1)
  if ! echo "$wrow" | grep -q '✅'; then
    echo "RED: wave '$wave' lost its ✅ done status: ${wrow:-<absent>}" >&2; exit 1
  fi
done

echo "GREEN: Integration flipped to ✅ done (cycle 12); neighbour rows intact" >&2
exit 0
