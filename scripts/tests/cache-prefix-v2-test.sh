#!/usr/bin/env bash
#
# cache-prefix-v2-test.sh — tests for build-invocation-context.sh and the
# subagent-run.sh v2 prompt ordering (Campaign A — Tier 1 cache layer).
#
# v8.61.0 Layer 1.
#
# Tests:
#   1. build-invocation-context.sh is deterministic (byte-identical output).
#   2. Common bedrock present in every role.
#   3. Auditor role includes "Adversarial Audit Mode" section.
#   4. Builder role includes "Builder operating notes" section.
#   5. Scout role includes "Scout operating notes" section.
#   6. Retrospective role includes "Retrospective operating notes" section.
#   7. Non-auditor roles do NOT include "Adversarial Audit Mode".
#   8. Missing role argument exits with code 2.
#   9. Unknown role emits bedrock-only (no role-specific extension).
#  10. Bedrock contains no random bytes / timestamps / env-leaked data.
#  11. EVOLVE_CACHE_PREFIX_V2 default is 0 (legacy behavior preserved).
#  12. build-invocation-context.sh is referenced by subagent-run.sh.
#
# Usage: bash scripts/tests/cache-prefix-v2-test.sh

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIC="$REPO_ROOT/scripts/dispatch/build-invocation-context.sh"
RUNNER="$REPO_ROOT/scripts/dispatch/subagent-run.sh"

PASS=0
FAIL=0
TESTS_TOTAL=0

pass()    { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()   { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header()  { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# --- Test 1: determinism -----------------------------------------------------
header "Test 1: build-invocation-context.sh is byte-identical across invocations"
out1=$(bash "$BIC" auditor 2>&1)
out2=$(bash "$BIC" auditor 2>&1)
out3=$(bash "$BIC" auditor 2>&1)
if [ "$out1" = "$out2" ] && [ "$out2" = "$out3" ]; then
    pass "auditor bedrock byte-identical across 3 invocations"
else
    fail_ "auditor bedrock changed between invocations"
fi

# Repeat for builder.
b1=$(bash "$BIC" builder 2>&1)
b2=$(bash "$BIC" builder 2>&1)
if [ "$b1" = "$b2" ]; then
    pass "builder bedrock byte-identical"
else
    fail_ "builder bedrock differs between invocations"
fi

# --- Test 2: common bedrock --------------------------------------------------
header "Test 2: common bedrock present in every role"
for r in scout builder auditor retrospective triage memo intent inspirer; do
    out=$(bash "$BIC" "$r" 2>&1)
    if echo "$out" | grep -q "EVOLVE-LOOP SUBAGENT INVOCATION" \
       && echo "$out" | grep -q "Mandatory output contract" \
       && echo "$out" | grep -q "Trust boundary reminders"; then
        pass "$r contains common bedrock"
    else
        fail_ "$r missing one of: header, contract, trust-boundary"
    fi
done

# --- Test 3: auditor includes Adversarial Audit Mode -------------------------
header "Test 3: auditor includes Adversarial Audit Mode"
if bash "$BIC" auditor | grep -q "Adversarial Audit Mode"; then
    pass "auditor bedrock contains Adversarial Audit Mode"
else
    fail_ "auditor bedrock missing Adversarial Audit Mode"
fi

# --- Test 4: builder includes Builder operating notes ------------------------
header "Test 4: builder includes Builder operating notes"
if bash "$BIC" builder | grep -q "Builder operating notes"; then
    pass "builder bedrock contains Builder operating notes"
else
    fail_ "builder bedrock missing Builder operating notes"
fi

# --- Test 5: scout includes Scout operating notes ----------------------------
header "Test 5: scout includes Scout operating notes"
if bash "$BIC" scout | grep -q "Scout operating notes"; then
    pass "scout bedrock contains Scout operating notes"
else
    fail_ "scout bedrock missing Scout operating notes"
fi

# --- Test 6: retrospective includes Retrospective operating notes ------------
header "Test 6: retrospective includes Retrospective operating notes"
if bash "$BIC" retrospective | grep -q "Retrospective operating notes"; then
    pass "retrospective bedrock contains Retrospective operating notes"
else
    fail_ "retrospective bedrock missing Retrospective operating notes"
fi

# --- Test 7: non-auditor roles do NOT include Adversarial Audit Mode ---------
header "Test 7: non-auditor roles do not include Adversarial Audit Mode"
for r in scout builder retrospective triage memo intent; do
    if bash "$BIC" "$r" | grep -q "Adversarial Audit Mode"; then
        fail_ "$r unexpectedly contains Adversarial Audit Mode"
    else
        pass "$r correctly omits Adversarial Audit Mode"
    fi
done

# --- Test 8: missing role argument -------------------------------------------
header "Test 8: missing role argument exits with code 2"
set +e
bash "$BIC" >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "2" ]; then
    pass "missing role exits 2"
else
    fail_ "expected exit 2, got $rc"
fi

# --- Test 9: unknown role emits bedrock-only ---------------------------------
header "Test 9: unknown role emits bedrock-only (no role-specific extension)"
out=$(bash "$BIC" some-unknown-role 2>&1)
if echo "$out" | grep -q "EVOLVE-LOOP SUBAGENT INVOCATION" \
   && ! echo "$out" | grep -qE "(Adversarial Audit Mode|Builder operating notes|Scout operating notes|Retrospective operating notes)"; then
    pass "unknown role gets bedrock without role-specific extension"
else
    fail_ "unknown role behavior unexpected (should be bedrock-only)"
fi

# --- Test 10: no random bytes / timestamps / env-leaked data -----------------
header "Test 10: bedrock has no timestamps / env-leaked data"
out=$(bash "$BIC" auditor)
# Common timestamp patterns to reject:
if echo "$out" | grep -qE '20[0-9][0-9]-[01][0-9]-[0-3][0-9]T'; then
    fail_ "auditor bedrock contains an ISO timestamp"
elif echo "$out" | grep -qE '/Users/|/home/|/var/folders/'; then
    fail_ "auditor bedrock contains a leaked filesystem path"
elif echo "$out" | grep -qE '\$[A-Z_]{4,}=' ; then
    fail_ "auditor bedrock contains a leaked env-var assignment"
else
    pass "auditor bedrock free of timestamps and leaked paths/env"
fi

# --- Test 11: EVOLVE_CACHE_PREFIX_V2 default is 0 (legacy preserved) ---------
header "Test 11: EVOLVE_CACHE_PREFIX_V2 default behavior preserved"
# Grep the runner for the explicit default.
if grep -q 'EVOLVE_CACHE_PREFIX_V2:-0' "$RUNNER"; then
    pass "subagent-run.sh defaults EVOLVE_CACHE_PREFIX_V2 to 0"
else
    fail_ "subagent-run.sh missing default EVOLVE_CACHE_PREFIX_V2:-0"
fi

# --- Test 12: build-invocation-context.sh referenced by subagent-run.sh ------
header "Test 12: subagent-run.sh references build-invocation-context.sh"
if grep -q "build-invocation-context.sh" "$RUNNER"; then
    pass "subagent-run.sh references build-invocation-context.sh"
else
    fail_ "subagent-run.sh does not reference build-invocation-context.sh"
fi

# --- Test 13: claude.sh adapter has v2 system-prompt block (Cycle A2) --------
header "Test 13 (Cycle A2): claude.sh has v2 system-prompt block"
ADAPTER="$REPO_ROOT/scripts/cli_adapters/claude.sh"
if grep -q "EVOLVE_CACHE_PREFIX_V2:-0" "$ADAPTER" \
   && grep -q -- "--append-system-prompt" "$ADAPTER" \
   && grep -q "build-invocation-context.sh" "$ADAPTER"; then
    pass "claude.sh wires bedrock to --append-system-prompt under V2"
else
    fail_ "claude.sh missing v2 system-prompt wiring"
fi

# --- Test 14: claude.sh uses --exclude-dynamic-system-prompt-sections under V2
header "Test 14 (Cycle A2): claude.sh uses --exclude-dynamic-system-prompt-sections under v2"
if grep -q -- "--exclude-dynamic-system-prompt-sections" "$ADAPTER"; then
    pass "claude.sh adds --exclude-dynamic-system-prompt-sections"
else
    fail_ "claude.sh missing --exclude-dynamic-system-prompt-sections"
fi

# --- Test 15: subagent-run.sh exports AGENT env to adapter -------------------
header "Test 15 (Cycle A2): subagent-run.sh exports AGENT env to adapter"
if grep -q 'AGENT="\$agent"' "$RUNNER"; then
    pass "subagent-run.sh exports AGENT to adapter"
else
    fail_ "subagent-run.sh does not export AGENT to adapter"
fi

# --- Test 16: under v2, user prompt no longer contains role bedrock at top ---
header "Test 16 (Cycle A2): v2 path emits INVOCATION CONTEXT first (not bedrock)"
# Look for the v2 fork in subagent-run.sh; check that INVOCATION CONTEXT is the
# FIRST major section in the user prompt under v2 (no bedrock prepend).
v2_section=$(awk '/EVOLVE_CACHE_PREFIX_V2:-0.*= ?"1"/,/--- v1 path/' "$RUNNER")
if echo "$v2_section" | grep -q "INVOCATION CONTEXT" \
   && ! echo "$v2_section" | grep -qE 'bash "?\$_bic_script"? "?\$agent"? > "?\$injected_prompt"?'; then
    pass "v2 path no longer prepends bedrock to user prompt (delegated to system prompt)"
else
    fail_ "v2 path still prepends bedrock to user prompt OR INVOCATION CONTEXT missing"
fi

# --- Test 17: claude.sh strips Adversarial Audit Mode when ADVERSARIAL_AUDIT=0
header "Test 17 (Cycle A2): claude.sh strips Adversarial Audit Mode when disabled"
# Extended-regex match — claude.sh has both the env-var check and the
# Adversarial Audit Mode literal needed for the strip awk pattern.
if grep -qE 'ADVERSARIAL_AUDIT.*=.*"0"' "$ADAPTER" \
   && grep -q "Adversarial Audit Mode" "$ADAPTER"; then
    pass "claude.sh has ADVERSARIAL_AUDIT=0 strip logic"
else
    fail_ "claude.sh missing ADVERSARIAL_AUDIT=0 strip logic for system prompt"
fi

# --- Summary -----------------------------------------------------------------
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="

[ "$FAIL" = "0" ]
