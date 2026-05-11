#!/usr/bin/env bash
# v9.1.x knowledge-base exclusion test.
#
# Verifies the two-folder content model: knowledge-base/ is git-tracked but
# excluded from agent runtime context across all CLIs.
#
# Three layers of enforcement, each asserted independently:
#   L1 — OS sandbox (profile deny_subpaths)
#   L2 — Adapter permission-mode (same deny_subpaths, applied at CLI gate)
#   L3 — Layer-B context-builder filter (role-context-builder.sh emit_artifact)
#
# Plus a best-effort distribution check (.gitattributes export-ignore).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
PROFILES_DIR="$PROJECT_ROOT/.evolve/profiles"
ROLE_CTX_BUILDER="$PROJECT_ROOT/scripts/lifecycle/role-context-builder.sh"
GITATTRIBUTES="$PROJECT_ROOT/.gitattributes"

PASS=0
FAIL=0

expect() {
    local label="$1" actual="$2" expected="$3"
    if [ "$actual" = "$expected" ]; then
        printf "  PASS: %s\n" "$label"; PASS=$((PASS + 1))
    else
        printf "  FAIL: %s (expected=%s actual=%s)\n" "$label" "$expected" "$actual" >&2
        FAIL=$((FAIL + 1))
    fi
}

expect_match() {
    local label="$1" actual="$2" pattern="$3"
    # bash [[ =~ ]] for SIGPIPE-safe matching on large multi-line input
    if [[ "$actual" =~ $pattern ]]; then
        printf "  PASS: %s\n" "$label"; PASS=$((PASS + 1))
    else
        printf "  FAIL: %s (pattern=%s)\n" "$label" "$pattern" >&2
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Test 1 (L1+L2): profile deny_subpaths includes knowledge-base ==="
for p in scout auditor orchestrator; do
    if jq -e '.sandbox.deny_subpaths | any(. == "knowledge-base")' \
            "$PROFILES_DIR/${p}.json" >/dev/null 2>&1; then
        expect "$p deny_subpaths blocks knowledge-base" "yes" "yes"
    else
        expect "$p deny_subpaths blocks knowledge-base" "no" "yes"
    fi
done

echo
echo "=== Test 2 (L1+L2 negative): builder profile does NOT need knowledge-base in deny_subpaths ==="
# Builder runs in worktree-only mode. Adding knowledge-base to its deny_subpaths
# would be redundant (knowledge-base is outside the worktree) but not harmful.
# This test documents the deliberate omission.
if jq -e '.add_dir | any(. == "{worktree_path}")' "$PROFILES_DIR/builder.json" >/dev/null 2>&1; then
    expect "builder restricted to worktree" "yes" "yes"
else
    # If builder gains "." add_dir in the future, this test catches it
    expect "builder restricted to worktree (regression alert if not)" "no" "yes"
fi

echo
echo "=== Test 3 (L3): role-context-builder.sh emit_artifact filters knowledge-base ==="
src=$(cat "$ROLE_CTX_BUILDER")
expect_match "knowledge-base filter present" "$src" "knowledge-base/.*return 0"
expect_match "filter has cross-CLI comment" "$src" "cross-CLI knowledge-base exclusion"
# The filter must catch the three common path shapes
expect_match "filter matches absolute prefix" "$src" "knowledge-base/\*"
expect_match "filter matches relative prefix" "$src" "\\./knowledge-base/\*"
expect_match "filter matches nested prefix" "$src" "\\*/knowledge-base/\*"

echo
echo "=== Test 4 (distribution): .gitattributes declares export-ignore ==="
if [ -f "$GITATTRIBUTES" ]; then
    if grep -q "^knowledge-base/[[:space:]]\+export-ignore" "$GITATTRIBUTES" 2>/dev/null; then
        expect ".gitattributes has knowledge-base/ export-ignore" "yes" "yes"
    else
        expect ".gitattributes has knowledge-base/ export-ignore" "no" "yes"
    fi
else
    expect ".gitattributes exists" "no" "yes"
fi

echo
echo "=== Test 5 (provenance): knowledge-base/research/ exists with 42 restored files ==="
KB_DIR="$PROJECT_ROOT/knowledge-base/research"
if [ -d "$KB_DIR" ]; then
    count=$(ls "$KB_DIR"/*.md 2>/dev/null | wc -l | tr -d ' ')
    expect "42 files restored to knowledge-base/research/" "$count" "42"
else
    expect "knowledge-base/research/ exists" "no" "yes"
fi

echo
echo "=== Test 6 (byte-identical): 3 spot-checks against git history ==="
for f in agent-economics.md workflow-dag-patterns.md hitl-trust-calibration.md; do
    if diff -q <(git -C "$PROJECT_ROOT" show "35b31c4^:docs/research/$f" 2>/dev/null) \
              "$KB_DIR/$f" >/dev/null 2>&1; then
        expect "$f byte-identical to 35b31c4^" "yes" "yes"
    else
        expect "$f byte-identical to 35b31c4^" "no" "yes"
    fi
done

echo
echo "=== Test 7 (no add_dir leak): no agent profile includes knowledge-base in add_dir ==="
for p in scout auditor orchestrator builder; do
    if jq -e '.add_dir | any(. == "knowledge-base" or . == "knowledge-base/" or . == "./knowledge-base")' \
            "$PROFILES_DIR/${p}.json" >/dev/null 2>&1; then
        expect "$p add_dir does NOT leak knowledge-base" "leaked" "clean"
    else
        expect "$p add_dir does NOT leak knowledge-base" "clean" "clean"
    fi
done

echo
echo "=== Test 8 (convention doc): knowledge-base/README.md + docs/architecture/knowledge-base.md exist ==="
[ -f "$PROJECT_ROOT/knowledge-base/README.md" ] \
    && expect "knowledge-base/README.md exists" "yes" "yes" \
    || expect "knowledge-base/README.md exists" "no" "yes"
[ -f "$PROJECT_ROOT/docs/architecture/knowledge-base.md" ] \
    && expect "docs/architecture/knowledge-base.md exists" "yes" "yes" \
    || expect "docs/architecture/knowledge-base.md exists" "no" "yes"

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
