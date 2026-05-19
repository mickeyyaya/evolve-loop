#!/usr/bin/env bash
#
# test-lint-acs-predicates.sh — Test suite for lint-acs-predicates.sh
#
# Creates inline fixture predicates in a temp dir, runs the linter, and
# asserts expected exit codes.
#
# Exit codes:
#   0 = all tests passed
#   1 = one or more tests failed

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LINTER="$REPO_ROOT/scripts/verification/lint-acs-predicates.sh"

PASS_COUNT=0
FAIL_COUNT=0

# Setup temp dir
TMPDIR_TEST="${TMPDIR:-/tmp}/test-lint-acs-predicates-$$"
mkdir -p "$TMPDIR_TEST"
cleanup() { rm -rf "$TMPDIR_TEST"; }
trap cleanup EXIT

assert_exit() {
    local test_name="$1"
    local expected_rc="$2"
    local actual_rc="$3"
    if [ "$actual_rc" = "$expected_rc" ]; then
        echo "  PASS: $test_name (exit $actual_rc)"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "  FAIL: $test_name (expected exit $expected_rc, got $actual_rc)"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
}

echo "=== test-lint-acs-predicates.sh ==="
echo ""

# ─── FAIL Fixtures (grep-only style — 7 total) ───

# Fixture F1: classic grep-q ; exit $?
FIXTURE_F1="$TMPDIR_TEST/f1"
mkdir -p "$FIXTURE_F1"
cat > "$FIXTURE_F1/pred.sh" <<'PRED'
#!/bin/bash
grep -q "PASS" result.txt ; exit $?
PRED
chmod +x "$FIXTURE_F1/pred.sh"

echo "Test F1: grep-only predicate (grep -q 'PASS' ... ; exit \$?) → expect FAIL (exit 1)"
actual_rc=0
bash "$LINTER" --predicates-dir "$FIXTURE_F1" 2>/dev/null || actual_rc=$?
assert_exit "F1 grep-only detected" 1 "$actual_rc"
echo ""

# Fixture F2: grep -qE variant
FIXTURE_F2="$TMPDIR_TEST/f2"
mkdir -p "$FIXTURE_F2"
cat > "$FIXTURE_F2/pred.sh" <<'PRED'
#!/bin/bash
grep -qE "verdict.*PASS" audit-report.md ; exit $?
PRED
chmod +x "$FIXTURE_F2/pred.sh"

echo "Test F2: grep-only variant (grep -qE regex ; exit \$?) → expect FAIL (exit 1)"
actual_rc=0
bash "$LINTER" --predicates-dir "$FIXTURE_F2" 2>/dev/null || actual_rc=$?
assert_exit "F2 grep-only-E variant detected" 1 "$actual_rc"
echo ""

# Fixture F3: set -uo pipefail + single grep -q (no subprocess)
FIXTURE_F3="$TMPDIR_TEST/f3"
mkdir -p "$FIXTURE_F3"
cat > "$FIXTURE_F3/pred.sh" <<'PRED'
#!/bin/bash
set -uo pipefail
grep -q "something" build.log
PRED
chmod +x "$FIXTURE_F3/pred.sh"

echo "Test F3: set -uo + single grep -q (no subprocess) → expect FAIL (exit 1)"
actual_rc=0
bash "$LINTER" --predicates-dir "$FIXTURE_F3" 2>/dev/null || actual_rc=$?
assert_exit "F3 set+grep-only detected" 1 "$actual_rc"
echo ""

# Fixture F4: grep -qi (case-insensitive) variant
FIXTURE_F4="$TMPDIR_TEST/f4"
mkdir -p "$FIXTURE_F4"
cat > "$FIXTURE_F4/pred.sh" <<'PRED'
#!/bin/bash
grep -qi "predicate quality" agents/evolve-auditor.md ; exit $?
PRED
chmod +x "$FIXTURE_F4/pred.sh"

echo "Test F4: grep -qi variant → expect FAIL (exit 1)"
actual_rc=0
bash "$LINTER" --predicates-dir "$FIXTURE_F4" 2>/dev/null || actual_rc=$?
assert_exit "F4 grep-qi variant detected" 1 "$actual_rc"
echo ""

# Fixture F5: mixed dir (1 behavioral + 1 grep-only) — fails due to grep-only
FIXTURE_F5="$TMPDIR_TEST/f5"
mkdir -p "$FIXTURE_F5"
cat > "$FIXTURE_F5/behavioral.sh" <<'PRED'
#!/bin/bash
count=$(wc -l < result.txt)
[ "$count" -gt 0 ]
PRED
chmod +x "$FIXTURE_F5/behavioral.sh"
cat > "$FIXTURE_F5/greponly.sh" <<'PRED'
#!/bin/bash
grep -q "PASS" result.txt ; exit $?
PRED
chmod +x "$FIXTURE_F5/greponly.sh"

echo "Test F5: mixed dir (1 behavioral + 1 grep-only) → expect FAIL (exit 1)"
actual_rc=0
bash "$LINTER" --predicates-dir "$FIXTURE_F5" 2>/dev/null || actual_rc=$?
assert_exit "F5 mixed dir fails on grep-only" 1 "$actual_rc"
echo ""

# Fixture F6: grep -qF (fixed-string) variant
FIXTURE_F6="$TMPDIR_TEST/f6"
mkdir -p "$FIXTURE_F6"
cat > "$FIXTURE_F6/pred.sh" <<'PRED'
#!/bin/bash
grep -qF "lint-acs-predicates.sh" scripts/lifecycle/phase-gate.sh ; exit $?
PRED
chmod +x "$FIXTURE_F6/pred.sh"

echo "Test F6: grep -qF variant → expect FAIL (exit 1)"
actual_rc=0
bash "$LINTER" --predicates-dir "$FIXTURE_F6" 2>/dev/null || actual_rc=$?
assert_exit "F6 grep-qF variant detected" 1 "$actual_rc"
echo ""

# Fixture F7: two grep-only files (both FAIL)
FIXTURE_F7="$TMPDIR_TEST/f7"
mkdir -p "$FIXTURE_F7"
cat > "$FIXTURE_F7/pred-a.sh" <<'PRED'
#!/bin/bash
grep -q "Predicate quality review" agents/evolve-auditor.md ; exit $?
PRED
chmod +x "$FIXTURE_F7/pred-a.sh"
cat > "$FIXTURE_F7/pred-b.sh" <<'PRED'
#!/bin/bash
grep -q "EVOLVE_TEST_PHASE_ENABLED" CLAUDE.md ; exit $?
PRED
chmod +x "$FIXTURE_F7/pred-b.sh"

echo "Test F7: two grep-only files in same dir → expect FAIL (exit 1)"
actual_rc=0
bash "$LINTER" --predicates-dir "$FIXTURE_F7" 2>/dev/null || actual_rc=$?
assert_exit "F7 two grep-only files both detected" 1 "$actual_rc"
echo ""

# ─── PASS Fixtures (behavioral — 2 total) ───

# Fixture P1: wc -l count check (behavioral)
FIXTURE_P1="$TMPDIR_TEST/p1"
mkdir -p "$FIXTURE_P1"
cat > "$FIXTURE_P1/pred.sh" <<'PRED'
#!/bin/bash
set -uo pipefail
count=$(wc -l < result.txt)
[ "$count" -gt 0 ] && echo PASS || echo FAIL
[ "$count" -gt 0 ]
PRED
chmod +x "$FIXTURE_P1/pred.sh"

echo "Test P1: behavioral predicate (wc -l count check) → expect PASS (exit 0)"
actual_rc=0
bash "$LINTER" --predicates-dir "$FIXTURE_P1" 2>/dev/null || actual_rc=$?
assert_exit "P1 behavioral passes" 0 "$actual_rc"
echo ""

# Fixture P2: jq invocation (behavioral)
FIXTURE_P2="$TMPDIR_TEST/p2"
mkdir -p "$FIXTURE_P2"
cat > "$FIXTURE_P2/pred.sh" <<'PRED'
#!/bin/bash
set -uo pipefail
rc=$(bash scripts/verification/lint-acs-predicates.sh --predicates-dir /tmp/fixture 2>&1; echo $?)
[ "$rc" = "1" ]
PRED
chmod +x "$FIXTURE_P2/pred.sh"

echo "Test P2: behavioral predicate (subprocess invocation) → expect PASS (exit 0)"
actual_rc=0
bash "$LINTER" --predicates-dir "$FIXTURE_P2" 2>/dev/null || actual_rc=$?
assert_exit "P2 behavioral subprocess passes" 0 "$actual_rc"
echo ""

# ─── --explain flag tests ───

echo "Test explain-F: --explain flag shows FAIL verdict for grep-only"
explain_out=$(bash "$LINTER" --predicates-dir "$FIXTURE_F1" --explain 2>&1 || true)
if echo "$explain_out" | grep -q "^FAIL "; then
    echo "  PASS: --explain emits FAIL line"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "  FAIL: --explain did not emit FAIL line (output: $explain_out)"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

echo "Test explain-P: --explain flag shows PASS verdict for behavioral"
explain_out=$(bash "$LINTER" --predicates-dir "$FIXTURE_P1" --explain 2>&1 || true)
if echo "$explain_out" | grep -q "^PASS "; then
    echo "  PASS: --explain emits PASS line"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "  FAIL: --explain did not emit PASS line (output: $explain_out)"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# ─── Summary ───
echo ""
TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo "=== Results: $PASS_COUNT/$TOTAL passed ==="
if [ "$FAIL_COUNT" -gt 0 ]; then
    echo "FAIL: $FAIL_COUNT test(s) failed"
    exit 1
fi
echo "PASS: all tests passed"
exit 0
