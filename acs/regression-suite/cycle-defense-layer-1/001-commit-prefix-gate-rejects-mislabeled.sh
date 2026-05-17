#!/usr/bin/env bash
#
# ACS predicate for Reward-Hacking Defense System Layer 1.
# Verifies that scripts/guards/commit-prefix-gate.sh correctly:
#   1. Rejects a mislabeled commit (docs: prefix + non-docs diff) with rc=2
#   2. Accepts a properly-labeled commit (docs: prefix + docs/ diff) with rc=0
#   3. Passes through unknown prefixes with rc=0
#
# Self-contained: creates isolated temp git repos, runs the real gate against them.
# No mocks — exercises the actual production code path.
#
# Exit 0 only if ALL 3 assertions hold.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
GATE="$REPO_ROOT/scripts/guards/commit-prefix-gate.sh"
MANIFEST="$REPO_ROOT/.evolve/commit-prefix-scope.json"

if [ ! -x "$GATE" ]; then
    echo "FAIL: gate not executable: $GATE" >&2
    exit 1
fi

if [ ! -f "$MANIFEST" ]; then
    echo "FAIL: manifest not present: $MANIFEST" >&2
    exit 1
fi

PASS_COUNT=0
FAIL_COUNT=0

assert_exit() {
    local label="$1"
    local expected="$2"
    local actual="$3"
    if [ "$expected" -eq "$actual" ]; then
        echo "  ✓ [$label] expected=rc=$expected got=rc=$actual"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "  ✗ [$label] expected=rc=$expected got=rc=$actual"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
}

# Helper: create a fresh isolated test repo for each scenario
make_test_repo() {
    local dir
    dir=$(mktemp -d "${TMPDIR:-/tmp}/prefix-gate-acs.XXXXXX")
    git -C "$dir" init -q .
    git -C "$dir" config user.email "acs@test.local"
    git -C "$dir" config user.name "ACS Test"
    echo "$dir"
}

run_gate_with_staged() {
    local repo="$1" msg="$2"
    bash "$GATE" --msg "$msg" --manifest "$MANIFEST" --repo-dir "$repo" >/dev/null 2>&1
}

echo "=== ACS-1: Reject mislabeled commit (docs: prefix, scripts/ diff) ==="
T1=$(make_test_repo)
trap 'rm -rf "$T1" "$T2" "$T3" "$T4" "$T5"' EXIT
mkdir -p "$T1/scripts/guards"
echo '#!/bin/bash' > "$T1/scripts/guards/example.sh"
git -C "$T1" add scripts/guards/example.sh
set +e; run_gate_with_staged "$T1" "docs: this is mislabeled"; rc1=$?; set -e
assert_exit "ACS-1 mislabeled commit" 2 "$rc1"

echo ""
echo "=== ACS-2: Accept properly-labeled commit (docs: prefix, docs/ diff) ==="
T2=$(make_test_repo)
mkdir -p "$T2/docs"
echo "# README" > "$T2/docs/example.md"
git -C "$T2" add docs/example.md
set +e; run_gate_with_staged "$T2" "docs: legitimate doc change"; rc2=$?; set -e
assert_exit "ACS-2 proper label" 0 "$rc2"

echo ""
echo "=== ACS-3: Pass-through for unknown prefix ==="
T3=$(make_test_repo)
mkdir -p "$T3/random"
echo "test" > "$T3/random/x.txt"
git -C "$T3" add random/x.txt
set +e; run_gate_with_staged "$T3" "wibble: unknown prefix"; rc3=$?; set -e
assert_exit "ACS-3 unknown prefix" 0 "$rc3"

echo ""
echo "=== ACS-4: feat(token-opt) with ONLY docs/ diff is rejected (forbidden_only_paths) ==="
T4=$(make_test_repo)
mkdir -p "$T4/docs"
echo "# token-opt notes" > "$T4/docs/token-opt.md"
git -C "$T4" add docs/token-opt.md
set +e; run_gate_with_staged "$T4" "feat(token-opt): docs only - should be docs prefix"; rc4=$?; set -e
assert_exit "ACS-4 token-opt docs-only rejection" 2 "$rc4"

echo ""
echo "=== ACS-5: chore: with any path is accepted (permissive) ==="
T5=$(make_test_repo)
mkdir -p "$T5/anywhere/at/all"
echo "chore" > "$T5/anywhere/at/all/file.txt"
git -C "$T5" add anywhere/at/all/file.txt
set +e; run_gate_with_staged "$T5" "chore: cleanup"; rc5=$?; set -e
assert_exit "ACS-5 chore permissive" 0 "$rc5"

echo ""
echo "============================================"
echo "PASS: $PASS_COUNT  FAIL: $FAIL_COUNT"
if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
fi
exit 0
