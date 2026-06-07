#!/usr/bin/env bash
# ACS cycle-246/001 — AC1-AC4: all four Wave-2 quality-gate phases pass
# `evolve phases validate` and register as USER phases in `evolve phases list`.
#
# Behavioral: invokes the evolve binary (DiscoverUserSpecs → Merge →
# ValidateUserSpec) as a subprocess; asserts on exit codes and list output.
# Includes the anti-no-op NEGATIVE: a corrupted copy of a spec
# (optional flipped to false) must FAIL validation — proving the positive
# validate calls exercise real floor validation, not a stub.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$ROOT/go/evolve"
[ -x "$BIN" ] || { echo "RED: evolve binary missing at $BIN" >&2; exit 1; }

for p in benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do
  EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases validate "$p" >/dev/null 2>&1 \
    || { echo "RED: evolve phases validate $p failed" >&2; exit 1; }
  EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list 2>/dev/null \
    | grep -E "^${p}[[:space:]]" | grep -q "user" \
    || { echo "RED: $p not listed as SOURCE=user" >&2; exit 1; }
done

# NEGATIVE: corrupted spec must be rejected (validator is real, not a no-op)
RP_JSON="$ROOT/.evolve/phases/rollback-plan/phase.json"
TMPROOT=$(mktemp -d)
mkdir -p "$TMPROOT/.evolve/phases/rollback-plan"
jq '.optional = false' "$RP_JSON" > "$TMPROOT/.evolve/phases/rollback-plan/phase.json"
if EVOLVE_PROJECT_ROOT="$TMPROOT" "$BIN" phases validate rollback-plan >/dev/null 2>&1; then
  rm -rf "$TMPROOT"
  echo "RED: validator accepted optional:false corruption — validation is a no-op" >&2
  exit 1
fi
rm -rf "$TMPROOT"

echo "PASS"; exit 0
