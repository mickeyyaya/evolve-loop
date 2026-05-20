#!/usr/bin/env bash
# ACS 001 — agy adapter exists, is executable, and cross-name resolution works.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADAPTER="$REPO_ROOT/scripts/cli_adapters/agy.sh"
PASS=0
FAIL=0

check() {
    local label="$1" result="$2"
    if [ "$result" = "ok" ]; then
        echo "  PASS: $label"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $label — $result"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== ACS 001: agy adapter dispatch resolves ==="

# 1. File exists
if [ -f "$ADAPTER" ]; then
    check "agy.sh exists" "ok"
else
    check "agy.sh exists" "not found at $ADAPTER"
fi

# 2. File is executable
if [ -x "$ADAPTER" ]; then
    check "agy.sh is executable (100755)" "ok"
else
    check "agy.sh is executable (100755)" "not executable"
fi

# 3. VALIDATE_ONLY dispatch exits 0 (no "adapter not executable" error)
_tmp_prompt=$(mktemp)
_tmp_stdout=$(mktemp)
_tmp_stderr=$(mktemp)
trap 'rm -f "$_tmp_prompt" "$_tmp_stdout" "$_tmp_stderr"' EXIT
echo "test prompt" > "$_tmp_prompt"

VALIDATE_ONLY=1 \
PROMPT_FILE="$_tmp_prompt" \
STDOUT_LOG="$_tmp_stdout" \
STDERR_LOG="$_tmp_stderr" \
ARTIFACT_PATH="/dev/null" \
PROFILE_PATH="/dev/null" \
    bash "$ADAPTER" >/dev/null 2>&1
_rc=$?
if [ "$_rc" = "0" ]; then
    check "VALIDATE_ONLY=1 exits 0" "ok"
else
    check "VALIDATE_ONLY=1 exits 0" "exit code $_rc"
fi

# 4. Cross-name resolver in subagent-run.sh: both sites patch
SUBAGENT_RUN="$REPO_ROOT/scripts/dispatch/subagent-run.sh"
_site1_count=$(grep -c 'vp_cli.*antigravity.*agy' "$SUBAGENT_RUN" 2>/dev/null || echo 0)
_site2_count=$(grep -c '"antigravity".*cli.*agy' "$SUBAGENT_RUN" 2>/dev/null || echo 0)
# Use a broader match to catch both resolver patterns
_resolver_count=$(grep -c 'antigravity.*agy' "$SUBAGENT_RUN" 2>/dev/null || echo 0)
if [ "$_resolver_count" -ge 2 ]; then
    check "subagent-run.sh has 2+ cross-name resolver lines" "ok"
else
    check "subagent-run.sh has 2+ cross-name resolver lines" "found $_resolver_count (need >=2)"
fi

echo ""
echo "Result: $PASS passed, $FAIL failed"
[ "$FAIL" = "0" ]
