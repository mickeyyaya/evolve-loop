#!/usr/bin/env bash
# tests/test-implement-security-scan-phase.sh
#
# TDD RED suite for cycle-214 Task 1: implement-security-scan-phase.
# Encodes the acceptance criteria from scout-report.md. These tests MUST FAIL
# at RED baseline (the phase.json does not exist yet) and pass only once Builder
# drops a valid .evolve/phases/security-scan/phase.json.
#
# Behavioral: the load-bearing checks invoke the `evolve` binary (phases
# list / phases validate) — i.e. the real DiscoverUserSpecs → Merge →
# ValidateUserSpec machinery — not a grep of the source file.
set -uo pipefail

ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"
PHASE_JSON="$ROOT/.evolve/phases/security-scan/phase.json"

PASS=0; FAIL=0
ok()   { echo "PASS: $1"; PASS=$((PASS+1)); }
no()   { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

# list_phases / validate_phase run the real binary against this tree.
list_phases()    { EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list 2>/dev/null; }
validate_phase() { EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases validate "$1" 2>/dev/null; }

if [ ! -x "$BIN" ]; then
  echo "FAIL: evolve binary not found/executable at $BIN"; exit 1
fi

# --- AC1.1: phase.json exists, is valid JSON, optional:true ------------------
if [ -f "$PHASE_JSON" ]; then ok "security-scan/phase.json exists on disk"
else no "security-scan/phase.json exists on disk"; fi

# git-tracking dual-check (cycle-92 gitignore footgun): file must be staged so
# it survives ship. Untracked at RED is the correct failing signal.
if git -C "$ROOT" ls-files --error-unmatch ".evolve/phases/security-scan/phase.json" >/dev/null 2>&1; then
  ok "security-scan/phase.json is git-tracked"
else no "security-scan/phase.json is git-tracked (untracked may be gitignored / dropped at ship)"; fi

if [ -f "$PHASE_JSON" ] && jq -e . "$PHASE_JSON" >/dev/null 2>&1; then ok "phase.json is valid JSON"
else no "phase.json is valid JSON"; fi

if [ -f "$PHASE_JSON" ] && [ "$(jq -r '.optional' "$PHASE_JSON" 2>/dev/null)" = "true" ]; then
  ok "phase.json optional == true"
else no "phase.json optional == true"; fi

# Behavioral: a malformed/non-optional/wrong-kind spec FAILS `phases validate`.
# `OK <name>` only prints when the real ValidateUserSpec passes — anti-no-op.
if validate_phase security-scan | grep -q "^OK    security-scan$"; then
  ok "evolve phases validate security-scan == OK (valid PhaseSpec)"
else no "evolve phases validate security-scan == OK (valid PhaseSpec)"; fi

# --- AC1.2: phase appears in `evolve phases list`, SOURCE=user ---------------
if list_phases | grep -q "security-scan"; then ok "security-scan appears in phases list"
else no "security-scan appears in phases list"; fi

# Adversarial: must be a USER phase, not smuggled into the builtin registry.
if list_phases | grep -E "^security-scan[[:space:]]" | grep -q "user"; then
  ok "security-scan SOURCE == user (dropped as user phase, not registry-edited)"
else no "security-scan SOURCE == user (dropped as user phase, not registry-edited)"; fi

# --- AC1.3: outputs.signals declares security.severity_max ------------------
if [ -f "$PHASE_JSON" ] && jq -e '.outputs.signals | index("security.severity_max")' "$PHASE_JSON" >/dev/null 2>&1; then
  ok "outputs.signals contains security.severity_max"
else no "outputs.signals contains security.severity_max"; fi

# --- AC1.4: routing trigger build.files_touched > 0 -------------------------
if [ -f "$PHASE_JSON" ] && \
   jq -e '.routing.insert_when[]? | select(.field=="build.files_touched" and .op=="gt" and (.value==0 or .value=="0"))' \
   "$PHASE_JSON" >/dev/null 2>&1; then
  ok "routing.insert_when triggers on build.files_touched gt 0"
else no "routing.insert_when triggers on build.files_touched gt 0"; fi

# --- AC1.5: phasespec package still passes (regression guard) ---------------
# This is a STAYS-GREEN guard (no production-code change in this cycle), so it
# is expected to PASS even at RED baseline — logged as pre-existing GREEN.
if (cd "$ROOT/go" && go test -count=1 ./internal/phasespec/... >/dev/null 2>&1); then
  ok "go test ./internal/phasespec/... passes (regression)"
else no "go test ./internal/phasespec/... passes (regression)"; fi

echo ""; echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
