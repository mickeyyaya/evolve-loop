#!/usr/bin/env bash
# ACS cycle-214 — dependency-audit user phase is discovered, valid, listed, and
# declares the dependency.severity_max signal.
#
# MIXED: behavioral validate+list (real pipeline) + jq config check for the
# declared output signal.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"
REL=".evolve/phases/dependency-audit/phase.json"
J="$ROOT/$REL"

[ -x "$BIN" ] || { echo "RED: evolve binary missing at $BIN"; exit 1; }
[ -f "$J" ]   || { echo "RED: $REL missing on disk"; exit 1; }
git -C "$ROOT" ls-files --error-unmatch "$REL" >/dev/null 2>&1 \
  || { echo "RED: $REL untracked — may be dropped at ship"; exit 1; }

if ! EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases validate dependency-audit 2>/dev/null \
   | grep -q "^OK    dependency-audit$"; then
  echo "RED: 'evolve phases validate dependency-audit' did not report OK"; exit 1
fi

if ! EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list 2>/dev/null \
   | grep -E "^dependency-audit[[:space:]]" | grep -q "user"; then
  echo "RED: dependency-audit not listed with SOURCE=user"; exit 1
fi

if ! jq -e '.outputs.signals | index("dependency.severity_max")' "$J" >/dev/null 2>&1; then
  echo "RED: outputs.signals does not contain dependency.severity_max"; exit 1
fi

echo "GREEN: dependency-audit discovered, valid, listed, declares dependency.severity_max"
exit 0
