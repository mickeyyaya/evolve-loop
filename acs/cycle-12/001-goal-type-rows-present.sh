#!/usr/bin/env bash
# acs-predicate: config-check — markdown table rows in an agent persona are an
# inherent doc-presence check; no subprocess emits the recipe table. Grep
# waiver per tdd-engineer predicate-quality classification.
# AC-ID:         cycle-12-001
# Description:   All 5 domain goal-type rows present in the Goal-Type Recipes
#                table of agents/evolve-router.md (spec §4.1).
# Evidence:      agents/evolve-router.md Goal-Type Recipes table
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md — extend-goal-type-recipes AC-1
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

ROUTER="agents/evolve-router.md"
[ -f "$ROUTER" ] || { echo "RED: $ROUTER missing" >&2; exit 1; }

for goal in project-management business-strategy accounting-close product-discovery ops-incident; do
  if ! grep -qE "^\| *${goal} *\|" "$ROUTER"; then
    echo "RED: goal-type row '$goal' not found in $ROUTER" >&2; exit 1
  fi
done

# Placement: rows must live inside the Goal-Type Recipes section, not pasted
# elsewhere in the persona.
if ! sed -n '/^## Goal-Type Recipes/,/^## /p' "$ROUTER" | grep -qE '^\| *project-management *\|'; then
  echo "RED: project-management row not inside the Goal-Type Recipes section" >&2; exit 1
fi

echo "GREEN: all 5 domain goal-type rows present in Goal-Type Recipes table" >&2
exit 0
