#!/usr/bin/env bash
#
# team-context-test.sh — Unit tests for scripts/team-context.sh.
#
# The team-context bus is a human-readable shared handoff document at
# .evolve/runs/cycle-N/team-context.md. Every pipeline agent appends a
# section before exiting; the next agent reads the bus before starting.
# Replaces fragile JSON handoffs with a single canonical narrative.
#
# Tests cover:
#   1. init creates the file with stub headers for all 5 sections
#      (Goal, Scout Findings, TDD Contract, Build Report, Audit Verdict)
#   2. append writes a role's body under the correct section header
#   3. append is idempotent — re-appending the same role replaces the
#      previous body rather than duplicating
#   4. verify exits 0 when all required sections are populated
#   5. verify exits non-zero when any --require section is empty
#   6. verify accepts comma-separated role list for --require
#
# Tests use a per-test temp workspace so the real .evolve/runs/ tree is
# never touched.
#
# Usage: bash scripts/team-context-test.sh
# Exit:  0 if all assertions pass; non-zero on any failure.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/team-context.sh"

PASS=0
FAIL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

fresh_workspace() {
    local d
    d=$(mktemp -d -t team-context-test.XXXXXX)
    echo "$d"
}

# --- Test 1: script exists ---------------------------------------------------
header "Test 1: scripts/team-context.sh exists"
if [ -f "$SCRIPT" ]; then
    pass "$SCRIPT present"
else
    fail_ "$SCRIPT missing — Phase 2 implementation required"
fi

# Short-circuit subsequent tests if script is missing (avoid noisy failures).
if [ ! -f "$SCRIPT" ]; then
    echo
    echo "=== Summary ==="
    echo "  PASS: $PASS"
    echo "  FAIL: $FAIL"
    exit 1
fi

# --- Test 2: init creates file with required section headers -----------------
header "Test 2: init creates team-context.md with 5 section headers"
WS=$(fresh_workspace)
mkdir -p "$WS/.evolve/runs/cycle-99"
TC_FILE="$WS/.evolve/runs/cycle-99/team-context.md"
if bash "$SCRIPT" init 99 "$WS/.evolve/runs/cycle-99" >/dev/null 2>&1; then
    pass "init exited 0"
else
    fail_ "init exited non-zero"
fi
if [ -f "$TC_FILE" ]; then
    pass "team-context.md created"
else
    fail_ "team-context.md not created"
fi
for section in "## Goal" "## Scout Findings" "## TDD Contract" "## Build Report" "## Audit Verdict"; do
    if grep -qF "$section" "$TC_FILE" 2>/dev/null; then
        pass "section header present: $section"
    else
        fail_ "section header missing: $section"
    fi
done

# --- Test 3: append writes role body under correct section -------------------
header "Test 3: append writes body under role section"
WS=$(fresh_workspace)
WS_DIR="$WS/.evolve/runs/cycle-99"
mkdir -p "$WS_DIR"
bash "$SCRIPT" init 99 "$WS_DIR" >/dev/null 2>&1
BODY=$(mktemp)
echo "Found 5 ranked tasks; selected TASK-2." > "$BODY"
if bash "$SCRIPT" append 99 "$WS_DIR" scout "$BODY" >/dev/null 2>&1; then
    pass "append scout exited 0"
else
    fail_ "append scout exited non-zero"
fi
if grep -q "Found 5 ranked tasks" "$WS_DIR/team-context.md"; then
    pass "scout body written into bus"
else
    fail_ "scout body not found in bus"
fi
rm -f "$BODY"

# --- Test 4: append is idempotent (re-append same role replaces) ------------
header "Test 4: append is idempotent — second append of same role replaces"
WS=$(fresh_workspace)
WS_DIR="$WS/.evolve/runs/cycle-99"
mkdir -p "$WS_DIR"
bash "$SCRIPT" init 99 "$WS_DIR" >/dev/null 2>&1
B1=$(mktemp); echo "FIRST scout body" > "$B1"
B2=$(mktemp); echo "SECOND scout body (replaces first)" > "$B2"
bash "$SCRIPT" append 99 "$WS_DIR" scout "$B1" >/dev/null 2>&1
bash "$SCRIPT" append 99 "$WS_DIR" scout "$B2" >/dev/null 2>&1
if grep -q "SECOND scout body" "$WS_DIR/team-context.md"; then
    pass "second append present"
else
    fail_ "second append missing"
fi
if ! grep -q "FIRST scout body" "$WS_DIR/team-context.md"; then
    pass "first append replaced (idempotent)"
else
    fail_ "first append still present — not idempotent"
fi
rm -f "$B1" "$B2"

# --- Test 5: verify --require exits 0 when sections populated ----------------
header "Test 5: verify --require passes when all required sections populated"
WS=$(fresh_workspace)
WS_DIR="$WS/.evolve/runs/cycle-99"
mkdir -p "$WS_DIR"
bash "$SCRIPT" init 99 "$WS_DIR" >/dev/null 2>&1
for role in scout tdd-engineer builder auditor; do
    B=$(mktemp); echo "$role finding" > "$B"
    bash "$SCRIPT" append 99 "$WS_DIR" "$role" "$B" >/dev/null 2>&1
    rm -f "$B"
done
if bash "$SCRIPT" verify 99 "$WS_DIR" --require scout,tdd-engineer,builder,auditor >/dev/null 2>&1; then
    pass "verify passes when all required sections populated"
else
    fail_ "verify rejected fully populated bus"
fi

# --- Test 6: verify --require exits non-zero when section missing -----------
header "Test 6: verify --require fails on missing section"
WS=$(fresh_workspace)
WS_DIR="$WS/.evolve/runs/cycle-99"
mkdir -p "$WS_DIR"
bash "$SCRIPT" init 99 "$WS_DIR" >/dev/null 2>&1
# Only append scout — tdd-engineer section is empty
B=$(mktemp); echo "scout finding" > "$B"
bash "$SCRIPT" append 99 "$WS_DIR" scout "$B" >/dev/null 2>&1
rm -f "$B"
if ! bash "$SCRIPT" verify 99 "$WS_DIR" --require scout,tdd-engineer >/dev/null 2>&1; then
    pass "verify fails when tdd-engineer section is empty"
else
    fail_ "verify incorrectly passed when required section was empty"
fi

# --- Summary -----------------------------------------------------------------
echo
echo "=== Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
if [ "$FAIL" -eq 0 ]; then
    echo "  ALL CHECKS PASS"
    exit 0
else
    echo "  $FAIL CHECK(S) FAILED"
    exit 1
fi
