#!/usr/bin/env bash
# tests/test-wave1-refactor-phases.sh
#
# TDD RED suite for cycle-217 Task 2: wave1-refactor-phases.
# Encodes the acceptance criteria from scout-report.md Task 2 + the intent
# constraint "phase specs must follow micro-phase-catalog.md §3 verbatim".
# RED at baseline (phase dirs absent); GREEN once Builder ships valid
# .evolve/phases/{behavior-baseline,behavior-compare,smell-scan}/.
#
# The behavior-baseline/behavior-compare pair STRADDLES the build phase
# (baseline after tdd, compare after build) — the golden-master safety net.
# That straddle is load-bearing and asserted below.
set -uo pipefail

ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$ROOT/go/evolve"

BB_JSON="$ROOT/.evolve/phases/behavior-baseline/phase.json"
BC_JSON="$ROOT/.evolve/phases/behavior-compare/phase.json"
SS_JSON="$ROOT/.evolve/phases/smell-scan/phase.json"

PASS=0; FAIL=0
ok() { echo "PASS: $1"; PASS=$((PASS+1)); }
no() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

list_phases()    { EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list 2>/dev/null; }
validate_phase() { EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases validate "$1" >/dev/null 2>&1; }

if [ ! -x "$BIN" ]; then
  echo "FAIL: evolve binary not found/executable at $BIN"; exit 1
fi

# --- AC2.1: all three phases validate green (behavioral) ---------------------
for p in behavior-baseline behavior-compare smell-scan; do
  if validate_phase "$p"; then ok "evolve phases validate $p exits 0"
  else no "evolve phases validate $p exits 0"; fi
done

# --- file presence + git-tracking dual-check (cycle-92 gitignore footgun) ----
for p in behavior-baseline behavior-compare smell-scan; do
  for base in phase.json agent.md; do
    rel=".evolve/phases/$p/$base"
    if [ -f "$ROOT/$rel" ]; then ok "$rel exists on disk"
    else no "$rel exists on disk"; fi
    if git -C "$ROOT" ls-files --error-unmatch "$rel" >/dev/null 2>&1; then
      ok "$rel is git-tracked"
    else no "$rel is git-tracked (untracked may be dropped at ship)"; fi
  done
done

# --- all three register as USER phases (zero-Go ADR-0035 path) ---------------
for p in behavior-baseline behavior-compare smell-scan; do
  if list_phases | grep -E "^${p}[[:space:]]" | grep -q "user"; then
    ok "$p SOURCE == user in phases list"
  else no "$p SOURCE == user in phases list"; fi
done

# --- AC2.2: behavior-compare is the gate of the pair -------------------------
if [ -f "$BC_JSON" ] && jq -e '.classify.fail_if_signal["behavior.preserved"] == "==false"' "$BC_JSON" >/dev/null 2>&1; then
  ok "behavior-compare fail_if_signal behavior.preserved == \"==false\""
else no "behavior-compare fail_if_signal behavior.preserved == \"==false\""; fi

if [ -f "$BC_JSON" ] && jq -e '.classify.require_sections | index("Comparison") and index("Verdict")' "$BC_JSON" >/dev/null 2>&1; then
  ok "behavior-compare require_sections has Comparison + Verdict"
else no "behavior-compare require_sections has Comparison + Verdict"; fi

# --- AC2.3: smell-scan is an evaluate-archetype, non-writing detector --------
if [ -f "$SS_JSON" ] && [ "$(jq -r '.archetype' "$SS_JSON" 2>/dev/null)" = "evaluate" ]; then
  ok "smell-scan archetype == evaluate"
else no "smell-scan archetype == evaluate"; fi

if [ -f "$SS_JSON" ] && [ "$(jq -r '.writes_source // false' "$SS_JSON" 2>/dev/null)" = "false" ]; then
  ok "smell-scan writes_source == false"
else no "smell-scan writes_source == false"; fi

if [ -f "$SS_JSON" ] && [ "$(jq -r '.classify.fail_if_empty' "$SS_JSON" 2>/dev/null)" = "true" ]; then
  ok "smell-scan classify.fail_if_empty == true"
else no "smell-scan classify.fail_if_empty == true"; fi

# --- AC2.4: behavior-baseline declares the pair's output signals -------------
if [ -f "$BB_JSON" ] && jq -e '.outputs.signals | index("behavior.preserved")' "$BB_JSON" >/dev/null 2>&1; then
  ok "behavior-baseline outputs.signals has behavior.preserved"
else no "behavior-baseline outputs.signals has behavior.preserved"; fi

if [ -f "$BB_JSON" ] && jq -e '.outputs.signals | index("behavior.delta_count")' "$BB_JSON" >/dev/null 2>&1; then
  ok "behavior-baseline outputs.signals has behavior.delta_count"
else no "behavior-baseline outputs.signals has behavior.delta_count"; fi

# --- catalog §3 verbatim: the straddle (baseline pre-build, compare post) ----
if [ -f "$BB_JSON" ] && [ "$(jq -r '.after' "$BB_JSON" 2>/dev/null)" = "tdd" ]; then
  ok "behavior-baseline after == tdd (pre-build capture)"
else no "behavior-baseline after == tdd (pre-build capture)"; fi

if [ -f "$BC_JSON" ] && [ "$(jq -r '.after' "$BC_JSON" 2>/dev/null)" = "build" ]; then
  ok "behavior-compare after == build (post-build diff)"
else no "behavior-compare after == build (post-build diff)"; fi

# --- catalog §3 verbatim: all three route on scout.goal_type == refactor -----
for f in "$BB_JSON" "$BC_JSON" "$SS_JSON"; do
  rel="${f#"$ROOT"/}"
  if [ -f "$f" ] && jq -e \
     '.routing.insert_when[]? | select(.field=="scout.goal_type" and (.op=="==" or .op=="eq") and .value=="refactor")' \
     "$f" >/dev/null 2>&1; then
    ok "$rel insert_when: scout.goal_type == refactor"
  else no "$rel insert_when: scout.goal_type == refactor"; fi
done

# --- NEGATIVE (anti-no-op): unsupported kind must FAIL validate --------------
# Corrupts a copy of smell-scan with kind:"python" (not in llm|native|command);
# ValidateUserSpec must reject it. Proves validation is real, not a file stat.
if [ -f "$SS_JSON" ]; then
  TMPROOT=$(mktemp -d)
  mkdir -p "$TMPROOT/.evolve/phases/smell-scan"
  jq '.kind = "python"' "$SS_JSON" > "$TMPROOT/.evolve/phases/smell-scan/phase.json" 2>/dev/null
  if EVOLVE_PROJECT_ROOT="$TMPROOT" "$BIN" phases validate smell-scan >/dev/null 2>&1; then
    no "NEGATIVE: validator rejects kind:python corruption of smell-scan"
  else
    ok "NEGATIVE: validator rejects kind:python corruption of smell-scan"
  fi
  rm -rf "$TMPROOT"
else
  no "NEGATIVE: validator rejects kind:python corruption of smell-scan (phase.json missing)"
fi

echo ""; echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
