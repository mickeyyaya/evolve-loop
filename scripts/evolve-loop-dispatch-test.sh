#!/usr/bin/env bash
#
# evolve-loop-dispatch-test.sh — Unit tests for scripts/evolve-loop-dispatch.sh.
#
# Tests three concerns:
#   1. Argument parsing — cycles + strategy + goal positional ordering and defaults
#   2. VALIDATE_ONLY/--dry-run — exits 0 without invoking run-cycle.sh
#   3. Ledger pipeline verification — passes when scout+builder+auditor present;
#      FAILS LOUD with rc=2 when any role is missing for a cycle (the regression
#      case that the v8.13.7 strict-mode fix is designed to catch)
#
# Tests use RUN_CYCLE_OVERRIDE / LEDGER_OVERRIDE / STATE_OVERRIDE env hooks to
# inject mock run-cycle.sh + synthetic ledger.jsonl files. No real cycles
# spawned, no Claude API calls.
#
# Usage: bash scripts/evolve-loop-dispatch-test.sh

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DISPATCH="$REPO_ROOT/scripts/evolve-loop-dispatch.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

cleanup_files=()
cleanup_dirs=()
# bash 3.2 + set -u explodes on `"${empty_array[@]}"`; guard with `+`-form.
trap '
    for f in ${cleanup_files[@]+"${cleanup_files[@]}"}; do rm -f "$f"; done
    for d in ${cleanup_dirs[@]+"${cleanup_dirs[@]}"}; do rm -rf "$d"; done
' EXIT

# Make a fresh isolated workspace (state.json, ledger.jsonl, mock run-cycle.sh).
make_workspace() {
    local d
    d=$(mktemp -d -t dispatch-test.XXXXXX)
    cleanup_dirs+=("$d")
    echo "$d"
}

# Synthesize a ledger with N agent_subprocess entries for a given cycle.
# Args: <ledger_path> <cycle_id> <role1> [role2 ...]
write_ledger() {
    local ledger="$1"; shift
    local cycle="$1"; shift
    : > "$ledger"
    for role in "$@"; do
        printf '{"ts":"2026-04-29T03:00:00Z","kind":"agent_subprocess","role":"%s","cycle":%s,"exit_code":0}\n' \
            "$role" "$cycle" >> "$ledger"
    done
}

# Make a state.json with lastCycleNumber set.
write_state() {
    local state="$1" last="$2"
    printf '{"lastCycleNumber":%s,"version":1}\n' "$last" > "$state"
}

# Make a mock run-cycle.sh that:
#   - increments lastCycleNumber in $STATE_OVERRIDE by 1
#   - appends scout, builder, auditor entries to $LEDGER_OVERRIDE for the new cycle
#   - exits 0
# The mock honors a $MOCK_SKIP_ROLE env to omit one role (simulates orchestrator
# shortcut — used by the "verification catches missing role" tests).
make_mock_run_cycle() {
    local f
    f=$(mktemp -t mock-run-cycle.XXXXXX.sh)
    cleanup_files+=("$f")
    cat > "$f" <<'EOF'
#!/usr/bin/env bash
# Mock run-cycle.sh — increments state.json + appends ledger entries.
set -uo pipefail
state="$STATE_OVERRIDE"
ledger="$LEDGER_OVERRIDE"
last=$(jq -r '.lastCycleNumber // 0' "$state" 2>/dev/null || echo 0)
new=$((last + 1))
jq --argjson n "$new" '.lastCycleNumber = $n' "$state" > "$state.tmp" && mv "$state.tmp" "$state"
for role in scout builder auditor; do
    if [ "${MOCK_SKIP_ROLE:-}" = "$role" ]; then continue; fi
    printf '{"ts":"2026-04-29T03:00:00Z","kind":"agent_subprocess","role":"%s","cycle":%s,"exit_code":0}\n' \
        "$role" "$new" >> "$ledger"
done
echo "[mock-run-cycle] cycle=$new (skip=${MOCK_SKIP_ROLE:-none})" >&2
exit 0
EOF
    chmod +x "$f"
    echo "$f"
}

# === Test 1: --help prints usage block ========================================
header "Test 1: --help prints usage and exits 0"
set +e
out=$(bash "$DISPATCH" --help 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "Strict dispatcher" && echo "$out" | grep -q "USAGE"; then
    pass "--help works"
else
    fail_ "rc=$rc (want 0); out tail: $(echo "$out" | tail -3)"
fi

# === Test 2: defaults applied when no args ====================================
header "Test 2: no args → cycles=2 strategy=balanced goal=<autonomous>"
out=$(VALIDATE_ONLY=1 bash "$DISPATCH" 2>&1)
if echo "$out" | grep -q "cycles=2 strategy=balanced goal='<autonomous>'"; then
    pass "defaults correct"
else
    fail_ "out: $out"
fi

# === Test 3: parse cycles + strategy + goal ===================================
header "Test 3: '5 harden polish the layout' → cycles=5 strategy=harden goal='polish the layout'"
out=$(VALIDATE_ONLY=1 bash "$DISPATCH" 5 harden polish the layout 2>&1)
if echo "$out" | grep -q "cycles=5 strategy=harden goal='polish the layout'"; then
    pass "all three parsed"
else
    fail_ "out: $out"
fi

# === Test 4: cycles only, strategy default ====================================
header "Test 4: '7' → cycles=7 strategy=balanced goal=<autonomous>"
out=$(VALIDATE_ONLY=1 bash "$DISPATCH" 7 2>&1)
if echo "$out" | grep -q "cycles=7 strategy=balanced goal='<autonomous>'"; then
    pass "cycles-only parsed; strategy/goal defaulted"
else
    fail_ "out: $out"
fi

# === Test 5: goal without cycles (first token non-numeric) ====================
header "Test 5: 'fix the typo' → cycles=2 strategy=balanced goal='fix the typo'"
# 'fix' is not a strategy keyword, so the whole string becomes the goal.
out=$(VALIDATE_ONLY=1 bash "$DISPATCH" fix the typo 2>&1)
if echo "$out" | grep -q "cycles=2 strategy=balanced goal='fix the typo'"; then
    pass "goal-only parsed; cycles/strategy defaulted"
else
    fail_ "out: $out"
fi

# === Test 6: cycles + goal (no strategy keyword) ==============================
header "Test 6: '3 implement feature X' → cycles=3 strategy=balanced goal='implement feature X'"
out=$(VALIDATE_ONLY=1 bash "$DISPATCH" 3 implement feature X 2>&1)
if echo "$out" | grep -q "cycles=3 strategy=balanced goal='implement feature X'"; then
    pass "cycles+goal parsed; strategy defaulted (not consumed by 'implement')"
else
    fail_ "out: $out"
fi

# === Test 7: BAD-ARG when cycles=0 ============================================
header "Test 7: cycles=0 → BAD-ARG rc=10"
set +e
out=$(bash "$DISPATCH" 0 2>&1)
rc=$?
set -e
if [ "$rc" = "10" ] && echo "$out" | grep -q "BAD-ARG: CYCLES must be >= 1"; then
    pass "cycles=0 rejected with rc=10"
else
    fail_ "rc=$rc (want 10); out: $out"
fi

# === Test 8: BAD-ARG on unknown --flag ========================================
header "Test 8: --bogus → BAD-ARG rc=10"
set +e
out=$(bash "$DISPATCH" --bogus 2>&1)
rc=$?
set -e
if [ "$rc" = "10" ] && echo "$out" | grep -q "BAD-ARG: unknown flag: --bogus"; then
    pass "unknown flag rejected with rc=10"
else
    fail_ "rc=$rc (want 10); out: $out"
fi

# === Test 9: --dry-run exits 0 without running ================================
header "Test 9: --dry-run exits 0 + skips run-cycle invocation"
out=$(bash "$DISPATCH" --dry-run 2 2>&1)
if echo "$out" | grep -q "VALIDATE_ONLY/DRY_RUN — not invoking run-cycle.sh"; then
    pass "dry-run skips invocation"
else
    fail_ "out: $out"
fi

# === Test 10: VALIDATE_ONLY skips ledger verification too =====================
# The plan output should still mention verify=on (verification setting),
# but no run-cycle invocation = no verification ever happens.
header "Test 10: VALIDATE_ONLY does not require run-cycle.sh to exist"
# Even with a non-existent override, validate-only should succeed.
out=$(VALIDATE_ONLY=1 RUN_CYCLE_OVERRIDE=/nonexistent/run-cycle.sh bash "$DISPATCH" 1 2>&1)
rc=$?
if [ "$rc" = "0" ]; then
    pass "validate-only doesn't touch run-cycle"
else
    fail_ "rc=$rc; out: $out"
fi

# === Test 11: end-to-end with mock run-cycle.sh — happy path ==================
header "Test 11: mock run-cycle.sh writes 3 roles per cycle → dispatcher rc=0"
ws=$(make_workspace)
state="$ws/state.json"
ledger="$ws/ledger.jsonl"
write_state "$state" 100
: > "$ledger"
mock=$(make_mock_run_cycle)
set +e
out=$(STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" RUN_CYCLE_OVERRIDE="$mock" \
      bash "$DISPATCH" 2 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "all 2 cycles completed AND verified"; then
    pass "happy path: 2 cycles each with scout+builder+auditor"
else
    fail_ "rc=$rc; out tail: $(echo "$out" | tail -8)"
fi

# === Test 12: end-to-end — orchestrator skips Builder ⇒ verification fails ===
# This is the central test: the regression case that strict-mode catches.
# Mock run-cycle.sh runs but $MOCK_SKIP_ROLE=builder makes it omit the
# builder ledger entry. Dispatcher must detect this and exit rc=2.
header "Test 12: missing builder entry → rc=2 + 'orchestrator shortcut detected'"
ws=$(make_workspace)
state="$ws/state.json"
ledger="$ws/ledger.jsonl"
write_state "$state" 100
: > "$ledger"
mock=$(make_mock_run_cycle)
set +e
out=$(MOCK_SKIP_ROLE=builder STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" \
      RUN_CYCLE_OVERRIDE="$mock" bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ] && echo "$out" | grep -q "orchestrator shortcut detected"; then
    pass "missing builder caught with rc=2 and clear diagnostic"
else
    fail_ "rc=$rc; out: $out"
fi

# === Test 13: missing scout entry caught ======================================
header "Test 13: missing scout entry → rc=2"
ws=$(make_workspace)
state="$ws/state.json"
ledger="$ws/ledger.jsonl"
write_state "$state" 100
: > "$ledger"
mock=$(make_mock_run_cycle)
set +e
out=$(MOCK_SKIP_ROLE=scout STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" \
      RUN_CYCLE_OVERRIDE="$mock" bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ] && echo "$out" | grep -q "scout=0"; then
    pass "missing scout caught with rc=2"
else
    fail_ "rc=$rc; out: $out"
fi

# === Test 14: missing auditor entry caught ====================================
header "Test 14: missing auditor entry → rc=2"
ws=$(make_workspace)
state="$ws/state.json"
ledger="$ws/ledger.jsonl"
write_state "$state" 100
: > "$ledger"
mock=$(make_mock_run_cycle)
set +e
out=$(MOCK_SKIP_ROLE=auditor STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" \
      RUN_CYCLE_OVERRIDE="$mock" bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ] && echo "$out" | grep -q "auditor=0"; then
    pass "missing auditor caught with rc=2"
else
    fail_ "rc=$rc; out: $out"
fi

# === Test 15: EVOLVE_DISPATCH_VERIFY=0 lets bad cycles through (legacy) ======
# The escape hatch should still WARN loudly, but not block — provided it's
# explicitly opted into. This documents the legacy behavior so a future
# accidental removal of the env-check is caught.
header "Test 15: EVOLVE_DISPATCH_VERIFY=0 + missing builder → rc=0 + WARN"
ws=$(make_workspace)
state="$ws/state.json"
ledger="$ws/ledger.jsonl"
write_state "$state" 100
: > "$ledger"
mock=$(make_mock_run_cycle)
set +e
out=$(EVOLVE_DISPATCH_VERIFY=0 MOCK_SKIP_ROLE=builder \
      STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" \
      RUN_CYCLE_OVERRIDE="$mock" bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] \
   && echo "$out" | grep -q "EVOLVE_DISPATCH_VERIFY=0" \
   && echo "$out" | grep -q "skipping ledger pipeline check"; then
    pass "verify=0 escape hatch warns + allows (legacy compat)"
else
    fail_ "rc=$rc; out tail: $(echo "$out" | tail -5)"
fi

# === Test 16: missing run-cycle.sh → rc=1 (FAIL, not BAD-ARG) =================
# rc=1 (runtime fail) is correct — args were valid; the prerequisite is missing.
header "Test 16: nonexistent RUN_CYCLE_OVERRIDE → rc=1"
ws=$(make_workspace)
state="$ws/state.json"
ledger="$ws/ledger.jsonl"
write_state "$state" 0
set +e
out=$(STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" \
      RUN_CYCLE_OVERRIDE=/nonexistent/run-cycle.sh \
      bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "1" ] && echo "$out" | grep -q "FAIL: missing run-cycle.sh"; then
    pass "missing run-cycle.sh → rc=1 with clear diagnostic"
else
    fail_ "rc=$rc; out: $out"
fi

# === Test 17: run-cycle.sh failing rc=N propagates as rc=1 (batch abort) =====
header "Test 17: run-cycle.sh failure aborts batch with rc=1"
ws=$(make_workspace)
state="$ws/state.json"
ledger="$ws/ledger.jsonl"
write_state "$state" 100
: > "$ledger"
# Mock that always fails.
fail_mock=$(mktemp -t mock-fail.XXXXXX.sh)
cleanup_files+=("$fail_mock")
echo '#!/usr/bin/env bash
echo "[mock-fail] simulated failure" >&2
exit 7' > "$fail_mock"
chmod +x "$fail_mock"
set +e
out=$(STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" \
      RUN_CYCLE_OVERRIDE="$fail_mock" \
      bash "$DISPATCH" 3 2>&1)
rc=$?
set -e
if [ "$rc" = "1" ] && echo "$out" | grep -q "run-cycle.sh cycle 1 exited rc=7"; then
    pass "first-cycle failure aborts batch with dispatch rc=1"
else
    fail_ "rc=$rc; out: $out"
fi

# === Test 18: lastCycleNumber stalled (e.g., audit FAIL) — still verifies =====
# When orchestrator audits FAIL, lastCycleNumber may not advance. The
# dispatcher uses last_before+1 as the cycle to verify. If that synthetic
# cycle has all 3 roles, it still passes (the cycle ran the pipeline
# completely; only the audit verdict was non-PASS — which is acceptable).
header "Test 18: lastCycleNumber stalled but ledger complete → rc=0"
ws=$(make_workspace)
state="$ws/state.json"
ledger="$ws/ledger.jsonl"
write_state "$state" 200
# Mock that DOES NOT increment state but DOES write ledger entries for cycle 201.
stall_mock=$(mktemp -t mock-stall.XXXXXX.sh)
cleanup_files+=("$stall_mock")
cat > "$stall_mock" <<'EOF'
#!/usr/bin/env bash
ledger="$LEDGER_OVERRIDE"
for role in scout builder auditor; do
    printf '{"ts":"2026-04-29T03:00:00Z","kind":"agent_subprocess","role":"%s","cycle":201,"exit_code":0}\n' \
        "$role" >> "$ledger"
done
echo "[mock-stall] ran cycle 201 but did not advance state (audit FAIL simulation)" >&2
exit 0
EOF
chmod +x "$stall_mock"
set +e
out=$(STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" \
      RUN_CYCLE_OVERRIDE="$stall_mock" \
      bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "lastCycleNumber did not advance"; then
    pass "stalled-state cycle still verified via last_before+1"
else
    fail_ "rc=$rc; out: $out"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
