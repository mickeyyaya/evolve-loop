#!/usr/bin/env bash
# ACS cycle-217 / NEGATIVE axis (adversarial-testing SKILL §6) — the validator
# must REJECT a corrupted wave-1 spec. This is the anti-no-op guard for
# predicates 001/003: it proves `evolve phases validate` exit 0 means real
# validation, not a stub that always succeeds.
#
# Construction: copy the shipped fault-localization phase.json into a scratch
# project root, flip optional → false (a user phase may never be mandatory —
# it could displace the build→audit→ship floor), and assert validate FAILS.
#
# RED at baseline (phase.json absent). GREEN only when the real spec exists,
# validates pristine, AND the corrupted variant is rejected.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

BIN="${EVOLVE_GO_BIN:-$TOP/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$TOP/go/evolve"
[ -x "$BIN" ] || { echo "RED: evolve binary not found" >&2; exit 1; }

SRC=".evolve/phases/fault-localization/phase.json"
[ -f "$SRC" ] || { echo "RED: $SRC missing (wave-1 bugfix pair not implemented)" >&2; exit 1; }

# Sanity: the pristine spec must validate (otherwise the negative proves nothing).
EVOLVE_PROJECT_ROOT="$TOP" "$BIN" phases validate fault-localization >/dev/null 2>&1 \
  || { echo "RED: pristine fault-localization does not validate" >&2; exit 1; }

TMPROOT=$(mktemp -d)
trap 'rm -rf "$TMPROOT"' EXIT
mkdir -p "$TMPROOT/.evolve/phases/fault-localization"
jq '.optional = false' "$SRC" > "$TMPROOT/.evolve/phases/fault-localization/phase.json" \
  || { echo "RED: failed to construct corrupted spec copy" >&2; exit 1; }

if EVOLVE_PROJECT_ROOT="$TMPROOT" "$BIN" phases validate fault-localization >/dev/null 2>&1; then
  echo "RED: validator ACCEPTED an optional:false user phase — ship-floor invariant broken" >&2
  exit 1
fi

echo "GREEN: validator rejects the corrupted (optional:false) wave-1 spec — floor invariant holds" >&2
exit 0
