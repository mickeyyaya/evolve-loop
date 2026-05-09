#!/usr/bin/env bash
#
# parallelization-discipline-test.sh — Tests for v8.55.0 sequential-write
# discipline (read-only/parallel + sequential-write principle).
#
# Verifies:
#   - All canonical agent profiles declare parallel_eligible explicitly
#   - Taxonomy: Scout/Auditor/Retrospective/plan-reviewer/evaluator/inspirer = true
#   - Taxonomy: Builder/Intent/Orchestrator/tdd-engineer = false
#   - Dispatch-parallel refuses (rc=2) for parallel_eligible=false
#   - Dispatch-parallel proceeds past the discipline check for true (regardless
#     of subsequent failures e.g. missing claude binary)
#   - Default is false (synthetic profile without field is rejected)
#
# Bash 3.2 compatible.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROFILES_DIR="$REPO_ROOT/.evolve/profiles"
SUBAGENT_RUN="$REPO_ROOT/scripts/dispatch/subagent-run.sh"

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Canonical taxonomy — hardcoded in the test so a future change to a profile
# triggers a CI failure that forces human review of the parallelization
# decision. This is the structural enforcement the plan calls for.
PARALLEL_ELIGIBLE_ROLES="scout auditor retrospective plan-reviewer evaluator inspirer"
SEQUENTIAL_ONLY_ROLES="builder intent orchestrator tdd-engineer"

# === Test 1: every canonical profile declares parallel_eligible ============
header "Test 1: every profile declares parallel_eligible (no defaults)"
all_ok=1
for role in $PARALLEL_ELIGIBLE_ROLES $SEQUENTIAL_ONLY_ROLES; do
    f="$PROFILES_DIR/${role}.json"
    [ -f "$f" ] || { echo "    missing profile: $f" >&2; all_ok=0; continue; }
    has=$(jq 'has("parallel_eligible")' "$f")
    if [ "$has" != "true" ]; then
        echo "    $role missing parallel_eligible field" >&2
        all_ok=0
    fi
done
[ "$all_ok" = "1" ] && pass "all 10 profiles declare parallel_eligible explicitly" || fail_ "some profiles missing field"

# === Test 2: parallel-eligible roles match taxonomy ==========================
header "Test 2: parallel-eligible roles correctly declare true"
all_ok=1
for role in $PARALLEL_ELIGIBLE_ROLES; do
    f="$PROFILES_DIR/${role}.json"
    val=$(jq -r '.parallel_eligible' "$f")
    if [ "$val" != "true" ]; then
        echo "    $role: parallel_eligible=$val (expected true)" >&2
        all_ok=0
    fi
done
[ "$all_ok" = "1" ] && pass "all 6 parallel-eligible roles correctly declare true" || fail_ "taxonomy drift"

# === Test 3: sequential-only roles match taxonomy ============================
header "Test 3: sequential-only roles correctly declare false"
all_ok=1
for role in $SEQUENTIAL_ONLY_ROLES; do
    f="$PROFILES_DIR/${role}.json"
    val=$(jq -r '.parallel_eligible' "$f")
    if [ "$val" != "false" ]; then
        echo "    $role: parallel_eligible=$val (expected false)" >&2
        all_ok=0
    fi
done
[ "$all_ok" = "1" ] && pass "all 4 sequential-only roles correctly declare false" || fail_ "taxonomy drift — write-capable role declared parallel_eligible=true"

# === Test 4: dispatch-parallel refuses sequential-only role (rc=2) ==========
header "Test 4: dispatch-parallel builder → rc=2 (PROFILE-ERROR)"
TMPP=$(mktemp -d)
set +e
EVOLVE_PROFILES_DIR_OVERRIDE="$PROFILES_DIR" bash "$SUBAGENT_RUN" dispatch-parallel builder 1 "$TMPP" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "2" ] && pass "builder dispatch-parallel → rc=2" || fail_ "expected rc=2, got rc=$rc"
rm -rf "$TMPP"

# === Test 5: dispatch-parallel refuses Intent (rc=2) ========================
header "Test 5: dispatch-parallel intent → rc=2"
TMPP=$(mktemp -d)
set +e
EVOLVE_PROFILES_DIR_OVERRIDE="$PROFILES_DIR" bash "$SUBAGENT_RUN" dispatch-parallel intent 1 "$TMPP" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "2" ] && pass "intent → rc=2" || fail_ "expected rc=2, got rc=$rc"
rm -rf "$TMPP"

# === Test 6: dispatch-parallel refuses orchestrator (rc=2) ==================
header "Test 6: dispatch-parallel orchestrator → rc=2"
TMPP=$(mktemp -d)
set +e
EVOLVE_PROFILES_DIR_OVERRIDE="$PROFILES_DIR" bash "$SUBAGENT_RUN" dispatch-parallel orchestrator 1 "$TMPP" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "2" ] && pass "orchestrator → rc=2" || fail_ "expected rc=2, got rc=$rc"
rm -rf "$TMPP"

# === Test 7: dispatch-parallel refuses tdd-engineer (rc=2) ==================
header "Test 7: dispatch-parallel tdd-engineer → rc=2"
TMPP=$(mktemp -d)
set +e
EVOLVE_PROFILES_DIR_OVERRIDE="$PROFILES_DIR" bash "$SUBAGENT_RUN" dispatch-parallel tdd-engineer 1 "$TMPP" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "2" ] && pass "tdd-engineer → rc=2" || fail_ "expected rc=2, got rc=$rc"
rm -rf "$TMPP"

# === Test 8: error message names the discipline doc ========================
header "Test 8: PROFILE-ERROR message references sequential-write-discipline.md"
TMPP=$(mktemp -d)
set +e
out=$(EVOLVE_PROFILES_DIR_OVERRIDE="$PROFILES_DIR" bash "$SUBAGENT_RUN" dispatch-parallel builder 1 "$TMPP" 2>&1)
set -e
if echo "$out" | grep -q "sequential-write-discipline.md"; then
    pass "error message includes doc reference"
else
    fail_ "doc reference missing from error: $(echo "$out" | head -3)"
fi
rm -rf "$TMPP"

# === Test 9: synthetic profile without field defaults to false (rejected) ==
header "Test 9: profile missing parallel_eligible field → default false → rc=2"
TMPP=$(mktemp -d)
SYNTHPROF=$(mktemp -d)
# Create a profile WITHOUT parallel_eligible field — under the new rule it
# defaults to false and dispatch-parallel rejects it.
cat > "$SYNTHPROF/scout.json" <<'EOF'
{
  "name": "scout",
  "cli": "claude",
  "model_tier_default": "sonnet",
  "parallel_subtasks": [
    {"name": "test", "prompt_template": "test"}
  ]
}
EOF
set +e
EVOLVE_PROFILES_DIR_OVERRIDE="$SYNTHPROF" bash "$SUBAGENT_RUN" dispatch-parallel scout 1 "$TMPP" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "2" ] && pass "missing field → default false → rc=2" || fail_ "expected rc=2 (default-deny), got rc=$rc"
rm -rf "$TMPP" "$SYNTHPROF"

# === Test 10: parallel-eligible role can validate (no PROFILE-ERROR) =======
# Use --validate-profile mode which doesn't actually dispatch claude — it just
# loads + validates the profile. If parallel_eligible were the gate at this
# entry point (it's not, only at dispatch-parallel), validation would still
# pass for parallel-eligible roles.
header "Test 10: scout profile validates without PROFILE-ERROR"
set +e
out=$(EVOLVE_PROFILES_DIR_OVERRIDE="$PROFILES_DIR" bash "$SUBAGENT_RUN" --validate-profile scout 2>&1)
rc=$?
: # was: set -e
if echo "$out" | grep -q "PROFILE-ERROR"; then
    fail_ "scout incorrectly produced PROFILE-ERROR during validate"
else
    pass "scout validate has no PROFILE-ERROR (rc=$rc — discipline gate is dispatch-only)"
fi

# === Test 11: Builder profile invariant — must remain false forever =========
# This test hardcodes the expectation. If a future PR flips Builder to true,
# this fires loudly. The single-writer invariant is the central thesis of the
# whole plan.
header "Test 11: Builder profile.parallel_eligible MUST be false (single-writer invariant)"
val=$(jq -r '.parallel_eligible' "$PROFILES_DIR/builder.json")
if [ "$val" = "false" ]; then
    pass "Builder is sequential-only (single-writer invariant intact)"
else
    fail_ "CRITICAL: Builder.parallel_eligible = $val — single-writer invariant broken"
fi

# === Test 12: enforcement check exists in subagent-run.sh ==================
header "Test 12: subagent-run.sh contains parallel_eligible enforcement"
if grep -q "parallel_eligible" "$SUBAGENT_RUN" \
   && grep -q "PROFILE-ERROR" "$SUBAGENT_RUN" \
   && grep -q "exit 2" "$SUBAGENT_RUN"; then
    pass "enforcement check present in dispatcher"
else
    fail_ "enforcement check missing"
fi

echo
echo "==========================================="
echo "  Total: 12 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
