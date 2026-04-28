#!/usr/bin/env bash
#
# claude-adapter-test.sh — Unit tests for scripts/cli_adapters/claude.sh.
#
# v8.13.4: introduced primarily to cover the EVOLVE_MAX_BUDGET_USD override
# path. As more adapter behaviors gain dedicated tests, they belong here too.
#
# Usage: bash scripts/claude-adapter-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ADAPTER="$REPO_ROOT/scripts/cli_adapters/claude.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# Build a minimal profile + invoke adapter in VALIDATE_ONLY mode.
# Captures combined stderr+stdout; tests assert on the [claude-adapter]
# command= and override= log lines.

make_profile() {
    local budget="$1"
    local f
    f=$(mktemp -t claude-adapter-test.XXXXXX.json)
    cat > "$f" <<EOF
{"name":"x","cli":"claude","model_tier_default":"sonnet","max_budget_usd":${budget},"max_turns":30,"permission_mode":"default","allowed_tools":[],"disallowed_tools":[],"add_dir":[],"output_artifact":"out.md","challenge_token_required":false,"extra_flags":[]}
EOF
    echo "$f"
}

run_adapter() {
    # Args: <profile_path> [extra env=value ...]
    local profile="$1"; shift
    local out
    out=$(env CYCLE=99 \
              WORKSPACE_PATH=/tmp \
              PROFILE_PATH="$profile" \
              RESOLVED_MODEL=sonnet \
              PROMPT_FILE=/dev/null \
              STDOUT_LOG=/dev/null \
              STDERR_LOG=/dev/null \
              ARTIFACT_PATH=/dev/null \
              VALIDATE_ONLY=1 \
              "$@" \
              bash "$ADAPTER" 2>&1)
    echo "$out"
}

cleanup_files=()
trap 'for f in "${cleanup_files[@]}"; do rm -f "$f"; done' EXIT

# === Test 1: no override → profile value used =================================
header "Test 1: no EVOLVE_MAX_BUDGET_USD → profile value (0.50)"
p=$(make_profile 0.50); cleanup_files+=("$p")
out=$(run_adapter "$p")
if echo "$out" | grep -q "max-budget-usd 0.50" \
   && ! echo "$out" | grep -q "override max-budget-usd"; then
    pass "profile value passed unchanged"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|override' | head -3)"
fi

# === Test 2: override picked up ===============================================
header "Test 2: EVOLVE_MAX_BUDGET_USD=1.50 → override emitted, override log line present"
p=$(make_profile 0.50); cleanup_files+=("$p")
out=$(run_adapter "$p" EVOLVE_MAX_BUDGET_USD=1.50)
if echo "$out" | grep -q "max-budget-usd 1.50" \
   && echo "$out" | grep -q "override max-budget-usd: 1.50"; then
    pass "override applied + logged"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|override' | head -3)"
fi

# === Test 3: override bumps below the profile value also works ===============
header "Test 3: EVOLVE_MAX_BUDGET_USD=0.10 (below profile) → override applied"
p=$(make_profile 1.00); cleanup_files+=("$p")
out=$(run_adapter "$p" EVOLVE_MAX_BUDGET_USD=0.10)
if echo "$out" | grep -q "max-budget-usd 0.10" \
   && echo "$out" | grep -q "override max-budget-usd: 0.10"; then
    pass "override below profile applied (operator can also TIGHTEN, not just loosen)"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|override' | head -3)"
fi

# === Test 4: malformed override → WARN, profile value retained ===============
header "Test 4: EVOLVE_MAX_BUDGET_USD='garbage' → WARN, profile fallback"
p=$(make_profile 0.50); cleanup_files+=("$p")
out=$(run_adapter "$p" EVOLVE_MAX_BUDGET_USD=garbage)
if echo "$out" | grep -q "WARN: EVOLVE_MAX_BUDGET_USD='garbage'" \
   && echo "$out" | grep -q "max-budget-usd 0.50"; then
    pass "malformed override warned + ignored"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|override|WARN' | head -3)"
fi

# === Test 5: empty-string override → treated as unset (profile retained) =====
header "Test 5: EVOLVE_MAX_BUDGET_USD='' → treated as unset"
p=$(make_profile 0.50); cleanup_files+=("$p")
out=$(run_adapter "$p" EVOLVE_MAX_BUDGET_USD="")
if echo "$out" | grep -q "max-budget-usd 0.50" \
   && ! echo "$out" | grep -q "override max-budget-usd" \
   && ! echo "$out" | grep -q "WARN"; then
    pass "empty string treated as unset (no override, no warn)"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|override|WARN' | head -3)"
fi

# === Test 6: integer override (no decimal) accepted ==========================
header "Test 6: EVOLVE_MAX_BUDGET_USD=2 (integer) → accepted"
p=$(make_profile 0.50); cleanup_files+=("$p")
out=$(run_adapter "$p" EVOLVE_MAX_BUDGET_USD=2)
if echo "$out" | grep -q "max-budget-usd 2" \
   && echo "$out" | grep -q "override max-budget-usd: 2"; then
    pass "integer override accepted"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|override' | head -3)"
fi

# === Test 7: negative override rejected (security: no neg cost cap) ==========
header "Test 7: EVOLVE_MAX_BUDGET_USD=-1 → rejected as malformed"
p=$(make_profile 0.50); cleanup_files+=("$p")
out=$(run_adapter "$p" EVOLVE_MAX_BUDGET_USD=-1)
if echo "$out" | grep -q "WARN: EVOLVE_MAX_BUDGET_USD='-1'" \
   && echo "$out" | grep -q "max-budget-usd 0.50"; then
    pass "negative value rejected"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|override|WARN' | head -3)"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
