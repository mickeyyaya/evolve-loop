#!/usr/bin/env bash
#
# failure-adapter-test.sh — Unit tests for the v8.22.0 deterministic decision kernel.
#
# Each test builds a synthetic state.json fixture and asserts the adapter's
# emitted JSON matches expectations. Covers the 7 decision rules + edge cases.
#
# Usage: bash scripts/failure-adapter-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/failure-adapter.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

cleanup_files=()
trap 'for f in "${cleanup_files[@]}"; do rm -f "$f"; done' EXIT

# Helpers for building fixture entries.
NOW_S=$(date -u +%s)
PAST_ISO()  { echo "$((NOW_S - 86400 * ${1:-2}))" | jq -r '. | todate'; }
FUTURE_ISO(){ echo "$((NOW_S + 86400 * ${1:-1}))" | jq -r '. | todate'; }
NOW_ISO=$(echo "$NOW_S" | jq -r '. | todate')

make_state() {
    local f
    f=$(mktemp -t failure-adapter-test.XXXXXX.json)
    cleanup_files+=("$f")
    cat > "$f" <<EOF
{
  "lastCycleNumber": 30,
  "failedApproaches": $1
}
EOF
    echo "$f"
}

decide() { bash "$SCRIPT" decide --state "$1"; }

# === Test 1: empty failedApproaches → PROCEED ================================
header "Test 1: no failures → PROCEED"
sf=$(make_state '[]')
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
if [ "$action" = "PROCEED" ]; then
    pass "PROCEED on empty"
else
    fail_ "expected PROCEED, got $action"
fi

# === Test 2: missing state file → PROCEED ====================================
header "Test 2: missing state file → PROCEED"
sf=$(mktemp -t failure-adapter-missing.XXXXXX.json); cleanup_files+=("$sf")
rm -f "$sf"
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
if [ "$action" = "PROCEED" ]; then
    pass "PROCEED on missing state"
else
    fail_ "expected PROCEED, got $action"
fi

# === Test 3: 1 infra-transient (non-expired) → RETRY-WITH-FALLBACK ===========
header "Test 3: 1 infra-transient → RETRY-WITH-FALLBACK + set_env"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[{\"cycle\":10,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"}]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
flag=$(echo "$out" | jq -r '.set_env.EVOLVE_SANDBOX_FALLBACK_ON_EPERM // "(unset)"')
if [ "$action" = "RETRY-WITH-FALLBACK" ] && [ "$flag" = "1" ]; then
    pass "RETRY-WITH-FALLBACK + EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1"
else
    fail_ "action=$action flag=$flag"
fi

# === Test 4: 3 consecutive infra-transient → BLOCK-OPERATOR-ACTION ===========
header "Test 4: 3 consecutive infra-transient (tail streak) → BLOCK-OPERATOR-ACTION"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"},
  {\"cycle\":3,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"}
]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block // "null"')
streak=$(echo "$out" | jq -r '.evidence.consecutive_infra_transient_streak')
if [ "$action" = "BLOCK-OPERATOR-ACTION" ] && [ "$verdict" = "BLOCKED-SYSTEMIC" ] && [ "$streak" = "3" ]; then
    pass "BLOCK-OPERATOR-ACTION (BLOCKED-SYSTEMIC), tail-streak=3"
else
    fail_ "action=$action verdict=$verdict streak=$streak"
fi

# === Test 5: streak interrupted by code failure → only RETRY-WITH-FALLBACK ===
header "Test 5: 2 infra-transient + 1 code-audit-fail + 1 infra-transient → RETRY (streak=1)"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"},
  {\"cycle\":3,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":4,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"}
]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
streak=$(echo "$out" | jq -r '.evidence.consecutive_infra_transient_streak')
# 1 code-audit-fail → does NOT trigger 2+ rule. 1 infra-transient at tail → RETRY.
if [ "$action" = "RETRY-WITH-FALLBACK" ] && [ "$streak" = "1" ]; then
    pass "RETRY-WITH-FALLBACK, code break interrupted streak (tail=1)"
else
    fail_ "action=$action tail-streak=$streak"
fi

# === Test 6: 2 code-audit-fail → BLOCK-CODE BLOCKED-RECURRING-AUDIT-FAIL =====
header "Test 6: 2 code-audit-fail → BLOCK-CODE + BLOCKED-RECURRING-AUDIT-FAIL"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"}
]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block')
if [ "$action" = "BLOCK-CODE" ] && [ "$verdict" = "BLOCKED-RECURRING-AUDIT-FAIL" ]; then
    pass "BLOCK-CODE + BLOCKED-RECURRING-AUDIT-FAIL"
else
    fail_ "action=$action verdict=$verdict"
fi

# === Test 7: 2 code-build-fail → BLOCK-CODE BLOCKED-RECURRING-BUILD-FAIL =====
header "Test 7: 2 code-build-fail → BLOCK-CODE + BLOCKED-RECURRING-BUILD-FAIL"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-build-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-build-fail\",\"expiresAt\":\"$exp\"}
]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block')
if [ "$action" = "BLOCK-CODE" ] && [ "$verdict" = "BLOCKED-RECURRING-BUILD-FAIL" ]; then
    pass "BLOCK-CODE + BLOCKED-RECURRING-BUILD-FAIL"
else
    fail_ "action=$action verdict=$verdict"
fi

# === Test 8: 1 intent-rejected → BLOCK-CODE SCOPE-REJECTED ===================
header "Test 8: 1 intent-rejected (non-expired) → BLOCK-CODE + SCOPE-REJECTED"
exp=$(FUTURE_ISO 365)  # intent-rejected has very long expiry
sf=$(make_state "[{\"cycle\":1,\"classification\":\"intent-rejected\",\"expiresAt\":\"$exp\"}]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block')
if [ "$action" = "BLOCK-CODE" ] && [ "$verdict" = "SCOPE-REJECTED" ]; then
    pass "BLOCK-CODE + SCOPE-REJECTED"
else
    fail_ "action=$action verdict=$verdict"
fi

# === Test 9: 1 infrastructure-systemic → BLOCK-OPERATOR-ACTION ===============
header "Test 9: 1 infra-systemic (non-expired) → BLOCK-OPERATOR-ACTION"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[{\"cycle\":1,\"classification\":\"infrastructure-systemic\",\"expiresAt\":\"$exp\",\"summary\":\"claude-cli not installed\"}]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block')
if [ "$action" = "BLOCK-OPERATOR-ACTION" ] && [ "$verdict" = "BLOCKED-SYSTEMIC" ]; then
    pass "BLOCK-OPERATOR-ACTION + BLOCKED-SYSTEMIC"
else
    fail_ "action=$action verdict=$verdict"
fi

# === Test 10: expired entries are auto-pruned and ignored ====================
header "Test 10: 5 expired infra-transient → PROCEED (entries pruned)"
expired=$(PAST_ISO 2)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$expired\"},
  {\"cycle\":2,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$expired\"},
  {\"cycle\":3,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$expired\"},
  {\"cycle\":4,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$expired\"},
  {\"cycle\":5,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$expired\"}
]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
non_expired=$(echo "$out" | jq -r '.evidence.non_expired_count')
if [ "$action" = "PROCEED" ] && [ "$non_expired" = "0" ]; then
    pass "PROCEED, all 5 entries pruned (non_expired_count=0)"
else
    fail_ "action=$action non_expired=$non_expired"
fi

# === Test 11: priority — intent-rejected wins over other failures ============
# An intent-rejected entry should block even if there are also code failures or
# infra streaks (operator must refine goal first).
header "Test 11: intent-rejected + 2 code-audit-fail → SCOPE-REJECTED (priority)"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":3,\"classification\":\"intent-rejected\",\"expiresAt\":\"$exp\"}
]")
out=$(decide "$sf")
verdict=$(echo "$out" | jq -r '.verdict_for_block')
if [ "$verdict" = "SCOPE-REJECTED" ]; then
    pass "SCOPE-REJECTED takes priority over BLOCKED-RECURRING-AUDIT-FAIL"
else
    fail_ "expected SCOPE-REJECTED, got $verdict"
fi

# === Test 12: legacy (no classification, no expiresAt) → PROCEED =============
# Pre-v8.22 entries with null classification and no expiresAt are inert noise.
# Adapter should not block on them.
header "Test 12: 5 legacy null-classification entries → PROCEED (defensive)"
sf=$(make_state "[
  {\"cycle\":1,\"summary\":\"old\"},
  {\"cycle\":2,\"summary\":\"older\"},
  {\"cycle\":3,\"summary\":\"oldest\"},
  {\"cycle\":4,\"summary\":\"ancient\"},
  {\"cycle\":5,\"summary\":\"prehistoric\"}
]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
if [ "$action" = "PROCEED" ]; then
    pass "legacy entries don't trigger any rule (PROCEED)"
else
    fail_ "expected PROCEED, got $action"
fi

# === Test 13: v8.27.0 — ship-gate-config does NOT trigger BLOCK-SYSTEMIC =====
# v8.27.0 introduces ship-gate-config classification (1d age-out, low severity)
# for cycles where audit declared PASS but ship-gate refused. It must NOT be
# counted toward infrastructure-systemic, even with multiple non-expired entries.
header "Test 13: v8.27.0 — 5 ship-gate-config entries → PROCEED (not BLOCK-SYSTEMIC)"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[
  {\"cycle\":10,\"classification\":\"ship-gate-config\",\"expiresAt\":\"$exp\"},
  {\"cycle\":11,\"classification\":\"ship-gate-config\",\"expiresAt\":\"$exp\"},
  {\"cycle\":12,\"classification\":\"ship-gate-config\",\"expiresAt\":\"$exp\"},
  {\"cycle\":13,\"classification\":\"ship-gate-config\",\"expiresAt\":\"$exp\"},
  {\"cycle\":14,\"classification\":\"ship-gate-config\",\"expiresAt\":\"$exp\"}
]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
if [ "$action" = "PROCEED" ]; then
    pass "ship-gate-config entries do not trigger BLOCK rules; loop continues"
else
    fail_ "expected PROCEED, got $action; full out: $out"
fi

# === Test 14: v8.27.0 — mix of ship-gate-config + 1 infra-systemic → BLOCK ===
# Sanity: the new classification doesn't accidentally suppress the existing
# BLOCK-SYSTEMIC rule when a real systemic entry is present.
header "Test 14: v8.27.0 — ship-gate-config doesn't mask infra-systemic block"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[
  {\"cycle\":10,\"classification\":\"ship-gate-config\",\"expiresAt\":\"$exp\"},
  {\"cycle\":11,\"classification\":\"infrastructure-systemic\",\"summary\":\"real systemic issue\",\"expiresAt\":\"$exp\"}
]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block // ""')
if [ "$action" = "BLOCK-OPERATOR-ACTION" ] && [ "$verdict" = "BLOCKED-SYSTEMIC" ]; then
    pass "real infra-systemic still blocks even when ship-gate-config also present"
else
    fail_ "expected BLOCK-OPERATOR-ACTION+BLOCKED-SYSTEMIC, got $action+$verdict"
fi

# === Summary =================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
