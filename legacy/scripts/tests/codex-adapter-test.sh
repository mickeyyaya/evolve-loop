#!/usr/bin/env bash
#
# codex-adapter-test.sh — Contract tests for the v8.51.0 Codex hybrid adapter.
#
# Pre-v8.51.0: codex.sh was a stub that always exited 99.
# v8.51.0+: codex.sh is a hybrid adapter mirroring gemini.sh — HYBRID delegation
# when claude binary is on PATH, DEGRADED same-session mode otherwise.
# These tests exercise both paths.
#
# Bash 3.2 compatible.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADAPTER="$REPO_ROOT/scripts/cli_adapters/codex.sh"
SCOUT_PROFILE="$REPO_ROOT/.evolve/profiles/scout.json"

PASS=0; FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# === Test 1: adapter exists + executable =====================================
header "Test 1: codex.sh exists and is executable"
if [ -f "$ADAPTER" ] && [ -x "$ADAPTER" ]; then
    pass "adapter present + executable"
else
    fail_ "missing or non-executable: $ADAPTER"
fi

# === Test 2: --probe with claude on PATH → hybrid tier =======================
header "Test 2: --probe with claude on PATH → tier=hybrid (rc=0)"
if command -v claude >/dev/null 2>&1; then
    set +e
    out=$(bash "$ADAPTER" --probe 2>&1)
    rc=$?
    set -e
    if [ "$rc" = "0" ] && echo "$out" | grep -q "tier=hybrid"; then
        pass "probe rc=0 with hybrid tier"
    else
        fail_ "rc=$rc; out: $out"
    fi
else
    pass "skipped: no claude binary in test env"
fi

# === Test 3: --probe with claude FORCED missing → degraded tier (rc=0) =======
header "Test 3: --probe with claude missing → degraded tier (no exit 99)"
set +e
out=$(EVOLVE_TESTING=1 EVOLVE_CODEX_CLAUDE_PATH="" bash "$ADAPTER" --probe 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] && echo "$out" | grep -qE "tier=(degraded|none)|DEGRADED"; then
    pass "probe rc=0 + degraded tier reported"
else
    fail_ "rc=$rc out: $out"
fi

# === Test 4: --probe with claude missing + --require-full → no exit 99 (probe is informational)
header "Test 4: --probe is informational; --require-full only enforces in run mode"
set +e
out=$(EVOLVE_TESTING=1 EVOLVE_CODEX_CLAUDE_PATH="" bash "$ADAPTER" --probe --require-full 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ]; then
    pass "probe rc=0 even with --require-full (require-full enforces only in run mode)"
else
    fail_ "rc=$rc"
fi

# === Test 5: run mode + --require-full + claude missing → exit 99 ============
header "Test 5: run mode + --require-full + claude missing → exit 99"
set +e
PROFILE_PATH=/dev/null RESOLVED_MODEL=sonnet PROMPT_FILE=/dev/null \
    CYCLE=0 WORKSPACE_PATH=/tmp STDOUT_LOG=/tmp/x STDERR_LOG=/tmp/y \
    ARTIFACT_PATH=/tmp/z \
    EVOLVE_TESTING=1 EVOLVE_CODEX_CLAUDE_PATH="" \
    EVOLVE_CODEX_REQUIRE_FULL=1 \
    bash "$ADAPTER" >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "99" ]; then
    pass "rc=99 with --require-full + claude missing"
else
    fail_ "expected rc=99, got rc=$rc"
fi

# === Test 6: run mode + claude missing (default) → DEGRADED, exit 0 ==========
header "Test 6: run mode + claude missing (default) → DEGRADED + exit 0"
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
echo "test prompt" > "$tmpdir/prompt"
set +e
PROFILE_PATH="$SCOUT_PROFILE" RESOLVED_MODEL=sonnet PROMPT_FILE="$tmpdir/prompt" \
    CYCLE=0 WORKSPACE_PATH="$tmpdir" \
    STDOUT_LOG="$tmpdir/stdout" STDERR_LOG="$tmpdir/stderr" \
    ARTIFACT_PATH="$tmpdir/artifact.md" \
    EVOLVE_TESTING=1 EVOLVE_CODEX_CLAUDE_PATH="" \
    bash "$ADAPTER" 2>"$tmpdir/stderr-cap"
rc=$?
set -e
if [ "$rc" = "0" ] && grep -q "DEGRADED MODE active" "$tmpdir/stderr-cap"; then
    pass "rc=0 + DEGRADED warning emitted"
else
    fail_ "rc=$rc; stderr: $(cat "$tmpdir/stderr-cap")"
fi
rm -rf "$tmpdir"
trap - EXIT

# === Test 7: degraded mode writes structured stdout for cost accounting ======
header "Test 7: degraded mode produces structured stdout JSON"
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
echo "p" > "$tmpdir/prompt"
set +e
PROFILE_PATH="$SCOUT_PROFILE" RESOLVED_MODEL=sonnet PROMPT_FILE="$tmpdir/prompt" \
    CYCLE=0 WORKSPACE_PATH="$tmpdir" \
    STDOUT_LOG="$tmpdir/stdout" STDERR_LOG="$tmpdir/stderr" \
    ARTIFACT_PATH="$tmpdir/artifact.md" \
    EVOLVE_TESTING=1 EVOLVE_CODEX_CLAUDE_PATH="" \
    bash "$ADAPTER" >/dev/null 2>&1
set -e
if [ -s "$tmpdir/stdout" ] && jq empty "$tmpdir/stdout" 2>/dev/null; then
    is_degraded=$(jq -r '.degraded_mode' "$tmpdir/stdout")
    adapter=$(jq -r '.adapter' "$tmpdir/stdout")
    if [ "$is_degraded" = "true" ] && [ "$adapter" = "codex" ]; then
        pass "stdout JSON has degraded_mode=true + adapter=codex"
    else
        fail_ "JSON malformed: degraded=$is_degraded adapter=$adapter"
    fi
else
    fail_ "stdout missing or invalid JSON"
fi
rm -rf "$tmpdir"
trap - EXIT

# === Test 8: hybrid delegation log line emitted when claude available ========
header "Test 8: hybrid mode delegates with HYBRID log line"
if command -v claude >/dev/null 2>&1 && [ -f "$SCOUT_PROFILE" ]; then
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT
    echo "p" > "$tmpdir/prompt"
    stderr_log="$tmpdir/stderr-cap"
    set +e
    VALIDATE_ONLY=1 \
        PROFILE_PATH="$SCOUT_PROFILE" RESOLVED_MODEL=sonnet PROMPT_FILE="$tmpdir/prompt" \
        CYCLE=0 WORKSPACE_PATH="$tmpdir" \
        STDOUT_LOG="$tmpdir/stdout" STDERR_LOG="$tmpdir/stderr" \
        ARTIFACT_PATH="$tmpdir/artifact.md" \
        bash "$ADAPTER" 2>"$stderr_log" >/dev/null
    rc=$?
    set -e
    if grep -q "HYBRID mode: delegating to claude.sh" "$stderr_log"; then
        pass "hybrid delegation log line present (rc=$rc)"
    else
        fail_ "hybrid log missing; stderr: $(cat "$stderr_log")"
    fi
    rm -rf "$tmpdir"
    trap - EXIT
else
    pass "skipped: claude not on PATH or scout profile missing"
fi

# === Test 9: --probe ignores run-mode env vars ==============================
header "Test 9: --probe doesn't require run-mode env vars"
set +e
PROFILE_PATH= RESOLVED_MODEL= PROMPT_FILE= bash "$ADAPTER" --probe >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "0" ]; then
    pass "probe rc=0 with no run-mode env"
else
    fail_ "rc=$rc"
fi

# === Test 10: codex.capabilities.json manifest is referenced ================
header "Test 10: codex.capabilities.json present + valid"
MANIFEST="$REPO_ROOT/scripts/cli_adapters/codex.capabilities.json"
if [ -f "$MANIFEST" ] && jq empty "$MANIFEST" 2>/dev/null; then
    adapter_name=$(jq -r '.adapter' "$MANIFEST")
    if [ "$adapter_name" = "codex" ]; then
        pass "manifest valid + adapter=codex"
    else
        fail_ "manifest adapter mismatch: $adapter_name"
    fi
else
    fail_ "manifest missing or invalid JSON"
fi

# === Test 11: claude.sh delegation target exists ============================
header "Test 11: claude.sh adapter present (delegation target)"
CLAUDE_SH="$REPO_ROOT/scripts/cli_adapters/claude.sh"
if [ -x "$CLAUDE_SH" ] || [ -f "$CLAUDE_SH" ]; then
    pass "claude.sh exists"
else
    fail_ "claude.sh missing — hybrid delegation would fail"
fi

# === Summary =================================================================
echo
echo "==========================================="
echo "  Total: 11 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
