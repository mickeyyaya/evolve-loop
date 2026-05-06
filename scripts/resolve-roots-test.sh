#!/usr/bin/env bash
#
# resolve-roots-test.sh — Unit tests for scripts/resolve-roots.sh (v8.18.0).
#
# resolve-roots.sh is the dual-root helper that fixes the plugin-vs-project
# boundary issue introduced when evolve-loop is installed as a Claude Code
# plugin (scripts at ~/.claude/plugins/cache/...) but invoked from a user
# project (cwd at ~/ai/claude/<project>/). Pre-v8.18.0, every kernel script
# used a single REPO_ROOT computed from `dirname/..`, which made it write
# state/ledger/runs/ under the plugin cache — a path Claude Code blocks as
# sensitive.
#
# resolve-roots.sh defines two roots:
#   EVOLVE_PLUGIN_ROOT   — read-only resources (scripts/, agents/, profiles/)
#   EVOLVE_PROJECT_ROOT  — writable state (state.json, ledger, runs/, instincts/)
#
# Test scenarios:
#   1. Dev mode: PROJECT == PLUGIN (sourced from inside the dev repo)
#   2. Plugin mode: scripts in a different dir than cwd (PROJECT != PLUGIN)
#   3. EVOLVE_PROJECT_ROOT env override wins over auto-detection
#   4. cwd not in git repo: falls back to $PWD
#   5. Idempotent: sourcing twice does not re-resolve or duplicate-export
#
# Usage: bash scripts/resolve-roots-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELPER="$REPO_ROOT/scripts/resolve-roots.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# Per-test sandbox dir cleanup
cleanup_dirs=()
trap 'for d in "${cleanup_dirs[@]}"; do rm -rf "$d"; done' EXIT

[ -f "$HELPER" ] || { echo "FATAL: $HELPER missing — write helper first"; exit 1; }

# === Test 1: dev mode (script lives in the same tree as cwd) ==================
header "Test 1: dev mode — PROJECT_ROOT and PLUGIN_ROOT both resolve to repo"
out=$(cd "$REPO_ROOT" && env -u EVOLVE_PROJECT_ROOT -u EVOLVE_PLUGIN_ROOT \
    -u EVOLVE_RESOLVE_ROOTS_LOADED \
    bash -c "source '$HELPER'; printf 'PLUGIN=%s\nPROJECT=%s\n' \"\$EVOLVE_PLUGIN_ROOT\" \"\$EVOLVE_PROJECT_ROOT\"" 2>&1)
plugin=$(printf '%s\n' "$out" | awk -F= '/^PLUGIN=/{print $2}')
project=$(printf '%s\n' "$out" | awk -F= '/^PROJECT=/{print $2}')
[ "$plugin" = "$REPO_ROOT" ] && pass "PLUGIN_ROOT=$plugin" || fail_ "PLUGIN_ROOT=$plugin (expected $REPO_ROOT)"
[ "$project" = "$REPO_ROOT" ] && pass "PROJECT_ROOT=$project (matches PLUGIN in dev mode)" || fail_ "PROJECT_ROOT=$project (expected $REPO_ROOT)"

# === Test 2: plugin mode (cwd elsewhere; script symlinked into a temp plugin dir)
header "Test 2: plugin mode — cwd in a separate git repo, scripts elsewhere"
fakeproject=$(mktemp -d -t evolve-test-project.XXXXXX); cleanup_dirs+=("$fakeproject")
fakeplugin=$(mktemp -d -t evolve-test-plugin.XXXXXX); cleanup_dirs+=("$fakeplugin")
mkdir -p "$fakeplugin/scripts"
cp "$HELPER" "$fakeplugin/scripts/resolve-roots.sh"
cd "$fakeproject" && git init -q . && git commit --allow-empty -q -m init >/dev/null 2>&1 || true
out=$(cd "$fakeproject" && env -u EVOLVE_PROJECT_ROOT -u EVOLVE_PLUGIN_ROOT \
    -u EVOLVE_RESOLVE_ROOTS_LOADED \
    bash -c "source '$fakeplugin/scripts/resolve-roots.sh'; printf 'PLUGIN=%s\nPROJECT=%s\n' \"\$EVOLVE_PLUGIN_ROOT\" \"\$EVOLVE_PROJECT_ROOT\"" 2>&1)
plugin=$(printf '%s\n' "$out" | awk -F= '/^PLUGIN=/{print $2}')
project=$(printf '%s\n' "$out" | awk -F= '/^PROJECT=/{print $2}')
# Resolve symlinks on macOS (mktemp returns /var/folders/... but realpath returns /private/var/folders/...)
fp_real=$(cd "$fakeplugin" && pwd -P)
fproj_real=$(cd "$fakeproject" && pwd -P)
[ "$plugin" = "$fp_real" ] || [ "$plugin" = "$fakeplugin" ] && pass "PLUGIN_ROOT=$plugin" || fail_ "PLUGIN_ROOT=$plugin (expected $fp_real or $fakeplugin)"
[ "$project" = "$fproj_real" ] || [ "$project" = "$fakeproject" ] && pass "PROJECT_ROOT=$project" || fail_ "PROJECT_ROOT=$project (expected $fproj_real or $fakeproject)"
[ "$plugin" != "$project" ] && pass "PLUGIN_ROOT != PROJECT_ROOT (separation enforced)" || fail_ "roots collapsed; expected separation"

# === Test 3: EVOLVE_PROJECT_ROOT env override =================================
header "Test 3: EVOLVE_PROJECT_ROOT env override wins over auto-detect"
override=$(mktemp -d -t evolve-test-override.XXXXXX); cleanup_dirs+=("$override")
out=$(cd "$REPO_ROOT" && EVOLVE_PROJECT_ROOT="$override" env -u EVOLVE_PLUGIN_ROOT \
    -u EVOLVE_RESOLVE_ROOTS_LOADED \
    bash -c "source '$HELPER'; printf 'PROJECT=%s\n' \"\$EVOLVE_PROJECT_ROOT\"" 2>&1)
project=$(printf '%s\n' "$out" | awk -F= '/^PROJECT=/{print $2}')
override_real=$(cd "$override" && pwd -P)
{ [ "$project" = "$override" ] || [ "$project" = "$override_real" ]; } && pass "PROJECT_ROOT=$project (override applied)" || fail_ "PROJECT_ROOT=$project (expected $override or $override_real)"

# === Test 4: cwd not in any git repo — falls back to $PWD =====================
header "Test 4: cwd not in git repo — PROJECT_ROOT falls back to \$PWD"
nogit=$(mktemp -d -t evolve-test-nogit.XXXXXX); cleanup_dirs+=("$nogit")
out=$(cd "$nogit" && env -u EVOLVE_PROJECT_ROOT -u EVOLVE_PLUGIN_ROOT \
    -u EVOLVE_RESOLVE_ROOTS_LOADED \
    bash -c "source '$HELPER'; printf 'PROJECT=%s\n' \"\$EVOLVE_PROJECT_ROOT\"" 2>&1)
project=$(printf '%s\n' "$out" | awk -F= '/^PROJECT=/{print $2}')
nogit_real=$(cd "$nogit" && pwd -P)
{ [ "$project" = "$nogit" ] || [ "$project" = "$nogit_real" ]; } && pass "PROJECT_ROOT=$project (PWD fallback)" || fail_ "PROJECT_ROOT=$project (expected $nogit or $nogit_real)"

# === Test 5: idempotent sourcing ==============================================
header "Test 5: sourcing twice is idempotent (no re-resolution, no errors)"
out=$(cd "$REPO_ROOT" && env -u EVOLVE_PROJECT_ROOT -u EVOLVE_PLUGIN_ROOT \
    -u EVOLVE_RESOLVE_ROOTS_LOADED \
    bash -c "source '$HELPER'; first=\$EVOLVE_PROJECT_ROOT; source '$HELPER'; second=\$EVOLVE_PROJECT_ROOT; [ \"\$first\" = \"\$second\" ] && echo OK || echo MISMATCH:\$first/\$second" 2>&1)
[ "$out" = "OK" ] && pass "second source preserved value" || fail_ "idempotency failed: $out"

# === Test 6: writable check — PROJECT_ROOT must be writable, PLUGIN need not =
header "Test 6: helper exposes EVOLVE_PROJECT_WRITABLE indicator"
out=$(cd "$REPO_ROOT" && env -u EVOLVE_PROJECT_ROOT -u EVOLVE_PLUGIN_ROOT \
    -u EVOLVE_RESOLVE_ROOTS_LOADED \
    bash -c "source '$HELPER'; echo \"\${EVOLVE_PROJECT_WRITABLE:-unset}\"" 2>&1)
[ "$out" = "1" ] && pass "EVOLVE_PROJECT_WRITABLE=1 (repo is writable)" || fail_ "EVOLVE_PROJECT_WRITABLE=$out (expected 1)"

# === Test 7: empty-string EVOLVE_PROJECT_ROOT override falls through to git ==
header "Test 7: empty-string override is treated as unset (falls through)"
out=$(cd "$REPO_ROOT" && env -u EVOLVE_PLUGIN_ROOT -u EVOLVE_RESOLVE_ROOTS_LOADED \
    EVOLVE_PROJECT_ROOT="" \
    bash -c "source '$HELPER'; printf 'PROJECT=%s\n' \"\$EVOLVE_PROJECT_ROOT\"" 2>&1)
project=$(printf '%s\n' "$out" | awk -F= '/^PROJECT=/{print $2}')
# Empty string should not pin PROJECT_ROOT to ""; auto-detect should resolve to repo root.
[ "$project" = "$REPO_ROOT" ] && pass "empty override → auto-detect engaged ($project)" || fail_ "empty override mishandled: PROJECT_ROOT=$project"

# === Test 8: nested git worktree — PROJECT_ROOT resolves to the worktree ====
header "Test 8: git worktree path is honored (not the main tree)"
mainrepo=$(mktemp -d -t evolve-test-main.XXXXXX); cleanup_dirs+=("$mainrepo")
worktreedir=$(mktemp -d -t evolve-test-wtparent.XXXXXX); cleanup_dirs+=("$worktreedir")
( cd "$mainrepo" && git init -q . && git commit --allow-empty -q -m init && git checkout -q -b feat 2>/dev/null && git worktree add -q "$worktreedir/wt" main 2>/dev/null ) >/dev/null 2>&1 || true
if [ -d "$worktreedir/wt/.git" ] || [ -f "$worktreedir/wt/.git" ]; then
    out=$(cd "$worktreedir/wt" && env -u EVOLVE_PROJECT_ROOT -u EVOLVE_PLUGIN_ROOT \
        -u EVOLVE_RESOLVE_ROOTS_LOADED \
        bash -c "source '$HELPER'; printf 'PROJECT=%s\n' \"\$EVOLVE_PROJECT_ROOT\"" 2>&1)
    project=$(printf '%s\n' "$out" | awk -F= '/^PROJECT=/{print $2}')
    wt_real=$(cd "$worktreedir/wt" && pwd -P)
    # git rev-parse --show-toplevel returns the worktree path (NOT the main tree).
    # If a future change accidentally prefers --show-superproject-working-tree
    # or similar, this assertion will catch the regression.
    { [ "$project" = "$worktreedir/wt" ] || [ "$project" = "$wt_real" ]; } && pass "worktree resolved correctly ($project)" || fail_ "worktree mishandled: PROJECT=$project"
else
    pass "worktree creation unsupported in this env — skipped (not a regression)"
fi

# === Summary ==================================================================
echo
echo "==========================================="
echo "  Total: $TESTS_TOTAL test groups"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
