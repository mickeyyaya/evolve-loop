#!/usr/bin/env bash
# v9.1.0 Cycle 3: reactive quota-likely classification test.
#
# Verifies the _quota_likely() helper in subagent-run.sh correctly identifies
# the Claude Code subscription quota-exhaustion signature:
#   - rc=1 (subagent failed)
#   - empty/blank stderr tail
#   - cumulative cost >= EVOLVE_QUOTA_DANGER_PCT% of EVOLVE_BATCH_BUDGET_CAP
#
# This test exercises the function in isolation by sourcing subagent-run.sh
# definitions and calling _quota_likely directly with crafted inputs. We
# can't fully exercise the failure-path inside subagent-run because that
# would require spawning a real claude binary.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
SUBAGENT_RUN="$PROJECT_ROOT/scripts/dispatch/subagent-run.sh"

PASS=0
FAIL=0

expect() {
    local label="$1"
    local actual="$2"
    local expected="$3"
    if [ "$actual" = "$expected" ]; then
        printf "  PASS: %s\n" "$label"
        PASS=$((PASS + 1))
    else
        printf "  FAIL: %s (expected=%s actual=%s)\n" "$label" "$expected" "$actual" >&2
        FAIL=$((FAIL + 1))
    fi
}

expect_match() {
    local label="$1"
    local actual="$2"
    local pattern="$3"
    # Use bash's =~ ERE operator instead of `echo | grep` to avoid SIGPIPE
    # races on large multi-line inputs under set -o pipefail.
    if [[ "$actual" =~ $pattern ]]; then
        printf "  PASS: %s\n" "$label"
        PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    pattern=%s\n" "$label" "$pattern" >&2
        FAIL=$((FAIL + 1))
    fi
}

# === Test 1: source code presence ===
echo "=== Test 1: Cycle 3 reactive classification wired into subagent-run.sh ==="
src=$(cat "$SUBAGENT_RUN")
expect_match "_quota_likely() defined" "$src" "_quota_likely\\(\\) \\{"
expect_match "called at CLI non-zero exit" "$src" "_quota_likely.*_stderr_tail.*cycle"
expect_match "writes checkpoint quota-likely" "$src" "checkpoint quota-likely"
expect_match "exports EVOLVE_CHECKPOINT_TRIGGERED" "$src" "export EVOLVE_CHECKPOINT_TRIGGERED"
expect_match "EVOLVE_CHECKPOINT_DISABLE opt-out" "$src" "EVOLVE_CHECKPOINT_DISABLE"
expect_match "danger_pct env var" "$src" "EVOLVE_QUOTA_DANGER_PCT"

# === Test 2: stderr non-empty → returns false (not quota-likely) ===
echo
echo "=== Test 2: function logic — non-empty stderr returns false ==="
# Source the helper into the current shell. The script uses early `exit` paths,
# so we extract just the helper function.
helper=$(awk '/^_quota_likely\(\) \{$/,/^\}$/' "$SUBAGENT_RUN")
# Create a stub log() to silence stderr.
eval "log() { :; }
$helper"

# Mock the dependencies that the helper consults.
export EVOLVE_PLUGIN_ROOT="/nonexistent-but-fine"
export EVOLVE_PROJECT_ROOT="/nonexistent-but-fine"

# Real-looking stderr → should return 1 (not quota-likely).
if _quota_likely "ERROR: invalid prompt at line 42" 999 >/dev/null 2>&1; then
    expect "stderr-with-content rejects" "true (BUG)" "false"
else
    expect "stderr-with-content rejects" "false" "false"
fi

# === Test 3: empty stderr + no cost lookup possible → returns false ===
echo
echo "=== Test 3: empty stderr but no show-cycle-cost.sh → returns false ==="
if _quota_likely "" 999 >/dev/null 2>&1; then
    expect "no-scc rejects (conservative)" "true (BUG)" "false"
else
    expect "no-scc rejects (conservative)" "false" "false"
fi

# === Test 4: empty stderr + cost above threshold → returns true ===
echo
echo "=== Test 4: empty stderr + high cost → returns true ==="
# Mock show-cycle-cost.sh to return a cost above the danger threshold.
TMPDIR_TEST=$(mktemp -d)
trap 'rm -rf "$TMPDIR_TEST"' EXIT INT TERM

mkdir -p "$TMPDIR_TEST/scripts/observability"
SCC_MOCK="$TMPDIR_TEST/scripts/observability/show-cycle-cost.sh"
cat > "$SCC_MOCK" <<'EOF'
#!/bin/bash
echo '{"total":{"cost_usd":18.50}}'
EOF
chmod +x "$SCC_MOCK"
export EVOLVE_PLUGIN_ROOT="$TMPDIR_TEST"
export EVOLVE_PROJECT_ROOT="$TMPDIR_TEST"
export EVOLVE_BATCH_BUDGET_CAP="20.00"
export EVOLVE_QUOTA_DANGER_PCT="80"
# $18.50 vs 80% of $20.00 = $16.00 → quota-likely

if _quota_likely "" 999 >/dev/null 2>&1; then
    expect "cost-above-danger classifies as quota-likely" "true" "true"
else
    expect "cost-above-danger classifies as quota-likely" "false (BUG)" "true"
fi

# === Test 5: empty stderr + low cost → returns false ===
echo
echo "=== Test 5: empty stderr + low cost → returns false ==="
cat > "$SCC_MOCK" <<'EOF'
#!/bin/bash
echo '{"total":{"cost_usd":2.00}}'
EOF
# $2.00 vs $16.00 threshold → not quota-likely

if _quota_likely "" 999 >/dev/null 2>&1; then
    expect "cost-below-danger rejects" "true (BUG)" "false"
else
    expect "cost-below-danger rejects" "false" "false"
fi

# === Test 6: stderr literal "<empty>" sentinel treated as empty ===
echo
echo "=== Test 6: stderr literal '<empty>' sentinel treated as empty ==="
cat > "$SCC_MOCK" <<'EOF'
#!/bin/bash
echo '{"total":{"cost_usd":17.00}}'
EOF
# tail returns "<empty>" sentinel when stderr_log is empty — the helper
# must recognize this as empty (not as literal stderr content).
if _quota_likely "<empty>" 999 >/dev/null 2>&1; then
    expect "<empty> sentinel classified" "true" "true"
else
    expect "<empty> sentinel classified" "false (BUG)" "true"
fi

# === Test 7: whitespace-only stderr treated as empty ===
echo
echo "=== Test 7: whitespace-only stderr treated as empty ==="
if _quota_likely $'  \n\n  \t' 999 >/dev/null 2>&1; then
    expect "whitespace-only treated as empty" "true" "true"
else
    expect "whitespace-only treated as empty" "false (BUG)" "true"
fi

# === Test 8: danger_pct=100 disables (only operator signal can checkpoint) ===
echo
echo "=== Test 8: EVOLVE_QUOTA_DANGER_PCT=100 → effectively disabled ==="
cat > "$SCC_MOCK" <<'EOF'
#!/bin/bash
echo '{"total":{"cost_usd":19.00}}'
EOF
# $19 vs 100% of $20 = $20 — does not cross threshold
EVOLVE_QUOTA_DANGER_PCT=100 _quota_likely "" 999 >/dev/null 2>&1
rc=$?
expect 'danger_pct=100 returns false at \$19/\$20' "$rc" "1"

# === Test 9: danger_pct=0 → fires for any empty-stderr rc=1 ===
echo
echo "=== Test 9: EVOLVE_QUOTA_DANGER_PCT=0 → fires for any empty-stderr ==="
cat > "$SCC_MOCK" <<'EOF'
#!/bin/bash
echo '{"total":{"cost_usd":0.01}}'
EOF
EVOLVE_QUOTA_DANGER_PCT=0 _quota_likely "" 999 >/dev/null 2>&1
rc=$?
expect 'danger_pct=0 fires even at $0.01' "$rc" "0"

# === Test 10: syntax/lint of full subagent-run.sh still clean ===
echo
echo "=== Test 10: subagent-run.sh syntax clean ==="
if bash -n "$SUBAGENT_RUN" 2>&1; then
    printf "  PASS: bash -n clean\n"
    PASS=$((PASS + 1))
else
    printf "  FAIL: bash -n failed\n" >&2
    FAIL=$((FAIL + 1))
fi

echo
echo "=== Summary ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"
if [ "$FAIL" -eq 0 ]; then
    echo "ALL TESTS PASSED"
    exit 0
else
    exit 1
fi
