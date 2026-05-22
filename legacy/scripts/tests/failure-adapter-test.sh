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

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/failure/failure-adapter.sh"

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

# v8.28.0: BLOCK semantics moved to opt-in via EVOLVE_STRICT_FAILURES=1.
# Legacy tests that expected BLOCK-* verdicts now use decide_strict, which
# sets the env var so the adapter restores pre-v8.28.0 blocking. New tests
# that assert v8.28.0 fluent semantics (PROCEED with awareness) use decide.
decide()        { bash "$SCRIPT" decide --state "$1"; }
decide_strict() { EVOLVE_STRICT_FAILURES=1 bash "$SCRIPT" decide --state "$1"; }

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

# === Test 3: 1 infra-transient + STRICT → RETRY-WITH-FALLBACK ================
# In v8.28.0 fluent default this becomes PROCEED+set_env; Test 17 covers that.
# Strict mode preserves the legacy RETRY-WITH-FALLBACK action verbatim.
header "Test 3: 1 infra-transient + STRICT → RETRY-WITH-FALLBACK + set_env"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[{\"cycle\":10,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"}]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
flag=$(echo "$out" | jq -r '.set_env.EVOLVE_SANDBOX_FALLBACK_ON_EPERM // "(unset)"')
if [ "$action" = "RETRY-WITH-FALLBACK" ] && [ "$flag" = "1" ]; then
    pass "RETRY-WITH-FALLBACK + EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1"
else
    fail_ "action=$action flag=$flag"
fi

# === Test 4: 3 consecutive infra-transient → BLOCK-OPERATOR-ACTION (strict) ===
header "Test 4: 3 consecutive infra-transient (tail streak) + STRICT → BLOCK-OPERATOR-ACTION"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"},
  {\"cycle\":3,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"}
]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block // "null"')
streak=$(echo "$out" | jq -r '.evidence.consecutive_infra_transient_streak')
if [ "$action" = "BLOCK-OPERATOR-ACTION" ] && [ "$verdict" = "BLOCKED-SYSTEMIC" ] && [ "$streak" = "3" ]; then
    pass "STRICT: BLOCK-OPERATOR-ACTION (BLOCKED-SYSTEMIC), tail-streak=3"
else
    fail_ "action=$action verdict=$verdict streak=$streak"
fi

# === Test 5: streak interrupted + STRICT → only RETRY-WITH-FALLBACK ==========
header "Test 5: 2 infra-transient + 1 code-audit-fail + 1 infra-transient + STRICT → RETRY (streak=1)"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"},
  {\"cycle\":3,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":4,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"}
]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
streak=$(echo "$out" | jq -r '.evidence.consecutive_infra_transient_streak')
# 1 code-audit-fail → does NOT trigger 2+ rule. 1 infra-transient at tail → RETRY.
if [ "$action" = "RETRY-WITH-FALLBACK" ] && [ "$streak" = "1" ]; then
    pass "RETRY-WITH-FALLBACK, code break interrupted streak (tail=1)"
else
    fail_ "action=$action tail-streak=$streak"
fi

# === Test 6: 2 code-audit-fail → BLOCK-CODE (strict) =========================
header "Test 6: 2 code-audit-fail + STRICT → BLOCK-CODE + BLOCKED-RECURRING-AUDIT-FAIL"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"}
]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block')
if [ "$action" = "BLOCK-CODE" ] && [ "$verdict" = "BLOCKED-RECURRING-AUDIT-FAIL" ]; then
    pass "STRICT: BLOCK-CODE + BLOCKED-RECURRING-AUDIT-FAIL"
else
    fail_ "action=$action verdict=$verdict"
fi

# === Test 7: 2 code-build-fail → BLOCK-CODE (strict) =========================
header "Test 7: 2 code-build-fail + STRICT → BLOCK-CODE + BLOCKED-RECURRING-BUILD-FAIL"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-build-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-build-fail\",\"expiresAt\":\"$exp\"}
]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block')
if [ "$action" = "BLOCK-CODE" ] && [ "$verdict" = "BLOCKED-RECURRING-BUILD-FAIL" ]; then
    pass "STRICT: BLOCK-CODE + BLOCKED-RECURRING-BUILD-FAIL"
else
    fail_ "action=$action verdict=$verdict"
fi

# === Test 8: 1 intent-rejected → BLOCK-CODE (strict) =========================
header "Test 8: 1 intent-rejected + STRICT → BLOCK-CODE + SCOPE-REJECTED"
exp=$(FUTURE_ISO 365)  # intent-rejected has very long expiry
sf=$(make_state "[{\"cycle\":1,\"classification\":\"intent-rejected\",\"expiresAt\":\"$exp\"}]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block')
if [ "$action" = "BLOCK-CODE" ] && [ "$verdict" = "SCOPE-REJECTED" ]; then
    pass "STRICT: BLOCK-CODE + SCOPE-REJECTED"
else
    fail_ "action=$action verdict=$verdict"
fi

# === Test 9: 1 infrastructure-systemic → BLOCK-OPERATOR-ACTION (strict) ======
header "Test 9: 1 infra-systemic + STRICT → BLOCK-OPERATOR-ACTION"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[{\"cycle\":1,\"classification\":\"infrastructure-systemic\",\"expiresAt\":\"$exp\",\"summary\":\"claude-cli not installed\"}]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block')
if [ "$action" = "BLOCK-OPERATOR-ACTION" ] && [ "$verdict" = "BLOCKED-SYSTEMIC" ]; then
    pass "STRICT: BLOCK-OPERATOR-ACTION + BLOCKED-SYSTEMIC"
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
header "Test 11: STRICT — intent-rejected + 2 code-audit-fail → SCOPE-REJECTED (priority)"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":3,\"classification\":\"intent-rejected\",\"expiresAt\":\"$exp\"}
]")
out=$(decide_strict "$sf")
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

# === Test 14: v8.27.0 — ship-gate-config + infra-systemic in STRICT → BLOCK ==
# Sanity: the new classification doesn't accidentally suppress the existing
# BLOCK-SYSTEMIC rule when a real systemic entry is present (strict mode).
header "Test 14: STRICT — ship-gate-config doesn't mask infra-systemic block"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[
  {\"cycle\":10,\"classification\":\"ship-gate-config\",\"expiresAt\":\"$exp\"},
  {\"cycle\":11,\"classification\":\"infrastructure-systemic\",\"summary\":\"real systemic issue\",\"expiresAt\":\"$exp\"}
]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block // ""')
if [ "$action" = "BLOCK-OPERATOR-ACTION" ] && [ "$verdict" = "BLOCKED-SYSTEMIC" ]; then
    pass "STRICT: real infra-systemic still blocks even when ship-gate-config also present"
else
    fail_ "expected BLOCK-OPERATOR-ACTION+BLOCKED-SYSTEMIC, got $action+$verdict"
fi

# === Test 15: v8.28.0 — fluent default: same scenario as Test 9 → PROCEED ====
# Pre-v8.28.0: 1+ infra-systemic = immediate BLOCK-OPERATOR-ACTION.
# v8.28.0: fluent by default — adapter emits PROCEED with awareness.
header "Test 15: v8.28.0 — 1 infra-systemic in FLUENT default → PROCEED with awareness"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[{\"cycle\":1,\"classification\":\"infrastructure-systemic\",\"expiresAt\":\"$exp\",\"summary\":\"the kind of failure that used to deadlock\"}]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
reason=$(echo "$out" | jq -r '.reason // ""')
if [ "$action" = "PROCEED" ] && echo "$reason" | grep -q "would-have-blocked"; then
    pass "FLUENT default: PROCEED with would-have-blocked awareness in reason"
else
    fail_ "expected PROCEED with awareness, got action=$action reason=$reason"
fi

# === Test 16: v8.28.0 — fluent default: 2 code-audit-fail → PROCEED ==========
header "Test 16: v8.28.0 — 2 code-audit-fail in FLUENT default → PROCEED with awareness"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"}
]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
if [ "$action" = "PROCEED" ]; then
    pass "FLUENT default: 2 code-audit-fail → PROCEED (loop continues, orchestrator gets awareness)"
else
    fail_ "expected PROCEED, got $action"
fi

# === Test 17: v8.28.0 — fluent emits set_env from infra-transient awareness ==
# When infra-transient awareness is accumulated in fluent mode, the
# EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 env hint should still be emitted so
# the next subagent attempt benefits from the fallback even though we
# didn't explicitly RETRY-WITH-FALLBACK as a verdict.
header "Test 17: v8.28.0 — fluent default: infra-transient awareness still sets EPERM fallback env"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[{\"cycle\":1,\"classification\":\"infrastructure-transient\",\"expiresAt\":\"$exp\"}]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
flag=$(echo "$out" | jq -r '.set_env.EVOLVE_SANDBOX_FALLBACK_ON_EPERM // ""')
if [ "$action" = "PROCEED" ] && [ "$flag" = "1" ]; then
    pass "FLUENT default: infra-transient → PROCEED + EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 in set_env"
else
    fail_ "action=$action set_env.fallback=$flag"
fi

# === Test 18: v8.35.0 — code-audit-warn classification works ===============
# code-audit-warn is added in v8.35.0 as the right home for WARN audits
# (previously misclassified as code-audit-fail). Severity=low, age-out=1d.
# The adapter should treat it like other low-severity entries: no BLOCK.
header "Test 18: v8.35.0 — 5 code-audit-warn entries → PROCEED (low severity)"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-warn\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-warn\",\"expiresAt\":\"$exp\"},
  {\"cycle\":3,\"classification\":\"code-audit-warn\",\"expiresAt\":\"$exp\"},
  {\"cycle\":4,\"classification\":\"code-audit-warn\",\"expiresAt\":\"$exp\"},
  {\"cycle\":5,\"classification\":\"code-audit-warn\",\"expiresAt\":\"$exp\"}
]")
out=$(decide "$sf")
action=$(echo "$out" | jq -r '.action')
if [ "$action" = "PROCEED" ]; then
    pass "5 code-audit-warn → PROCEED (no BLOCK; warn is low-severity awareness only)"
else
    fail_ "expected PROCEED, got $action"
fi

# === Test 19: v8.35.0 — STRICT mode also doesn't block on code-audit-warn ====
# Even in strict mode, code-audit-warn should not trigger a code-block.
# Only code-audit-fail (FAIL verdicts) trigger that rule.
header "Test 19: v8.35.0 — STRICT — 3 code-audit-warn → PROCEED (still not a FAIL block)"
exp=$(FUTURE_ISO 1)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-warn\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-warn\",\"expiresAt\":\"$exp\"},
  {\"cycle\":3,\"classification\":\"code-audit-warn\",\"expiresAt\":\"$exp\"}
]")
out=$(EVOLVE_STRICT_FAILURES=1 decide "$sf")
action=$(echo "$out" | jq -r '.action')
if [ "$action" = "PROCEED" ]; then
    pass "STRICT + 3 code-audit-warn → PROCEED (warn ≠ fail)"
else
    fail_ "expected PROCEED, got $action; reason: $(echo "$out" | jq -r '.reason')"
fi

# === Test 20: v8.35.0 — failure-classifications metadata for code-audit-warn ==
# Direct assertion against the helper functions (sourced).
header "Test 20: v8.35.0 — failure-classifications.sh metadata: code-audit-warn"
# Source classifications under a fresh shell (the file uses an idempotency guard).
# Use a subshell to avoid polluting parent env.
result=$(EVOLVE_FAILURE_CLASSIFICATIONS_LOADED=0 bash -c '
    . "'"$REPO_ROOT"'/scripts/failure/failure-classifications.sh"
    echo "age=$(failure_age_out_seconds code-audit-warn)"
    echo "sev=$(failure_severity_of code-audit-warn)"
    echo "ret=$(failure_retry_policy code-audit-warn)"
    echo "norm_warn=$(failure_normalize_legacy WARN)"
    echo "norm_fail=$(failure_normalize_legacy FAIL)"
')
age=$(echo "$result" | grep '^age=' | cut -d= -f2)
sev=$(echo "$result" | grep '^sev=' | cut -d= -f2)
ret=$(echo "$result" | grep '^ret=' | cut -d= -f2)
norm_warn=$(echo "$result" | grep '^norm_warn=' | cut -d= -f2)
norm_fail=$(echo "$result" | grep '^norm_fail=' | cut -d= -f2)
if [ "$age" = "86400" ] && [ "$sev" = "low" ] && [ "$ret" = "yes" ] \
   && [ "$norm_warn" = "code-audit-warn" ] && [ "$norm_fail" = "code-audit-fail" ]; then
    pass "metadata: age=86400, severity=low, retry=yes; WARN→code-audit-warn, FAIL→code-audit-fail"
else
    fail_ "got age=$age sev=$sev ret=$ret norm_warn=$norm_warn norm_fail=$norm_fail"
fi

# === Test 21: same-cycle duplicates → NO false BLOCK (strict) ================
# Regression test for the retry-storm false-positive. Two entries from the same
# cycle should count as 1 distinct cycle, which is below the threshold of 2.
header "Test 21: same-cycle duplicates → no false BLOCK (strict)"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":5,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":5,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"}
]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
by_class_count=$(echo "$out" | jq -r '.evidence.by_class["code-audit-fail"] // 0')
# Fix: 1 distinct cycle < threshold 2 → PROCEED; forensic raw count still shows 2.
if [ "$action" = "PROCEED" ] && [ "$by_class_count" = "2" ]; then
    pass "STRICT: same-cycle dupes → PROCEED (1 distinct cycle); forensic by_class still shows 2"
else
    fail_ "action=$action (expected PROCEED), by_class[code-audit-fail]=$by_class_count (expected 2)"
fi

# === Test 22: distinct cycles → correct BLOCK (strict) =======================
# Ensure the threshold still fires when cycles ARE genuinely distinct.
header "Test 22: distinct cycles → correct BLOCK (strict)"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"}
]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block')
if [ "$action" = "BLOCK-CODE" ] && [ "$verdict" = "BLOCKED-RECURRING-AUDIT-FAIL" ]; then
    pass "STRICT: 2 distinct cycles → BLOCK-CODE + BLOCKED-RECURRING-AUDIT-FAIL"
else
    fail_ "action=$action verdict=$verdict"
fi

# === Test 23: mixed partial dedup → correct count (strict) ===================
# {cycle:5, cycle:5, cycle:6} reduces to 2 distinct cycles → still blocks.
header "Test 23: mixed partial dedup — 2 distinct cycles → BLOCK (strict)"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":5,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":5,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":6,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"}
]")
out=$(decide_strict "$sf")
action=$(echo "$out" | jq -r '.action')
verdict=$(echo "$out" | jq -r '.verdict_for_block')
by_class_count=$(echo "$out" | jq -r '.evidence.by_class["code-audit-fail"] // 0')
# {5,5,6} → distinct={5,6} = 2 → threshold met → BLOCK; raw count=3
if [ "$action" = "BLOCK-CODE" ] && [ "$verdict" = "BLOCKED-RECURRING-AUDIT-FAIL" ] && [ "$by_class_count" = "3" ]; then
    pass "STRICT: {5,5,6} → 2 distinct → BLOCK-CODE; forensic raw count=3"
else
    fail_ "action=$action verdict=$verdict by_class=$by_class_count"
fi

# === Test 24: v8.60+ bridge — EVOLVE_STRICT_FAILURES=1 bridges to STRICT_AUDIT ===
# When only STRICT_FAILURES is set (no STRICT_AUDIT), the bridge emits a WARN
# and sets STRICT_AUDIT=1 so the adapter blocks as if STRICT_AUDIT=1.
header "Test 24: v8.60+ bridge — EVOLVE_STRICT_FAILURES=1 only → blocks + emits WARN"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"}
]")
out=$(EVOLVE_STRICT_FAILURES=1 _STRICT_FAILURES_WARNED="" decide "$sf" 2>&1)
stderr_part=$(EVOLVE_STRICT_FAILURES=1 _STRICT_FAILURES_WARNED="" bash "$SCRIPT" decide --state "$sf" 2>&1 >/dev/null || true)
action=$(echo "$out" | grep -v '^WARN:' | jq -r '.action' 2>/dev/null || echo "")
if echo "$stderr_part" | grep -q "EVOLVE_STRICT_FAILURES is deprecated"; then
    pass "bridge emits deprecation WARN"
else
    fail_ "bridge WARN not emitted; stderr: $stderr_part"
fi
out2=$(EVOLVE_STRICT_FAILURES=1 _STRICT_FAILURES_WARNED="" decide "$sf")
action2=$(echo "$out2" | jq -r '.action')
if [ "$action2" = "BLOCK-CODE" ]; then
    pass "STRICT_FAILURES=1 bridge → BLOCK-CODE (same as STRICT_AUDIT=1)"
else
    fail_ "expected BLOCK-CODE via bridge, got $action2"
fi

# === Test 25: v8.60+ bridge — EVOLVE_STRICT_AUDIT=1 only (canonical, no WARN) ===
header "Test 25: v8.60+ canonical — EVOLVE_STRICT_AUDIT=1 → blocks without WARN"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"}
]")
stderr_canonical=$(EVOLVE_STRICT_AUDIT=1 bash "$SCRIPT" decide --state "$sf" 2>&1 >/dev/null || true)
out=$(EVOLVE_STRICT_AUDIT=1 decide "$sf")
action=$(echo "$out" | jq -r '.action')
if [ "$action" = "BLOCK-CODE" ]; then
    pass "STRICT_AUDIT=1 canonical → BLOCK-CODE"
else
    fail_ "expected BLOCK-CODE, got $action"
fi
if ! echo "$stderr_canonical" | grep -q "deprecated"; then
    pass "canonical STRICT_AUDIT=1 emits no deprecation WARN"
else
    fail_ "unexpected deprecation WARN from canonical flag; stderr: $stderr_canonical"
fi

# === Test 26: v8.60+ bridge — both STRICT_FAILURES=1 + STRICT_AUDIT=1 → STRICT_AUDIT wins ===
header "Test 26: v8.60+ bridge — both STRICT_FAILURES=1 + STRICT_AUDIT=1 → STRICT_AUDIT takes precedence"
exp=$(FUTURE_ISO 5)
sf=$(make_state "[
  {\"cycle\":1,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"},
  {\"cycle\":2,\"classification\":\"code-audit-fail\",\"expiresAt\":\"$exp\"}
]")
stderr_both=$(EVOLVE_STRICT_FAILURES=1 EVOLVE_STRICT_AUDIT=1 _STRICT_FAILURES_WARNED="" bash "$SCRIPT" decide --state "$sf" 2>&1 >/dev/null || true)
out=$(EVOLVE_STRICT_FAILURES=1 EVOLVE_STRICT_AUDIT=1 _STRICT_FAILURES_WARNED="" decide "$sf")
action=$(echo "$out" | jq -r '.action')
if echo "$stderr_both" | grep -q "STRICT_AUDIT takes precedence"; then
    pass "both set → STRICT_AUDIT-takes-precedence WARN emitted"
else
    fail_ "missing precedence WARN; stderr: $stderr_both"
fi
if [ "$action" = "BLOCK-CODE" ]; then
    pass "both set → BLOCK-CODE (STRICT_AUDIT wins)"
else
    fail_ "expected BLOCK-CODE, got $action"
fi

# === Summary =================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
