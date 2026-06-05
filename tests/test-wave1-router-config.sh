#!/usr/bin/env bash
# tests/test-wave1-router-config.sh
#
# TDD RED suite for cycle-217 Task 3: wave1-router-config.
# Encodes the acceptance criteria from scout-report.md Task 4:
#   (1) docs/architecture/phase-registry.json config.max_optional_insertions 4→6
#   (2) agents/evolve-router.md gains a goal-type recipe table (catalog §4.1)
# RED at baseline (value is 4; router.md has no recipe table).
#
# Behavioral anchors: python3 parses the REAL registry JSON (same bytes the Go
# loader reads) and `evolve phases list` proves the registry still loads after
# the edit. Recipe-table checks are content checks on a persona document —
# inherently presence checks (config-check class), lexically varied.
set -uo pipefail

ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$ROOT/go/evolve"

REGISTRY="$ROOT/docs/architecture/phase-registry.json"
ROUTER="$ROOT/agents/evolve-router.md"

PASS=0; FAIL=0
ok() { echo "PASS: $1"; PASS=$((PASS+1)); }
no() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

if [ ! -x "$BIN" ]; then
  echo "FAIL: evolve binary not found/executable at $BIN"; exit 1
fi

# --- AC4.1: max_optional_insertions == 6 (exact integer) ---------------------
if python3 -c "import json,sys; d=json.load(open('$REGISTRY')); sys.exit(0 if d['config']['max_optional_insertions']==6 else 1)" 2>/dev/null; then
  ok "config.max_optional_insertions == 6"
else no "config.max_optional_insertions == 6"; fi

# --- AC4.5: registry JSON remains valid after the edit -----------------------
if python3 -c "import json; json.load(open('$REGISTRY'))" 2>/dev/null; then
  ok "phase-registry.json is valid JSON"
else no "phase-registry.json is valid JSON"; fi

# --- behavioral regression guard: registry still loads through the Go loader -
# (pre-existing GREEN at RED baseline; guards the JSON edit against breakage)
if EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list >/dev/null 2>&1; then
  ok "evolve phases list exits 0 (registry loads end-to-end)"
else no "evolve phases list exits 0 (registry loads end-to-end)"; fi

phase_rows=$(EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list 2>/dev/null | tail -n +2 | wc -l | tr -d ' ')
if [ "${phase_rows:-0}" -ge 15 ]; then
  ok "phases list shows >= 15 phases (no registry entries lost; got $phase_rows)"
else no "phases list shows >= 15 phases (no registry entries lost; got $phase_rows)"; fi

# --- AC4.2: router persona has a Goal-Type Recipes section -------------------
if grep -q "^## Goal-Type Recipes" "$ROUTER" 2>/dev/null; then
  ok "evolve-router.md has '## Goal-Type Recipes' section"
else no "evolve-router.md has '## Goal-Type Recipes' section"; fi

# All 7 goal types from catalog §4.1 appear as recipe-table rows.
for gt in bugfix feature refactor security performance release docs; do
  if grep -E '^\|' "$ROUTER" 2>/dev/null | grep -qi "$gt"; then
    ok "recipe table covers goal type: $gt"
  else no "recipe table covers goal type: $gt"; fi
done

# --- AC4.3: bugfix row wires the new wave-1 bugfix chain ---------------------
if grep -E '^\|' "$ROUTER" 2>/dev/null | grep -i bugfix | grep -q "fault-localization"; then
  ok "bugfix recipe row references fault-localization"
else no "bugfix recipe row references fault-localization"; fi

if grep -E '^\|' "$ROUTER" 2>/dev/null | grep -i bugfix | grep -q "reproduce-bug"; then
  ok "bugfix recipe row references reproduce-bug"
else no "bugfix recipe row references reproduce-bug"; fi

# --- AC4.4: recipes documented as guidance, kernel clamp is the safety net ---
if grep -q "ClampPlanToFloor" "$ROUTER" 2>/dev/null; then
  ok "recipe section cites ClampPlanToFloor as the safety net"
else no "recipe section cites ClampPlanToFloor as the safety net"; fi

# --- NEGATIVE (anti-no-op): the OLD cap value must be gone -------------------
if python3 -c "import json,sys; d=json.load(open('$REGISTRY')); sys.exit(1 if d['config']['max_optional_insertions']==4 else 0)" 2>/dev/null; then
  ok "NEGATIVE: max_optional_insertions != 4 (old cap removed)"
else no "NEGATIVE: max_optional_insertions != 4 (old cap removed)"; fi

# --- intent AC-4 stays-green guard: zero changes under go/ (ADR-0035) --------
# (pre-existing GREEN at RED baseline; goes RED if Builder touches Go source)
go_dirty=$(git -C "$ROOT" status --porcelain -- go/ 2>/dev/null | wc -l | tr -d ' ')
if [ "${go_dirty:-1}" -eq 0 ]; then
  ok "zero uncommitted changes under go/ (ADR-0035 zero-Go invariant)"
else no "zero uncommitted changes under go/ (ADR-0035 zero-Go invariant; $go_dirty dirty paths)"; fi

echo ""; echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
