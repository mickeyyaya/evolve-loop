#!/usr/bin/env bash
# consensus-dispatch-test.sh — Tests for v8.54.0 cross-CLI consensus auditor
# dispatch (scripts/dispatch/consensus-dispatch.sh).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DISPATCH="$REPO_ROOT/scripts/dispatch/consensus-dispatch.sh"
AUDITOR_PROFILE="$REPO_ROOT/.evolve/profiles/auditor.json"

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# === Test 1: consensus-dispatch.sh exists + executable =====================
header "Test 1: consensus-dispatch.sh present and executable"
if [ -x "$DISPATCH" ]; then
    pass "exists + executable"
else
    fail_ "missing"
fi

# === Test 2: auditor profile has consensus block =============================
header "Test 2: auditor.json has consensus schema"
if jq -e '.consensus' "$AUDITOR_PROFILE" >/dev/null 2>&1; then
    enabled=$(jq -r '.consensus.enabled' "$AUDITOR_PROFILE")
    voters=$(jq -r '.consensus.cli_voters | length' "$AUDITOR_PROFILE")
    quorum=$(jq -r '.consensus.quorum' "$AUDITOR_PROFILE")
    if [ "$enabled" = "false" ] && [ "$voters" -ge 2 ] && [ "$quorum" -ge 1 ]; then
        pass "schema valid: enabled=$enabled voters=$voters quorum=$quorum"
    else
        fail_ "schema invalid: enabled=$enabled voters=$voters quorum=$quorum"
    fi
else
    fail_ "consensus block missing from auditor.json"
fi

# === Test 3: refuses to dispatch when consensus.enabled=false ===============
header "Test 3: refuses when consensus.enabled=false"
TMPP=$(mktemp -d)
mkdir -p "$TMPP/.evolve/runs/cycle-1"
echo "test prompt" > "$TMPP/prompt"
cat > "$TMPP/auditor-disabled.json" <<EOF
{
  "name": "auditor",
  "cli": "claude",
  "model_tier_default": "sonnet",
  "consensus": {
    "enabled": false,
    "cli_voters": ["claude", "gemini"],
    "quorum": 1
  }
}
EOF
set +e
CYCLE=1 WORKSPACE_PATH="$TMPP/.evolve/runs/cycle-1" \
    PROFILE_PATH="$TMPP/auditor-disabled.json" \
    PROMPT_FILE="$TMPP/prompt" \
    bash "$DISPATCH" 2>/dev/null
rc=$?
: # was: set -e
if [ "$rc" = "10" ]; then
    pass "rc=10 when consensus disabled"
else
    fail_ "expected rc=10, got rc=$rc"
fi
rm -rf "$TMPP"

# === Test 4: refuses when EVOLVE_CONSENSUS_AUDIT=0 ==========================
header "Test 4: refuses when EVOLVE_CONSENSUS_AUDIT=0 (operator opt-out)"
TMPP=$(mktemp -d)
mkdir -p "$TMPP/.evolve/runs/cycle-1"
echo "x" > "$TMPP/prompt"
cat > "$TMPP/auditor.json" <<EOF
{"name":"auditor","cli":"claude","consensus":{"enabled":true,"cli_voters":["claude","gemini"],"quorum":1}}
EOF
set +e
EVOLVE_CONSENSUS_AUDIT=0 CYCLE=1 WORKSPACE_PATH="$TMPP/.evolve/runs/cycle-1" \
    PROFILE_PATH="$TMPP/auditor.json" PROMPT_FILE="$TMPP/prompt" \
    bash "$DISPATCH" 2>/dev/null
rc=$?
: # was: set -e
if [ "$rc" = "2" ]; then
    pass "rc=2 with EVOLVE_CONSENSUS_AUDIT=0"
else
    fail_ "expected rc=2, got rc=$rc"
fi
rm -rf "$TMPP"

# === Test 5: refuses when fewer than 2 eligible voters ======================
header "Test 5: refuses when too few eligible voters"
TMPP=$(mktemp -d)
mkdir -p "$TMPP/.evolve/runs/cycle-1"
echo "x" > "$TMPP/prompt"
# Single voter — below 2-voter minimum
cat > "$TMPP/auditor.json" <<EOF
{"name":"auditor","cli":"claude","model_tier_default":"sonnet","consensus":{"enabled":true,"cli_voters":["claude"],"quorum":1,"require_min_tier":"hybrid"}}
EOF
set +e
CYCLE=1 WORKSPACE_PATH="$TMPP/.evolve/runs/cycle-1" \
    PROFILE_PATH="$TMPP/auditor.json" PROMPT_FILE="$TMPP/prompt" \
    bash "$DISPATCH" 2>/dev/null
rc=$?
: # was: set -e
if [ "$rc" = "2" ]; then
    pass "rc=2 when single voter (below min 2)"
else
    fail_ "expected rc=2, got rc=$rc"
fi
rm -rf "$TMPP"

# === Test 6: profile schema validation ======================================
header "Test 6: missing consensus.cli_voters → rc=10"
TMPP=$(mktemp -d)
mkdir -p "$TMPP/.evolve/runs/cycle-1"
echo "x" > "$TMPP/prompt"
cat > "$TMPP/bad.json" <<EOF
{"name":"auditor","cli":"claude","consensus":{"enabled":true,"cli_voters":[],"quorum":2}}
EOF
set +e
CYCLE=1 WORKSPACE_PATH="$TMPP/.evolve/runs/cycle-1" \
    PROFILE_PATH="$TMPP/bad.json" PROMPT_FILE="$TMPP/prompt" \
    bash "$DISPATCH" 2>/dev/null
rc=$?
: # was: set -e
[ "$rc" = "10" ] && pass "empty cli_voters → rc=10" || fail_ "expected rc=10, got rc=$rc"
rm -rf "$TMPP"

# === Test 7: dispatcher reads MIN_TIER and excludes ineligible ==============
# Smoke: even without claude/gemini in PATH, the dispatcher should at least
# parse the manifest and reach the eligibility check. Since our test env may
# not have claude/gemini binaries, the eligibility check will exclude voters
# at degraded tier. We just verify the script doesn't crash.
header "Test 7: dispatcher parses MIN_TIER without crashing"
TMPP=$(mktemp -d)
mkdir -p "$TMPP/.evolve/runs/cycle-1"
echo "x" > "$TMPP/prompt"
cat > "$TMPP/auditor.json" <<EOF
{"name":"auditor","cli":"claude","model_tier_default":"sonnet","consensus":{"enabled":true,"cli_voters":["claude","gemini","codex"],"quorum":2,"require_min_tier":"none"}}
EOF
set +e
out=$(CYCLE=1 WORKSPACE_PATH="$TMPP/.evolve/runs/cycle-1" \
    PROFILE_PATH="$TMPP/auditor.json" PROMPT_FILE="$TMPP/prompt" \
    bash "$DISPATCH" 2>&1)
rc=$?
: # was: set -e
# rc could be 0/1/2 depending on whether actual claude/gemini binaries run.
# We just verify the script reaches MIN_TIER processing.
if echo "$out" | grep -qE "validating voter capabilities"; then
    pass "MIN_TIER processing reached (rc=$rc)"
else
    fail_ "MIN_TIER processing not reached; out: $(echo "$out" | head -3)"
fi
rm -rf "$TMPP"

# === Test 8: --help-style env var contract documented in script =============
header "Test 8: required env vars documented in script"
required="CYCLE WORKSPACE_PATH PROFILE_PATH PROMPT_FILE"
all_doc=1
for v in $required; do
    if ! grep -q "$v" "$DISPATCH"; then
        echo "    $v not documented" >&2
        all_doc=0
    fi
done
[ "$all_doc" = "1" ] && pass "all 4 required env vars documented" || fail_ "incomplete documentation"

# === Test 9: aggregator cross-cli-vote integration ==========================
header "Test 9: dispatcher invokes aggregator with cross-cli-vote"
if grep -q "cross-cli-vote" "$DISPATCH"; then
    pass "dispatcher uses cross-cli-vote merge mode"
else
    fail_ "cross-cli-vote not referenced"
fi

# === Test 10: consensus block schema includes all required fields ==========
header "Test 10: auditor consensus schema has cli_voters, quorum, require_min_tier"
required_fields="cli_voters quorum require_min_tier"
all_present=1
for field in $required_fields; do
    if ! jq -e --arg f "$field" '.consensus | has($f)' "$AUDITOR_PROFILE" >/dev/null 2>&1; then
        all_present=0
    fi
done
[ "$all_present" = "1" ] && pass "all 3 schema fields present" || fail_ "schema incomplete"

echo
echo "==========================================="
echo "  Total: 10 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
