#!/usr/bin/env bash
#
# run-all-regression-tests-test.sh — Unit tests for run-all-regression-tests.sh.
# Uses SUITES_OVERRIDE env var (v8.13.6) to inject stub suites without
# rewriting the script.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/run-all-regression-tests.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

cleanup_dirs=()
trap 'for d in "${cleanup_dirs[@]}"; do rm -rf "$d"; done' EXIT

# Build a sandbox dir with stub test scripts. Returns the sandbox path so tests
# can build a SUITES_OVERRIDE pointing at relative paths inside it.
make_stub_repo() {
    local good="$1" bad="$2"
    local d
    d=$(mktemp -d -t runall-test.XXXXXX)
    mkdir -p "$d/scripts"
    local i=0
    for ((i=0; i<good; i++)); do
        cat > "$d/scripts/stub-good-$i.sh" <<'EOF'
#!/usr/bin/env bash
echo "ok"
exit 0
EOF
        chmod +x "$d/scripts/stub-good-$i.sh"
    done
    for ((i=0; i<bad; i++)); do
        cat > "$d/scripts/stub-bad-$i.sh" <<'EOF'
#!/usr/bin/env bash
echo "FAIL: stub bad" >&2
exit 1
EOF
        chmod +x "$d/scripts/stub-bad-$i.sh"
    done
    echo "$d"
}

# Build the SUITES_OVERRIDE string for a stub repo.
build_suites_str() {
    local good="$1" bad="$2"
    local list=""
    for ((i=0; i<good; i++)); do list+="scripts/stub-good-$i.sh "; done
    for ((i=0; i<bad; i++)); do list+="scripts/stub-bad-$i.sh "; done
    echo "$list"
}

# Invoke run-all from inside the stub repo so $REPO_ROOT resolves there.
# We need to copy run-all itself into the stub.
run_with_stubs() {
    local repo="$1"; shift
    cp "$SCRIPT" "$repo/scripts/run-all-regression-tests.sh"
    chmod +x "$repo/scripts/run-all-regression-tests.sh"
    bash "$repo/scripts/run-all-regression-tests.sh" "$@"
}

# === Test 1: all stubs pass → rc=0 ===========================================
header "Test 1: 3 stubs all pass → rc=0"
d=$(make_stub_repo 3 0); cleanup_dirs+=("$d")
set +e; out=$(SUITES_OVERRIDE="$(build_suites_str 3 0)" run_with_stubs "$d" 2>&1); rc=$?; set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "Total: 3  Passed: 3  Failed: 0"; then
    pass "all-pass scenario"
else
    fail_ "rc=$rc out=$(echo "$out" | tail -10)"
fi

# === Test 2: 2 pass + 1 fail → rc=1, FAIL flagged + diagnostic ==============
header "Test 2: 2 pass + 1 fail → rc=1 with diagnostic tail"
d=$(make_stub_repo 2 1); cleanup_dirs+=("$d")
set +e; out=$(SUITES_OVERRIDE="$(build_suites_str 2 1)" run_with_stubs "$d" 2>&1); rc=$?; set -e
if [ "$rc" = "1" ] \
   && echo "$out" | grep -q "Total: 3  Passed: 2  Failed: 1" \
   && echo "$out" | grep -q "✗" \
   && echo "$out" | grep -q "Diagnostic"; then
    pass "one-fail scenario with diagnostic"
else
    fail_ "rc=$rc out=$(echo "$out" | tail -10)"
fi

# === Test 3: --json emits valid summary ======================================
header "Test 3: --json emits valid summary with totals + suites array"
d=$(make_stub_repo 2 1); cleanup_dirs+=("$d")
set +e; out=$(SUITES_OVERRIDE="$(build_suites_str 2 1)" run_with_stubs "$d" --json 2>&1); rc=$?; set -e
total=$(echo "$out" | jq -r '.total' 2>/dev/null || echo "")
passed=$(echo "$out" | jq -r '.passed' 2>/dev/null || echo "")
failed=$(echo "$out" | jq -r '.failed' 2>/dev/null || echo "")
suites_len=$(echo "$out" | jq -r '.suites | length' 2>/dev/null || echo "")
if [ "$rc" = "1" ] && [ "$total" = "3" ] && [ "$passed" = "2" ] && [ "$failed" = "1" ] && [ "$suites_len" = "3" ]; then
    pass "json schema correct"
else
    fail_ "rc=$rc total=$total passed=$passed failed=$failed suites=$suites_len"
fi

# === Test 4: --parallel mode runs successfully ==============================
header "Test 4: --parallel mode honored"
d=$(make_stub_repo 4 0); cleanup_dirs+=("$d")
set +e; out=$(SUITES_OVERRIDE="$(build_suites_str 4 0)" run_with_stubs "$d" --parallel 2>&1); rc=$?; set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "Mode: PARALLEL" && echo "$out" | grep -q "Total: 4  Passed: 4"; then
    pass "parallel mode all-pass"
else
    fail_ "rc=$rc out=$(echo "$out" | tail -10)"
fi

# === Test 5: bad flag → rc=10 ===============================================
header "Test 5: --bogus flag → rc=10"
set +e; out=$(bash "$SCRIPT" --bogus 2>&1); rc=$?; set -e
if [ "$rc" = "10" ]; then pass "bad flag → 10"
else fail_ "rc=$rc out=$out"; fi

# === Test 6: --help → rc=0 ==================================================
header "Test 6: --help short-circuits"
set +e; out=$(bash "$SCRIPT" --help 2>&1); rc=$?; set -e
if [ "$rc" = "0" ] && echo "$out" | grep -qi "Usage:"; then
    pass "help shown"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 7: SUITES_OVERRIDE with single suite ==============================
header "Test 7: SUITES_OVERRIDE with 1 suite → only that one runs"
d=$(make_stub_repo 1 0); cleanup_dirs+=("$d")
set +e; out=$(SUITES_OVERRIDE="scripts/stub-good-0.sh" run_with_stubs "$d" 2>&1); rc=$?; set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "Total: 1  Passed: 1"; then
    pass "single-suite override"
else
    fail_ "rc=$rc out=$(echo "$out" | tail -10)"
fi

echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
