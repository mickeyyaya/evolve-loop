#!/usr/bin/env bash
#
# changelog-gen-test.sh — Unit tests for changelog-gen.sh.
#
# Each test sets up a temp git repo with controlled commit messages, runs
# changelog-gen.sh, and asserts on the produced output.
#
# Usage: bash scripts/release/changelog-gen-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GEN="$REPO_ROOT/scripts/release/changelog-gen.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# Build a temp repo with given commit subjects (one per arg).
make_repo_with_commits() {
    local d
    d=$(mktemp -d -t changelog-test.XXXXXX)
    (
        cd "$d"
        git init -q -b main
        git config user.email t@t.t
        git config user.name t
        echo "init" > a.txt && git add . && git commit -q -m "init"
        git tag from-tag
        for msg in "$@"; do
            echo "$msg" >> a.txt
            git add . && git commit -q -m "$msg"
        done
    )
    echo "$d"
}

run_gen() {
    # Args: <repo> <from> <to> <version> [extra flags...]
    local repo="$1"; shift
    # Copy the script into the temp repo so REPO_ROOT resolves correctly.
    mkdir -p "$repo/scripts/release"
    cp "$GEN" "$repo/scripts/release/changelog-gen.sh"
    chmod +x "$repo/scripts/release/changelog-gen.sh"
    (cd "$repo" && bash "$repo/scripts/release/changelog-gen.sh" "$@" 2>&1)
}

cleanup_repos=()
trap 'for r in "${cleanup_repos[@]}"; do rm -rf "$r"; done' EXIT

# === Test 1: feat: → ### Added ================================================
header "Test 1: feat: prefix → ### Added"
r=$(make_repo_with_commits "feat: add login flow"); cleanup_repos+=("$r")
out=$(run_gen "$r" from-tag HEAD 1.0.0 --dry-run)
if echo "$out" | grep -q "### Added" && echo "$out" | grep -q "add login flow"; then
    pass "feat → Added"
else
    fail_ "out=$out"
fi

# === Test 2: fix: → ### Fixed =================================================
header "Test 2: fix: prefix → ### Fixed"
r=$(make_repo_with_commits "fix: nil pointer in auth"); cleanup_repos+=("$r")
out=$(run_gen "$r" from-tag HEAD 1.0.0 --dry-run)
if echo "$out" | grep -q "### Fixed" && echo "$out" | grep -q "nil pointer in auth"; then
    pass "fix → Fixed"
else
    fail_ "out=$out"
fi

# === Test 3: refactor: → ### Changed ==========================================
header "Test 3: refactor: prefix → ### Changed"
r=$(make_repo_with_commits "refactor: simplify token validation"); cleanup_repos+=("$r")
out=$(run_gen "$r" from-tag HEAD 1.0.0 --dry-run)
if echo "$out" | grep -q "### Changed" && echo "$out" | grep -q "simplify token"; then
    pass "refactor → Changed"
else
    fail_ "out=$out"
fi

# === Test 4: perf: → ### Changed ==============================================
header "Test 4: perf: prefix → ### Changed"
r=$(make_repo_with_commits "perf: cache user lookups"); cleanup_repos+=("$r")
out=$(run_gen "$r" from-tag HEAD 1.0.0 --dry-run)
if echo "$out" | grep -q "### Changed" && echo "$out" | grep -q "cache user lookups"; then
    pass "perf → Changed"
else
    fail_ "out=$out"
fi

# === Test 5: docs: → ### Documentation ========================================
header "Test 5: docs: prefix → ### Documentation"
r=$(make_repo_with_commits "docs: update README"); cleanup_repos+=("$r")
out=$(run_gen "$r" from-tag HEAD 1.0.0 --dry-run)
if echo "$out" | grep -q "### Documentation" && echo "$out" | grep -q "update README"; then
    pass "docs → Documentation"
else
    fail_ "out=$out"
fi

# === Test 6: no prefix → ### Other (the ~60% case) ============================
header "Test 6: no type prefix → ### Other"
r=$(make_repo_with_commits "Changed something without a prefix"); cleanup_repos+=("$r")
out=$(run_gen "$r" from-tag HEAD 1.0.0 --dry-run)
if echo "$out" | grep -q "### Other" && echo "$out" | grep -q "Changed something without"; then
    pass "no prefix → Other"
else
    fail_ "out=$out"
fi

# === Test 7: chore:, ci:, test: skipped =======================================
header "Test 7: chore: / ci: / test: → skipped"
r=$(make_repo_with_commits "chore: bump deps" "ci: tweak workflow" "test: add edge case" "feat: visible feature"); cleanup_repos+=("$r")
out=$(run_gen "$r" from-tag HEAD 1.0.0 --dry-run)
if echo "$out" | grep -q "visible feature" \
   && ! echo "$out" | grep -q "bump deps" \
   && ! echo "$out" | grep -q "tweak workflow" \
   && ! echo "$out" | grep -q "edge case"; then
    pass "skipped types not present, kept type present"
else
    fail_ "out=$out"
fi

# === Test 8: scope syntax feat(scope): handled ================================
header "Test 8: feat(scope): subject → ### Added with stripped scope"
r=$(make_repo_with_commits "feat(auth): add OAuth"); cleanup_repos+=("$r")
out=$(run_gen "$r" from-tag HEAD 1.0.0 --dry-run)
if echo "$out" | grep -q "### Added" && echo "$out" | grep -q "add OAuth"; then
    pass "feat(scope) → Added"
else
    fail_ "out=$out"
fi

# === Test 9: idempotency — existing entry preserved ===========================
header "Test 9: existing [<version>] preserved (idempotent skip)"
r=$(make_repo_with_commits "feat: x"); cleanup_repos+=("$r")
mkdir -p "$r"
cat > "$r/CHANGELOG.md" <<'EOF'
# Changelog

## [1.0.0] - 2025-01-01

### Added

- hand-curated entry that should NOT be replaced
EOF
out=$(run_gen "$r" from-tag HEAD 1.0.0)
# Output should mention idempotent skip; CHANGELOG should be unchanged.
if echo "$out" | grep -qi "preserving\|idempotent" \
   && grep -q "hand-curated entry" "$r/CHANGELOG.md"; then
    pass "existing entry preserved"
else
    fail_ "out=$out CHANGELOG=$(cat "$r/CHANGELOG.md")"
fi

# === Test 10: writes to non-existent CHANGELOG.md =============================
header "Test 10: missing CHANGELOG.md → script creates it"
r=$(make_repo_with_commits "feat: bootstrap"); cleanup_repos+=("$r")
[ -f "$r/CHANGELOG.md" ] && rm -f "$r/CHANGELOG.md"
out=$(run_gen "$r" from-tag HEAD 1.0.0)
if [ -f "$r/CHANGELOG.md" ] && grep -q "## \[1.0.0\]" "$r/CHANGELOG.md" \
   && grep -q "bootstrap" "$r/CHANGELOG.md"; then
    pass "fresh CHANGELOG.md created with entry"
else
    fail_ "out=$out file=$( [ -f "$r/CHANGELOG.md" ] && cat "$r/CHANGELOG.md" || echo '<missing>')"
fi

# === Test 11: prepends above existing entry ===================================
header "Test 11: prepends above existing [<old>] entry"
r=$(make_repo_with_commits "feat: new feature"); cleanup_repos+=("$r")
cat > "$r/CHANGELOG.md" <<'EOF'
# Changelog

## [0.9.0] - 2025-01-01

### Other

- old entry
EOF
out=$(run_gen "$r" from-tag HEAD 1.0.0)
# New [1.0.0] should appear BEFORE [0.9.0].
new_line=$(grep -nE '^## \[1.0.0\]' "$r/CHANGELOG.md" | head -1 | cut -d: -f1)
old_line=$(grep -nE '^## \[0.9.0\]' "$r/CHANGELOG.md" | head -1 | cut -d: -f1)
if [ -n "$new_line" ] && [ -n "$old_line" ] && [ "$new_line" -lt "$old_line" ]; then
    pass "new entry inserted above old"
else
    fail_ "new_line=$new_line old_line=$old_line file=$(cat "$r/CHANGELOG.md")"
fi

# === Test 12: empty range → placeholder entry =================================
header "Test 12: from-ref equals to-ref → placeholder"
r=$(make_repo_with_commits); cleanup_repos+=("$r")
out=$(run_gen "$r" from-tag from-tag 1.0.0 --dry-run)
if echo "$out" | grep -qi "no commits found\|placeholder"; then
    pass "empty range produces placeholder"
else
    fail_ "out=$out"
fi

# === Test 13: bad from-ref → exit 1 ===========================================
header "Test 13: nonexistent from-ref → exit 1"
r=$(make_repo_with_commits "feat: x"); cleanup_repos+=("$r")
set +e; out=$(run_gen "$r" definitely-not-a-ref HEAD 1.0.0 --dry-run); rc=$?; set -e
if [ "$rc" = "1" ] && echo "$out" | grep -qi "from-ref does not exist"; then
    pass "bad from-ref denied (rc=1)"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 14: --dry-run never writes ==========================================
header "Test 14: --dry-run never writes to CHANGELOG.md"
r=$(make_repo_with_commits "feat: would write"); cleanup_repos+=("$r")
echo "# Original" > "$r/CHANGELOG.md"
original_sha=$(shasum -a 256 "$r/CHANGELOG.md" | awk '{print $1}')
out=$(run_gen "$r" from-tag HEAD 1.0.0 --dry-run)
new_sha=$(shasum -a 256 "$r/CHANGELOG.md" | awk '{print $1}')
if [ "$original_sha" = "$new_sha" ] && echo "$out" | grep -q "DRY-RUN"; then
    pass "dry-run preserved file"
else
    fail_ "original=$original_sha new=$new_sha out=$out"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
