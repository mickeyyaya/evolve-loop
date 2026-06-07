#!/usr/bin/env bash
# ACS cycle-247 — recover-wave2-phases AC2 + AC6 (behavioral).
# Invokes the system under test (`evolve phases validate`) per phase:
#   AC2: all 5 wave-2 phases (4 recovered + mutation-gate) validate exit 0.
#   AC6 (negative): an unknown phase name is REJECTED (non-zero exit + message).
# validate accepts ONE name per invocation (extra args silently ignored on the
# cycle-247 binary) — loop, never pass a list.
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel)

EVOLVE_BIN=""
for cand in "$ROOT/go/bin/evolve" "$ROOT/go/evolve"; do
  [ -x "$cand" ] && { EVOLVE_BIN="$cand"; break; }
done
[ -n "$EVOLVE_BIN" ] || { echo "RED: no evolve binary (go/bin/evolve or go/evolve)" >&2; exit 1; }

for p in benchmark-gate fuzz-probe cleanup-sweep rollback-plan mutation-gate; do
  if ! EVOLVE_PHASE_ROOTS="$ROOT/.evolve/phases" "$EVOLVE_BIN" phases validate "$p" >/dev/null 2>&1; then
    echo "RED: evolve phases validate $p — non-zero exit" >&2
    exit 1
  fi
done
echo "GREEN: all 5 wave-2 phases validate (exit 0 each)" >&2

# Negative leg: rejection behavior is part of the criterion (anti-no-op).
out=$(EVOLVE_PHASE_ROOTS="$ROOT/.evolve/phases" "$EVOLVE_BIN" phases validate cycle247-no-such-phase 2>&1)
rc=$?
if [ "$rc" -eq 0 ]; then
  echo "RED: validate accepted unknown phase 'cycle247-no-such-phase' (exit 0)" >&2
  exit 1
fi
if ! echo "$out" | grep -q "no user phase named"; then
  echo "RED: unknown-phase rejection missing diagnostic; got: $out" >&2
  exit 1
fi
echo "GREEN: validate rejects unknown phase (rc=$rc)" >&2
exit 0
