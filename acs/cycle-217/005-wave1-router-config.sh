#!/usr/bin/env bash
# ACS cycle-217 / Task-3 (wave1-router-config) — the advisor-integration pair
# from micro-phase-catalog.md §4:
#   AC1: docs/architecture/phase-registry.json config.max_optional_insertions == 6
#   AC2: agents/evolve-router.md carries the goal-type recipe table (7 goal types)
#   AC3: bugfix row wires fault-localization + bug-reproduction
#   AC4: recipes documented as guidance (ClampPlanToFloor is the safety net)
#   AC5: registry JSON stays valid AND still loads through the real Go loader
#
# Behavioral portion: python3 parses the exact registry bytes the Go loader
# reads, and `evolve phases list` exercises the load end-to-end (a registry
# broken by the edit fails here). The recipe-table rows are persona prose —
# inherently content-presence checks.
# acs-predicate: config-check (recipe-table greps on agents/evolve-router.md)
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

BIN="${EVOLVE_GO_BIN:-$TOP/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$TOP/go/evolve"
[ -x "$BIN" ] || { echo "RED: evolve binary not found" >&2; exit 1; }

REGISTRY="docs/architecture/phase-registry.json"
ROUTER="agents/evolve-router.md"

# AC5 — valid JSON (and AC1 — the exact cap value), via a real parser.
python3 -c "import json,sys; d=json.load(open('$REGISTRY')); sys.exit(0 if d['config']['max_optional_insertions']==6 else 1)" \
  || { echo "RED: $REGISTRY config.max_optional_insertions != 6 (or JSON invalid)" >&2; exit 1; }

# AC5 behavioral — the Go loader still accepts the edited registry.
EVOLVE_PROJECT_ROOT="$TOP" "$BIN" phases list >/dev/null 2>&1 \
  || { echo "RED: evolve phases list fails — registry edit broke the loader" >&2; exit 1; }

# AC2 — recipe section + all 7 goal-type rows.
grep -q "^## Goal-Type Recipes" "$ROUTER" \
  || { echo "RED: $ROUTER missing '## Goal-Type Recipes' section" >&2; exit 1; }
for gt in bugfix feature refactor security performance release docs; do
  grep -E '^\|' "$ROUTER" | grep -qi "$gt" \
    || { echo "RED: recipe table missing goal type: $gt" >&2; exit 1; }
done

# AC3 — bugfix row wires the wave-1 bugfix chain.
grep -E '^\|' "$ROUTER" | grep -i bugfix | grep -q "fault-localization" \
  || { echo "RED: bugfix recipe row does not reference fault-localization" >&2; exit 1; }
grep -E '^\|' "$ROUTER" | grep -i bugfix | grep -q "bug-reproduction" \
  || { echo "RED: bugfix recipe row does not reference bug-reproduction" >&2; exit 1; }

# AC4 — guidance-not-law note.
grep -q "ClampPlanToFloor" "$ROUTER" \
  || { echo "RED: recipe section does not cite ClampPlanToFloor as the safety net" >&2; exit 1; }

echo "GREEN: router recipe table + max_optional_insertions=6 in place; registry loads" >&2
exit 0
