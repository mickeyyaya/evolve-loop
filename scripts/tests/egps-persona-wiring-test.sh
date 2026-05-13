#!/usr/bin/env bash
#
# egps-persona-wiring-test.sh — Verify v10.1.0 EGPS persona directives are in place.
#
# This test does NOT exercise persona behavior (LLM-driven); it just checks that
# the persona files carry the directives that operationalize the v10.0.0
# infrastructure (acs/, validate-predicate.sh, run-acs-suite.sh).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
AGENTS="$PROJECT_ROOT/agents"

PASS=0; FAIL=0; TOTAL=0
pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }
header() { echo; echo "=== $* ==="; }

# ─────────────────────────────────────────────────────────────────────────
header "TEST 1 — Builder persona has EGPS Predicate Authoring section"
grep -q "^## EGPS Predicate Authoring" "$AGENTS/evolve-builder.md" \
    && pass "section header found" || fail "section header missing"
grep -qE "acs/cycle-N/\{NNN\}" "$AGENTS/evolve-builder.md" \
    && pass "predicate path format documented" || fail "predicate path format missing"
grep -q "validate-predicate.sh" "$AGENTS/evolve-builder.md" \
    && pass "validate-predicate.sh referenced" || fail "validate-predicate.sh not mentioned"
grep -q "grep -q.*as the only check" "$AGENTS/evolve-builder.md" \
    && pass "grep-only banned pattern documented" || fail "grep-only ban not explained"

# ─────────────────────────────────────────────────────────────────────────
header "TEST 2 — Auditor persona has EGPS Verdict Computation section"
grep -q "^## EGPS Verdict Computation" "$AGENTS/evolve-auditor.md" \
    && pass "section header found" || fail "section header missing"
grep -q "run-acs-suite.sh" "$AGENTS/evolve-auditor.md" \
    && pass "run-acs-suite.sh referenced" || fail "run-acs-suite.sh not mentioned"
grep -q "acs-verdict.json" "$AGENTS/evolve-auditor.md" \
    && pass "acs-verdict.json mentioned" || fail "acs-verdict.json not mentioned"
grep -qi "WARN.*DEPRECATED.*v10\|deprecated in v10" "$AGENTS/evolve-auditor.md" \
    && pass "WARN deprecation documented" || fail "WARN deprecation not explained"

# ─────────────────────────────────────────────────────────────────────────
header "TEST 3 — Orchestrator persona has EGPS Verdict-of-Record section"
grep -q "^## EGPS Verdict-of-Record" "$AGENTS/evolve-orchestrator.md" \
    && pass "section header found" || fail "section header missing"
grep -q "acs-verdict.json is the verdict-of-record\|acs-verdict.json.*authoritative" "$AGENTS/evolve-orchestrator.md" \
    && pass "verdict-of-record statement present" || fail "verdict-of-record not declared"
grep -q "promote-acs-to-regression.sh" "$AGENTS/evolve-orchestrator.md" \
    && pass "post-ship promotion referenced" || fail "promotion not documented"
grep -q "regression-suite" "$AGENTS/evolve-orchestrator.md" \
    && pass "regression-suite mentioned" || fail "regression-suite not referenced"

# ─────────────────────────────────────────────────────────────────────────
header "TEST 4 — Cross-reference integrity (v10 design doc cited)"
for p in evolve-builder.md evolve-auditor.md evolve-orchestrator.md; do
    if grep -q "egps-v10.md\|docs/architecture/egps-v10" "$AGENTS/$p"; then
        pass "$p references egps-v10 design doc"
    else
        fail "$p missing reference to egps-v10 design doc"
    fi
done

# ─────────────────────────────────────────────────────────────────────────
echo
echo "=== egps-persona-wiring-test.sh — $PASS/$TOTAL PASS ($FAIL fail) ==="
[ "$FAIL" = "0" ] && exit 0 || exit 1
