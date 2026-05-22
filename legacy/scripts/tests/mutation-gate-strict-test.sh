#!/usr/bin/env bash
#
# mutation-gate-strict-test.sh — Verify v10.2.0 EVOLVE_MUTATION_GATE_STRICT
# env var promotes mutation-gate from WARN to FAIL in phase-gate.sh.
#
# This is a structural test: verifies the STRICT branch exists in both
# mutation rollup sites and contains the expected FAIL semantics.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
PG="$PROJECT_ROOT/scripts/lifecycle/phase-gate.sh"

PASS=0; FAIL=0; TOTAL=0
pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }
header() { echo; echo "=== $* ==="; }

# ─────────────────────────────────────────────────────────────────────────
header "TEST 1 — EVOLVE_MUTATION_GATE_STRICT plumbing"
count=$(grep -c "EVOLVE_MUTATION_GATE_STRICT" "$PG" 2>/dev/null || echo 0)
[ "$count" -ge "2" ] && pass "STRICT env-var referenced ≥2 times (both rollup sites): $count" \
    || fail "expected ≥2 references, got $count"

# Verify both rollups have FAIL path
fail_paths=$(grep -c 'MUTATION-FAIL' "$PG" 2>/dev/null || echo 0)
[ "$fail_paths" -ge "2" ] && pass "MUTATION-FAIL path present in both rollup sites: $fail_paths" \
    || fail "expected ≥2 MUTATION-FAIL paths, got $fail_paths"

# Verify both have return 1 in STRICT branch
return_one=$(grep -c "EVOLVE_MUTATION_GATE_STRICT.*=.*1" "$PG" 2>/dev/null || echo 0)
[ "$return_one" -ge "2" ] && pass "STRICT comparison present ≥2 times" \
    || fail "expected ≥2 STRICT comparisons, got $return_one"

# ─────────────────────────────────────────────────────────────────────────
header "TEST 2 — Default behavior (STRICT not set) preserved"
# When STRICT not set, the rollup should still log WARN. We verify the WARN
# message is reachable (in the else branch of the STRICT check).
warn_after_strict=$(grep -A 6 'EVOLVE_MUTATION_GATE_STRICT.*=.*1' "$PG" | grep -c 'MUTATION-WARN' || echo 0)
[ "$warn_after_strict" -ge "2" ] && pass "WARN message preserved in non-STRICT branch" \
    || fail "WARN path missing or unreachable: $warn_after_strict"

# ─────────────────────────────────────────────────────────────────────────
header "TEST 3 — STRICT migration hint present in WARN message"
hint_count=$(grep -c "set EVOLVE_MUTATION_GATE_STRICT=1 to FAIL" "$PG" 2>/dev/null || echo 0)
[ "$hint_count" -ge "1" ] && pass "WARN message hints at STRICT promotion (operator discovery)" \
    || fail "no operator-discovery hint found"

# ─────────────────────────────────────────────────────────────────────────
header "TEST 4 — bash syntax check"
if bash -n "$PG" 2>/dev/null; then
    pass "phase-gate.sh syntax valid"
else
    fail "phase-gate.sh has syntax errors"
fi

# ─────────────────────────────────────────────────────────────────────────
echo
echo "=== mutation-gate-strict-test.sh — $PASS/$TOTAL PASS ($FAIL fail) ==="
[ "$FAIL" = "0" ] && exit 0 || exit 1
