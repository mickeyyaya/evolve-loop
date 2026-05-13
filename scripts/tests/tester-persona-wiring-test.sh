#!/usr/bin/env bash
#
# tester-persona-wiring-test.sh — Verify v10.3.0 evolve-tester persona + profile + cross-references.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"

TESTER_PERSONA="$PROJECT_ROOT/agents/evolve-tester.md"
TESTER_PROFILE="$PROJECT_ROOT/.evolve/profiles/tester.json"
BUILDER_PERSONA="$PROJECT_ROOT/agents/evolve-builder.md"
ORCHESTRATOR_PERSONA="$PROJECT_ROOT/agents/evolve-orchestrator.md"

PASS=0; FAIL=0; TOTAL=0
pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }
header() { echo; echo "=== $* ==="; }

# ─────────────────────────────────────────────────────────────────────────
header "TEST 1 — evolve-tester.md persona exists with required structure"
[ -f "$TESTER_PERSONA" ] && pass "agents/evolve-tester.md exists" || fail "persona file missing"
grep -q "^name: evolve-tester" "$TESTER_PERSONA" && pass "name frontmatter set" || fail "name frontmatter missing"
grep -q "predicate" "$TESTER_PERSONA" && pass "describes predicate authorship" || fail "no predicate mention"
grep -q "validate-predicate.sh" "$TESTER_PERSONA" && pass "references validator" || fail "no validator reference"
grep -q "Adversarial mindset\|adversarial" "$TESTER_PERSONA" && pass "adversarial framing present" || fail "no adversarial framing"
grep -q "tester-report.md" "$TESTER_PERSONA" && pass "output artifact specified" || fail "no output artifact spec"

# ─────────────────────────────────────────────────────────────────────────
header "TEST 2 — tester.json profile created with scoped permissions"
[ -f "$TESTER_PROFILE" ] && pass "profile file exists" || fail "profile file missing"
name=$(jq -r '.name' "$TESTER_PROFILE")
[ "$name" = "tester" ] && pass "profile name=tester" || fail "name=$name (expected tester)"
# Should allow validate-predicate + run-acs-suite
jq -e '.allowed_tools[] | select(test("validate-predicate"))' "$TESTER_PROFILE" >/dev/null \
    && pass "allows validate-predicate.sh" || fail "validate-predicate not in allowed_tools"
# Should disallow Write to scripts/dispatch (production code paths)
jq -e '.disallowed_tools[] | select(test("scripts/dispatch"))' "$TESTER_PROFILE" >/dev/null \
    && pass "disallows production code writes (scripts/dispatch/**)" \
    || fail "no disallow for scripts/dispatch — Tester could write production code"
jq -e '.disallowed_tools[] | select(test("agents/"))' "$TESTER_PROFILE" >/dev/null \
    && pass "disallows agents/** edits" || fail "Tester can edit personas"

# ─────────────────────────────────────────────────────────────────────────
header "TEST 3 — Builder persona has v10.3 deferral amendment"
grep -q "v10.3.0+ amendment\|Tester subagent's responsibility" "$BUILDER_PERSONA" \
    && pass "Builder defers predicate authorship to Tester" || fail "no v10.3 deferral note"

# ─────────────────────────────────────────────────────────────────────────
header "TEST 4 — Orchestrator persona has Tester phase step"
grep -q "## EGPS Tester Phase" "$ORCHESTRATOR_PERSONA" \
    && pass "EGPS Tester Phase section present" || fail "Tester phase section missing"
grep -q "subagent-run.sh tester" "$ORCHESTRATOR_PERSONA" \
    && pass "spawn command documented" || fail "no subagent-run.sh tester command"
grep -q "Builder.*Tester.*Auditor\|Tester.*Auditor" "$ORCHESTRATOR_PERSONA" \
    && pass "phase sequence updated" || fail "phase sequence not documented"

# ─────────────────────────────────────────────────────────────────────────
header "TEST 5 — JSON profile is valid + tools shape correct"
jq empty "$TESTER_PROFILE" 2>/dev/null && pass "tester.json is valid JSON" || fail "tester.json invalid"
allowed_count=$(jq '.allowed_tools | length' "$TESTER_PROFILE")
[ "$allowed_count" -ge "10" ] && pass "≥10 allowed tools (inherited from builder)" || fail "too few allowed_tools ($allowed_count)"
disallowed_count=$(jq '.disallowed_tools | length' "$TESTER_PROFILE")
[ "$disallowed_count" -ge "10" ] && pass "≥10 disallowed tools (Tester properly scoped)" || fail "disallowed_tools count low ($disallowed_count)"

# ─────────────────────────────────────────────────────────────────────────
echo
echo "=== tester-persona-wiring-test.sh — $PASS/$TOTAL PASS ($FAIL fail) ==="
[ "$FAIL" = "0" ] && exit 0 || exit 1
