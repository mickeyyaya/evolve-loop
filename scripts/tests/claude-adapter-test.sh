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

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
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

# v8.13.5: profile with budget_tiers. Tiers override max_budget_usd when
# EVOLVE_TASK_MODE is set and matches a key.
make_profile_with_tiers() {
    local default_budget="$1"
    local tiers_json="$2"   # e.g., '{"research":1.50,"deep":2.50}'
    local f
    f=$(mktemp -t claude-adapter-test.XXXXXX.json)
    cat > "$f" <<EOF
{"name":"x","cli":"claude","model_tier_default":"sonnet","max_budget_usd":${default_budget},"max_turns":30,"permission_mode":"default","allowed_tools":[],"disallowed_tools":[],"add_dir":[],"output_artifact":"out.md","challenge_token_required":false,"extra_flags":[],"budget_tiers":${tiers_json}}
EOF
    echo "$f"
}

run_adapter() {
    # Args: <profile_path> [extra env=value ...]
    # v8.26.0: EVOLVE_BUDGET_ENFORCE=1 is set by default so legacy budget-
    # resolution tests continue to verify the resolved value reaches the
    # final command line. Tests that want the v8.26.0 default-unlimited
    # path use run_adapter_unlimited (defined below).
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
              EVOLVE_BUDGET_ENFORCE=1 \
              "$@" \
              bash "$ADAPTER" 2>&1)
    echo "$out"
}

# v8.26.0: variant that exercises the new default-unlimited behavior.
# Used by new tests asserting that --max-budget-usd=999999 is the default.
run_adapter_unlimited() {
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

# === Test 8 (v8.13.5): EVOLVE_TASK_MODE=research with tiers → tier value ====
header "Test 8: EVOLVE_TASK_MODE=research with budget_tiers → tier value used"
p=$(make_profile_with_tiers 0.50 '{"research":1.50,"deep":2.50}'); cleanup_files+=("$p")
out=$(run_adapter "$p" EVOLVE_TASK_MODE=research)
if echo "$out" | grep -q "max-budget-usd 1.50" \
   && echo "$out" | grep -q "task-mode tier: research"; then
    pass "tier resolved + applied"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|task-mode|override|WARN' | head -3)"
fi

# === Test 9: EVOLVE_TASK_MODE with mode key absent → WARN, profile default ==
header "Test 9: EVOLVE_TASK_MODE=nonexistent → WARN, fallback to profile"
p=$(make_profile_with_tiers 0.50 '{"research":1.50}'); cleanup_files+=("$p")
out=$(run_adapter "$p" EVOLVE_TASK_MODE=nonexistent)
if echo "$out" | grep -q "WARN: EVOLVE_TASK_MODE='nonexistent'" \
   && echo "$out" | grep -q "max-budget-usd 0.50"; then
    pass "missing tier key warned + profile fallback"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|task-mode|override|WARN' | head -3)"
fi

# === Test 10: EVOLVE_TASK_MODE without budget_tiers in profile → WARN =======
header "Test 10: EVOLVE_TASK_MODE=research with NO budget_tiers in profile → WARN"
p=$(make_profile 0.50); cleanup_files+=("$p")  # no budget_tiers key at all
out=$(run_adapter "$p" EVOLVE_TASK_MODE=research)
if echo "$out" | grep -q "WARN: EVOLVE_TASK_MODE='research'" \
   && echo "$out" | grep -q "max-budget-usd 0.50"; then
    pass "no-tiers profile → WARN, profile fallback"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|task-mode|override|WARN' | head -3)"
fi

# === Test 11: EVOLVE_MAX_BUDGET_USD wins over EVOLVE_TASK_MODE ==============
# Precedence chain: env-var override > task-mode tier > profile default.
header "Test 11: EVOLVE_MAX_BUDGET_USD overrides EVOLVE_TASK_MODE"
p=$(make_profile_with_tiers 0.50 '{"research":1.50}'); cleanup_files+=("$p")
out=$(run_adapter "$p" EVOLVE_TASK_MODE=research EVOLVE_MAX_BUDGET_USD=3.00)
if echo "$out" | grep -q "max-budget-usd 3.00" \
   && echo "$out" | grep -q "task-mode tier: research" \
   && echo "$out" | grep -q "override max-budget-usd: 3.00"; then
    pass "precedence: override > tier > default"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|task-mode|override|WARN' | head -3)"
fi

# === Test 12: EVOLVE_TASK_MODE=default → tier value used ====================
# The "default" key is just another tier name, no special meaning. Operators
# can declare it for explicitness, but EVOLVE_TASK_MODE=default still has to
# match a key called "default" in budget_tiers — there's no implicit fallback.
header "Test 12: EVOLVE_TASK_MODE=default with 'default' key in tiers → resolved"
p=$(make_profile_with_tiers 0.50 '{"default":0.75,"research":1.50}'); cleanup_files+=("$p")
out=$(run_adapter "$p" EVOLVE_TASK_MODE=default)
if echo "$out" | grep -q "max-budget-usd 0.75" \
   && echo "$out" | grep -q "task-mode tier: default"; then
    pass "explicit 'default' tier resolves"
else
    fail_ "out=$(echo "$out" | grep -E 'max-budget|task-mode|override|WARN' | head -3)"
fi

# === Test 13: v8.25.1 — environment.json:inner_sandbox=false → SANDBOX_USE=0 ===
# Construct a fake project root containing .evolve/environment.json with
# auto_config.inner_sandbox=false. Adapter must respect it: SANDBOX_USE=0
# AND log the source as "environment.json:auto_config.inner_sandbox=false".
header "Test 13: v8.25.1 — environment.json:inner_sandbox=false → sandbox=0 from environment.json"
ws13=$(mktemp -d -t claude-adapter-inner.XXXXXX)
cleanup_files+=("$ws13")  # cleanup uses rm -f; rm -rf for dirs handled by trap
mkdir -p "$ws13/.evolve"
cat > "$ws13/.evolve/environment.json" <<'EOF'
{
  "schema_version": 3,
  "auto_config": {
    "inner_sandbox": false,
    "inner_sandbox_reason": "test-injected: nested-Claude scenario"
  }
}
EOF
# Profile has sandbox.enabled=true to test the override hierarchy correctly.
p13=$(mktemp -t claude-adapter-test.XXXXXX.json)
cleanup_files+=("$p13")
cat > "$p13" <<EOF
{"name":"x","cli":"claude","model_tier_default":"sonnet","max_budget_usd":0.50,"max_turns":30,"permission_mode":"default","allowed_tools":[],"disallowed_tools":[],"add_dir":[],"output_artifact":"out.md","challenge_token_required":false,"extra_flags":[],"sandbox":{"enabled":true}}
EOF
out=$(env CYCLE=99 WORKSPACE_PATH=/tmp PROFILE_PATH="$p13" \
       RESOLVED_MODEL=sonnet PROMPT_FILE=/dev/null \
       STDOUT_LOG=/dev/null STDERR_LOG=/dev/null \
       ARTIFACT_PATH=/dev/null VALIDATE_ONLY=1 \
       EVOLVE_PROJECT_ROOT="$ws13" \
       bash "$ADAPTER" 2>&1)
rm -rf "$ws13"
if echo "$out" | grep -q "inner sandbox-exec DISABLED" \
   && echo "$out" | grep -qE "sandbox=0 \(source: environment.json"; then
    pass "environment.json:inner_sandbox=false → sandbox=0 with explanation logged"
else
    fail_ "out=$(echo "$out" | grep -E 'sandbox|inner' | head -5)"
fi

# === Test 14: v8.25.1 — EVOLVE_FORCE_INNER_SANDBOX=1 overrides environment.json ===
# Operator paranoid mode: force inner sandbox ON even when environment.json says false.
header "Test 14: v8.25.1 — EVOLVE_FORCE_INNER_SANDBOX=1 → sandbox=1 from operator override"
ws14=$(mktemp -d -t claude-adapter-force.XXXXXX)
mkdir -p "$ws14/.evolve"
cat > "$ws14/.evolve/environment.json" <<'EOF'
{"schema_version":3,"auto_config":{"inner_sandbox":false,"inner_sandbox_reason":"nested-Claude"}}
EOF
out=$(env CYCLE=99 WORKSPACE_PATH=/tmp PROFILE_PATH="$p13" \
       RESOLVED_MODEL=sonnet PROMPT_FILE=/dev/null \
       STDOUT_LOG=/dev/null STDERR_LOG=/dev/null \
       ARTIFACT_PATH=/dev/null VALIDATE_ONLY=1 \
       EVOLVE_PROJECT_ROOT="$ws14" EVOLVE_FORCE_INNER_SANDBOX=1 \
       bash "$ADAPTER" 2>&1)
rm -rf "$ws14"
if echo "$out" | grep -qE "sandbox=1 \(source: EVOLVE_FORCE_INNER_SANDBOX=1"; then
    pass "operator force-enable overrides environment.json"
else
    fail_ "out=$(echo "$out" | grep -E 'sandbox|inner' | head -5)"
fi

# === Test 15: v8.25.1 — EVOLVE_INNER_SANDBOX=0 overrides profile-enabled sandbox ===
# Operator explicit-disable: even without environment.json, EVOLVE_INNER_SANDBOX=0
# disables the inner sandbox.
header "Test 15: v8.25.1 — EVOLVE_INNER_SANDBOX=0 → sandbox=0 (no environment.json)"
ws15=$(mktemp -d -t claude-adapter-disable.XXXXXX)
mkdir -p "$ws15/.evolve"
# No environment.json — adapter uses profile (sandbox.enabled=true) AND env override
out=$(env CYCLE=99 WORKSPACE_PATH=/tmp PROFILE_PATH="$p13" \
       RESOLVED_MODEL=sonnet PROMPT_FILE=/dev/null \
       STDOUT_LOG=/dev/null STDERR_LOG=/dev/null \
       ARTIFACT_PATH=/dev/null VALIDATE_ONLY=1 \
       EVOLVE_PROJECT_ROOT="$ws15" EVOLVE_INNER_SANDBOX=0 \
       bash "$ADAPTER" 2>&1)
rm -rf "$ws15"
if echo "$out" | grep -qE "sandbox=0 \(source: EVOLVE_INNER_SANDBOX=0"; then
    pass "operator explicit-disable wins over profile.sandbox.enabled"
else
    fail_ "out=$(echo "$out" | grep -E 'sandbox|inner' | head -5)"
fi

# === Test 16: v8.26.0 — default unlimited budget when ENFORCE not set =======
# v8.26.0: by default, the adapter passes --max-budget-usd 999999 (effectively
# unlimited) regardless of what the profile says. This eliminates the
# BUDGET_EXCEEDED friction that aborted complex meta-goal cycles in v8.25.x.
# Anti-gaming is unaffected — budget caps don't prevent reward hacking.
header "Test 16: v8.26.0 — default unlimited (no EVOLVE_BUDGET_ENFORCE) → max-budget-usd 999999"
p=$(make_profile 0.50); cleanup_files+=("$p")
out=$(run_adapter_unlimited "$p")
if echo "$out" | grep -qE 'budget cap unlimited \(max-budget-usd=999999\)' \
   && echo "$out" | grep -q "max-budget-usd 999999" \
   && echo "$out" | grep -q "was \$0.50 from profile"; then
    pass "default unlimited budget; profile value preserved in log for traceability"
else
    fail_ "out=$(echo "$out" | grep -E 'budget|max-budget' | head -3)"
fi

# === Test 17: v8.26.0 — EVOLVE_BUDGET_CAP pins a hard cap ==================
# Operator override: explicit hard cap. Wins over both default and ENFORCE.
header "Test 17: v8.26.0 — EVOLVE_BUDGET_CAP=2.50 → hard cap, profile value ignored"
p=$(make_profile 0.50); cleanup_files+=("$p")
out=$(run_adapter_unlimited "$p" EVOLVE_BUDGET_CAP=2.50)
if echo "$out" | grep -qE 'EVOLVE_BUDGET_CAP=\$2\.50 \(operator pin' \
   && echo "$out" | grep -q "max-budget-usd 2.50"; then
    pass "operator hard cap honored"
else
    fail_ "out=$(echo "$out" | grep -E 'budget|max-budget' | head -3)"
fi

# === Test 18: v8.26.0 — EVOLVE_BUDGET_ENFORCE=1 uses resolved profile value =
# Legacy strict mode: operator can opt back into the pre-v8.26.0 behavior
# where the profile/env-resolved value gates the spend.
header "Test 18: v8.26.0 — EVOLVE_BUDGET_ENFORCE=1 → uses resolved budget"
p=$(make_profile 0.50); cleanup_files+=("$p")
out=$(run_adapter_unlimited "$p" EVOLVE_BUDGET_ENFORCE=1)
if echo "$out" | grep -q "EVOLVE_BUDGET_ENFORCE=1: using resolved budget" \
   && echo "$out" | grep -q "max-budget-usd 0.50"; then
    pass "ENFORCE=1 restores legacy strict cap"
else
    fail_ "out=$(echo "$out" | grep -E 'budget|max-budget' | head -3)"
fi

# === Test 19: v8.26.0 — invalid EVOLVE_BUDGET_CAP falls through to unlimited =
header "Test 19: v8.26.0 — EVOLVE_BUDGET_CAP=garbage → WARN + fall through to unlimited"
p=$(make_profile 0.50); cleanup_files+=("$p")
out=$(run_adapter_unlimited "$p" EVOLVE_BUDGET_CAP=garbage)
if echo "$out" | grep -q "WARN: EVOLVE_BUDGET_CAP='garbage' invalid" \
   && echo "$out" | grep -q "max-budget-usd 999999"; then
    pass "invalid cap → WARN + fall through to default unlimited"
else
    fail_ "out=$(echo "$out" | grep -E 'budget|max-budget|WARN' | head -3)"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
