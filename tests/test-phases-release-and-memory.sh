#!/usr/bin/env bash
# tests/test-phases-release-and-memory.sh — TDD contract for cycle-247 task 2.
#
# Encodes acceptance criteria for the Wave-3 release/feature/memory phases:
# changelog-sync, post-ship-monitor, api-contract-design, context-condense —
# authored per docs/architecture/micro-phase-catalog.md §3, plus the catalog
# card updates (router, AlphaCodium cards on spec-verify/architecture-design,
# ExpeL lesson-extract note on retro).
#
# RED at authoring time (cycle-247 TDD phase): none of the 4 phases exist.
# Builder makes this GREEN by authoring config-only descriptors.
# DO NOT modify this file to make it pass.
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

PASS=0; FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

assert_file() {
  local label="$1" path="$2"
  if [ -f "$path" ]; then pass "$label"; else fail "$label ($path missing)"; fi
}

assert_tracked() {
  local path="$1"
  if [ ! -f "$path" ]; then fail "tracked: $path (missing on disk)"; return; fi
  if git ls-files --error-unmatch "$path" >/dev/null 2>&1; then
    pass "tracked: $path"
  else
    fail "tracked: $path (on disk but untracked — gitignore shadow? use git add -f)"
  fi
}

EVOLVE_BIN=""
for cand in "$ROOT/go/bin/evolve" "$ROOT/go/evolve"; do
  [ -x "$cand" ] && { EVOLVE_BIN="$cand"; break; }
done
if [ -z "$EVOLVE_BIN" ]; then
  echo "FATAL: no evolve binary at go/bin/evolve or go/evolve"; exit 2
fi

WAVE3="changelog-sync post-ship-monitor api-contract-design context-condense"

echo "=== AC1: 4 wave-3 phase dirs present (phase.json + agent.md) ==="
for p in $WAVE3; do
  assert_file "phase.json present: $p" ".evolve/phases/$p/phase.json"
  assert_file "agent.md present: $p"   ".evolve/phases/$p/agent.md"
done
for p in $WAVE3; do
  assert_file "profile present: $p" ".evolve/profiles/$p.json"
  assert_file "persona present: $p" "agents/evolve-$p.md"
done

echo ""
echo "=== AC2: evolve phases validate exits 0 for all 4 wave-3 phases ==="
# validate takes ONE name per invocation (extra args silently ignored). Loop.
for p in $WAVE3; do
  if EVOLVE_PHASE_ROOTS="$ROOT/.evolve/phases" "$EVOLVE_BIN" phases validate "$p" >/dev/null 2>&1; then
    pass "validate exits 0: $p"
  else
    fail "validate exits 0: $p"
  fi
done

echo ""
echo "=== AC3: 4 profiles JSON-valid with required fields ==="
# Required keys mirror the shipped profile schema (.evolve/profiles/mutation-gate.json).
for p in $WAVE3; do
  if [ ! -f ".evolve/profiles/$p.json" ]; then fail "profile schema: $p (file missing)"; continue; fi
  if python3 - "$p" <<'PY'
import json, sys
p = sys.argv[1]
required = ["name", "cli", "model_tier_default", "role", "sandbox",
            "max_turns", "max_budget_usd", "allowed_tools", "output_artifact"]
d = json.load(open(f".evolve/profiles/{p}.json"))
missing = [k for k in required if k not in d]
if missing:
    print(f"missing keys in {p}.json: {missing}", file=sys.stderr); sys.exit(1)
if d["name"] != p:
    print(f"profile name {d['name']!r} != {p!r}", file=sys.stderr); sys.exit(1)
PY
  then pass "profile JSON-valid w/ required fields: $p"
  else fail "profile JSON-valid w/ required fields: $p"
  fi
done

echo ""
echo "=== AC4: two-tier naming enforced (<object>-<action>, name == dirname) ==="
for p in $WAVE3; do
  if ! echo "$p" | grep -qE '^[a-z]+(-[a-z]+)+$'; then
    fail "two-tier name shape: $p"; continue
  fi
  if [ ! -f ".evolve/phases/$p/phase.json" ]; then
    fail "two-tier naming: $p (phase.json missing)"; continue
  fi
  jname=$(python3 -c "import json;print(json.load(open('.evolve/phases/$p/phase.json'))['name'])" 2>/dev/null)
  if [ "$jname" = "$p" ]; then
    pass "two-tier naming + name==dirname: $p"
  else
    fail "two-tier naming: $p (phase.json name=$jname)"
  fi
done

echo ""
echo "=== AC7: archetype contracts (changelog-sync=control is the named AC) ==="
check_archetype() {
  local p="$1" want="$2"
  if [ ! -f ".evolve/phases/$p/phase.json" ]; then fail "archetype $want: $p (missing)"; return; fi
  got=$(python3 -c "import json;print(json.load(open('.evolve/phases/$p/phase.json')).get('archetype'))" 2>/dev/null)
  if [ "$got" = "$want" ]; then pass "archetype $want: $p"; else fail "archetype $want: $p (got $got)"; fi
}
check_archetype changelog-sync     control
check_archetype post-ship-monitor  control
check_archetype context-condense   control
check_archetype api-contract-design plan

echo ""
echo "=== Signal namespaces per micro-phase-catalog §3 / §4.3 ==="
check_signal() {
  local p="$1" sig="$2"
  if [ ! -f ".evolve/phases/$p/phase.json" ]; then fail "signal $sig: $p (missing)"; return; fi
  if python3 -c "
import json,sys
d=json.load(open('.evolve/phases/$p/phase.json'))
sys.exit(0 if '$sig' in d.get('outputs',{}).get('signals',[]) else 1)" 2>/dev/null; then
    pass "output signal $sig: $p"
  else
    fail "output signal $sig: $p"
  fi
}
check_signal changelog-sync     changelog.drift_count
check_signal post-ship-monitor  post_ship.health
check_signal api-contract-design contract.surfaces
check_signal context-condense   condense.ratio

echo ""
echo "=== Catalog card / persona updates (task target files) ==="
# Catalog-card rows are backtick-quoted table rows (`| \`name\` | ...`);
# a bare name-grep is pre-existing GREEN via the goal-type recipe table
# (router lines 54/58) and proves nothing — require the card row shape.
for p in $WAVE3; do
  if grep -qF "| \`$p\` |" agents/evolve-router.md; then
    pass "router catalog-card row: $p"
  else
    fail "router catalog-card row: $p"
  fi
done
if grep -qi "problem-reflection" agents/evolve-spec-verify.md; then
  pass "spec-verify carries AlphaCodium problem-reflection card"
else
  fail "spec-verify carries AlphaCodium problem-reflection card"
fi
if grep -qi "solution-ranking" agents/evolve-architecture-design.md; then
  pass "architecture-design carries AlphaCodium solution-ranking card"
else
  fail "architecture-design carries AlphaCodium solution-ranking card"
fi
# NOTE: scout-report names the target "agents/evolve-retro.md", which does not
# exist; the retro persona on disk is agents/evolve-retrospective.md. Encoding
# the real path (drift documented in test-report.md).
if grep -qi "lesson-extract" agents/evolve-retrospective.md; then
  pass "retrospective carries ExpeL lesson-extract note"
else
  fail "retrospective carries ExpeL lesson-extract note"
fi

echo ""
echo "=== AC8 structural floor: agent.md non-boilerplate floor (substance = Auditor checklist) ==="
for p in $WAVE3; do
  f=".evolve/phases/$p/agent.md"
  if [ ! -f "$f" ]; then fail "agent.md floor: $p (missing)"; continue; fi
  lines=$(wc -l < "$f" | tr -d ' ')
  if [ "$lines" -ge 15 ]; then
    pass "agent.md floor (>=15 lines, got $lines): $p"
  else
    fail "agent.md floor (<15 lines, got $lines): $p"
  fi
done

echo ""
echo "=== AC5: all wave-3 artifacts git-tracked (dual-check) ==="
for p in $WAVE3; do
  assert_tracked ".evolve/phases/$p/phase.json"
  assert_tracked ".evolve/phases/$p/agent.md"
  assert_tracked ".evolve/profiles/$p.json"
  assert_tracked "agents/evolve-$p.md"
done
assert_tracked ".evolve/evals/phases-release-and-memory.md"

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
