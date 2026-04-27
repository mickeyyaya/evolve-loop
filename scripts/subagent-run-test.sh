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
# in scripts/run-cycle.sh / phase-gate.sh integration runs.
#
# Usage: bash scripts/subagent-run-test.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNNER="$REPO_ROOT/scripts/subagent-run.sh"
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

Body content here, with file references like scripts/subagent-run.sh and
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

# --- Summary -----------------------------------------------------------------
rm -rf "$TMPDIR_TEST"
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed (of $((PASS + FAIL)) checks)"
echo "==========================================="
[ "$FAIL" -eq 0 ]
