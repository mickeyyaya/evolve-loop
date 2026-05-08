#!/usr/bin/env bash
#
# subagent-run-test.sh — Smoke tests for subagent-run.sh.
#
# Tests:
#   1. --validate-profile succeeds for every shipped profile.
#   2. Forgery detection: artifact without challenge token is rejected
#      (cmd_check_token exits 2).
#   3. Token presence: artifact with challenge token is accepted.
#   4. Unknown agent rejected with exit 1.
#   5. Missing required binary (jq) check is wired up. (Optional, skipped if
#      jq is present.)
#
# These tests do NOT execute the live `claude` CLI — they exercise the runner
# and adapter logic up to and including --validate-profile (which short-circuits
# before claude runs). End-to-end CLI invocation is covered by the smoke tests
# in scripts/dispatch/run-cycle.sh / phase-gate.sh integration runs.
#
# Usage: bash scripts/subagent-run-test.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNNER="$REPO_ROOT/scripts/dispatch/subagent-run.sh"
PROFILES_DIR="$REPO_ROOT/.evolve/profiles"

PASS=0
FAIL=0
TESTS_TOTAL=0

pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# --- Test 1: validate every profile ------------------------------------------
header "Test 1: --validate-profile succeeds for every shipped profile"
for agent in scout builder auditor inspirer evaluator retrospective orchestrator; do
    if [ "$agent" = "builder" ]; then
        # Builder profile uses {worktree_path}; provide a stub.
        if WORKTREE_PATH="$REPO_ROOT" bash "$RUNNER" --validate-profile "$agent" >/dev/null 2>&1; then
            pass "builder profile validates (with WORKTREE_PATH stub)"
        else
            fail "builder profile failed validation"
        fi
    else
        if bash "$RUNNER" --validate-profile "$agent" >/dev/null 2>&1; then
            pass "$agent profile validates"
        else
            fail "$agent profile failed validation"
        fi
    fi
done

# --- Test 2: forgery detection -----------------------------------------------
header "Test 2: --check-token rejects artifact without challenge token"
TMPDIR_TEST=$(mktemp -d)
FAKE_ARTIFACT="$TMPDIR_TEST/fake-report.md"
cat > "$FAKE_ARTIFACT" <<'EOF'
# Forged Report

This report was written without the proper challenge token.
Even though it has plenty of words and looks plausible, it lacks the
secret challenge token that the runner mints per invocation.
EOF

if bash "$RUNNER" --check-token "$FAKE_ARTIFACT" "deadbeefcafe0001" >/dev/null 2>&1; then
    fail "forged artifact (no token) was accepted — integrity broken"
else
    rc=$?
    if [ "$rc" -eq 2 ]; then
        pass "forged artifact rejected with exit code 2 (integrity fail)"
    else
        fail "forged artifact rejected but with wrong exit code: $rc (expected 2)"
    fi
fi

# --- Test 3: token presence accepted -----------------------------------------
header "Test 3: --check-token accepts artifact containing challenge token"
GOOD_ARTIFACT="$TMPDIR_TEST/good-report.md"
TOKEN="abc123def456cafe"
cat > "$GOOD_ARTIFACT" <<EOF
<!-- challenge-token: $TOKEN -->
# Genuine Report

Body content here, with file references like scripts/dispatch/subagent-run.sh and
plenty of words to satisfy the substance check.
EOF

if bash "$RUNNER" --check-token "$GOOD_ARTIFACT" "$TOKEN" >/dev/null 2>&1; then
    pass "valid artifact with token accepted"
else
    fail "valid artifact with token rejected (false positive)"
fi

# --- Test 4: unknown agent rejected ------------------------------------------
header "Test 4: unknown agent rejected"
if bash "$RUNNER" "nonexistent_agent" 1 "$TMPDIR_TEST" >/dev/null 2>&1; then
    fail "unknown agent was accepted"
else
    rc=$?
    if [ "$rc" -eq 1 ]; then
        pass "unknown agent rejected with exit code 1"
    else
        fail "unknown agent rejected but with wrong exit code: $rc (expected 1)"
    fi
fi

# --- Test 5: missing artifact rejected ---------------------------------------
header "Test 5: --check-token on missing artifact rejected with exit 2"
if bash "$RUNNER" --check-token "$TMPDIR_TEST/does-not-exist.md" "anything" >/dev/null 2>&1; then
    fail "missing artifact accepted"
else
    rc=$?
    if [ "$rc" -eq 2 ]; then
        pass "missing artifact rejected with integrity exit 2"
    else
        fail "missing artifact rejected with wrong exit: $rc"
    fi
fi

# --- Test 6: legacy fallback signal ------------------------------------------
header "Test 6: LEGACY_AGENT_DISPATCH=1 prints LEGACY_DISPATCH and exits 0"
mkdir -p "$TMPDIR_TEST/cycle-99"
out=$(LEGACY_AGENT_DISPATCH=1 bash "$RUNNER" scout 99 "$TMPDIR_TEST/cycle-99" 2>/dev/null || true)
if [ "$out" = "LEGACY_DISPATCH" ]; then
    pass "legacy fallback emitted correct signal"
else
    fail "legacy fallback signal wrong: got '$out' expected 'LEGACY_DISPATCH'"
fi

# --- Test 7: v8.35.0 — auditor + trivial diff resolves to sonnet ------------
# When MODEL_TIER_HINT is unset and the diff is trivial (≤3 files, ≤100 lines,
# no security paths), resolve_model_tier should auto-downgrade auditor to
# sonnet. Pre-v8.35.0 behavior: always opus (profile default). v8.35.0 saves
# ~$1.89/cycle on routine cycles.
header "Test 7: v8.35.0 — auditor + trivial diff → sonnet"
TIER_REPO=$(mktemp -d -t "subagent-tier-XXX")
cd "$TIER_REPO"
git init -q
git config user.email t@t.t
git config user.name t
echo "init" > seed.txt
git add seed.txt
EVOLVE_BYPASS_SHIP_GATE=1 git -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q -m initial 2>&1 >/dev/null || true
echo "small change" > tiny.txt
git add tiny.txt
# Resolve tier with WORKTREE_PATH pointing here. Note: diff-complexity.sh uses
# `git diff HEAD` by default which sees uncommitted+staged changes.
unset MODEL_TIER_HINT EVOLVE_AUDITOR_TIER_OVERRIDE EVOLVE_DIFF_COMPLEXITY_DISABLE
out=$(WORKTREE_PATH="$TIER_REPO" bash "$RUNNER" --resolve-tier auditor 2>&1)
if [ "$out" = "sonnet" ]; then
    pass "v8.35.0: trivial diff → auditor=sonnet"
else
    fail "expected 'sonnet', got '$out'"
fi
cd "$REPO_ROOT" >/dev/null
rm -rf "$TIER_REPO"

# --- Test 8: v8.35.0 — auditor + complex diff stays opus --------------------
header "Test 8: v8.35.0 — auditor + complex diff (>10 files) → opus"
TIER_REPO=$(mktemp -d -t "subagent-tier-XXX")
cd "$TIER_REPO"
git init -q
git config user.email t@t.t
git config user.name t
echo "init" > seed.txt
git add seed.txt
EVOLVE_BYPASS_SHIP_GATE=1 git -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q -m initial 2>&1 >/dev/null || true
for i in $(seq 1 12); do echo "$i" > "f_$i.txt"; done
git add -A
unset MODEL_TIER_HINT EVOLVE_AUDITOR_TIER_OVERRIDE EVOLVE_DIFF_COMPLEXITY_DISABLE
out=$(WORKTREE_PATH="$TIER_REPO" bash "$RUNNER" --resolve-tier auditor 2>&1)
if [ "$out" = "opus" ]; then
    pass "v8.35.0: complex diff → auditor=opus (profile default)"
else
    fail "expected 'opus', got '$out'"
fi
cd "$REPO_ROOT" >/dev/null
rm -rf "$TIER_REPO"

# --- Test 9: v8.35.0 — MODEL_TIER_HINT overrides auto-tier ------------------
header "Test 9: v8.35.0 — MODEL_TIER_HINT=opus + trivial diff → opus (hint wins)"
TIER_REPO=$(mktemp -d -t "subagent-tier-XXX")
cd "$TIER_REPO"
git init -q
git config user.email t@t.t
git config user.name t
echo "init" > seed.txt
git add seed.txt
EVOLVE_BYPASS_SHIP_GATE=1 git -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q -m initial 2>&1 >/dev/null || true
echo "trivial" > tiny.txt
git add -A
out=$(MODEL_TIER_HINT=opus WORKTREE_PATH="$TIER_REPO" bash "$RUNNER" --resolve-tier auditor 2>&1)
if [ "$out" = "opus" ]; then
    pass "v8.35.0: MODEL_TIER_HINT=opus wins over auto-tier"
else
    fail "expected 'opus', got '$out'"
fi
cd "$REPO_ROOT" >/dev/null
rm -rf "$TIER_REPO"

# --- Test 10: v8.35.0 — non-auditor agents unaffected by auto-tier ----------
header "Test 10: v8.35.0 — non-auditor (scout) trivial diff → profile default"
TIER_REPO=$(mktemp -d -t "subagent-tier-XXX")
cd "$TIER_REPO"
git init -q
git config user.email t@t.t
git config user.name t
echo "init" > seed.txt
git add seed.txt
EVOLVE_BYPASS_SHIP_GATE=1 git -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q -m initial 2>&1 >/dev/null || true
echo "trivial" > tiny.txt
git add -A
unset MODEL_TIER_HINT EVOLVE_AUDITOR_TIER_OVERRIDE
expected_default=$(jq -r '.model_tier_default' "$PROFILES_DIR/scout.json")
out=$(WORKTREE_PATH="$TIER_REPO" bash "$RUNNER" --resolve-tier scout 2>&1)
if [ "$out" = "$expected_default" ]; then
    pass "v8.35.0: scout uses profile default ($expected_default), unaffected by auto-tier"
else
    fail "expected '$expected_default' (scout default), got '$out'"
fi
cd "$REPO_ROOT" >/dev/null
rm -rf "$TIER_REPO"

# --- Test 11: v8.35.0 — kill switch disables auto-tier ----------------------
header "Test 11: v8.35.0 — EVOLVE_DIFF_COMPLEXITY_DISABLE=1 → profile default"
TIER_REPO=$(mktemp -d -t "subagent-tier-XXX")
cd "$TIER_REPO"
git init -q
git config user.email t@t.t
git config user.name t
echo "init" > seed.txt
git add seed.txt
EVOLVE_BYPASS_SHIP_GATE=1 git -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q -m initial 2>&1 >/dev/null || true
echo "trivial" > tiny.txt
git add -A
unset MODEL_TIER_HINT EVOLVE_AUDITOR_TIER_OVERRIDE
out=$(EVOLVE_DIFF_COMPLEXITY_DISABLE=1 WORKTREE_PATH="$TIER_REPO" \
    bash "$RUNNER" --resolve-tier auditor 2>&1)
if [ "$out" = "opus" ]; then
    pass "v8.35.0: kill switch returns profile default (opus)"
else
    fail "expected 'opus', got '$out'"
fi
cd "$REPO_ROOT" >/dev/null
rm -rf "$TIER_REPO"

# --- Summary -----------------------------------------------------------------
rm -rf "$TMPDIR_TEST"
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed (of $((PASS + FAIL)) checks)"
echo "==========================================="
[ "$FAIL" -eq 0 ]
