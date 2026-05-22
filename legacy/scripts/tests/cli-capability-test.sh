#!/usr/bin/env bash
#
# cli-capability-test.sh — Tests for capability manifest schema + resolution
# (v8.51.0).
#
# Validates:
#   - All 3 manifests parse and conform to the schema
#   - capability-check.sh emits well-formed JSON for each adapter
#   - Probe logic resolves correctly (claude_on_path → hybrid; missing → degraded)
#   - quality_tier aggregates lowest mode across capabilities
#   - bin/check-caps wrapper delegates correctly
#
# Bash 3.2 compatible.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADAPTERS="$REPO_ROOT/scripts/cli_adapters"
CAP_CHECK="$ADAPTERS/_capability-check.sh"
SCHEMA="$ADAPTERS/_capabilities-schema.json"
CHECK_CAPS_BIN="$REPO_ROOT/bin/check-caps"

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# === Test 1: schema file exists and is valid JSON ===============================
header "Test 1: capabilities schema exists + valid JSON"
if [ -f "$SCHEMA" ] && jq empty "$SCHEMA" 2>/dev/null; then
    pass "schema present + parseable"
else
    fail_ "schema missing or malformed"
fi

# === Test 2: all 3 adapter manifests exist + valid JSON ========================
header "Test 2: claude/gemini/codex manifests exist + valid JSON"
all_ok=1
for a in claude gemini codex; do
    m="$ADAPTERS/${a}.capabilities.json"
    if [ ! -f "$m" ]; then
        echo "    missing: $m" >&2
        all_ok=0
    elif ! jq empty "$m" 2>/dev/null; then
        echo "    malformed: $m" >&2
        all_ok=0
    fi
done
[ "$all_ok" = "1" ] && pass "all 3 manifests valid" || fail_ "manifest issue"

# === Test 3: each manifest declares all 5 required capabilities ================
header "Test 3: each manifest covers all 5 capabilities"
required="subprocess_isolation budget_cap sandbox profile_permissions challenge_token"
all_ok=1
for a in claude gemini codex; do
    m="$ADAPTERS/${a}.capabilities.json"
    for cap in $required; do
        if ! jq -e --arg c "$cap" '.capabilities | has($c)' "$m" >/dev/null 2>&1; then
            echo "    $a missing capability: $cap" >&2
            all_ok=0
        fi
    done
done
[ "$all_ok" = "1" ] && pass "all manifests cover all 5 capabilities" || fail_ "incomplete capability coverage"

# === Test 4: claude resolves to quality_tier=full ==============================
header "Test 4: claude adapter resolves to full quality tier"
out=$(bash "$CAP_CHECK" claude 2>&1)
tier=$(echo "$out" | jq -r '.quality_tier' 2>/dev/null)
if [ "$tier" = "full" ]; then
    pass "claude → quality_tier=full"
else
    fail_ "claude → tier=$tier (expected full); out: $out"
fi

# === Test 5: gemini with claude on PATH resolves to hybrid ====================
header "Test 5: gemini + claude on PATH → hybrid"
if command -v claude >/dev/null 2>&1; then
    out=$(bash "$CAP_CHECK" gemini 2>&1)
    tier=$(echo "$out" | jq -r '.quality_tier' 2>/dev/null)
    if [ "$tier" = "hybrid" ]; then
        pass "gemini + claude → hybrid"
    else
        fail_ "expected hybrid, got $tier"
    fi
else
    pass "skipped: no claude binary in test env"
fi

# === Test 6: gemini with claude FORCED missing resolves to none/degraded ======
header "Test 6: gemini + claude FORCED missing → quality_tier degraded or worse"
out=$(EVOLVE_TESTING=1 EVOLVE_GEMINI_CLAUDE_PATH="" bash "$CAP_CHECK" gemini 2>&1)
tier=$(echo "$out" | jq -r '.quality_tier' 2>/dev/null)
if [ "$tier" = "degraded" ] || [ "$tier" = "none" ]; then
    pass "gemini-no-claude → tier=$tier (degraded path active)"
else
    fail_ "expected degraded/none, got tier=$tier"
fi

# === Test 7: gemini-no-claude emits warnings ==================================
header "Test 7: degraded gemini emits warning array"
out=$(EVOLVE_TESTING=1 EVOLVE_GEMINI_CLAUDE_PATH="" bash "$CAP_CHECK" gemini 2>&1)
warning_count=$(echo "$out" | jq -r '.warnings | length' 2>/dev/null)
if [ "$warning_count" -gt "0" ]; then
    pass "warnings emitted (count=$warning_count)"
else
    fail_ "no warnings emitted"
fi

# === Test 8: codex follows same pattern as gemini =============================
header "Test 8: codex adapter resolves quality_tier"
out=$(bash "$CAP_CHECK" codex 2>&1)
tier=$(echo "$out" | jq -r '.quality_tier' 2>/dev/null)
if [ "$tier" = "hybrid" ] || [ "$tier" = "degraded" ] || [ "$tier" = "none" ]; then
    pass "codex resolves to tier=$tier (depends on test env claude availability)"
else
    fail_ "unexpected tier: $tier"
fi

# === Test 9: --probe-only emits probe results only ============================
header "Test 9: --probe-only mode returns just probe results"
out=$(bash "$CAP_CHECK" gemini --probe-only 2>&1)
has_claude_probe=$(echo "$out" | jq -e 'has("claude_on_path")' 2>/dev/null)
no_resolved=$(echo "$out" | jq -e 'has("resolved") | not' 2>/dev/null)
if [ "$has_claude_probe" = "true" ] && [ "$no_resolved" = "true" ]; then
    pass "probe-only output has probes, no resolved field"
else
    fail_ "probe-only output malformed: $out"
fi

# === Test 10: --human mode renders table =====================================
header "Test 10: --human mode produces readable table"
out=$(bash "$CAP_CHECK" claude --human 2>&1)
if echo "$out" | grep -q "Quality tier:" && echo "$out" | grep -q "Capability"; then
    pass "human mode includes header + table"
else
    fail_ "human output missing expected lines"
fi

# === Test 11: --list-adapters lists all 3 ====================================
header "Test 11: --list-adapters returns claude, gemini, codex"
out=$(bash "$CAP_CHECK" --list-adapters 2>&1 | sort | tr '\n' ' ')
expected="claude codex gemini "
if [ "$out" = "$expected" ]; then
    pass "list returns all 3 adapter names"
else
    fail_ "list mismatch: got [$out] expected [$expected]"
fi

# === Test 12: bin/check-caps wrapper delegates correctly =====================
header "Test 12: bin/check-caps wrapper produces same output as direct call"
out_wrapper=$(bash "$CHECK_CAPS_BIN" claude --json 2>&1 | jq -r '.adapter')
out_direct=$(bash "$CAP_CHECK" claude 2>&1 | jq -r '.adapter')
if [ "$out_wrapper" = "$out_direct" ] && [ "$out_wrapper" = "claude" ]; then
    pass "wrapper + direct match (adapter=$out_wrapper)"
else
    fail_ "wrapper=$out_wrapper direct=$out_direct"
fi

# === Test 13: unknown adapter → exit 1 =======================================
header "Test 13: unknown adapter → exit 1"
set +e
bash "$CAP_CHECK" nonexistent >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "1" ]; then
    pass "unknown adapter → rc=1"
else
    fail_ "expected rc=1, got rc=$rc"
fi

# === Test 14: bad arguments → exit 10 ========================================
header "Test 14: bad arguments → exit 10"
set +e
bash "$CAP_CHECK" --bogus >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "10" ]; then
    pass "bad flag → rc=10"
else
    fail_ "expected rc=10, got rc=$rc"
fi

# === Test 15: gemini probes affect ALL 5 capabilities (whole-adapter degradation)
header "Test 15: gemini-no-claude affects all 5 capabilities"
out=$(EVOLVE_TESTING=1 EVOLVE_GEMINI_CLAUDE_PATH="" bash "$CAP_CHECK" gemini 2>&1)
# Count capabilities that resolved to degraded or none
degraded_count=$(echo "$out" | jq -r '.resolved | to_entries | map(select(.value.mode == "degraded" or .value.mode == "none")) | length' 2>/dev/null)
if [ "$degraded_count" = "5" ]; then
    pass "all 5 capabilities degraded when claude missing"
else
    fail_ "only $degraded_count of 5 degraded; resolved: $(echo "$out" | jq -c .resolved)"
fi

# === Summary ====================================================================
echo
echo "==========================================="
echo "  Total: 15 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
