#!/usr/bin/env bash
# ACS cycle-214 — security-scan declares the security.severity_max signal and a
# build.files_touched>0 routing trigger.
#
# MIXED predicate: the behavioral half runs `evolve phases validate` (the spec
# must load+validate as a real PhaseSpec); the config half asserts the two
# declared contract fields via jq. The signal/trigger are config-presence
# attributes of a declarative phase — not invocable behavior — so jq on the
# loadable, validated file is the correct check.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"
J="$ROOT/.evolve/phases/security-scan/phase.json"

[ -x "$BIN" ] || { echo "RED: evolve binary missing at $BIN"; exit 1; }
[ -f "$J" ]    || { echo "RED: security-scan/phase.json missing"; exit 1; }

# Behavioral gate: must be a valid, loadable PhaseSpec first.
if ! EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases validate security-scan 2>/dev/null \
   | grep -q "^OK    security-scan$"; then
  echo "RED: security-scan does not validate — config checks below are moot"; exit 1
fi

# Config: declares the security.severity_max output signal.
if ! jq -e '.outputs.signals | index("security.severity_max")' "$J" >/dev/null 2>&1; then
  echo "RED: outputs.signals does not contain security.severity_max"; exit 1
fi

# Config: routes in when the build touched files.
if ! jq -e '.routing.insert_when[]? | select(.field=="build.files_touched" and .op=="gt" and (.value==0 or .value=="0"))' \
     "$J" >/dev/null 2>&1; then
  echo "RED: routing.insert_when missing build.files_touched gt 0 trigger"; exit 1
fi

echo "GREEN: security-scan declares security.severity_max + build.files_touched>0 routing"
exit 0
