#!/usr/bin/env bash
# ACS cycle-247 — phases-release-and-memory AC4 (two-tier naming) + AC7
# (changelog-sync archetype=control; other 3 archetypes asserted per
# micro-phase-catalog §3 spec as supporting contracts).
# Mixed predicate: the load-bearing leg first re-runs `evolve phases validate`
# per phase (the engine must parse the descriptor), then asserts the parsed
# JSON contract values via a real JSON parser.
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

EVOLVE_BIN=""
for cand in "$ROOT/go/bin/evolve" "$ROOT/go/evolve"; do
  [ -x "$cand" ] && { EVOLVE_BIN="$cand"; break; }
done
[ -n "$EVOLVE_BIN" ] || { echo "RED: no evolve binary (go/bin/evolve or go/evolve)" >&2; exit 1; }

rc=0

check_phase() {
  local p="$1" want_archetype="$2"

  # Two-tier naming (AC4): minted phases are <object>-<action>, never single-word.
  echo "$p" | grep -qE '^[a-z]+(-[a-z]+)+$' \
    || { echo "RED: $p violates two-tier <object>-<action> naming" >&2; return 1; }

  # Engine must accept the descriptor (behavioral leg).
  EVOLVE_PHASE_ROOTS="$ROOT/.evolve/phases" "$EVOLVE_BIN" phases validate "$p" >/dev/null 2>&1 \
    || { echo "RED: validate $p failed — cannot assert contracts on invalid phase" >&2; return 1; }

  # Contract values (AC4: name==dirname; AC7: archetype).
  python3 - "$p" "$want_archetype" <<'PY'
import json, sys
p, want = sys.argv[1], sys.argv[2]
d = json.load(open(f".evolve/phases/{p}/phase.json"))
if d.get("name") != p:
    print(f"name {d.get('name')!r} != dirname {p!r}", file=sys.stderr); sys.exit(1)
if d.get("archetype") != want:
    print(f"{p}: archetype {d.get('archetype')!r} != {want!r}", file=sys.stderr); sys.exit(1)
PY
}

check_phase changelog-sync      control || rc=1   # AC7 (the named criterion)
check_phase post-ship-monitor   control || rc=1
check_phase context-condense    control || rc=1
check_phase api-contract-design plan    || rc=1

[ "$rc" -eq 0 ] && echo "GREEN: two-tier naming + archetype contracts hold for all 4" >&2
exit "$rc"
