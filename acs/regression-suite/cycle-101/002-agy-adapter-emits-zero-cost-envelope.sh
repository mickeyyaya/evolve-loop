#!/usr/bin/env bash
# ACS 002 — agy adapter emits zero-cost envelope or cost_blind marker in DEGRADED mode.
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

echo "=== ACS 002: agy adapter emits zero-cost envelope ==="

_tmp_prompt=$(mktemp)
_tmp_stdout=$(mktemp)
_tmp_stderr=$(mktemp)
trap 'rm -f "$_tmp_prompt" "$_tmp_stdout" "$_tmp_stderr"' EXIT
echo "test prompt" > "$_tmp_prompt"

# Force DEGRADED mode: override agy binary to empty (no binary), override claude to empty.
# EVOLVE_TESTING=1 activates the test seams.
EVOLVE_TESTING=1 \
EVOLVE_AGY_BINARY="" \
EVOLVE_AGY_CLAUDE_PATH="" \
PROFILE_PATH="$_tmp_prompt" \
PROMPT_FILE="$_tmp_prompt" \
STDOUT_LOG="$_tmp_stdout" \
STDERR_LOG="$_tmp_stderr" \
ARTIFACT_PATH="/dev/null" \
    bash "$ADAPTER" >/dev/null 2>&1
_rc=$?

# DEGRADED mode exits 0
if [ "$_rc" = "0" ]; then
    check "DEGRADED mode exits 0" "ok"
else
    check "DEGRADED mode exits 0" "exit code $_rc"
fi

# STDOUT_LOG must contain "adapter":"agy"
if grep -q '"adapter"' "$_tmp_stdout" && grep -q '"agy"' "$_tmp_stdout"; then
    check 'STDOUT_LOG contains "adapter":"agy"' "ok"
else
    check 'STDOUT_LOG contains "adapter":"agy"' "not found; content: $(cat "$_tmp_stdout" | head -5)"
fi

# STDOUT_LOG must contain cost_blind or cost_usd indicator
if grep -q 'cost_blind\|cost_usd\|degraded_mode' "$_tmp_stdout"; then
    check "STDOUT_LOG contains cost marker (cost_blind or degraded_mode)" "ok"
else
    check "STDOUT_LOG contains cost marker (cost_blind or degraded_mode)" "not found"
fi

# Verify NATIVE mode envelope format: agy.sh source contains the zero-cost envelope fields
if grep -q 'cost_blind.*true' "$ADAPTER" && grep -q '"adapter":"agy"' "$ADAPTER"; then
    check "agy.sh source contains zero-cost NATIVE envelope template" "ok"
else
    check "agy.sh source contains zero-cost NATIVE envelope template" "not found in source"
fi

echo ""
echo "Result: $PASS passed, $FAIL failed"
[ "$FAIL" = "0" ]
