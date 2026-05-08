#!/usr/bin/env bash
#
# run-cycle-worktree-test.sh — Regression guard for v8.36.0 stale-admin-entry recovery.
#
# Reproduces the bug downstream user hit: when a prior cycle's worktree directory
# is gone but `.git/worktrees/<name>/` admin entry persists (typical in nested-Claude
# where $TMPDIR changes per session), `git branch -D` silently no-ops on the
# still-"checked-out" branch and `git worktree add` fails with "branch already
# exists". The fix is `git worktree prune` BEFORE the branch deletion.
#
# This test does NOT exercise run-cycle.sh end-to-end (too complex to mock).
# It validates the GIT-COMMAND-SEQUENCE fix at the underlying-behavior level —
# proving that the prune-before-delete sequence is the correct recovery for
# stale admin entries. If anyone refactors run-cycle.sh and accidentally drops
# the prune, this test catches the regression.
#
# Usage: bash scripts/run-cycle-worktree-test.sh

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRATCH=$(mktemp -d -t "run-cycle-worktree-XXX")
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Set up a tmp git repo with one initial commit + a stale .git/worktrees/cycle-1/
# admin entry pointing at a non-existent path.
make_repo_with_stale_admin() {
    local repo="$SCRATCH/repo-$RANDOM"
    mkdir -p "$repo"
    cd "$repo"
    git init -q
    git config user.email t@t.t
    git config user.name t
    git config core.hooksPath /dev/null
    echo "init" > seed.txt
    git add seed.txt
    EVOLVE_BYPASS_SHIP_GATE=1 git -c commit.gpgsign=false commit -q -m initial
    # Create the branch ref directly (bypasses worktree-add mechanism so we
    # can synthesize the exact stale-state scenario).
    git branch evolve/cycle-1
    # Synthesize a stale admin entry pointing at a non-existent directory.
    local admin_dir="$repo/.git/worktrees/cycle-1"
    mkdir -p "$admin_dir"
    echo "ref: refs/heads/evolve/cycle-1" > "$admin_dir/HEAD"
    # gitdir points at the (defunct) worktree's .git file location
    echo "/tmp/nonexistent-old-hash-$$/cycle-1/.git" > "$admin_dir/gitdir"
    # commondir points back to the main repo
    echo "../.." > "$admin_dir/commondir"
    echo "$repo"
}

# === Test 1: bug reproduction — branch-D blocked by stale admin =============
# Without the prune fix: git branch -D silently no-ops AND git worktree add
# fails. This test confirms the failure mode exists when prune is omitted.
header "Test 1: pre-fix reproduction — stale admin blocks branch-D + worktree-add"
REPO=$(make_repo_with_stale_admin)
cd "$REPO"

# Verify the stale state: branch exists AND admin entry exists.
if ! git branch --list evolve/cycle-1 | grep -q evolve/cycle-1; then
    fail_ "setup: evolve/cycle-1 branch should exist"
fi
if [ ! -f ".git/worktrees/cycle-1/HEAD" ]; then
    fail_ "setup: .git/worktrees/cycle-1/HEAD should exist (stale admin entry)"
fi

# Try `git branch -D` WITHOUT pruning first. Git should refuse because the
# branch is admin-checked-out via the stale entry.
set +e
out=$(git branch -D evolve/cycle-1 2>&1)
rc=$?
set -e
# Branch should still exist (delete refused) — that's the bug.
if [ "$rc" != "0" ] && git branch --list evolve/cycle-1 | grep -q evolve/cycle-1; then
    pass "stale admin blocks branch-D (rc=$rc, branch still present) — bug reproduced"
else
    fail_ "expected branch-D to fail with stale admin; rc=$rc, out: $out"
fi

# Try `git worktree add` — should fail with "branch already exists".
NEW_PATH="$SCRATCH/new-cycle-1-$RANDOM"
set +e
add_out=$(git worktree add -b evolve/cycle-1 "$NEW_PATH" HEAD 2>&1)
add_rc=$?
set -e
if [ "$add_rc" != "0" ] && echo "$add_out" | grep -qi "already exists"; then
    pass "worktree-add fails with 'branch already exists' — full bug confirmed"
else
    fail_ "expected worktree-add to fail; rc=$add_rc, out: $add_out"
fi
cd "$REPO_ROOT" >/dev/null

# === Test 2: post-fix — prune unblocks the recovery sequence ================
# With the v8.36.0 prune-before-delete fix: git worktree prune removes the
# stale admin entry, then git branch -D succeeds, then git worktree add
# succeeds.
header "Test 2: post-fix — prune→branch-D→worktree-add succeeds"
REPO=$(make_repo_with_stale_admin)
cd "$REPO"

# v8.36.0 fix: prune first.
git worktree prune
# Verify the stale admin entry is gone.
if [ ! -d ".git/worktrees/cycle-1" ]; then
    pass "git worktree prune removed stale .git/worktrees/cycle-1/ admin entry"
else
    fail_ "stale admin entry still present after prune"
fi

# Branch -D should now succeed (no stale checkout claim).
set +e
git branch -D evolve/cycle-1 >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "0" ] && ! git branch --list evolve/cycle-1 | grep -q evolve/cycle-1; then
    pass "branch -D succeeded after prune; branch ref removed"
else
    fail_ "branch -D rc=$rc; branch list: $(git branch --list evolve/cycle-1)"
fi

# Worktree add should now succeed.
NEW_PATH="$SCRATCH/new-cycle-1-$RANDOM"
set +e
git worktree add -b evolve/cycle-1 "$NEW_PATH" HEAD >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "0" ] && [ -d "$NEW_PATH" ] && [ -f "$NEW_PATH/seed.txt" ]; then
    pass "worktree add succeeded — branch evolve/cycle-1 provisioned at $NEW_PATH"
else
    fail_ "worktree add rc=$rc; new path exists: $([ -d "$NEW_PATH" ] && echo yes || echo no)"
fi
cd "$REPO_ROOT" >/dev/null

# === Test 3: prune is safe when no stale entries exist ======================
# Idempotency check: prune should succeed with zero side effects on a clean repo.
header "Test 3: prune is safe on clean repo (idempotent, no errors)"
REPO="$SCRATCH/clean-repo-$RANDOM"
mkdir -p "$REPO" && cd "$REPO"
git init -q
git config user.email t@t.t
git config user.name t
git config core.hooksPath /dev/null
echo "init" > seed.txt
git add seed.txt
EVOLVE_BYPASS_SHIP_GATE=1 git -c commit.gpgsign=false commit -q -m initial
set +e
out=$(git worktree prune 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ]; then
    pass "prune on clean repo: rc=0 (idempotent, no errors)"
else
    fail_ "prune rc=$rc; out: $out"
fi
cd "$REPO_ROOT" >/dev/null

# === Test 4: prune does NOT touch active worktrees ==========================
# Critical safety check: prune should NEVER remove admin entries for worktrees
# whose directories still exist. This protects concurrent cycles.
header "Test 4: prune preserves active worktree admin entries"
REPO="$SCRATCH/active-wt-repo-$RANDOM"
mkdir -p "$REPO" && cd "$REPO"
git init -q
git config user.email t@t.t
git config user.name t
git config core.hooksPath /dev/null
echo "init" > seed.txt
git add seed.txt
EVOLVE_BYPASS_SHIP_GATE=1 git -c commit.gpgsign=false commit -q -m initial
ACTIVE_WT="$SCRATCH/active-wt-$RANDOM"
git worktree add -b active-feature "$ACTIVE_WT" HEAD >/dev/null 2>&1
# Confirm admin entry exists for this active worktree.
if [ ! -d ".git/worktrees/$(basename "$ACTIVE_WT")" ]; then
    fail_ "setup: admin entry should exist for active worktree"
fi
git worktree prune
# Admin entry should still be there (directory still exists).
if [ -d ".git/worktrees/$(basename "$ACTIVE_WT")" ] && [ -d "$ACTIVE_WT" ]; then
    pass "prune preserved active worktree admin entry (concurrent cycle safety)"
else
    fail_ "prune incorrectly removed active worktree admin"
fi
cd "$REPO_ROOT" >/dev/null

# === Test 5: run-cycle.sh contains the prune fix ============================
# Static assertion that the v8.36.0 fix is present in the script. This catches
# the case where someone refactors run-cycle.sh and accidentally drops the prune.
header "Test 5: run-cycle.sh contains v8.36.0 prune in pre-flight cleanup"
if grep -q "v8.36.0: prune stale worktree admin entries" "$REPO_ROOT/scripts/dispatch/run-cycle.sh"; then
    pass "v8.36.0 prune fix is present in run-cycle.sh"
else
    fail_ "v8.36.0 prune fix is MISSING from run-cycle.sh — regression!"
fi

# === Summary =================================================================
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
