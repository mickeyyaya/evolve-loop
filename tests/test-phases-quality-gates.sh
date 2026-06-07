#!/usr/bin/env bash
# tests/test-phases-quality-gates.sh
#
# TDD RED suite for cycle-246 Task 1: phases-quality-gates (Wave 2).
# Encodes the 10 acceptance criteria from scout-report.md Task 1 + the intent
# constraint "phase specs follow docs/architecture/micro-phase-catalog.md §3
# verbatim". These tests MUST FAIL at RED baseline (the four phase dirs do not
# exist yet) and pass only once Builder authors, for each of
# {benchmark-gate, fuzz-probe, cleanup-sweep, rollback-plan}:
#   .evolve/phases/NAME/{phase.json,agent.md}
#   agents/evolve-NAME.md
#   .evolve/profiles/NAME.json
#
# Behavioral: the load-bearing checks invoke the `evolve` binary
# (phases validate / phases list) — the real DiscoverUserSpecs → Merge →
# ValidateUserSpec machinery — not a grep of Go source. Field-level checks
# parse the actual phase.json with jq (the same JSON the loader reads).
# agent.md content checks are config-presence checks on the authored
# deliverable itself (acs-predicate: config-check class).
set -uo pipefail

ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$ROOT/go/evolve"

PHASES="benchmark-gate fuzz-probe cleanup-sweep rollback-plan"

PASS=0; FAIL=0
ok() { echo "PASS: $1"; PASS=$((PASS+1)); }
no() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

list_phases()    { EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list 2>/dev/null; }
validate_phase() { EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases validate "$1" >/dev/null 2>&1; }

if [ ! -x "$BIN" ]; then
  echo "FAIL: evolve binary not found/executable at $BIN"; exit 1
fi

# --- AC1-AC4: `evolve phases validate NAME` exits 0 (behavioral) -------------
for p in $PHASES; do
  if validate_phase "$p"; then ok "evolve phases validate $p exits 0"
  else no "evolve phases validate $p exits 0"; fi
done

# --- all four register as USER phases (zero-Go ADR-0035/0038 path) -----------
for p in $PHASES; do
  if list_phases | grep -E "^${p}[[:space:]]" | grep -q "user"; then
    ok "$p SOURCE == user in phases list"
  else no "$p SOURCE == user in phases list"; fi
done

# --- verifiableBy: all 5 Wave-2 phases visible (mutation-gate + 4 new) -------
n=$(list_phases | grep -cE '^(mutation-gate|benchmark-gate|fuzz-probe|cleanup-sweep|rollback-plan)[[:space:]]' || true)
if [ "$n" = "5" ]; then ok "phases list shows all 5 Wave-2 phases (got $n)"
else no "phases list shows all 5 Wave-2 phases (got $n)"; fi

# --- AC5 + AC6 + spec files: presence + git-tracking dual-check --------------
# (cycle-92 gitignore footgun: [ -f ] alone passes in the worktree but the
# file is silently dropped at ship if untracked/gitignored.)
for p in $PHASES; do
  for rel in ".evolve/phases/$p/phase.json" ".evolve/phases/$p/agent.md" \
             "agents/evolve-$p.md" ".evolve/profiles/$p.json"; do
    f="$ROOT/$rel"
    if [ -s "$f" ]; then ok "$rel exists and is non-empty"
    else no "$rel exists and is non-empty"; fi
    if git -C "$ROOT" ls-files --error-unmatch "$rel" >/dev/null 2>&1; then
      ok "$rel is git-tracked"
    else no "$rel is git-tracked (untracked may be dropped at ship)"; fi
  done
done

# --- AC6 (cont): profiles parse as JSON and self-name correctly --------------
for p in $PHASES; do
  f="$ROOT/.evolve/profiles/$p.json"
  if [ -f "$f" ] && jq -e --arg n "$p" '.name == $n' "$f" >/dev/null 2>&1; then
    ok "profiles/$p.json valid JSON with name == $p"
  else no "profiles/$p.json valid JSON with name == $p"; fi
done

# --- floor invariants: all four optional:true, writes_source:false -----------
# (cleanup-sweep especially: catalog §3 says detection only — never a writer)
for p in $PHASES; do
  f="$ROOT/.evolve/phases/$p/phase.json"
  if [ -f "$f" ] && [ "$(jq -r '.optional' "$f" 2>/dev/null)" = "true" ]; then
    ok "$p optional == true"
  else no "$p optional == true"; fi
  if [ -f "$f" ] && [ "$(jq -r '.writes_source // false' "$f" 2>/dev/null)" = "false" ]; then
    ok "$p writes_source == false"
  else no "$p writes_source == false"; fi
done

# --- AC7: benchmark-gate agent.md = multi-sample statistical comparison ------
# acs-predicate: config-check (agent.md content IS the authored deliverable)
BG_AGENT="$ROOT/.evolve/phases/benchmark-gate/agent.md"
if [ -f "$BG_AGENT" ] && grep -qi 'benchstat' "$BG_AGENT"; then
  ok "benchmark-gate agent.md references benchstat"
else no "benchmark-gate agent.md references benchstat"; fi
if [ -f "$BG_AGENT" ] && grep -qiE 'count=|multi[- ]sample|samples|[0-9]+ (runs|times|iterations)' "$BG_AGENT"; then
  ok "benchmark-gate agent.md instructs multi-sample collection (not single-run)"
else no "benchmark-gate agent.md instructs multi-sample collection (not single-run)"; fi
# catalog §3 verbatim: classify gate on perf.significant
BG_JSON="$ROOT/.evolve/phases/benchmark-gate/phase.json"
if [ -f "$BG_JSON" ] && jq -e '.classify.fail_if_signal["perf.significant"] == "==true"' "$BG_JSON" >/dev/null 2>&1; then
  ok "benchmark-gate classify.fail_if_signal perf.significant == \"==true\""
else no "benchmark-gate classify.fail_if_signal perf.significant == \"==true\""; fi

# --- AC8: cleanup-sweep agent.md explicitly detection-only -------------------
# acs-predicate: config-check
CS_AGENT="$ROOT/.evolve/phases/cleanup-sweep/agent.md"
if [ -f "$CS_AGENT" ] && grep -qiE 'detection[ -]only' "$CS_AGENT"; then
  ok "cleanup-sweep agent.md states detection-only"
else no "cleanup-sweep agent.md states detection-only"; fi
if [ -f "$CS_AGENT" ] && grep -qiE 'do (NOT|not).*(edit|remove|delete|modify)|no (file )?(edits|removals|deletions)' "$CS_AGENT"; then
  ok "cleanup-sweep agent.md forbids edits/removals"
else no "cleanup-sweep agent.md forbids edits/removals"; fi

# --- AC9: fuzz-probe routing diff-scoped to parser/decoder/unmarshal ---------
FP_JSON="$ROOT/.evolve/phases/fuzz-probe/phase.json"
if [ -f "$FP_JSON" ] && jq -r '.routing' "$FP_JSON" 2>/dev/null | grep -qiE 'pars|decod|unmarshal'; then
  ok "fuzz-probe routing scopes to parser/decoder/unmarshal surfaces"
else no "fuzz-probe routing scopes to parser/decoder/unmarshal surfaces"; fi
# catalog §3 verbatim: a single crasher blocks the cycle
if [ -f "$FP_JSON" ] && jq -e '.classify.fail_if_signal["fuzz.crashers"] == ">0"' "$FP_JSON" >/dev/null 2>&1; then
  ok "fuzz-probe classify.fail_if_signal fuzz.crashers == \">0\""
else no "fuzz-probe classify.fail_if_signal fuzz.crashers == \">0\""; fi

# --- AC10: rollback-plan revert-readiness gate --------------------------------
RP_JSON="$ROOT/.evolve/phases/rollback-plan/phase.json"
if [ -f "$RP_JSON" ] && jq -e '.classify.fail_if_signal["rollback.ready"] == "==false"' "$RP_JSON" >/dev/null 2>&1; then
  ok "rollback-plan classify.fail_if_signal rollback.ready == \"==false\""
else no "rollback-plan classify.fail_if_signal rollback.ready == \"==false\""; fi

# --- NEGATIVE (anti-no-op): a corrupted spec must FAIL validate --------------
# Copies rollback-plan's phase.json into a scratch root with optional flipped
# to false; ValidateUserSpec must reject it (floor invariant). Proves the
# validate calls above exercise real validation, not a stub.
if [ -f "$RP_JSON" ]; then
  TMPROOT=$(mktemp -d)
  mkdir -p "$TMPROOT/.evolve/phases/rollback-plan"
  jq '.optional = false' "$RP_JSON" > "$TMPROOT/.evolve/phases/rollback-plan/phase.json" 2>/dev/null
  if EVOLVE_PROJECT_ROOT="$TMPROOT" "$BIN" phases validate rollback-plan >/dev/null 2>&1; then
    no "NEGATIVE: validator rejects optional:false corruption of rollback-plan"
  else
    ok "NEGATIVE: validator rejects optional:false corruption of rollback-plan"
  fi
  rm -rf "$TMPROOT"
else
  no "NEGATIVE: validator rejects optional:false corruption of rollback-plan (phase.json missing)"
fi

# --- EDGE (validator sanity; expected pre-existing GREEN) ---------------------
if validate_phase no-such-phase-xyz; then
  no "EDGE: validate of a nonexistent phase exits non-zero"
else
  ok "EDGE: validate of a nonexistent phase exits non-zero"
fi

# --- supplementary (scout audit skill): insertion cap fits refactor recipe ----
# expected pre-existing GREEN (shipped in Wave 1)
if [ "$(jq -r '..|.max_optional_insertions? // empty' "$ROOT/docs/architecture/phase-registry.json" 2>/dev/null | head -1)" = "6" ]; then
  ok "phase-registry max_optional_insertions == 6 (pre-existing)"
else no "phase-registry max_optional_insertions == 6 (pre-existing)"; fi

echo ""; echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
