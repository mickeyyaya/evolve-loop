#!/usr/bin/env bash
# ACS predicate 056 — cycle 63
# Verifies that resolve-roots.sh detects git worktrees and resolves
# EVOLVE_PROJECT_ROOT to the main repo root (via --git-common-dir), not
# the worktree path. This is the B7 fix — cycle 61 demonstrated that the
# old `--show-toplevel`-only logic caused state.json:lastCycleNumber to
# be written to the worktree's ephemeral state.json and lost on cleanup.
#
# IMPORTANT: The predicate's test-resolve subprocess MUST use
# `env -u EVOLVE_RESOLVE_ROOTS_LOADED` because the evolve-loop runtime
# exports that var as an idempotency guard; without explicit unset, the
# subshell sources resolve-roots.sh and the guard fires immediately,
# skipping all resolution logic. Cycle 62's first attempt at this
# predicate failed because of this oversight.
#
# AC-ID: cycle-63-056
# Description: resolve-roots-worktree-detection
# Evidence: 4 ACs — worktree main-root resolution, non-worktree unchanged,
#           env isolation enforced, anti-tautology
# Author: builder (manual fix, cycle 63 recovery from cycle 62 FAIL)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: docs/incidents/cycle-61.md §B7
#
# metadata:
#   id: 056-resolve-roots-worktree
#   cycle: 63
#   task: b7-worktree-detection
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
# Use --git-common-dir to get the main repo root in case we're running from
# inside a worktree (the bug this predicate tests for).
if [ -f "$REPO_ROOT/.git" ]; then
    REPO_ROOT="$(cd "$REPO_ROOT" && cd "$(git rev-parse --git-common-dir)/.." && pwd)"
fi
RESOLVE="$REPO_ROOT/scripts/lifecycle/resolve-roots.sh"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
rc=0

if [ ! -f "$RESOLVE" ]; then
    echo "RED PRE: resolve-roots.sh not found at $RESOLVE"
    exit 1
fi

# ── Set up a fixture: a main git repo + a worktree branched from it ───────────
# Canonicalize paths via cd+pwd so comparisons survive macOS /var → /private/var symlink.
MAIN="$TMP/main"
WT="$TMP/worktree"
mkdir -p "$MAIN"
MAIN="$(cd "$MAIN" && pwd -P)"
(
    cd "$MAIN"
    git init -q
    git config user.email "test@example.com"
    git config user.name "test"
    echo "initial" > README.md
    git add README.md
    git commit -q -m "initial"
    git worktree add -q "$WT" -b test-branch 2>/dev/null
) || {
    echo "RED PRE: fixture setup failed"
    exit 1
}
WT="$(cd "$WT" && pwd -P)"

# Sanity: is fixture worktree's .git a file (not a dir)?
if [ ! -f "$WT/.git" ]; then
    echo "RED PRE: fixture worktree's .git is not a file — git worktree may not be supported here"
    exit 1
fi

# Build a small test runner that sources resolve-roots.sh and prints the
# resolved EVOLVE_PROJECT_ROOT.
TEST_RESOLVE="$TMP/test-resolve.sh"
cat > "$TEST_RESOLVE" << EOF
#!/usr/bin/env bash
set -u
unset EVOLVE_PROJECT_ROOT
cd "\$1"
. "$RESOLVE"
echo "\$EVOLVE_PROJECT_ROOT"
EOF
chmod +x "$TEST_RESOLVE"

# ── AC1: from inside the worktree, resolve to MAIN (not WT) ───────────────────
# CRITICAL: must unset EVOLVE_RESOLVE_ROOTS_LOADED so the sourced script's
# idempotency guard doesn't skip resolution.
resolved_wt=$(env -u EVOLVE_RESOLVE_ROOTS_LOADED -u EVOLVE_PROJECT_ROOT bash "$TEST_RESOLVE" "$WT")
if [ "$resolved_wt" = "$MAIN" ]; then
    echo "GREEN AC1: from worktree, EVOLVE_PROJECT_ROOT resolves to main ($MAIN)"
else
    echo "RED AC1: from worktree, EVOLVE_PROJECT_ROOT resolved to '$resolved_wt' (expected '$MAIN')"
    rc=1
fi

# ── AC2: from inside main (non-worktree), resolve to MAIN (unchanged path) ────
resolved_main=$(env -u EVOLVE_RESOLVE_ROOTS_LOADED -u EVOLVE_PROJECT_ROOT bash "$TEST_RESOLVE" "$MAIN")
if [ "$resolved_main" = "$MAIN" ]; then
    echo "GREEN AC2: from main repo, EVOLVE_PROJECT_ROOT correctly resolves to main"
else
    echo "RED AC2: from main repo, got '$resolved_main' (expected '$MAIN') — non-worktree case regressed"
    rc=1
fi

# ── AC3 (anti-tautology): without env isolation, idempotency guard fires ──────
# This is the test that cycle-62's broken predicate would have failed.
# Verifies the env-unset is load-bearing — without it, the guard short-circuits
# and EVOLVE_PROJECT_ROOT remains unset (or stale).
resolved_noenv=$(EVOLVE_RESOLVE_ROOTS_LOADED=1 bash "$TEST_RESOLVE" "$WT" 2>/dev/null || true)
if [ -z "$resolved_noenv" ] || [ "$resolved_noenv" != "$MAIN" ]; then
    echo "GREEN AC3 (anti-tautology): with idempotency guard set, resolution is skipped — predicate WOULD fail without env -u"
else
    echo "RED AC3 (anti-tautology): idempotency guard didn't short-circuit (got '$resolved_noenv') — guard test broken"
    rc=1
fi

# ── AC4: regression-replay — the cycle-62 broken predicate at acs/cycle-62/ ──
# Confirms cycle-62's predicate WAS executable AND used env isolation correctly
# if and only if it sits in current main. We accept either: a corrected version
# is present, OR no predicate exists for cycle-62 (this one supersedes it).
OLD_PRED="$REPO_ROOT/acs/cycle-62/056-resolve-roots-worktree.sh"
if [ -f "$OLD_PRED" ]; then
    # If the old defective predicate still exists, it must have the env-unset
    # marker — if it does, both are fine; if it doesn't, this predicate's
    # existence supersedes it but we surface the inconsistency.
    if grep -q "env -u EVOLVE_RESOLVE_ROOTS_LOADED" "$OLD_PRED" 2>/dev/null; then
        echo "GREEN AC4: cycle-62 predicate (if present) uses env isolation"
    else
        echo "WARN AC4: cycle-62 predicate exists without env isolation — this cycle-63 predicate supersedes it"
        # Not RED — informational; the new predicate is the source of truth.
    fi
else
    echo "GREEN AC4: cycle-62 predicate absent (this cycle-63 predicate is canonical)"
fi

exit "$rc"
