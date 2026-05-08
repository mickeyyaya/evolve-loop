#!/usr/bin/env bash
#
# cross-cli-parity-test.sh — Verify all 3 adapters maintain pipeline parity
# (v8.51.0+).
#
# Same prompt + same profile through claude / gemini / codex adapters must
# produce structurally compatible artifacts. Content may differ (different
# LLMs, or no LLM call in degraded mode), but the contract — env vars
# consumed, stdout schema, error handling, exit codes — must match.
#
# This is a SCHEMA test, not a content test. We do NOT assert byte-equal
# output, only that the adapter contract holds across all three.
#
# Bash 3.2 compatible.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADAPTER_DIR="$REPO_ROOT/scripts/cli_adapters"
SCOUT_PROFILE="$REPO_ROOT/.evolve/profiles/scout.json"

PASS=0; FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# === Test 1: all 3 adapter scripts present + executable =====================
header "Test 1: claude.sh, gemini.sh, codex.sh all present + executable"
all_ok=1
for a in claude gemini codex; do
    f="$ADAPTER_DIR/${a}.sh"
    if [ ! -x "$f" ]; then
        echo "    not executable: $f" >&2
        all_ok=0
    fi
done
[ "$all_ok" = "1" ] && pass "all 3 executable" || fail_ "missing exec bit"

# === Test 2: all 3 adapters declare the same ENV CONTRACT ===================
# We verify by grep that each adapter references all 8 standard env vars.
header "Test 2: adapter env var contract uniform across all 3"
required_envs="PROFILE_PATH PROMPT_FILE STDOUT_LOG STDERR_LOG ARTIFACT_PATH"
all_ok=1
for a in claude gemini codex; do
    f="$ADAPTER_DIR/${a}.sh"
    for e in $required_envs; do
        if ! grep -q "$e" "$f"; then
            echo "    $a missing env: $e" >&2
            all_ok=0
        fi
    done
done
[ "$all_ok" = "1" ] && pass "all 3 reference all 5 required env vars" || fail_ "env contract drift"

# === Test 3: all 3 adapters expose --probe mode =============================
header "Test 3: --probe mode supported by all 3 adapters"
all_ok=1
for a in claude gemini codex; do
    f="$ADAPTER_DIR/${a}.sh"
    set +e
    bash "$f" --probe >/dev/null 2>&1
    rc=$?
    set -e
    # claude.sh may not have --probe; gemini + codex do. Only test the latter two.
    if [ "$a" != "claude" ] && [ "$rc" != "0" ] && [ "$rc" != "99" ]; then
        echo "    $a probe rc=$rc (expected 0 or 99)" >&2
        all_ok=0
    fi
done
[ "$all_ok" = "1" ] && pass "gemini + codex --probe both work" || fail_ "probe contract drift"

# === Test 4: all 3 adapters have a capability manifest =====================
header "Test 4: capabilities.json manifests present + valid for all 3"
all_ok=1
for a in claude gemini codex; do
    m="$ADAPTER_DIR/${a}.capabilities.json"
    if [ ! -f "$m" ] || ! jq empty "$m" 2>/dev/null; then
        echo "    $a manifest missing or invalid: $m" >&2
        all_ok=0
    fi
done
[ "$all_ok" = "1" ] && pass "all 3 manifests present + valid JSON" || fail_ "manifest drift"

# === Test 5: all 3 manifests cover the same 5 capabilities ==================
header "Test 5: capability dimensions identical across manifests"
required_caps="subprocess_isolation budget_cap sandbox profile_permissions challenge_token"
all_ok=1
for a in claude gemini codex; do
    m="$ADAPTER_DIR/${a}.capabilities.json"
    for cap in $required_caps; do
        if ! jq -e --arg c "$cap" '.capabilities | has($c)' "$m" >/dev/null 2>&1; then
            echo "    $a missing cap: $cap" >&2
            all_ok=0
        fi
    done
done
[ "$all_ok" = "1" ] && pass "all 3 cover all 5 capability dimensions" || fail_ "capability dimension drift"

# === Test 6: gemini + codex degraded-mode stdout schema parity =============
# Both adapters in DEGRADED mode emit a structured stdout JSON for cost
# accounting. Must include: degraded_mode (bool), adapter (string),
# cost_usd (number), prompt_file (string), artifact_path (string).
header "Test 6: gemini + codex DEGRADED stdout JSON schema parity"
required_fields="degraded_mode adapter cost_usd prompt_file artifact_path"
all_ok=1
for a in gemini codex; do
    f="$ADAPTER_DIR/${a}.sh"
    seam_var=""
    # bash 3.2 doesn't support ${var^^}, but we know the names:
    if [ "$a" = "gemini" ]; then seam_var="EVOLVE_GEMINI_CLAUDE_PATH"; fi
    if [ "$a" = "codex" ]; then seam_var="EVOLVE_CODEX_CLAUDE_PATH"; fi
    
    TMPD=$(mktemp -d)
    echo "p" > "$TMPD/prompt"
    set +e
    PROFILE_PATH="$SCOUT_PROFILE" RESOLVED_MODEL=sonnet PROMPT_FILE="$TMPD/prompt" \
        CYCLE=0 WORKSPACE_PATH="$TMPD" \
        STDOUT_LOG="$TMPD/stdout" STDERR_LOG="$TMPD/stderr" \
        ARTIFACT_PATH="$TMPD/artifact.md" \
        EVOLVE_TESTING=1 \
        env "${seam_var}=" bash "$f" >/dev/null 2>&1
    set -e
    
    if [ -s "$TMPD/stdout" ] && jq empty "$TMPD/stdout" 2>/dev/null; then
        for field in $required_fields; do
            if ! jq -e --arg k "$field" 'has($k)' "$TMPD/stdout" >/dev/null 2>&1; then
                echo "    $a stdout missing field: $field" >&2
                all_ok=0
            fi
        done
    else
        echo "    $a stdout not parseable JSON" >&2
        all_ok=0
    fi
    rm -rf "$TMPD"
done
[ "$all_ok" = "1" ] && pass "gemini + codex stdout schema parity (5 fields each)" || fail_ "stdout schema drift"

# === Test 7: claude.capabilities declares full caps ========================
header "Test 7: claude.capabilities.json declares full subprocess_isolation"
m="$ADAPTER_DIR/claude.capabilities.json"
sub_iso=$(jq -r '.capabilities.subprocess_isolation' "$m")
if [ "$sub_iso" = "full" ]; then
    pass "claude → subprocess_isolation: full"
else
    fail_ "expected full, got $sub_iso"
fi

# === Test 8: gemini + codex have a probe declaration ========================
header "Test 8: gemini + codex manifests declare claude_on_path probe"
all_ok=1
for a in gemini codex; do
    m="$ADAPTER_DIR/${a}.capabilities.json"
    has_probe=$(jq -r '.probes // [] | map(select(.check == "claude_on_path")) | length' "$m")
    if [ "$has_probe" -lt 1 ]; then
        echo "    $a missing claude_on_path probe" >&2
        all_ok=0
    fi
done
[ "$all_ok" = "1" ] && pass "both have claude_on_path probe" || fail_ "probe declaration drift"

# === Test 9: hybrid → resolves to same tier for gemini + codex (claude on PATH)
header "Test 9: gemini + codex both resolve to hybrid when claude on PATH"
if command -v claude >/dev/null 2>&1; then
    CAP_CHECK="$ADAPTER_DIR/_capability-check.sh"
    g_tier=$(bash "$CAP_CHECK" gemini | jq -r '.quality_tier')
    c_tier=$(bash "$CAP_CHECK" codex | jq -r '.quality_tier')
    if [ "$g_tier" = "$c_tier" ] && [ "$g_tier" = "hybrid" ]; then
        pass "both → hybrid tier"
    else
        fail_ "tier mismatch: gemini=$g_tier codex=$c_tier"
    fi
else
    pass "skipped: claude not on PATH"
fi

# === Test 10: degraded → both gemini + codex resolve to same low tier ======
header "Test 10: gemini + codex degraded resolution parity"
CAP_CHECK="$ADAPTER_DIR/_capability-check.sh"
g_tier=$(EVOLVE_TESTING=1 EVOLVE_GEMINI_CLAUDE_PATH="" bash "$CAP_CHECK" gemini | jq -r '.quality_tier')
c_tier=$(EVOLVE_TESTING=1 EVOLVE_CODEX_CLAUDE_PATH="" bash "$CAP_CHECK" codex | jq -r '.quality_tier')
if [ "$g_tier" = "$c_tier" ]; then
    pass "both degrade to tier=$g_tier (parity preserved)"
else
    fail_ "tier mismatch: gemini=$g_tier codex=$c_tier"
fi

# === Summary =================================================================
echo
echo "==========================================="
echo "  Total: 10 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
