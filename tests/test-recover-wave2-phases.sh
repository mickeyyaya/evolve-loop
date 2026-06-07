#!/usr/bin/env bash
# tests/test-recover-wave2-phases.sh — TDD contract for cycle-247 task 1.
#
# Encodes the acceptance criteria for recovering the dangling cycle-246
# Wave-2 commit (aea56ca): benchmark-gate, fuzz-probe, cleanup-sweep,
# rollback-plan restored to the tree, all 5 wave-2 phases (incl. existing
# mutation-gate) validating, all artifacts git-tracked, zero Go changes.
#
# RED at authoring time (cycle-247 TDD phase): the 4 phase dirs do not
# exist on the post-f01a323 tree. Builder makes this GREEN by cherry-picking
# aea56ca. DO NOT modify this file to make it pass.
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

PASS=0; FAIL=0
BASELINE_SHA="f01a323"   # scout-report cycle-247 "Last commit" — pre-cycle base

pass() { echo "PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

assert_file() {
  local label="$1" path="$2"
  if [ -f "$path" ]; then pass "$label"; else fail "$label ($path missing)"; fi
}

# Dual-check rule (cycle-93+): disk presence AND git tracking. A gitignored
# worktree file passes [ -f ] but is silently dropped at ship.
assert_tracked() {
  local path="$1"
  if [ ! -f "$path" ]; then fail "tracked: $path (missing on disk)"; return; fi
  if git ls-files --error-unmatch "$path" >/dev/null 2>&1; then
    pass "tracked: $path"
  else
    fail "tracked: $path (on disk but untracked — gitignore shadow? use git add -f)"
  fi
}

# Resolve the evolve binary: go/bin/evolve preferred, tracked go/evolve fallback
# (trust-kernel convention — go/bin may be wiped; see 28aa4c3).
EVOLVE_BIN=""
for cand in "$ROOT/go/bin/evolve" "$ROOT/go/evolve"; do
  [ -x "$cand" ] && { EVOLVE_BIN="$cand"; break; }
done
if [ -z "$EVOLVE_BIN" ]; then
  echo "FATAL: no evolve binary at go/bin/evolve or go/evolve"; exit 2
fi

WAVE2="benchmark-gate fuzz-probe cleanup-sweep rollback-plan"

echo "=== AC1: 4 wave-2 phase dirs present (phase.json + agent.md) ==="
for p in $WAVE2; do
  assert_file "phase.json present: $p" ".evolve/phases/$p/phase.json"
  assert_file "agent.md present: $p"   ".evolve/phases/$p/agent.md"
done

echo ""
echo "=== AC3: 4 profiles + 4 persona files present ==="
for p in $WAVE2; do
  assert_file "profile present: $p" ".evolve/profiles/$p.json"
  assert_file "persona present: $p" "agents/evolve-$p.md"
done

echo ""
echo "=== AC2: evolve phases validate exits 0 for all 5 wave-2 phases ==="
# NOTE: validate takes ONE name per invocation (extra args silently ignored —
# verified against the cycle-247 binary). Loop, never pass a list.
for p in $WAVE2 mutation-gate; do
  if EVOLVE_PHASE_ROOTS="$ROOT/.evolve/phases" "$EVOLVE_BIN" phases validate "$p" >/dev/null 2>&1; then
    pass "validate exits 0: $p"
  else
    fail "validate exits 0: $p"
  fi
done

echo ""
echo "=== AC6 (negative): validate rejects unknown phase ==="
out=$(EVOLVE_PHASE_ROOTS="$ROOT/.evolve/phases" "$EVOLVE_BIN" phases validate cycle247-no-such-phase 2>&1)
rc=$?
if [ "$rc" -ne 0 ] && echo "$out" | grep -q "no user phase named"; then
  pass "validate rejects unknown phase (rc=$rc)"
else
  fail "validate rejects unknown phase (rc=$rc, out=$out)"
fi

echo ""
echo "=== AC4: all recovered artifacts git-tracked (dual-check) ==="
for p in $WAVE2; do
  assert_tracked ".evolve/phases/$p/phase.json"
  assert_tracked ".evolve/phases/$p/agent.md"
  assert_tracked ".evolve/profiles/$p.json"
  assert_tracked "agents/evolve-$p.md"
done
# Cherry-pick completeness: cycle-246 ACS suite, test suite, and eval ride along.
assert_tracked "acs/cycle-246/001-wave2-phases-validate.sh"
assert_tracked "acs/cycle-246/002-wave2-artifacts-tracked.sh"
assert_tracked "acs/cycle-246/003-wave2-content-contracts.sh"
assert_tracked "tests/test-phases-quality-gates.sh"
assert_tracked ".evolve/evals/phases-quality-gates.md"

echo ""
echo "=== AC5: zero Go source changes since pre-cycle baseline ==="
if git cat-file -e "$BASELINE_SHA" 2>/dev/null; then
  go_changes=$(git diff --name-only "$BASELINE_SHA"..HEAD -- 'go' | grep '\.go$' || true)
  if [ -z "$go_changes" ]; then
    pass "zero .go files changed since $BASELINE_SHA"
  else
    fail "Go sources changed since $BASELINE_SHA: $go_changes"
  fi
else
  fail "baseline SHA $BASELINE_SHA not found in object store"
fi

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
