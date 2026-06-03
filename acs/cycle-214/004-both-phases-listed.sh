#!/usr/bin/env bash
# ACS cycle-214 — BOTH new user phases appear in `evolve phases list` (AC2.4).
#
# BEHAVIORAL: the count is derived from the live binary's discover+merge+list
# output, exercising both user phases together. This is the strongest anti-no-op
# signal for the cycle — it requires two real, discoverable PhaseSpecs on disk.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"

[ -x "$BIN" ] || { echo "RED: evolve binary missing at $BIN"; exit 1; }

cnt=$(EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list 2>/dev/null \
      | grep -cE "^(security-scan|dependency-audit)[[:space:]]")

if [ "$cnt" -ne 2 ]; then
  echo "RED: expected 2 new user phases in 'phases list', got $cnt"; exit 1
fi

echo "GREEN: both security-scan and dependency-audit appear in phases list (count=2)"
exit 0
