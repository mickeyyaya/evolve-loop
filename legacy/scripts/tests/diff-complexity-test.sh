#!/usr/bin/env bash
#
# diff-complexity-test.sh — Unit tests for scripts/utility/diff-complexity.sh.
#
# Tests cover the three tier rules + edge cases:
#   1. trivial: ≤3 files AND ≤100 lines AND no security paths
#   2. complex: >10 files OR >500 lines OR security paths matched
#   3. standard: everything in between
#   4. malformed git output / missing HEAD: safe fallback (trivial — 0/0)
#
# Each test creates a temp git repo, populates it, and shells out to
# diff-complexity.sh. We use `EVOLVE_BYPASS_SHIP_GATE=1` so the ship-gate
# kernel hook (running in the test parent) doesn't block our throwaway
# git commits.
#
# Usage: bash scripts/diff-complexity-test.sh

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/utility/diff-complexity.sh"
SCRATCH=$(mktemp -d -t "diff-complexity-XXXXXX")
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Initialize a tmp repo with one initial commit.
make_repo() {
    local d="$SCRATCH/repo-$RANDOM"
    mkdir -p "$d" && cd "$d"
    git init -q
    git config user.email t@t.t
    git config user.name t
    git config core.hooksPath /dev/null   # disable any global pre-commit hooks
    echo "init" > seed.txt
    git add seed.txt
    EVOLVE_BYPASS_SHIP_GATE=1 git -c commit.gpgsign=false commit -q -m initial 2>&1 \
        | grep -v "^$" >&2 || true
    echo "$d"
}

# Run diff-complexity.sh with the args, expect a JSON line, parse with jq.
# Args: <expected-tier> <expected-files> <expected-security-bool> <reason>
# Remaining args go to diff-complexity.sh.
expect_tier() {
    local expected_tier="$1" expected_files="$2" expected_security="$3" reason="$4"
    shift 4
    local out tier files security
    out=$(bash "$SCRIPT" "$@" 2>&1)
    local rc=$?
    if [ "$rc" != "0" ]; then
        fail_ "$reason: rc=$rc (expected 0); output: $out"
        return
    fi
    tier=$(echo "$out" | jq -r '.tier' 2>/dev/null)
    files=$(echo "$out" | jq -r '.files_changed' 2>/dev/null)
    security=$(echo "$out" | jq -r '.security_paths' 2>/dev/null)
    if [ "$tier" = "$expected_tier" ] && [ "$files" = "$expected_files" ] \
       && [ "$security" = "$expected_security" ]; then
        pass "$reason: tier=$tier files=$files security=$security"
    else
        fail_ "$reason: got tier=$tier files=$files security=$security; expected tier=$expected_tier files=$expected_files security=$expected_security; raw: $out"
    fi
}

# --- Test 1: trivial — 1 file, ~5 lines, no security ------------------------
header "Test 1: trivial diff (1 file, 5 lines)"
REPO=$(make_repo)
cd "$REPO"
printf 'a\nb\nc\nd\ne\n' > foo.txt
git add foo.txt
expect_tier "trivial" "1" "false" "1 file 5 lines uncommitted" --cached
cd "$REPO_ROOT" >/dev/null

# --- Test 2: complex — 12 files ----------------------------------------------
header "Test 2: complex diff (12 files exceeds threshold)"
REPO=$(make_repo)
cd "$REPO"
for i in $(seq 1 12); do echo "$i" > "file_$i.txt"; done
git add -A
expect_tier "complex" "12" "false" "12 files staged" --cached
cd "$REPO_ROOT" >/dev/null

# --- Test 3: complex — security path triggers regardless of size -------------
header "Test 3: complex diff (auth/ path matched)"
REPO=$(make_repo)
cd "$REPO"
mkdir -p src/auth
echo "x" > src/auth/login.ts
git add -A
expect_tier "complex" "1" "true" "single auth/ file" --cached
cd "$REPO_ROOT" >/dev/null

# --- Test 4: complex — .env file matched -------------------------------------
header "Test 4: complex diff (.env file matched)"
REPO=$(make_repo)
cd "$REPO"
echo "DB_URL=foo" > .env.local
git add -A
expect_tier "complex" "1" "true" ".env.local file" --cached
cd "$REPO_ROOT" >/dev/null

# --- Test 5: standard — 5 files, ~200 lines, no security --------------------
header "Test 5: standard diff (5 files, 200 lines, no security)"
REPO=$(make_repo)
cd "$REPO"
for i in $(seq 1 5); do
    # 40 lines per file = 200 lines total
    seq 1 40 > "src_$i.txt"
done
git add -A
expect_tier "standard" "5" "false" "5 files * 40 lines" --cached
cd "$REPO_ROOT" >/dev/null

# --- Test 6: trivial boundary — exactly 3 files, 100 lines ------------------
header "Test 6: trivial boundary (3 files, 99 lines, no security)"
REPO=$(make_repo)
cd "$REPO"
for i in $(seq 1 3); do
    seq 1 33 > "b_$i.txt"   # 33 lines * 3 = 99 lines
done
git add -A
expect_tier "trivial" "3" "false" "3 files at 99 lines" --cached
cd "$REPO_ROOT" >/dev/null

# --- Test 7: complex boundary — 4 files but >500 lines ----------------------
header "Test 7: complex by line count (4 files, 600 lines)"
REPO=$(make_repo)
cd "$REPO"
for i in $(seq 1 4); do seq 1 150 > "lc_$i.txt"; done   # 4 * 150 = 600 lines
git add -A
expect_tier "complex" "4" "false" "4 files but 600 lines" --cached
cd "$REPO_ROOT" >/dev/null

# --- Test 8: empty diff — trivial fallback ----------------------------------
header "Test 8: empty diff (no changes) → trivial 0/0"
REPO=$(make_repo)
cd "$REPO"
expect_tier "trivial" "0" "false" "no changes"
cd "$REPO_ROOT" >/dev/null

# --- Test 9: invalid arg rejected -------------------------------------------
header "Test 9: --bogus rejected with rc=10"
set +e
out=$(bash "$SCRIPT" --bogus 2>&1)
rc=$?
set -e
if [ "$rc" = "10" ]; then
    pass "unknown flag rejected"
else
    fail_ "rc=$rc (expected 10), out=$out"
fi

# --- Test 10: jq output is valid JSON ----------------------------------------
header "Test 10: output parseable as single JSON object"
REPO=$(make_repo)
cd "$REPO"
echo "x" > a.txt
git add a.txt
out=$(bash "$SCRIPT" --cached 2>&1)
if echo "$out" | jq -e 'type == "object" and has("tier")' >/dev/null 2>&1; then
    pass "JSON shape valid (object with tier field)"
else
    fail_ "jq rejected output: $out"
fi
cd "$REPO_ROOT" >/dev/null

echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
