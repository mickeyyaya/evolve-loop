#!/usr/bin/env bash
#
# preflight-environment-test.sh — Unit tests for scripts/preflight-environment.sh.
#
# Tests:
#   1. Default invocation emits valid JSON (jq parseable, schema_version=2)
#   2. --summary mode emits human-readable text
#   3. nested-Claude detection drives auto_config (sandbox-fallback + worktree relocation)
#   4. Standalone shell selects in-project worktree when writable
#   5. EVOLVE_WORKTREE_BASE explicit override is honored when writable
#   6. Unwritable EVOLVE_WORKTREE_BASE falls through priority order
#   7. --write persists the profile to .evolve/environment.json
#   8. auto_config NEVER recommends EVOLVE_SKIP_WORKTREE (v8.25.0 invariant)
#
# Test isolation: each test runs the script as a subprocess with controlled
# env; no global state mutated. Probes happen against ephemeral temp dirs.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFLIGHT="$REPO_ROOT/scripts/preflight-environment.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

cleanup_dirs=()
trap '
    for d in ${cleanup_dirs[@]+"${cleanup_dirs[@]}"}; do rm -rf "$d"; done
' EXIT

# === Test 1: default invocation emits valid JSON ============================
header "Test 1: JSON output is parseable and reports schema_version=3"
out=$(bash "$PREFLIGHT" 2>/dev/null)
schema=$(echo "$out" | jq -r '.schema_version // ""' 2>/dev/null || echo "")
if [ "$schema" = "3" ]; then
    pass "schema_version=3 in JSON output"
else
    fail_ "schema_version=$schema (want 3); first 200 chars of out: ${out:0:200}"
fi

# === Test 2: --summary mode produces text not JSON ==========================
header "Test 2: --summary emits human-readable lines"
out=$(bash "$PREFLIGHT" --summary 2>/dev/null)
if echo "$out" | grep -q "Environment Profile" && echo "$out" | grep -q "Auto-config:"; then
    pass "summary contains expected sections"
else
    fail_ "summary mode missing expected text; got: $(echo "$out" | head -3)"
fi

# === Test 3: nested-Claude env triggers fallback + worktree relocation ======
header "Test 3: CLAUDECODE=1 → fallback=1 + worktree_base from non-project location"
out=$(env CLAUDECODE=1 bash "$PREFLIGHT" 2>/dev/null)
nested=$(echo "$out" | jq -r '.claude_code.nested')
fallback=$(echo "$out" | jq -r '.auto_config.EVOLVE_SANDBOX_FALLBACK_ON_EPERM')
wt_base=$(echo "$out" | jq -r '.auto_config.worktree_base')
if [ "$nested" = "true" ] && [ "$fallback" = "1" ] && echo "$wt_base" | grep -qE "evolve-loop/[a-f0-9]+$"; then
    pass "nested → fallback=1 + worktree_base=$wt_base"
else
    fail_ "nested=$nested fallback=$fallback wt_base=$wt_base"
fi

# === Test 4: schema invariants — no SKIP_WORKTREE; nested → inner_sandbox=false ===
# v8.25.0 dropped EVOLVE_SKIP_WORKTREE from auto_config (replaced by relocation).
# v8.25.1 ADDS inner_sandbox: must be false in nested-Claude (decouple inner
# sandbox-exec from the outer Claude Code OS sandbox).
header "Test 4: v8.25.1 — schema invariants (no SKIP_WORKTREE; nested→inner_sandbox=false)"
out=$(env CLAUDECODE=1 bash "$PREFLIGHT" 2>/dev/null)
# Use `has()` instead of `//` because jq's // treats `false` as missing.
skip_field=$(echo "$out" | jq -r '.auto_config | if has("EVOLVE_SKIP_WORKTREE") then .EVOLVE_SKIP_WORKTREE | tostring else "MISSING" end')
inner_sb=$(echo "$out" | jq -r '.auto_config | if has("inner_sandbox") then .inner_sandbox | tostring else "MISSING" end')
inner_reason=$(echo "$out" | jq -r '.auto_config.inner_sandbox_reason // ""')
if [ "$skip_field" = "MISSING" ] && [ "$inner_sb" = "false" ] && echo "$inner_reason" | grep -q "nested-Claude"; then
    pass "no SKIP_WORKTREE field; nested→inner_sandbox=false (reason: $inner_reason)"
else
    fail_ "skip_field=$skip_field (want MISSING); inner_sb=$inner_sb (want false); reason=$inner_reason"
fi

# === Test 5: explicit EVOLVE_WORKTREE_BASE is honored if writable ===========
header "Test 5: operator-set EVOLVE_WORKTREE_BASE wins when writable"
custom=$(mktemp -d -t test-pre-wt.XXXXXX)
cleanup_dirs+=("$custom")
out=$(env EVOLVE_WORKTREE_BASE="$custom" bash "$PREFLIGHT" 2>/dev/null)
wt_base=$(echo "$out" | jq -r '.auto_config.worktree_base')
wt_reason=$(echo "$out" | jq -r '.auto_config.worktree_base_reason')
if [ "$wt_base" = "$custom" ] && echo "$wt_reason" | grep -q "operator-provided"; then
    pass "operator override honored: $wt_base"
else
    fail_ "wt_base=$wt_base (want $custom); reason=$wt_reason"
fi

# === Test 6: unwritable EVOLVE_WORKTREE_BASE falls through priority ========
# Use a path that genuinely cannot be made writable (root-owned + chmod 555).
# Skip on systems where we can't make a non-writable temp dir.
header "Test 6: unwritable operator override → fallthrough"
ro_dir=$(mktemp -d -t test-pre-ro.XXXXXX)
cleanup_dirs+=("$ro_dir")
chmod 555 "$ro_dir"
out=$(env EVOLVE_WORKTREE_BASE="$ro_dir" bash "$PREFLIGHT" 2>/dev/null)
chmod 755 "$ro_dir"  # restore so trap can clean up
wt_base=$(echo "$out" | jq -r '.auto_config.worktree_base')
# Should NOT be the readonly dir; should fall through to a writable choice.
if [ "$wt_base" != "$ro_dir" ] && [ -n "$wt_base" ]; then
    pass "fall-through skipped readonly override → $wt_base"
else
    fail_ "wt_base=$wt_base — should have fallen through past readonly $ro_dir"
fi

# === Test 7: --write persists profile to .evolve/environment.json ===========
# Don't pollute the real .evolve/. Use EVOLVE_PROJECT_ROOT to point at a temp
# dir. resolve-roots.sh honors EVOLVE_PROJECT_ROOT_OVERRIDE; the env we set
# determines where --write lands.
header "Test 7: --write persists profile to project's .evolve/environment.json"
ws=$(mktemp -d -t test-pre-write.XXXXXX)
cleanup_dirs+=("$ws")
( cd "$ws" && git init -q . 2>/dev/null && EVOLVE_BYPASS_SHIP_GATE=1 git commit --allow-empty -q -m init 2>/dev/null ) || true
out=$(cd "$ws" && env -u EVOLVE_PROJECT_ROOT -u EVOLVE_PLUGIN_ROOT -u EVOLVE_RESOLVE_ROOTS_LOADED bash "$PREFLIGHT" --write 2>&1 >/dev/null)
if [ -f "$ws/.evolve/environment.json" ] \
   && jq -e '.schema_version == 3' "$ws/.evolve/environment.json" >/dev/null 2>&1; then
    pass "wrote environment.json with schema_version=3"
else
    fail_ "environment.json not persisted or invalid; out: $out; ls: $(ls -la "$ws/.evolve/" 2>&1)"
fi

# === Test 8: standalone shell prefers in-project worktree base ==============
# unset CLAUDECODE so detector returns standalone. With in-project writable,
# auto_config should select the in-project location (not TMPDIR).
header "Test 8: standalone shell + writable in-project → in-project base preferred"
ws8=$(mktemp -d -t test-pre-std.XXXXXX)
cleanup_dirs+=("$ws8")
( cd "$ws8" && git init -q . 2>/dev/null && EVOLVE_BYPASS_SHIP_GATE=1 git commit --allow-empty -q -m init 2>/dev/null ) || true
out=$(cd "$ws8" && env -u CLAUDECODE -u CLAUDE_CODE_ENTRYPOINT -u CLAUDE_CODE_EXECPATH \
    -u EVOLVE_WORKTREE_BASE -u EVOLVE_PROJECT_ROOT -u EVOLVE_PLUGIN_ROOT -u EVOLVE_RESOLVE_ROOTS_LOADED \
    bash "$PREFLIGHT" 2>/dev/null)
nested=$(echo "$out" | jq -r '.claude_code.nested')
wt_base=$(echo "$out" | jq -r '.auto_config.worktree_base')
wt_reason=$(echo "$out" | jq -r '.auto_config.worktree_base_reason')
if [ "$nested" = "false" ] && echo "$wt_base" | grep -qF "$ws8/.evolve/worktrees" && echo "$wt_reason" | grep -q "in-project"; then
    pass "standalone selected in-project base: $wt_base"
else
    fail_ "nested=$nested wt_base=$wt_base reason=$wt_reason"
fi

# === Test 9: standalone shell with working sandbox → inner_sandbox=true =====
# v8.25.1 invariant: inner_sandbox is true (defense-in-depth) when:
#   - Not nested-Claude
#   - sandbox.expected_to_work is true
# This test runs in a standalone subshell on Darwin (sandbox-exec available).
# Linux CI without bwrap would yield expected_to_work=false → inner_sandbox=false,
# so we only assert true on Darwin standalone.
header "Test 9: v8.25.1 — standalone (Darwin/sandbox-exec available) → inner_sandbox=true"
ws9=$(mktemp -d -t test-pre-std-inner.XXXXXX)
cleanup_dirs+=("$ws9")
( cd "$ws9" && git init -q . 2>/dev/null && EVOLVE_BYPASS_SHIP_GATE=1 git commit --allow-empty -q -m init 2>/dev/null ) || true
out=$(cd "$ws9" && env -u CLAUDECODE -u CLAUDE_CODE_ENTRYPOINT -u CLAUDE_CODE_EXECPATH bash "$PREFLIGHT" 2>/dev/null)
nested=$(echo "$out" | jq -r '.claude_code.nested')
sb_works=$(echo "$out" | jq -r '.sandbox.expected_to_work')
inner=$(echo "$out" | jq -r '.auto_config.inner_sandbox')
# When standalone AND sandbox works, expect inner=true (defense-in-depth).
# When standalone but sandbox unavailable (Linux without bwrap), inner=false is correct.
if [ "$nested" = "false" ]; then
    if [ "$sb_works" = "true" ] && [ "$inner" = "true" ]; then
        pass "standalone+sandbox-works → inner_sandbox=true (defense-in-depth)"
    elif [ "$sb_works" = "false" ] && [ "$inner" = "false" ]; then
        pass "standalone+sandbox-broken → inner_sandbox=false (no point wrapping with broken sandbox)"
    else
        fail_ "nested=$nested sb_works=$sb_works inner=$inner — schema mismatch"
    fi
else
    fail_ "Test environment is detected as nested when it shouldn't be"
fi

# === Summary ================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
