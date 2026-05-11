#!/usr/bin/env bash
# audit-advisory-review-test.sh — targeted unit tests for audit-advisory-review.sh
#
# Tests:
#   Case 1: Default (flag unset) → script is no-op, no artifact written
#   Case 2: Flag set + clean diff → artifact written, exit 0
#   Case 3: Flag set + diff with known smell → artifact captures diff, exit 0
#   Case 4: Cycle ships normally regardless of advisory output
#
# Bash 3.2 portable.

set -uo pipefail

PASS=0
FAIL=0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ADVISORY_SCRIPT="$SCRIPT_DIR/../lifecycle/audit-advisory-review.sh"

pass() { echo "[PASS] $1"; PASS=$((PASS+1)); }
fail() { echo "[FAIL] $1"; FAIL=$((FAIL+1)); }

# Set up a temp workspace and minimal git repo for each test
setup_workspace() {
    local ws
    ws=$(mktemp -d)
    echo "$ws"
}

setup_git_repo() {
    local ws="$1"
    cd "$ws" || return 1
    git init -q
    git config user.email "test@test.com"
    git config user.name "Test"
    echo "initial" > README.md
    git add README.md
    git commit -q -m "initial"
    echo "changed" > README.md
    git add README.md
    git commit -q -m "change"
    cd - > /dev/null || return 1
}

cleanup_workspace() {
    local ws="$1"
    rm -rf "$ws"
}

# ─── Case 1: Default (flag unset) → no-op, no artifact written ───

run_case1() {
    local ws
    ws=$(setup_workspace)
    local artifact="$ws/audit-advisory-review.md"

    unset EVOLVE_AUDIT_ADVISORY_REVIEW 2>/dev/null || true
    bash "$ADVISORY_SCRIPT" "1" "$ws" 2>/dev/null
    local rc=$?

    if [ "$rc" -eq 0 ] && [ ! -f "$artifact" ]; then
        pass "Case 1: flag unset → no-op, exit 0, no artifact"
    else
        fail "Case 1: expected exit 0 + no artifact (got rc=$rc, artifact_exists=$([ -f "$artifact" ] && echo yes || echo no))"
    fi

    cleanup_workspace "$ws"
}

# ─── Case 2: Flag set + clean diff → artifact written, exit 0 ───

run_case2() {
    local ws
    ws=$(setup_workspace)
    setup_git_repo "$ws"
    local artifact="$ws/audit-advisory-review.md"

    local prev_dir
    prev_dir=$(pwd)
    cd "$ws" || return 1

    EVOLVE_AUDIT_ADVISORY_REVIEW=1 bash "$ADVISORY_SCRIPT" "2" "$ws" 2>/dev/null
    local rc=$?

    cd "$prev_dir" || return 1

    if [ "$rc" -eq 0 ] && [ -f "$artifact" ] && [ -s "$artifact" ]; then
        pass "Case 2: flag set + diff → artifact written, exit 0"
    else
        fail "Case 2: expected exit 0 + non-empty artifact (got rc=$rc, artifact_exists=$([ -f "$artifact" ] && echo yes || echo no))"
    fi

    cleanup_workspace "$ws"
}

# ─── Case 3: Flag set + diff with known smell → artifact captures diff, exit 0 ───

run_case3() {
    local ws
    ws=$(setup_workspace)
    setup_git_repo "$ws"
    local artifact="$ws/audit-advisory-review.md"

    local prev_dir
    prev_dir=$(pwd)
    cd "$ws" || return 1

    # Add a known "smell" — a very long function (simulated)
    python3 -c "print('def ' + 'x' * 80 + '():\\n    pass')" >> README.md 2>/dev/null || echo "smell_function_placeholder" >> README.md
    git add README.md
    git commit -q -m "add smell"

    EVOLVE_AUDIT_ADVISORY_REVIEW=1 bash "$ADVISORY_SCRIPT" "3" "$ws" 2>/dev/null
    local rc=$?

    cd "$prev_dir" || return 1

    if [ "$rc" -eq 0 ] && [ -f "$artifact" ] && [ -s "$artifact" ]; then
        # Artifact should contain diff content
        if grep -q "Diff" "$artifact" 2>/dev/null; then
            pass "Case 3: diff with smell → artifact captures diff summary, exit 0"
        else
            fail "Case 3: artifact exists but missing Diff section"
        fi
    else
        fail "Case 3: expected exit 0 + non-empty artifact (got rc=$rc, artifact_exists=$([ -f "$artifact" ] && echo yes || echo no))"
    fi

    cleanup_workspace "$ws"
}

# ─── Case 4: Cycle ships normally regardless of advisory output ───
# Simulates gate_audit_to_ship behavior: advisory script runs, exit 0,
# and the gate continues to ship regardless of artifact content.

run_case4() {
    local ws
    ws=$(setup_workspace)
    setup_git_repo "$ws"

    local prev_dir
    prev_dir=$(pwd)
    cd "$ws" || return 1

    EVOLVE_AUDIT_ADVISORY_REVIEW=1 bash "$ADVISORY_SCRIPT" "4" "$ws" 2>/dev/null
    local advisory_rc=$?

    cd "$prev_dir" || return 1

    # Simulate gate: advisory exit 0 → gate continues → ship succeeds (rc=0)
    local gate_continues=0
    if [ "$advisory_rc" -eq 0 ]; then
        gate_continues=1
    fi

    if [ "$gate_continues" -eq 1 ]; then
        pass "Case 4: advisory exit 0 → gate continues normally (ship not blocked)"
    else
        fail "Case 4: advisory returned rc=$advisory_rc — gate would have been blocked"
    fi

    cleanup_workspace "$ws"
}

# ─── Run all cases ───

echo "=== audit-advisory-review-test.sh ==="
run_case1
run_case2
run_case3
run_case4

echo ""
echo "Results: $PASS/4 PASS, $FAIL/4 FAIL"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
