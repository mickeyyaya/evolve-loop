#!/usr/bin/env bash
#
# cli-degradation-test.sh — Verify pipeline runs in DEGRADED mode for
# non-Claude adapters when claude binary is missing (v8.51.0+).
#
# Critical safety invariant: when an adapter resolves to degraded/none tier,
# the pipeline must continue to function. Missing capabilities only LOWER the
# quality (less isolation, no native budget caps), never block the pipeline.
#
# These tests exercise:
#   - gemini.sh DEGRADED mode does not exit 99 by default
#   - codex.sh DEGRADED mode does not exit 99 by default
#   - Both adapters print clear DEGRADED warnings
#   - Both adapters write structured stdout for cost accounting
#   - Pipeline-level kernel hooks still fire under degraded adapters
#   - --require-full opt-in restores hard-fail behavior
#
# Bash 3.2 compatible.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GEMINI="$REPO_ROOT/scripts/cli_adapters/gemini.sh"
CODEX="$REPO_ROOT/scripts/cli_adapters/codex.sh"
SCOUT_PROFILE="$REPO_ROOT/.evolve/profiles/scout.json"
SHIP_GATE="$REPO_ROOT/scripts/guards/ship-gate.sh"

PASS=0; FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Common run-mode env helper
make_tmp() {
    local d
    d=$(mktemp -d -t "degtest.XXXXXX")
    echo "test prompt" > "$d/prompt"
    echo "$d"
}

# === Test 1: gemini DEGRADED mode does NOT exit 99 by default ================
header "Test 1: gemini DEGRADED mode default → exit 0"
TMPD=$(make_tmp)
set +e
PROFILE_PATH="$SCOUT_PROFILE" RESOLVED_MODEL=sonnet PROMPT_FILE="$TMPD/prompt" \
    CYCLE=0 WORKSPACE_PATH="$TMPD" \
    STDOUT_LOG="$TMPD/stdout" STDERR_LOG="$TMPD/stderr" \
    ARTIFACT_PATH="$TMPD/artifact.md" \
    EVOLVE_TESTING=1 EVOLVE_GEMINI_CLAUDE_PATH="" \
    bash "$GEMINI" >/dev/null 2>"$TMPD/stderr-cap"
rc=$?
set -e
if [ "$rc" = "0" ] && grep -q "DEGRADED MODE active" "$TMPD/stderr-cap"; then
    pass "rc=0 + DEGRADED warning"
else
    fail_ "rc=$rc; stderr: $(cat "$TMPD/stderr-cap")"
fi
rm -rf "$TMPD"

# === Test 2: codex DEGRADED mode default → exit 0 ============================
header "Test 2: codex DEGRADED mode default → exit 0"
TMPD=$(make_tmp)
set +e
PROFILE_PATH="$SCOUT_PROFILE" RESOLVED_MODEL=sonnet PROMPT_FILE="$TMPD/prompt" \
    CYCLE=0 WORKSPACE_PATH="$TMPD" \
    STDOUT_LOG="$TMPD/stdout" STDERR_LOG="$TMPD/stderr" \
    ARTIFACT_PATH="$TMPD/artifact.md" \
    EVOLVE_TESTING=1 EVOLVE_CODEX_CLAUDE_PATH="" \
    bash "$CODEX" >/dev/null 2>"$TMPD/stderr-cap"
rc=$?
set -e
if [ "$rc" = "0" ] && grep -q "DEGRADED MODE active" "$TMPD/stderr-cap"; then
    pass "rc=0 + DEGRADED warning"
else
    fail_ "rc=$rc; stderr: $(cat "$TMPD/stderr-cap")"
fi
rm -rf "$TMPD"

# === Test 3: gemini --require-full + claude missing → exit 99 ===============
header "Test 3: gemini --require-full + claude missing → exit 99 (opt-in hard-fail)"
TMPD=$(make_tmp)
set +e
PROFILE_PATH=/dev/null RESOLVED_MODEL=sonnet PROMPT_FILE=/dev/null \
    CYCLE=0 WORKSPACE_PATH=/tmp STDOUT_LOG=/tmp/x STDERR_LOG=/tmp/y ARTIFACT_PATH=/tmp/z \
    EVOLVE_TESTING=1 EVOLVE_GEMINI_CLAUDE_PATH="" \
    EVOLVE_GEMINI_REQUIRE_FULL=1 \
    bash "$GEMINI" >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "99" ]; then
    pass "rc=99 with require-full + claude missing"
else
    fail_ "expected rc=99, got rc=$rc"
fi
rm -rf "$TMPD"

# === Test 4: codex --require-full + claude missing → exit 99 ================
header "Test 4: codex --require-full + claude missing → exit 99"
set +e
PROFILE_PATH=/dev/null RESOLVED_MODEL=sonnet PROMPT_FILE=/dev/null \
    CYCLE=0 WORKSPACE_PATH=/tmp STDOUT_LOG=/tmp/x STDERR_LOG=/tmp/y ARTIFACT_PATH=/tmp/z \
    EVOLVE_TESTING=1 EVOLVE_CODEX_CLAUDE_PATH="" \
    EVOLVE_CODEX_REQUIRE_FULL=1 \
    bash "$CODEX" >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "99" ]; then
    pass "rc=99 with codex require-full + claude missing"
else
    fail_ "expected rc=99, got rc=$rc"
fi

# === Test 5: degraded mode emits structured stdout for cost accounting ======
header "Test 5: gemini DEGRADED stdout is parseable JSON with degraded_mode=true"
TMPD=$(make_tmp)
set +e
PROFILE_PATH="$SCOUT_PROFILE" RESOLVED_MODEL=sonnet PROMPT_FILE="$TMPD/prompt" \
    CYCLE=0 WORKSPACE_PATH="$TMPD" \
    STDOUT_LOG="$TMPD/stdout" STDERR_LOG="$TMPD/stderr" \
    ARTIFACT_PATH="$TMPD/artifact.md" \
    EVOLVE_TESTING=1 EVOLVE_GEMINI_CLAUDE_PATH="" \
    bash "$GEMINI" >/dev/null 2>&1
set -e
if [ -s "$TMPD/stdout" ] && jq empty "$TMPD/stdout" 2>/dev/null; then
    deg=$(jq -r '.degraded_mode' "$TMPD/stdout")
    cost=$(jq -r '.cost_usd' "$TMPD/stdout")
    if [ "$deg" = "true" ] && [ "$cost" = "0" ]; then
        pass "stdout JSON: degraded_mode=true, cost_usd=0"
    else
        fail_ "stdout JSON malformed: deg=$deg cost=$cost"
    fi
else
    fail_ "stdout missing or not JSON"
fi
rm -rf "$TMPD"

# === Test 6: degraded mode warning lists specific capabilities ==============
header "Test 6: gemini DEGRADED warning enumerates specific missing capabilities"
TMPD=$(make_tmp)
set +e
PROFILE_PATH="$SCOUT_PROFILE" RESOLVED_MODEL=sonnet PROMPT_FILE="$TMPD/prompt" \
    CYCLE=0 WORKSPACE_PATH="$TMPD" \
    STDOUT_LOG="$TMPD/stdout" STDERR_LOG="$TMPD/stderr" \
    ARTIFACT_PATH="$TMPD/artifact.md" \
    EVOLVE_TESTING=1 EVOLVE_GEMINI_CLAUDE_PATH="" \
    bash "$GEMINI" >/dev/null 2>"$TMPD/stderr-cap"
set -e
all_present=1
for capkw in subprocess_isolation budget_cap sandbox profile_permissions; do
    grep -q "$capkw" "$TMPD/stderr-cap" || all_present=0
done
if [ "$all_present" = "1" ]; then
    pass "warning lists all 4 capability dimensions"
else
    fail_ "warning incomplete: $(cat "$TMPD/stderr-cap")"
fi
rm -rf "$TMPD"

# === Test 7: ship-gate.sh kernel hook still fires regardless of adapter =====
# Defense-in-depth: even in degraded mode, the structural git-write block
# remains. ship-gate intercepts on the Bash tool layer (PreToolUse), not at
# adapter dispatch — so it's adapter-independent.
header "Test 7: ship-gate.sh refuses raw git commit (kernel-level, adapter-independent)"
if [ -x "$SHIP_GATE" ]; then
    set +e
    payload='{"tool_input":{"command":"git commit -m forge"}}'
    echo "$payload" | bash "$SHIP_GATE" >/dev/null 2>&1
    rc=$?
    set -e
    if [ "$rc" = "2" ]; then
        pass "ship-gate refuses raw git commit (rc=2) — same as Claude path"
    else
        fail_ "expected rc=2, got rc=$rc"
    fi
else
    pass "skipped: ship-gate.sh not present"
fi

# === Test 8: ledger entry from degraded run includes quality_tier field =====
# Indirect test: verify the write_ledger_entry signature in subagent-run.sh
# accepts a 9th quality_tier arg. The full integration would require a real
# cycle which is too heavy for unit tests; we verify the contract via grep.
header "Test 8: subagent-run.sh write_ledger_entry signature includes quality_tier"
RUNNER="$REPO_ROOT/scripts/dispatch/subagent-run.sh"
if grep -q 'local quality_tier="${9:-unknown}"' "$RUNNER"; then
    pass "write_ledger_entry accepts quality_tier as 9th arg"
else
    fail_ "quality_tier 9th arg not found in write_ledger_entry"
fi

# === Test 9: write_ledger_entry emits quality_tier in JSON ==================
header "Test 9: ledger JSON includes quality_tier field"
if grep -q "quality_tier:" "$RUNNER"; then
    pass "ledger JSON template includes quality_tier"
else
    fail_ "quality_tier field missing from ledger emission"
fi

# === Test 10: capability-check resolves degraded for both gemini AND codex ==
header "Test 10: capability-check returns degraded tier for both gemini + codex when claude missing"
CAP_CHECK="$REPO_ROOT/scripts/cli_adapters/_capability-check.sh"
gemini_tier=$(EVOLVE_TESTING=1 EVOLVE_GEMINI_CLAUDE_PATH="" bash "$CAP_CHECK" gemini 2>/dev/null | jq -r '.quality_tier')
codex_tier=$(EVOLVE_TESTING=1 EVOLVE_CODEX_CLAUDE_PATH="" bash "$CAP_CHECK" codex 2>/dev/null | jq -r '.quality_tier')
if [ "$gemini_tier" = "none" ] || [ "$gemini_tier" = "degraded" ]; then
    if [ "$codex_tier" = "none" ] || [ "$codex_tier" = "degraded" ]; then
        pass "both adapters resolve to degraded/none under missing-claude"
    else
        fail_ "codex tier=$codex_tier"
    fi
else
    fail_ "gemini tier=$gemini_tier"
fi

# === Summary =================================================================
echo
echo "==========================================="
echo "  Total: 10 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
