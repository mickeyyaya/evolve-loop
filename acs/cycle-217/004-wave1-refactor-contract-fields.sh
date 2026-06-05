#!/usr/bin/env bash
# ACS cycle-217 / Task-2 AC2+AC3+AC4 — refactor-trio contract fields per
# micro-phase-catalog.md §3 Wave 1:
#   - behavior-compare is the GATE: fail_if_signal behavior.preserved=="==false"
#     + require_sections ["Comparison","Verdict"]
#   - smell-scan is archetype:evaluate, writes_source:false, fail_if_empty:true
#   - behavior-baseline declares outputs.signals behavior.preserved +
#     behavior.delta_count
#   - the pair STRADDLES build: baseline after=="tdd", compare after=="build"
#     (the golden-master safety net is worthless if both run on the same side)
#
# Mixed predicate: behavioral anchor = `evolve phases validate` on all three;
# jq asserts the field contract on the exact bytes the Go loader reads.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

BIN="${EVOLVE_GO_BIN:-$TOP/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$TOP/go/evolve"
[ -x "$BIN" ] || { echo "RED: evolve binary not found" >&2; exit 1; }

BB=".evolve/phases/behavior-baseline/phase.json"
BC=".evolve/phases/behavior-compare/phase.json"
SS=".evolve/phases/smell-scan/phase.json"
for f in "$BB" "$BC" "$SS"; do
  [ -f "$f" ] || { echo "RED: $f missing" >&2; exit 1; }
done

# Behavioral anchor: all three specs must pass real validation first.
for p in behavior-baseline behavior-compare smell-scan; do
  EVOLVE_PROJECT_ROOT="$TOP" "$BIN" phases validate "$p" >/dev/null 2>&1 \
    || { echo "RED: phases validate $p failed — field checks below would be moot" >&2; exit 1; }
done

# AC2 — behavior-compare gate.
jq -e '.classify.fail_if_signal["behavior.preserved"] == "==false"' "$BC" >/dev/null 2>&1 \
  || { echo "RED: behavior-compare lacks fail_if_signal behavior.preserved==\"==false\"" >&2; exit 1; }
jq -e '.classify.require_sections | index("Comparison") and index("Verdict")' "$BC" >/dev/null 2>&1 \
  || { echo "RED: behavior-compare require_sections missing Comparison/Verdict" >&2; exit 1; }

# AC3 — smell-scan shape.
[ "$(jq -r '.archetype' "$SS")" = "evaluate" ] \
  || { echo "RED: smell-scan archetype != evaluate" >&2; exit 1; }
[ "$(jq -r '.writes_source // false' "$SS")" = "false" ] \
  || { echo "RED: smell-scan writes_source != false" >&2; exit 1; }
[ "$(jq -r '.classify.fail_if_empty' "$SS")" = "true" ] \
  || { echo "RED: smell-scan classify.fail_if_empty != true" >&2; exit 1; }

# AC4 — baseline output signals.
jq -e '.outputs.signals | index("behavior.preserved") and index("behavior.delta_count")' "$BB" >/dev/null 2>&1 \
  || { echo "RED: behavior-baseline outputs.signals missing behavior.preserved/behavior.delta_count" >&2; exit 1; }

# Straddle — catalog §3 verbatim.
[ "$(jq -r '.after' "$BB")" = "tdd" ] \
  || { echo "RED: behavior-baseline after != tdd (must capture pre-build)" >&2; exit 1; }
[ "$(jq -r '.after' "$BC")" = "build" ] \
  || { echo "RED: behavior-compare after != build (must diff post-build)" >&2; exit 1; }

echo "GREEN: refactor-trio contract fields match catalog §3 (gate, shape, signals, straddle)" >&2
exit 0
