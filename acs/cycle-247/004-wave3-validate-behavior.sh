#!/usr/bin/env bash
# ACS cycle-247 — phases-release-and-memory AC2 (behavioral).
# Invokes `evolve phases validate` once per wave-3 phase; all 4 must exit 0.
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel)

EVOLVE_BIN=""
for cand in "$ROOT/go/bin/evolve" "$ROOT/go/evolve"; do
  [ -x "$cand" ] && { EVOLVE_BIN="$cand"; break; }
done
[ -n "$EVOLVE_BIN" ] || { echo "RED: no evolve binary (go/bin/evolve or go/evolve)" >&2; exit 1; }

for p in changelog-sync post-ship-monitor api-contract-design context-condense; do
  if ! EVOLVE_PHASE_ROOTS="$ROOT/.evolve/phases" "$EVOLVE_BIN" phases validate "$p" >/dev/null 2>&1; then
    echo "RED: evolve phases validate $p — non-zero exit" >&2
    exit 1
  fi
done
echo "GREEN: all 4 wave-3 phases validate (exit 0 each)" >&2
exit 0
