#!/usr/bin/env bash
# tests/test-wave1-bugfix-phases.sh
#
# TDD RED suite for cycle-217 Task 1: wave1-bugfix-phases.
# Encodes the acceptance criteria from scout-report.md Task 1 + the intent
# constraint "phase specs must follow micro-phase-catalog.md §3 verbatim".
# These tests MUST FAIL at RED baseline (the phase dirs do not exist yet) and
# pass only once Builder drops valid .evolve/phases/{fault-localization,
# bug-reproduction}/{phase.json,agent.md}.
#
# Behavioral: the load-bearing checks invoke the `evolve` binary (phases
# validate / phases list) — the real DiscoverUserSpecs → Merge →
# ValidateUserSpec machinery — not a grep of the source file. Field-level
# checks parse the actual phase.json with jq (the same JSON the loader reads).
set -uo pipefail

ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$ROOT/go/evolve"

FL_JSON="$ROOT/.evolve/phases/fault-localization/phase.json"
FL_AGENT="$ROOT/.evolve/phases/fault-localization/agent.md"
RB_JSON="$ROOT/.evolve/phases/bug-reproduction/phase.json"
RB_AGENT="$ROOT/.evolve/phases/bug-reproduction/agent.md"

PASS=0; FAIL=0
ok() { echo "PASS: $1"; PASS=$((PASS+1)); }
no() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

list_phases()    { EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list 2>/dev/null; }
validate_phase() { EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases validate "$1" >/dev/null 2>&1; }

if [ ! -x "$BIN" ]; then
  echo "FAIL: evolve binary not found/executable at $BIN"; exit 1
fi

# --- AC1.1: `evolve phases validate fault-localization` exits 0 (behavioral) -
if validate_phase fault-localization; then ok "evolve phases validate fault-localization exits 0"
else no "evolve phases validate fault-localization exits 0"; fi

# --- AC1.2: `evolve phases validate bug-reproduction` exits 0 (behavioral) ------
if validate_phase bug-reproduction; then ok "evolve phases validate bug-reproduction exits 0"
else no "evolve phases validate bug-reproduction exits 0"; fi

# --- file presence + git-tracking dual-check (cycle-92 gitignore footgun) ----
for f in "$FL_JSON" "$FL_AGENT" "$RB_JSON" "$RB_AGENT"; do
  rel="${f#"$ROOT"/}"
  if [ -f "$f" ]; then ok "$rel exists on disk"
  else no "$rel exists on disk"; fi
  if git -C "$ROOT" ls-files --error-unmatch "$rel" >/dev/null 2>&1; then
    ok "$rel is git-tracked"
  else no "$rel is git-tracked (untracked may be dropped at ship)"; fi
done

# --- both register as USER phases (zero-Go ADR-0035 path, not registry edit) -
for p in fault-localization bug-reproduction; do
  if list_phases | grep -E "^${p}[[:space:]]" | grep -q "user"; then
    ok "$p SOURCE == user in phases list"
  else no "$p SOURCE == user in phases list"; fi
done

# --- AC1.3: fault-localization routing gates on scout.goal_type == bugfix ----
if [ -f "$FL_JSON" ] && jq -e \
   '.routing.insert_when[]? | select(.field=="scout.goal_type" and (.op=="==" or .op=="eq") and .value=="bugfix")' \
   "$FL_JSON" >/dev/null 2>&1; then
  ok "fault-localization insert_when: scout.goal_type == bugfix"
else no "fault-localization insert_when: scout.goal_type == bugfix"; fi

# --- AC1.4: bug-reproduction FAIL_TO_PASS gate (the strongest signal gate) ------
if [ -f "$RB_JSON" ] && jq -e '.classify.fail_if_signal["repro.failing"] == "==false"' "$RB_JSON" >/dev/null 2>&1; then
  ok "bug-reproduction classify.fail_if_signal repro.failing == \"==false\""
else no "bug-reproduction classify.fail_if_signal repro.failing == \"==false\""; fi

# --- AC1.5: both phases optional:true, writes_source:false -------------------
for f in "$FL_JSON" "$RB_JSON"; do
  rel="${f#"$ROOT"/}"
  if [ -f "$f" ] && [ "$(jq -r '.optional' "$f" 2>/dev/null)" = "true" ]; then
    ok "$rel optional == true"
  else no "$rel optional == true"; fi
  if [ -f "$f" ] && [ "$(jq -r '.writes_source // false' "$f" 2>/dev/null)" = "false" ]; then
    ok "$rel writes_source == false"
  else no "$rel writes_source == false"; fi
done

# --- catalog §3 verbatim: pipeline placement + contract sections -------------
if [ -f "$FL_JSON" ] && [ "$(jq -r '.after' "$FL_JSON" 2>/dev/null)" = "triage" ]; then
  ok "fault-localization after == triage"
else no "fault-localization after == triage"; fi

if [ -f "$RB_JSON" ] && jq -e '.classify.require_sections | index("Reproduction") and index("Verification")' "$RB_JSON" >/dev/null 2>&1; then
  ok "bug-reproduction require_sections has Reproduction + Verification"
else no "bug-reproduction require_sections has Reproduction + Verification"; fi

if [ -f "$FL_JSON" ] && jq -e '.classify.require_sections | index("Suspect Ranking") and index("Edit Locations")' "$FL_JSON" >/dev/null 2>&1; then
  ok "fault-localization require_sections has Suspect Ranking + Edit Locations"
else no "fault-localization require_sections has Suspect Ranking + Edit Locations"; fi

# --- NEGATIVE (anti-no-op): a corrupted spec must FAIL validate --------------
# Copies the real phase.json into a scratch project root with optional flipped
# to false; ValidateUserSpec must reject it (floor invariant). Proves the
# validate calls above exercise real validation, not a stub.
if [ -f "$FL_JSON" ]; then
  TMPROOT=$(mktemp -d)
  mkdir -p "$TMPROOT/.evolve/phases/fault-localization"
  jq '.optional = false' "$FL_JSON" > "$TMPROOT/.evolve/phases/fault-localization/phase.json" 2>/dev/null
  if EVOLVE_PROJECT_ROOT="$TMPROOT" "$BIN" phases validate fault-localization >/dev/null 2>&1; then
    no "NEGATIVE: validator rejects optional:false corruption of fault-localization"
  else
    ok "NEGATIVE: validator rejects optional:false corruption of fault-localization"
  fi
  rm -rf "$TMPROOT"
else
  no "NEGATIVE: validator rejects optional:false corruption of fault-localization (phase.json missing)"
fi

# --- EDGE (validator sanity; expected pre-existing GREEN) --------------------
if validate_phase no-such-phase-xyz; then
  no "EDGE: validate of a nonexistent phase exits non-zero"
else
  ok "EDGE: validate of a nonexistent phase exits non-zero"
fi

echo ""; echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
