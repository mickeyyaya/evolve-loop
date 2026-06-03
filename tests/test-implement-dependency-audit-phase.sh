#!/usr/bin/env bash
# tests/test-implement-dependency-audit-phase.sh
#
# TDD RED suite for cycle-214 Task 2: implement-dependency-audit-phase.
# Encodes the acceptance criteria from scout-report.md. MUST FAIL at RED
# (phase.json absent); passes once Builder drops a valid
# .evolve/phases/dependency-audit/phase.json.
#
# Behavioral: load-bearing checks invoke the `evolve` binary (the real
# discovery+validate+list pipeline), not a grep of the source file.
set -uo pipefail

ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"
PHASE_JSON="$ROOT/.evolve/phases/dependency-audit/phase.json"

PASS=0; FAIL=0
ok() { echo "PASS: $1"; PASS=$((PASS+1)); }
no() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

list_phases()    { EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list 2>/dev/null; }
validate_phase() { EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases validate "$1" 2>/dev/null; }

if [ ! -x "$BIN" ]; then
  echo "FAIL: evolve binary not found/executable at $BIN"; exit 1
fi

# --- AC2.1: phase.json exists, valid JSON, optional:true --------------------
if [ -f "$PHASE_JSON" ]; then ok "dependency-audit/phase.json exists on disk"
else no "dependency-audit/phase.json exists on disk"; fi

if git -C "$ROOT" ls-files --error-unmatch ".evolve/phases/dependency-audit/phase.json" >/dev/null 2>&1; then
  ok "dependency-audit/phase.json is git-tracked"
else no "dependency-audit/phase.json is git-tracked (untracked may be dropped at ship)"; fi

if [ -f "$PHASE_JSON" ] && jq -e . "$PHASE_JSON" >/dev/null 2>&1; then ok "phase.json is valid JSON"
else no "phase.json is valid JSON"; fi

if [ -f "$PHASE_JSON" ] && [ "$(jq -r '.optional' "$PHASE_JSON" 2>/dev/null)" = "true" ]; then
  ok "phase.json optional == true"
else no "phase.json optional == true"; fi

# Behavioral anti-no-op: validate must report OK (real PhaseSpec, not a stub).
if validate_phase dependency-audit | grep -q "^OK    dependency-audit$"; then
  ok "evolve phases validate dependency-audit == OK (valid PhaseSpec)"
else no "evolve phases validate dependency-audit == OK (valid PhaseSpec)"; fi

# --- AC2.2: phase appears in `evolve phases list`, SOURCE=user --------------
if list_phases | grep -q "dependency-audit"; then ok "dependency-audit appears in phases list"
else no "dependency-audit appears in phases list"; fi

if list_phases | grep -E "^dependency-audit[[:space:]]" | grep -q "user"; then
  ok "dependency-audit SOURCE == user"
else no "dependency-audit SOURCE == user"; fi

# --- AC2.3: outputs.signals declares dependency.severity_max ----------------
if [ -f "$PHASE_JSON" ] && jq -e '.outputs.signals | index("dependency.severity_max")' "$PHASE_JSON" >/dev/null 2>&1; then
  ok "outputs.signals contains dependency.severity_max"
else no "outputs.signals contains dependency.severity_max"; fi

# --- AC2.4: BOTH new phases present in list, count == 2 ----------------------
# Strongest behavioral check: exercises full discovery+merge+list for both
# user phases together (depends on Task 1 also being complete).
cnt=$(list_phases | grep -cE "^(security-scan|dependency-audit)[[:space:]]")
if [ "$cnt" -eq 2 ]; then ok "both user phases listed (count == 2)"
else no "both user phases listed (count == 2, got $cnt)"; fi

echo ""; echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
