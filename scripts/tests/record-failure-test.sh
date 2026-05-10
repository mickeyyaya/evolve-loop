#!/usr/bin/env bash
#
# record-failure-test.sh — Unit tests for record-failure-to-state.sh treeStateSha fix.
#
# Verifies that the `treeStateSha` field in failedApproaches[] entries is
# content-addressable (git tree-object SHA) rather than the SHA-256 of empty
# string (the pre-fix bug: `git diff HEAD` after a builder commit is always empty).
#
# Usage: bash scripts/tests/record-failure-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/failure/record-failure-to-state.sh"

EMPTY_STRING_SHA="e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

cleanup_dirs=()
cleanup_files=()
trap 'for d in "${cleanup_dirs[@]}"; do rm -rf "$d"; done; for f in "${cleanup_files[@]}"; do rm -f "$f"; done' EXIT

# Create a minimal git repo with one empty commit and a known tree.
# Sets GIT_DIR and GIT_WORK_TREE via -C; returns the repo directory path.
make_git_repo() {
    local d
    d=$(mktemp -d)
    cleanup_dirs+=("$d")
    git -C "$d" init -q
    git -C "$d" config user.email "test@test.local"
    git -C "$d" config user.name "Test"
    git -C "$d" commit --allow-empty -m "init" -q
    echo "$d"
}

# Create a minimal workspace with a stub audit-report.md.
make_workspace() {
    local d
    d=$(mktemp -d)
    cleanup_dirs+=("$d")
    cat > "$d/audit-report.md" <<'AUDIT'
## Verdict
**WARN**
## Defects Found
AUDIT
    echo "$d"
}

# Run the recorder and return the treeStateSha from the resulting state.json.
# Args: git_repo_dir workspace_dir state_file
run_recorder() {
    local git_dir="$1" ws="$2" sf="$3"
    # Initialize a minimal state.json for the recorder to append to.
    echo '{"failedApproaches":[]}' > "$sf"
    ( cd "$git_dir" && \
      EVOLVE_STATE_FILE_OVERRIDE="$sf" \
      EVOLVE_PROJECT_ROOT="$git_dir" \
      bash "$SCRIPT" "$ws" WARN 2>/dev/null )
}

extract_tree_sha() {
    local sf="$1"
    jq -r '.failedApproaches[-1].treeStateSha // "MISSING"' "$sf"
}

# === Test 1: clean committed worktree → treeStateSha ≠ empty-string-SHA =======
header "Test 1: clean committed worktree → treeStateSha ≠ SHA256(empty)"
gr=$(make_git_repo)
ws=$(make_workspace)
sf=$(mktemp -t record-failure-test.XXXXXX.json); cleanup_files+=("$sf")
run_recorder "$gr" "$ws" "$sf"
ts=$(extract_tree_sha "$sf")
if [ "$ts" != "$EMPTY_STRING_SHA" ] && [ "$ts" != "MISSING" ] && [ "$ts" != "unknown" ]; then
    pass "treeStateSha=$ts (not empty-string-SHA)"
else
    fail_ "treeStateSha=$ts — expected a real tree SHA (not e3b0c44... or unknown)"
fi

# === Test 2: same commit re-recorded twice → identical treeStateSha (idempotency) =
header "Test 2: same commit re-recorded twice → identical treeStateSha"
gr=$(make_git_repo)
ws=$(make_workspace)
sf=$(mktemp -t record-failure-test.XXXXXX.json); cleanup_files+=("$sf")
echo '{"failedApproaches":[]}' > "$sf"
( cd "$gr" && EVOLVE_STATE_FILE_OVERRIDE="$sf" EVOLVE_PROJECT_ROOT="$gr" bash "$SCRIPT" "$ws" WARN 2>/dev/null )
ts1=$(extract_tree_sha "$sf")
# Append a second entry (state.json already has one entry; recorder appends again).
( cd "$gr" && EVOLVE_STATE_FILE_OVERRIDE="$sf" EVOLVE_PROJECT_ROOT="$gr" bash "$SCRIPT" "$ws" WARN 2>/dev/null )
ts2=$(jq -r '.failedApproaches[-1].treeStateSha // "MISSING"' "$sf")
if [ "$ts1" = "$ts2" ] && [ "$ts1" != "MISSING" ] && [ "$ts1" != "$EMPTY_STRING_SHA" ]; then
    pass "replay determinism: both recordings produced treeStateSha=$ts1"
else
    fail_ "ts1=$ts1 ts2=$ts2 — must be equal, non-empty, non-corrupt"
fi

# === Test 3: two distinct commits → two distinct treeStateSha values ===========
header "Test 3: two distinct commits → two distinct treeStateSha values"
gr=$(make_git_repo)
ws=$(make_workspace)
sf1=$(mktemp -t record-failure-test.XXXXXX.json); cleanup_files+=("$sf1")
sf2=$(mktemp -t record-failure-test.XXXXXX.json); cleanup_files+=("$sf2")
# Record after first commit.
run_recorder "$gr" "$ws" "$sf1"
ts1=$(extract_tree_sha "$sf1")
# Make a second commit (add a file to change the tree).
echo "cycle6" > "$gr/marker.txt"
git -C "$gr" add marker.txt
git -C "$gr" commit -m "second commit" -q
# Record after second commit.
run_recorder "$gr" "$ws" "$sf2"
ts2=$(extract_tree_sha "$sf2")
if [ "$ts1" != "$ts2" ] && [ "$ts1" != "MISSING" ] && [ "$ts2" != "MISSING" ]; then
    pass "cross-commit distinction: ts1=$ts1 ≠ ts2=$ts2"
else
    fail_ "ts1=$ts1 ts2=$ts2 — expected distinct, non-MISSING values"
fi

# === Test 4: no commits (empty repo) or not-a-git-repo → treeStateSha = "unknown" =
header "Test 4: empty repo (no commits) → treeStateSha = \"unknown\""
# Create a git repo with NO commits (init-only).
gr_empty=$(mktemp -d); cleanup_dirs+=("$gr_empty")
git -C "$gr_empty" init -q
git -C "$gr_empty" config user.email "test@test.local"
git -C "$gr_empty" config user.name "Test"
ws=$(make_workspace)
sf=$(mktemp -t record-failure-test.XXXXXX.json); cleanup_files+=("$sf")
echo '{"failedApproaches":[]}' > "$sf"
( cd "$gr_empty" && EVOLVE_STATE_FILE_OVERRIDE="$sf" EVOLVE_PROJECT_ROOT="$gr_empty" bash "$SCRIPT" "$ws" WARN 2>/dev/null ) || true
ts=$(extract_tree_sha "$sf")
if [ "$ts" = "unknown" ]; then
    pass "defensive guard: empty repo → treeStateSha=unknown"
else
    fail_ "expected \"unknown\" for empty repo, got treeStateSha=$ts"
fi

# === Test 5: legacy EVOLVE_STATE_OVERRIDE emits deprecation WARN on stderr ====
header "Test 5: legacy EVOLVE_STATE_OVERRIDE → deprecation WARN on stderr"
gr=$(make_git_repo)
ws=$(make_workspace)
sf=$(mktemp -t record-failure-test.XXXXXX.json); cleanup_files+=("$sf")
echo '{"failedApproaches":[]}' > "$sf"
dep_warn=$( cd "$gr" && EVOLVE_STATE_OVERRIDE="$sf" EVOLVE_PROJECT_ROOT="$gr" bash "$SCRIPT" "$ws" WARN 2>&1 >/dev/null ) || true
if echo "$dep_warn" | grep -q "EVOLVE_STATE_FILE_OVERRIDE"; then
    pass "legacy flag emits deprecation warning naming EVOLVE_STATE_FILE_OVERRIDE"
else
    fail_ "expected deprecation WARN naming EVOLVE_STATE_FILE_OVERRIDE in stderr; got: $dep_warn"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
