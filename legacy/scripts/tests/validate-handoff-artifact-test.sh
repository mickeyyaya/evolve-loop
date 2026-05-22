#!/usr/bin/env bash
#
# validate-handoff-artifact-test.sh — tests for validate-handoff-artifact.sh.
# Verifies PASS on valid fixtures (real cycle-37 artifacts) and FAIL on
# stripped fixtures missing required sections.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VALIDATOR="$REPO_ROOT/scripts/tests/validate-handoff-artifact.sh"
SCRATCH=$(mktemp -d)

PASS=0
FAIL=0

pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# --- Test 1: validator exists and is executable ----------------------------
header "Test 1: validator exists and is executable"
[ -f "$VALIDATOR" ] && pass "validate-handoff-artifact.sh exists" || fail "missing $VALIDATOR"
[ -x "$VALIDATOR" ] && pass "validate-handoff-artifact.sh is executable" || fail "$VALIDATOR not executable"

# --- Test 2: schema files exist -------------------------------------------
header "Test 2: schema files exist and are valid JSON"
for t in scout build audit; do
    SCHEMA="$REPO_ROOT/schemas/handoff/${t}-report.schema.json"
    [ -f "$SCHEMA" ] && pass "$t schema file exists" || fail "missing $SCHEMA"
    jq . "$SCHEMA" >/dev/null 2>&1 && pass "$t schema is valid JSON" || fail "$t schema invalid JSON: $SCHEMA"
done

# --- Test 3: PASS on cycle-37 real artifacts ------------------------------
header "Test 3: PASS on cycle-37 real artifacts (backward-compat)"
C37_DIR="$REPO_ROOT/.evolve/runs/cycle-37"
if [ -d "$C37_DIR" ]; then
    for type_file in "scout:scout-report.md" "build:build-report.md" "audit:audit-report.md"; do
        atype="${type_file%%:*}"
        afile="${type_file##*:}"
        artifact="$C37_DIR/$afile"
        if [ -f "$artifact" ]; then
            out=$(bash "$VALIDATOR" --artifact "$artifact" --type "$atype" 2>&1)
            rc=$?
            [ "$rc" -eq 0 ] && pass "cycle-37 $afile PASS (exit 0)" \
                            || fail "cycle-37 $afile should PASS, got exit $rc: $out"
        else
            fail "cycle-37 $afile not found (skip PASS test)"
        fi
    done
else
    fail "cycle-37 workspace missing at $C37_DIR — cannot run backward-compat tests"
fi

# --- Test 4: usage error (exit 2) on missing required args ----------------
header "Test 4: usage error (exit 2) on missing --type"
tmp_art="$SCRATCH/tmp-art.md"
echo "<!-- challenge-token: test -->" > "$tmp_art"
bash "$VALIDATOR" --artifact "$tmp_art" 2>/dev/null
rc=$?
[ "$rc" -eq 2 ] && pass "exit 2 when --type missing" || fail "expected exit 2, got $rc"

# --- Test 5: FAIL on scout-report missing ## Proposed Tasks ---------------
header "Test 5: FAIL on scout-report missing required section"
cat > "$SCRATCH/bad-scout.md" <<'EOF'
<!-- challenge-token: abc123def456 -->
# Scout Report — Test

## Research Summary
Some content here that provides at least one hundred words so the min_words
check does not trigger. We need to ensure the required section check fires
independently. Adding more text to comfortably exceed the minimum word
threshold for this test fixture. This is filler text to meet the count.
Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod.

## Exit Criteria
| # | Criterion |
|---|-----------|
| 1 | Something |
EOF
out=$(bash "$VALIDATOR" --artifact "$SCRATCH/bad-scout.md" --type scout 2>&1)
rc=$?
[ "$rc" -eq 1 ] && pass "exit 1 on missing ## Proposed Tasks" || fail "expected exit 1, got $rc"
echo "$out" | grep -qi "VIOLATION\[proposed_tasks\]" && pass "violation named proposed_tasks" \
    || fail "expected proposed_tasks violation, got: $out"

# --- Test 6: FAIL on build-report missing ## Quality Signals --------------
header "Test 6: FAIL on build-report missing ## Quality Signals"
cat > "$SCRATCH/bad-build.md" <<'EOF'
<!-- challenge-token: abc123def456 -->
# Build Report — Test

**Status:** PASS

<!-- ANCHOR:diff_summary -->
## Changes
- file.sh: added feature

<!-- ANCHOR:test_results -->
## Self-Verification
Tests: 5/5 PASS

EOF
out=$(bash "$VALIDATOR" --artifact "$SCRATCH/bad-build.md" --type build 2>&1)
rc=$?
[ "$rc" -eq 1 ] && pass "exit 1 on missing ## Quality Signals" || fail "expected exit 1, got $rc"
echo "$out" | grep -qi "VIOLATION\[quality_signals\]" && pass "violation named quality_signals" \
    || fail "expected quality_signals violation, got: $out"

# --- Test 7: FAIL on audit-report missing challenge token -----------------
header "Test 7: FAIL on audit-report missing challenge token on line 1"
cat > "$SCRATCH/bad-audit.md" <<'EOF'
# Audit Report — Test (no token on line 1)
<!-- challenge-token: abc123 -->

## Artifacts Reviewed
Some artifacts here.

## Verdict

**PASS**

**Confidence: 0.90**

More text here to exceed one hundred word minimum so min_words check
does not fire. Adding filler text for this test fixture purpose only.
Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod
tempor incididunt ut labore et dolore magna aliqua ut enim ad minim.
EOF
out=$(bash "$VALIDATOR" --artifact "$SCRATCH/bad-audit.md" --type audit 2>&1)
rc=$?
[ "$rc" -eq 1 ] && pass "exit 1 on missing challenge token line 1" || fail "expected exit 1, got $rc"
echo "$out" | grep -qi "VIOLATION\[first_line\]" && pass "violation named first_line" \
    || fail "expected first_line violation, got: $out"

# --- Test 8: FAIL on audit-report missing verdict value -------------------
header "Test 8: FAIL on audit-report missing **PASS|WARN|FAIL** verdict value"
cat > "$SCRATCH/bad-audit2.md" <<'EOF'
<!-- challenge-token: abc123def456 -->
# Audit Report — Test

## Artifacts Reviewed
Some artifacts here.

## Verdict

Verdict is undecided.

**Confidence: 0.90**

More text here to exceed one hundred word minimum so min_words check
does not fire. Adding filler text for this test fixture purpose only.
Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod
tempor incididunt ut labore et dolore magna aliqua ut enim ad minim.
EOF
out=$(bash "$VALIDATOR" --artifact "$SCRATCH/bad-audit2.md" --type audit 2>&1)
rc=$?
[ "$rc" -eq 1 ] && pass "exit 1 on missing verdict value" || fail "expected exit 1, got $rc"
echo "$out" | grep -qi "VIOLATION\[verdict_value\]" && pass "violation named verdict_value" \
    || fail "expected verdict_value violation, got: $out"

# --- Test 9: conditional_sections — skip when no state provided -----------
header "Test 9: conditional carryover section skipped when no --state provided"
cat > "$SCRATCH/scout-no-state.md" <<'EOF'
<!-- challenge-token: abc123def456 -->
# Scout Report — Test

<!-- ANCHOR:proposed_tasks -->
## Proposed Tasks
### Task: do something
Build something useful with proper implementation details.
This task involves creating new files and modifying existing ones.
The scope is well-defined and achievable within a single cycle.

<!-- ANCHOR:acceptance_criteria -->
## Exit Criteria
| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | Something works | run test |
| 2 | No regression | run suite |

## Research Summary
More filler content here to comfortably exceed the one hundred word
minimum threshold that the schema enforces for word count validation.
Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod
tempor incididunt ut labore et dolore magna aliqua ut enim ad minim
veniam quis nostrud exercitation ullamco laboris nisi ut aliquip ex.
EOF
out=$(bash "$VALIDATOR" --artifact "$SCRATCH/scout-no-state.md" --type scout 2>&1)
rc=$?
[ "$rc" -eq 0 ] && pass "exit 0 without --state (conditional section skipped)" \
    || fail "expected exit 0, got $rc: $out"

# --- Test 10: conditional_sections — FAIL when state has carryover todos --
header "Test 10: conditional carryover section FAIL when state has carryoverTodos"
STATE="$SCRATCH/state-with-todos.json"
cat > "$STATE" <<'EOF'
{
  "carryoverTodos": [
    {"id": "todo-test-1", "action": "do something", "priority": "HIGH"}
  ],
  "failedApproaches": []
}
EOF
# Reuse the artifact from Test 9 — it has no ## Carryover Decisions
out=$(bash "$VALIDATOR" --artifact "$SCRATCH/scout-no-state.md" --type scout --state "$STATE" 2>&1)
rc=$?
[ "$rc" -eq 1 ] && pass "exit 1 when carryoverTodos present and section missing" \
    || fail "expected exit 1, got $rc: $out"
echo "$out" | grep -qi "VIOLATION\[carryover_decisions\]" && pass "violation named carryover_decisions" \
    || fail "expected carryover_decisions violation, got: $out"

# --- Test 11: conditional_sections — PASS when state has no carryover -----
header "Test 11: conditional carryover section PASS when state has no carryoverTodos"
STATE_EMPTY="$SCRATCH/state-empty-todos.json"
cat > "$STATE_EMPTY" <<'EOF'
{
  "carryoverTodos": [],
  "failedApproaches": []
}
EOF
out=$(bash "$VALIDATOR" --artifact "$SCRATCH/scout-no-state.md" --type scout --state "$STATE_EMPTY" 2>&1)
rc=$?
[ "$rc" -eq 0 ] && pass "exit 0 when carryoverTodos empty (conditional skip)" \
    || fail "expected exit 0, got $rc: $out"

# --- Test 12: unknown type exits 2 ----------------------------------------
header "Test 12: unknown artifact type exits 2"
bash "$VALIDATOR" --artifact "$SCRATCH/tmp-art.md" --type unknown 2>/dev/null
rc=$?
[ "$rc" -eq 2 ] && pass "exit 2 for unknown type" || fail "expected exit 2, got $rc"

# --- Summary ---------------------------------------------------------------
rm -rf "$SCRATCH"
echo
echo "==========================================="
echo "$PASS pass / $FAIL fail"
echo "==========================================="
[ "$FAIL" -eq 0 ]
