#!/usr/bin/env bash
# AC-ID: cycle-95-auditor-mastery-gate
#
# Verifies P2: subagent-run.sh resolves auditor model based on
# state.json:mastery.consecutiveSuccesses.
#
# Required behavior:
#   consecutiveSuccesses >= 1  → "sonnet"  (steady-state, cost-optimized)
#   consecutiveSuccesses == 0  → "opus"    (recovery audit; ditto when missing)
#   non-auditor agent          → profile default, mastery gate inert
#
# The mastery gate MUST take precedence over diff-complexity (otherwise a
# trivial diff on the first post-FAIL audit downgrades to sonnet, defeating
# the "Opus floor on recovery audit" intent contract).
#
# Test seam: subagent-run.sh exposes `--resolve-tier <agent>` (existing
# testability hook at line ~1684) which prints the resolved tier and exits.
# We point EVOLVE_PROJECT_ROOT at a per-test fixture dir containing
# .evolve/state.json with controlled mastery values, and run --resolve-tier.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
SUBAGENT_RUN="$REPO_ROOT/scripts/dispatch/subagent-run.sh"
PROFILES_DIR="$REPO_ROOT/.evolve/profiles"

if [ ! -x "$SUBAGENT_RUN" ] && [ ! -f "$SUBAGENT_RUN" ]; then
    echo "RED cycle-95-auditor-mastery-gate: subagent-run.sh not found at $SUBAGENT_RUN" >&2
    exit 1
fi
if [ ! -f "$PROFILES_DIR/auditor.json" ]; then
    echo "RED cycle-95-auditor-mastery-gate: auditor.json not found at $PROFILES_DIR/auditor.json" >&2
    exit 1
fi

fail=0
errors=""

make_fixture() {
    # $1 = mastery JSON (e.g. '{"consecutiveSuccesses":1,"level":"competent"}' or 'null')
    # Echos the fixture dir path on stdout.
    local mastery_json="$1"
    local fixture
    fixture=$(mktemp -d 2>/dev/null) || fixture="/tmp/cycle-95-mastery-fixture-$$"
    mkdir -p "$fixture/.evolve"
    if [ "$mastery_json" = "null" ]; then
        # Omit mastery field entirely — Builder must treat this as "streak < 1".
        printf '%s\n' '{"version":1,"lastCycleNumber":42}' > "$fixture/.evolve/state.json"
    else
        printf '{"version":1,"lastCycleNumber":42,"mastery":%s}\n' "$mastery_json" \
            > "$fixture/.evolve/state.json"
    fi
    echo "$fixture"
}

resolve_tier_for() {
    # $1 = fixture dir, $2 = agent name
    # Runs the resolver with diff-complexity disabled so the mastery gate is
    # observed in isolation. Returns rc passes through.
    local fixture="$1" agent="$2"
    EVOLVE_PROJECT_ROOT="$fixture" \
    EVOLVE_DIFF_COMPLEXITY_DISABLE=1 \
    MODEL_TIER_HINT="" \
    EVOLVE_AUDITOR_TIER_OVERRIDE="" \
        bash "$SUBAGENT_RUN" --resolve-tier "$agent" 2>/dev/null
}

assert_tier() {
    # $1 = label, $2 = expected, $3 = actual
    local label="$1" expected="$2" actual="$3"
    if [ "$actual" != "$expected" ]; then
        errors="${errors}\n  [$label] expected '$expected' got '$actual'"
        fail=$((fail + 1))
    fi
}

# --- Test 1: streak >= 1 → sonnet (steady-state) ---
fx_streak=$(make_fixture '{"consecutiveSuccesses":1,"level":"novice"}')
actual=$(resolve_tier_for "$fx_streak" auditor)
assert_tier "streak=1 → sonnet" "sonnet" "$actual"
rm -rf "$fx_streak" 2>/dev/null || true

# --- Test 2: streak == 0 → opus (recovery audit floor) ---
fx_zero=$(make_fixture '{"consecutiveSuccesses":0,"level":"novice"}')
actual=$(resolve_tier_for "$fx_zero" auditor)
assert_tier "streak=0 → opus" "opus" "$actual"
rm -rf "$fx_zero" 2>/dev/null || true

# --- Test 3: missing mastery field → opus (safe default; missing == failure-state) ---
fx_missing=$(make_fixture null)
actual=$(resolve_tier_for "$fx_missing" auditor)
assert_tier "missing-mastery → opus" "opus" "$actual"
rm -rf "$fx_missing" 2>/dev/null || true

# --- Test 4: streak >= 2 (large) → sonnet (still allowed) ---
fx_big=$(make_fixture '{"consecutiveSuccesses":10,"level":"proficient"}')
actual=$(resolve_tier_for "$fx_big" auditor)
assert_tier "streak=10 → sonnet" "sonnet" "$actual"
rm -rf "$fx_big" 2>/dev/null || true

# --- Test 5: non-auditor agent — mastery gate inert, returns profile default ---
# Pick a non-auditor profile with a stable model_tier_default. Scout default
# is the safest choice (long-lived agent, default unlikely to change).
if [ -f "$PROFILES_DIR/scout.json" ]; then
    scout_default=$(jq -r '.model_tier_default // "sonnet"' "$PROFILES_DIR/scout.json")
    fx_scout_zero=$(make_fixture '{"consecutiveSuccesses":0,"level":"novice"}')
    actual=$(resolve_tier_for "$fx_scout_zero" scout)
    assert_tier "scout streak=0 → profile-default ($scout_default)" "$scout_default" "$actual"
    rm -rf "$fx_scout_zero" 2>/dev/null || true
fi

# --- Test 6: MODEL_TIER_HINT still wins over mastery gate ---
fx_hint=$(make_fixture '{"consecutiveSuccesses":0,"level":"novice"}')
actual=$(EVOLVE_PROJECT_ROOT="$fx_hint" \
    EVOLVE_DIFF_COMPLEXITY_DISABLE=1 \
    MODEL_TIER_HINT=haiku \
    bash "$SUBAGENT_RUN" --resolve-tier auditor 2>/dev/null)
assert_tier "MODEL_TIER_HINT=haiku overrides mastery gate" "haiku" "$actual"
rm -rf "$fx_hint" 2>/dev/null || true

if [ "$fail" -gt 0 ]; then
    echo "RED cycle-95-auditor-mastery-gate: $fail assertion(s) failed"
    printf "%b\n" "$errors" >&2
    exit 1
fi

echo "GREEN cycle-95-auditor-mastery-gate: mastery gate honors streak>=1→sonnet, streak<1→opus, non-auditor inert, hint-overrides preserved"
exit 0
