#!/usr/bin/env bash
# acs-predicate: config-check — wave-status table rows are an inherent doc-presence
# check; there is no subprocess that emits the catalog status. Grep waiver per
# tdd-engineer predicate-quality classification (Auditor reviews validity).
# AC-ID:         cycle-6-012
# Description:   Negative — Strategy, Product, Integration rows remain "⬜ queued" (PM flip must not disturb in-flight waves; Accounting/Ops already ✅ from cycle 5)
# Evidence:      docs/architecture/domain-phase-catalog.md wave status table (eval C3+C4)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#2 — wave-pm-status-flip
# NOTE: negative invariant — expected GREEN at RED baseline AND after build.

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

CATALOG="docs/architecture/domain-phase-catalog.md"
[ -f "$CATALOG" ] || { echo "RED: $CATALOG missing" >&2; exit 1; }

fail=0
for wave in Strategy Product Integration; do
  row=$(grep -E "^\| *$wave *\|" "$CATALOG" | head -1)
  if [ -z "$row" ]; then
    echo "RED: $wave row missing from wave-status table" >&2; fail=1; continue
  fi
  if ! echo "$row" | grep -q '⬜'; then
    echo "RED: $wave row no longer queued (unexpected flip): $row" >&2; fail=1
  fi
  if echo "$row" | grep -q '✅'; then
    echo "RED: $wave row incorrectly marked done: $row" >&2; fail=1
  fi
done
[ "$fail" -eq 0 ] || exit 1

echo "GREEN: Strategy, Product, Integration remain queued" >&2
exit 0
