#!/usr/bin/env bash
# test-doctor-subscription-auth.sh — Unit tests for doctor-subscription-auth.sh
#
# Exit 0 if all tests pass, non-zero if any fail.
# Prints PASS/FAIL per test + final count.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCTOR="$SCRIPT_DIR/../../scripts/utility/doctor-subscription-auth.sh"

pass_count=0
fail_count=0

run_test() {
    local name="$1"
    local expected="$2"
    local actual="$3"

    if [ "$actual" = "$expected" ]; then
        printf 'PASS  %s\n' "$name"
        pass_count=$((pass_count + 1))
    else
        printf 'FAIL  %s\n' "$name"
        printf '      expected: %s\n' "$expected"
        printf '      actual:   %s\n' "$actual"
        fail_count=$((fail_count + 1))
    fi
}

# --- Test 1: CUSTOM_PROXY via EVOLVE_ANTHROPIC_BASE_URL ---
{
    actual=$(env -i EVOLVE_ANTHROPIC_BASE_URL="http://localhost:4000" \
        HOME="$HOME" \
        bash "$DOCTOR" --json 2>/dev/null | grep -o '"mode":"[^"]*"' | grep -o '"[^"]*"$' | tr -d '"')
    run_test "CUSTOM_PROXY verdict (EVOLVE_ANTHROPIC_BASE_URL)" "CUSTOM_PROXY" "$actual"
}

# --- Test 2: API_KEY verdict ---
{
    actual=$(env -i ANTHROPIC_API_KEY="sk-test-key" \
        HOME="$HOME" \
        bash "$DOCTOR" --json 2>/dev/null | grep -o '"mode":"[^"]*"' | grep -o '"[^"]*"$' | tr -d '"')
    run_test "API_KEY verdict (ANTHROPIC_API_KEY set)" "API_KEY" "$actual"
}

# --- Test 3: CUSTOM_PROXY via ANTHROPIC_BASE_URL ---
{
    actual=$(env -i ANTHROPIC_BASE_URL="http://proxy.example.com/v1" \
        HOME="$HOME" \
        bash "$DOCTOR" --json 2>/dev/null | grep -o '"mode":"[^"]*"' | grep -o '"[^"]*"$' | tr -d '"')
    run_test "CUSTOM_PROXY verdict (ANTHROPIC_BASE_URL)" "CUSTOM_PROXY" "$actual"
}

# --- Test 4: SUBSCRIPTION_OAUTH verdict (mock cred file via env override) ---
{
    tmp_cred_dir=$(mktemp -d)
    tmp_cred_file="$tmp_cred_dir/credentials.json"
    printf '{"claudeAiOauth":{"accessToken":"fake-token-abc123","refreshToken":"fake-refresh"}}\n' > "$tmp_cred_file"

    actual=$(env -i \
        EVOLVE_DOCTOR_CRED_FILE_OVERRIDE="$tmp_cred_file" \
        HOME="$tmp_cred_dir" \
        bash "$DOCTOR" --json 2>/dev/null | grep -o '"mode":"[^"]*"' | grep -o '"[^"]*"$' | tr -d '"')
    run_test "SUBSCRIPTION_OAUTH verdict (mock credentials.json)" "SUBSCRIPTION_OAUTH" "$actual"
    rm -rf "$tmp_cred_dir"
}

# --- Test 5: MISCONFIGURED verdict (no vars, nonexistent cred file) ---
{
    actual=$(env -i \
        EVOLVE_DOCTOR_CRED_FILE_OVERRIDE="/tmp/nonexistent-cred-file-$$" \
        HOME="/tmp" \
        bash "$DOCTOR" --json 2>/dev/null | grep -o '"mode":"[^"]*"' | grep -o '"[^"]*"$' | tr -d '"')
    run_test "MISCONFIGURED verdict (no auth)" "MISCONFIGURED" "$actual"
}

# --- Test 6: --json output is valid JSON with mode field ---
{
    json_out=$(env -i ANTHROPIC_API_KEY="sk-test" \
        HOME="$HOME" \
        bash "$DOCTOR" --json 2>/dev/null)
    # Check it has both "mode" and "notes" fields
    has_mode=$(printf '%s' "$json_out" | grep -c '"mode"' || true)
    has_notes=$(printf '%s' "$json_out" | grep -c '"notes"' || true)
    if [ "$has_mode" -ge 1 ] && [ "$has_notes" -ge 1 ]; then
        run_test "--json output contains mode and notes fields" "ok" "ok"
    else
        run_test "--json output contains mode and notes fields" "ok" "missing fields in: $json_out"
    fi
}

echo ""
printf 'Results: %d PASS, %d FAIL\n' "$pass_count" "$fail_count"

if [ "$fail_count" -gt 0 ]; then
    exit 1
fi
exit 0
