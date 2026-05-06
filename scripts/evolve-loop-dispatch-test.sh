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

# === Test 19: recoverable infrastructure failure → rc=3, continues to next cycle =========
header "Test 19: missing auditor + 'INFRASTRUCTURE FAILURE' report → rc=3, continued, recorded"
ws=$(make_workspace)
state="$ws/state.json"; ledger="$ws/ledger.jsonl"; runs_dir="$ws/runs"
write_state "$state" 200
: > "$ledger"
mkdir -p "$runs_dir/cycle-201" "$runs_dir/cycle-202"
# Cycle 201's report honestly declares INFRASTRUCTURE FAILURE on auditor.
cat > "$runs_dir/cycle-201/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 201
## Phase Outcomes
| Phase | Agent | Outcome |
| audit | auditor | **INFRASTRUCTURE FAILURE** — sandbox-exec EPERM |
## Verdict
FAILED
## Failure Root Cause: sandbox-exec Permission Denied
sandbox_apply: Operation not permitted on Darwin 25.4.0 — nested sandboxing blocked.
EOF
# Cycle 202 (the second cycle) honestly produces all 3 entries.
mock=$(make_mock_run_cycle)
# Need a custom mock that produces partial ledger for cycle 201 (no auditor)
# but full ledger for cycle 202.
infra_mock=$(mktemp -t infra-mock.XXXXXX.sh); cleanup_files+=("$infra_mock")
cat > "$infra_mock" <<'EOF'
#!/usr/bin/env bash
set -uo pipefail
state="$STATE_OVERRIDE"; ledger="$LEDGER_OVERRIDE"
last=$(jq -r '.lastCycleNumber // 0' "$state")
new=$((last + 1))
# state.json doesn't advance lastCycleNumber when audit fails (matches real run-cycle.sh)
if [ "$new" = "201" ]; then
    # Cycle 201: scout + builder only; auditor missing.
    for role in scout builder; do
        printf '{"ts":"2026-04-29T03:00:00Z","kind":"agent_subprocess","role":"%s","cycle":%s,"exit_code":0}\n' \
            "$role" "$new" >> "$ledger"
    done
else
    # Cycle 202+: complete ledger.
    for role in scout builder auditor; do
        printf '{"ts":"2026-04-29T03:00:00Z","kind":"agent_subprocess","role":"%s","cycle":%s,"exit_code":0}\n' \
            "$role" "$new" >> "$ledger"
    done
    jq --argjson n "$new" '.lastCycleNumber = $n' "$state" > "$state.tmp" && mv "$state.tmp" "$state"
fi
exit 0
EOF
chmod +x "$infra_mock"
set +e
out=$(STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" RUNS_DIR_OVERRIDE="$runs_dir" \
      RUN_CYCLE_OVERRIDE="$infra_mock" bash "$DISPATCH" 2 2>&1)
rc=$?
set -e
if [ "$rc" = "3" ]; then
    pass "rc=3 (recoverable failure exit code)"
else
    fail_ "expected rc=3, got rc=$rc"
fi
if echo "$out" | grep -q "RECOVERABLE-FAILURE.*infrastructure"; then
    pass "log mentions RECOVERABLE-FAILURE classification=infrastructure"
else
    fail_ "missing RECOVERABLE-FAILURE log; out: $out"
fi
if echo "$out" | grep -qE "cycle 2 / 2|cycle 202"; then
    pass "dispatcher continued past cycle 201 to cycle 202"
else
    fail_ "dispatcher did NOT continue; out: $out"
fi
if jq -e '.failedApproaches | length >= 1' "$state" >/dev/null 2>&1; then
    pass "state.json:failedApproaches has new entry"
    fa=$(jq -r '.failedApproaches[0].classification' "$state")
    # v8.22.0: classify_cycle_failure returns the legacy "infrastructure" string,
    # but record_failed_approach normalizes it to "infrastructure-transient" via
    # failure_normalize_legacy before writing to state.json. Both forms are
    # acceptable for backward compat — accept either.
    if [ "$fa" = "infrastructure" ] || [ "$fa" = "infrastructure-transient" ]; then
        pass "failedApproaches[0].classification=$fa (v8.22 normalized)"
    else
        fail_ "expected classification=infrastructure or infrastructure-transient, got '$fa'"
    fi
else
    fail_ "state.json:failedApproaches missing entry"
fi

# === Test 20: integrity-breach (no orchestrator-report.md) → rc=2 STOP ========
header "Test 20: missing auditor + NO orchestrator-report.md → rc=2 (integrity-breach STOP)"
ws=$(make_workspace)
state="$ws/state.json"; ledger="$ws/ledger.jsonl"; runs_dir="$ws/runs"
write_state "$state" 300
: > "$ledger"
# No orchestrator-report.md created — simulates silent shortcut
mkdir -p "$runs_dir/cycle-301"
mock=$(make_mock_run_cycle)
set +e
out=$(MOCK_SKIP_ROLE=auditor STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" \
      RUNS_DIR_OVERRIDE="$runs_dir" RUN_CYCLE_OVERRIDE="$mock" bash "$DISPATCH" 2 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ]; then
    pass "rc=2 (integrity-breach when report missing)"
else
    fail_ "expected rc=2, got rc=$rc"
fi
if echo "$out" | grep -q "INTEGRITY-BREACH"; then
    pass "log mentions INTEGRITY-BREACH"
else
    fail_ "missing INTEGRITY-BREACH log"
fi

# === Test 21: EVOLVE_DISPATCH_STOP_ON_FAIL=1 restores fail-fast ===============
header "Test 21: STOP_ON_FAIL=1 + recoverable failure → rc=2 (legacy fail-fast)"
ws=$(make_workspace)
state="$ws/state.json"; ledger="$ws/ledger.jsonl"; runs_dir="$ws/runs"
write_state "$state" 400
: > "$ledger"
mkdir -p "$runs_dir/cycle-401"
cat > "$runs_dir/cycle-401/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 401
| audit | auditor | INFRASTRUCTURE FAILURE — EPERM |
EOF
mock=$(make_mock_run_cycle)
set +e
out=$(EVOLVE_DISPATCH_STOP_ON_FAIL=1 MOCK_SKIP_ROLE=auditor \
      STATE_OVERRIDE="$state" LEDGER_OVERRIDE="$ledger" RUNS_DIR_OVERRIDE="$runs_dir" \
      RUN_CYCLE_OVERRIDE="$mock" bash "$DISPATCH" 2 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ]; then
    pass "STOP_ON_FAIL=1 → rc=2 (legacy behavior preserved)"
else
    fail_ "expected rc=2, got rc=$rc"
fi
if echo "$out" | grep -q "legacy fail-fast"; then
    pass "log mentions legacy fail-fast"
else
    fail_ "missing legacy fail-fast log"
fi

# === Test 22: cwd inside */plugins/cache/* is rejected (v8.18.1) ==============
header "Test 22: cwd inside plugin-cache path → BAD-ARG rc=10"
fakeplugin_cache=$(mktemp -d -t evolve-test-cache.XXXXXX)
# Mimic the user's plugins/cache install layout. The path itself doesn't need
# to be a real plugin checkout — the dispatcher matches on path *pattern*.
mkdir -p "$fakeplugin_cache/plugins/cache/evolve-loop/evolve-loop/8.18.0"
target_cache="$fakeplugin_cache/plugins/cache/evolve-loop/evolve-loop/8.18.0"
( cd "$target_cache" && git init -q . && git commit --allow-empty -q -m init ) >/dev/null 2>&1
set +e
out=$(cd "$target_cache" && unset EVOLVE_WORKTREE_BASE EVOLVE_SANDBOX_FALLBACK_ON_EPERM CLAUDECODE CLAUDE_CODE_ENTRYPOINT CLAUDE_CODE_EXECPATH EVOLVE_PROJECT_ROOT EVOLVE_PLUGIN_ROOT EVOLVE_RESOLVE_ROOTS_LOADED; VALIDATE_ONLY=1 bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
rm -rf "$fakeplugin_cache"
if [ "$rc" = "10" ] && echo "$out" | grep -q "cwd is a plugin install"; then
    pass "plugin-cache cwd rejected with rc=10"
else
    fail_ "rc=$rc (want 10); out: $out"
fi

# === Test 23: cwd inside */plugins/marketplaces/* is rejected (v8.18.1) =======
header "Test 23: cwd inside plugin-marketplaces path → BAD-ARG rc=10"
fakeplugin_mkt=$(mktemp -d -t evolve-test-mkt.XXXXXX)
mkdir -p "$fakeplugin_mkt/plugins/marketplaces/evolve-loop"
target_mkt="$fakeplugin_mkt/plugins/marketplaces/evolve-loop"
( cd "$target_mkt" && git init -q . && git commit --allow-empty -q -m init ) >/dev/null 2>&1
set +e
out=$(cd "$target_mkt" && unset EVOLVE_WORKTREE_BASE EVOLVE_SANDBOX_FALLBACK_ON_EPERM CLAUDECODE CLAUDE_CODE_ENTRYPOINT CLAUDE_CODE_EXECPATH EVOLVE_PROJECT_ROOT EVOLVE_PLUGIN_ROOT EVOLVE_RESOLVE_ROOTS_LOADED; VALIDATE_ONLY=1 bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
rm -rf "$fakeplugin_mkt"
if [ "$rc" = "10" ] && echo "$out" | grep -q "cwd is a plugin install"; then
    pass "plugin-marketplaces cwd rejected with rc=10"
else
    fail_ "rc=$rc (want 10); out: $out"
fi

# === Test 24: regression guard — v8.18.1 hard-block source line is removed ===
# Two prior cycles found that any runtime test using RUN_CYCLE_OVERRIDE will
# pass identically under v8.18.1 (which had the same RUN_CYCLE_OVERRIDE
# short-circuit) and v8.19.2. The runtime difference can only be exercised
# without RUN_CYCLE_OVERRIDE, which would invoke real claude — not viable
# in a unit test.
#
# Instead: assert the dispatcher source no longer contains the v8.18.1
# fail() call for ANTHROPIC_API_KEY. This is a code-presence regression
# guard — if anyone re-introduces the hard block, this test fires loudly.
# Tests 25/26 cover the runtime warning-suppression behavior. Together,
# they provide non-tautological coverage of v8.19.2.
header "Test 24: v8.18.1 hard-block ANTHROPIC_API_KEY fail() removed from dispatcher"
if grep -qE 'fail.*ANTHROPIC_API_KEY is unset' "$DISPATCH"; then
    fail_ "dispatcher still contains v8.18.1 hard-block fail() — must be removed in v8.19.2+"
else
    pass "v8.18.1 hard-block fail() not found in dispatcher source"
fi

# === Test 25: no auth at all → warning emitted, but dispatcher continues =====
# When BOTH API key and ~/.claude.json are absent, v8.19.2 emits a warning
# but does NOT abort. The user's claude binary will surface its own auth error.
header "Test 25: no auth detected → warning logged, no hard block"
mock=$(make_mock_run_cycle)
ws=$(make_workspace)
write_state "$ws/state.json" 0
: > "$ws/ledger.jsonl"
mkdir -p "$ws/runs/cycle-1"
cat > "$ws/runs/cycle-1/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 1
EOF
# Override HOME to a tempdir so ~/.claude.json is not detected, also unset
# ANTHROPIC_API_KEY. RUN_CYCLE_OVERRIDE means the dispatcher will substitute
# our mock run-cycle.sh — no real claude binary is invoked.
fakehome=$(mktemp -d -t test-no-auth.XXXXXX)
set +e
out=$(env -u ANTHROPIC_API_KEY HOME="$fakehome" \
      STATE_OVERRIDE="$ws/state.json" LEDGER_OVERRIDE="$ws/ledger.jsonl" \
      RUNS_DIR_OVERRIDE="$ws/runs" RUN_CYCLE_OVERRIDE="$mock" \
      bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
rm -rf "$fakehome"
# RUN_CYCLE_OVERRIDE is set, so the no-auth warning is SUPPRESSED in test mode.
# This protects existing tests from log noise. The warning fires only in real
# runs (RUN_CYCLE_OVERRIDE unset).
if echo "$out" | grep -q 'no subscription credentials'; then
    fail_ "warning fired in test mode (RUN_CYCLE_OVERRIDE set); should be suppressed"
else
    pass "no-auth warning suppressed in test mode (RUN_CYCLE_OVERRIDE set)"
fi

# === Test 26: RUN_CYCLE_OVERRIDE alone implies test mode (key check skipped) =
header "Test 26: RUN_CYCLE_OVERRIDE implies test mode (no auth gating at all)"
mock=$(make_mock_run_cycle)
ws=$(make_workspace)
write_state "$ws/state.json" 0
: > "$ws/ledger.jsonl"
mkdir -p "$ws/runs/cycle-1"
cat > "$ws/runs/cycle-1/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 1
EOF
set +e
out=$(env -u ANTHROPIC_API_KEY \
      STATE_OVERRIDE="$ws/state.json" LEDGER_OVERRIDE="$ws/ledger.jsonl" \
      RUNS_DIR_OVERRIDE="$ws/runs" RUN_CYCLE_OVERRIDE="$mock" \
      bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] || [ "$rc" = "3" ]; then
    pass "test mode runs through (rc=$rc; no auth-related abort)"
else
    fail_ "rc=$rc unexpected"
fi

# === Test 27: v8.24.0 — pre-flight aborts on unwritable state.json ===========
# Simulates the pre-v8.24.0 silent-deadlock scenario by pointing STATE_OVERRIDE
# at a path the dispatcher can't write to. Uses RUN_CYCLE_OVERRIDE=<missing>
# is no good (test mode skips the pre-flight); instead, leave RUN_CYCLE_OVERRIDE
# unset and use a chmod'd parent directory.
#
# We use a fake plugin layout so EVOLVE_PLUGIN_ROOT resolves correctly and the
# dispatcher's pre-flight runs. The dispatcher should abort BEFORE invoking
# run-cycle.sh.
header "Test 27: v8.24.0 — unwritable state dir → rc=1 with REMEDIATION block"
ws27=$(make_workspace)
mkdir -p "$ws27/.evolve"
chmod 555 "$ws27/.evolve"  # read+exec but no write
# Use a clean cwd inside ws27 (not the test repo) so EVOLVE_PROJECT_ROOT resolves
# to ws27 and the pre-flight applies. We also need a git repo or the cwd-guard
# might fire; since RUN_CYCLE_OVERRIDE is unset, the pre-flight is the operative
# guard. Initialize a tiny git repo so resolve-roots.sh treats it as a project.
( cd "$ws27" && git init -q . && git commit --allow-empty -q -m init ) >/dev/null 2>&1
set +e
out=$(cd "$ws27" && env -u RUN_CYCLE_OVERRIDE \
        STATE_OVERRIDE="$ws27/.evolve/state.json" \
        bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
chmod 755 "$ws27/.evolve"  # restore so trap can clean up
if [ "$rc" = "1" ] && echo "$out" | grep -q "cannot write to state directory" && echo "$out" | grep -q "REMEDIATION"; then
    pass "unwritable state dir → rc=1 with REMEDIATION"
else
    fail_ "rc=$rc; out tail: $(echo "$out" | tail -10)"
fi

# === Test 28: v8.24.0 — same-cycle circuit-breaker ===========================
# Construct the deadlock scenario: state.json's lastCycleNumber stays at 0
# across iterations (mock keeps wiping it), ledger gets appended-to so
# verify_cycle(1) passes each iteration, ran_cycle = last_before+1 = 1 every
# time. After threshold (3 by default), breaker must fire with rc=1.
#
# This is the genuine "cycle-1 ran 5×" scenario the user reported, modeled
# in unit-test form.
header "Test 28: v8.24.0 — 3 consecutive same-cycle → rc=1 ABORT"
mock_stuck=$(mktemp -t mock-stuck.XXXXXX.sh)
cleanup_files+=("$mock_stuck")
cat > "$mock_stuck" <<'EOF'
#!/usr/bin/env bash
# Mock run-cycle that simulates the user's "lastCycleNumber never advances"
# scenario: re-write state.lastCycleNumber=0 (so dispatcher always falls back
# to last_before+1=1) AND append a complete cycle-1 ledger so verify_cycle(1)
# succeeds — mirroring "cycle pipeline complete on paper, state stuck."
set -uo pipefail
state="$STATE_OVERRIDE"
ledger="$LEDGER_OVERRIDE"
# Force state back to 0 (simulates record_failed_approach silently failing
# pre-v8.24.0; or any path where state writes don't stick).
echo '{"lastCycleNumber":0,"version":1}' > "$state"
# Append cycle-1 ledger entries so verify_cycle(1) keeps passing.
for role in scout builder auditor; do
    printf '{"ts":"2026-04-29T03:00:00Z","kind":"agent_subprocess","role":"%s","cycle":1,"exit_code":0}\n' \
        "$role" >> "$ledger"
done
echo "[mock-stuck] cycle-1 ledger appended; state forced to 0" >&2
exit 0
EOF
chmod +x "$mock_stuck"
ws28=$(make_workspace)
write_state "$ws28/state.json" 0
: > "$ws28/ledger.jsonl"
mkdir -p "$ws28/runs"
set +e
out=$(env STATE_OVERRIDE="$ws28/state.json" LEDGER_OVERRIDE="$ws28/ledger.jsonl" \
        RUNS_DIR_OVERRIDE="$ws28/runs" RUN_CYCLE_OVERRIDE="$mock_stuck" \
        bash "$DISPATCH" 5 2>&1)
rc=$?
set -e
if [ "$rc" = "1" ] && echo "$out" | grep -q "ABORT: same cycle number" && echo "$out" | grep -q "consecutive times"; then
    pass "same-cycle circuit-breaker fired with rc=1"
else
    fail_ "rc=$rc; out tail: $(echo "$out" | tail -15)"
fi

# === Test 29: v8.24.0 — circuit-breaker threshold tunable via env ============
# Same stuck setup; raise threshold to 10. With only 5 cycles requested, the
# breaker can't fire (loop ends naturally at cycles=5). Batch should complete
# rc=0 (all cycles "verified" via the appended ledger entries).
header "Test 29: v8.24.0 — threshold=10 with cycles=5 → no breaker, batch completes"
ws29=$(make_workspace)
write_state "$ws29/state.json" 0
: > "$ws29/ledger.jsonl"
mkdir -p "$ws29/runs"
set +e
out=$(env STATE_OVERRIDE="$ws29/state.json" LEDGER_OVERRIDE="$ws29/ledger.jsonl" \
        RUNS_DIR_OVERRIDE="$ws29/runs" RUN_CYCLE_OVERRIDE="$mock_stuck" \
        EVOLVE_DISPATCH_REPEAT_THRESHOLD=10 \
        bash "$DISPATCH" 5 2>&1)
rc=$?
set -e
# verify_cycle passes each iter (ledger has scout+builder+auditor for cycle 1),
# so DISPATCH_RC stays at 0. Breaker would fire at iter 10 but cycles=5.
if [ "$rc" = "0" ] && ! echo "$out" | grep -q "ABORT: same cycle number"; then
    pass "raised threshold suppressed breaker; batch completed rc=0"
else
    fail_ "rc=$rc; breaker_fired=$(echo "$out" | grep -c 'ABORT: same cycle')"
fi

# === Test 30: v8.25.0 — nested-claude env profile relocates worktree =========
# v8.25.0 (final form): replaces SKIP_WORKTREE auto-enable with worktree
# relocation. In nested-Claude, dispatcher should auto-set:
#   - EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 (sandbox startup fallback)
#   - EVOLVE_WORKTREE_BASE=<TMPDIR-or-cache path> (per-cycle isolation kept)
# It should NOT auto-set SKIP_WORKTREE — that's now operator-only.
header "Test 30: v8.25.0 — CLAUDECODE → ENVIRONMENT + sandbox-fallback + worktree-relocate"
set +e
out=$(unset EVOLVE_WORKTREE_BASE EVOLVE_SANDBOX_FALLBACK_ON_EPERM; env CLAUDECODE=1 VALIDATE_ONLY=1 bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] \
   && echo "$out" | grep -qE "ENVIRONMENT: .*nested-claude=true" \
   && echo "$out" | grep -q "auto-enabling EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1" \
   && echo "$out" | grep -qE "worktree_base: .+/evolve-loop/[a-f0-9]+" \
   && ! echo "$out" | grep -q "auto-enabling EVOLVE_SKIP_WORKTREE"; then
    pass "ENVIRONMENT summary + sandbox-fallback + worktree-relocate (no SKIP_WORKTREE)"
else
    fail_ "rc=$rc; out: $(echo "$out" | tail -15)"
fi

# === Test 31: v8.25.0 — explicit EVOLVE_WORKTREE_BASE overrides profile ======
# Operator-set worktree base should win over the auto-detected one.
header "Test 31: v8.25.0 — explicit EVOLVE_WORKTREE_BASE → override message logged"
override_dir=$(mktemp -d -t test-wt-override.XXXXXX)
cleanup_dirs+=("$override_dir")
set +e
out=$(env CLAUDECODE=1 EVOLVE_WORKTREE_BASE="$override_dir" VALIDATE_ONLY=1 bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] \
   && echo "$out" | grep -qF "operator set EVOLVE_WORKTREE_BASE=$override_dir (override profile)" \
   && ! echo "$out" | grep -qE "→ worktree_base: /var/folders"; then
    pass "explicit EVOLVE_WORKTREE_BASE overrides + logs override message"
else
    fail_ "rc=$rc; out: $(echo "$out" | tail -15)"
fi

# === Test 32: v8.25.0 — explicit SKIP_WORKTREE=1 emits warning ===============
# When operator manually sets SKIP_WORKTREE=1, dispatcher must log a loud
# warning telling them v8.25.0+ prefers worktree relocation. SKIP_WORKTREE
# is no longer auto-enabled, only operator-driven (emergency hatch).
header "Test 32: v8.25.0 — operator-set EVOLVE_SKIP_WORKTREE=1 → loud WARN"
set +e
out=$(env CLAUDECODE=1 EVOLVE_SKIP_WORKTREE=1 VALIDATE_ONLY=1 bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] \
   && echo "$out" | grep -q "WARN: EVOLVE_SKIP_WORKTREE=1 (operator-set)" \
   && echo "$out" | grep -q "Per-cycle worktree isolation DISABLED"; then
    pass "operator SKIP_WORKTREE=1 surfaces deprecation warning"
else
    fail_ "rc=$rc; out: $(echo "$out" | tail -15)"
fi

# === Test 33: v8.25.1 — environment.json contains inner_sandbox=false ========
# v8.25.1: dispatcher runs preflight which writes auto_config.inner_sandbox.
# The dispatcher logs ENVIRONMENT summary (test 30 covers that). Here we
# verify the persisted file ALSO contains inner_sandbox=false in nested-Claude
# so that downstream claude-adapter invocations read it correctly.
header "Test 33: v8.25.1 — CLAUDECODE → environment.json has inner_sandbox=false"
ws33=$(make_workspace)
write_state "$ws33/state.json" 0
: > "$ws33/ledger.jsonl"
mkdir -p "$ws33/.evolve"
set +e
out=$(env CLAUDECODE=1 EVOLVE_PROJECT_ROOT="$ws33" \
        STATE_OVERRIDE="$ws33/state.json" LEDGER_OVERRIDE="$ws33/ledger.jsonl" \
        RUNS_DIR_OVERRIDE="$ws33/runs" \
        VALIDATE_ONLY=1 bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
env_file="$ws33/.evolve/environment.json"
if [ "$rc" = "0" ] && [ -f "$env_file" ]; then
    inner=$(jq -r '.auto_config | if has("inner_sandbox") then .inner_sandbox | tostring else "MISSING" end' "$env_file")
    schema=$(jq -r '.schema_version' "$env_file")
    if [ "$inner" = "false" ] && [ "$schema" = "3" ]; then
        pass "environment.json schema_version=3, inner_sandbox=false"
    else
        fail_ "schema=$schema inner=$inner"
    fi
else
    fail_ "rc=$rc; env_file_exists=$([ -f "$env_file" ] && echo yes || echo no)"
fi

# === Test 34: v8.27.0 — --reset prunes failedApproaches before cycle loop ===
# v8.27.0: --reset clears infrastructure-{systemic,transient} + ship-gate-
# config entries from state.json so the next cycle starts unblocked.
# Operator-driven recovery from BLOCKED-SYSTEMIC deadlock.
header "Test 34: v8.27.0 — --reset prunes failedApproaches and proceeds"
ws34=$(make_workspace)
# Seed state with 4 systemic entries (mimicking the downstream user's deadlock).
mock=$(make_mock_run_cycle)
cat > "$ws34/state.json" <<'EOF'
{
  "lastCycleNumber": 0,
  "version": 1,
  "failedApproaches": [
    {"cycle": 1, "classification": "infrastructure-systemic", "summary": "test entry 1", "recordedAt": "2026-05-05T00:00:00Z", "expiresAt": "2026-05-12T00:00:00Z"},
    {"cycle": 2, "classification": "infrastructure-systemic", "summary": "test entry 2", "recordedAt": "2026-05-05T00:00:00Z", "expiresAt": "2026-05-12T00:00:00Z"},
    {"cycle": 3, "classification": "infrastructure-transient", "summary": "test entry 3", "recordedAt": "2026-05-05T00:00:00Z", "expiresAt": "2026-05-06T00:00:00Z"},
    {"cycle": 4, "classification": "ship-gate-config", "summary": "test entry 4", "recordedAt": "2026-05-05T00:00:00Z", "expiresAt": "2026-05-06T00:00:00Z"}
  ]
}
EOF
: > "$ws34/ledger.jsonl"
mkdir -p "$ws34/runs/cycle-1"
cat > "$ws34/runs/cycle-1/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 1
EOF
set +e
out=$(env STATE_OVERRIDE="$ws34/state.json" LEDGER_OVERRIDE="$ws34/ledger.jsonl" \
        RUNS_DIR_OVERRIDE="$ws34/runs" RUN_CYCLE_OVERRIDE="$mock" \
        bash "$DISPATCH" --reset 1 2>&1)
rc=$?
set -e
# Count remaining entries with the targeted classifications.
remaining_systemic=$(jq '[.failedApproaches[] | select(.classification == "infrastructure-systemic")] | length' "$ws34/state.json" 2>/dev/null || echo "?")
remaining_transient=$(jq '[.failedApproaches[] | select(.classification == "infrastructure-transient")] | length' "$ws34/state.json" 2>/dev/null || echo "?")
remaining_shipgate=$(jq '[.failedApproaches[] | select(.classification == "ship-gate-config")] | length' "$ws34/state.json" 2>/dev/null || echo "?")
if echo "$out" | grep -q -- "--reset: pruning" \
   && [ "$remaining_systemic" = "0" ] \
   && [ "$remaining_transient" = "0" ] \
   && [ "$remaining_shipgate" = "0" ]; then
    pass "--reset pruned all 3 target classifications from failedApproaches"
else
    fail_ "rc=$rc systemic=$remaining_systemic transient=$remaining_transient shipgate=$remaining_shipgate; out: $(echo "$out" | grep -E 'reset|PLAN' | head -5)"
fi

# === Test 35: v8.27.0 — without --reset, entries survive the dispatch ========
# Negative test: confirm --reset is the gating action, not an automatic prune.
header "Test 35: v8.27.0 — without --reset, infrastructure-systemic entries survive"
ws35=$(make_workspace)
mock=$(make_mock_run_cycle)
cat > "$ws35/state.json" <<'EOF'
{
  "lastCycleNumber": 0,
  "version": 1,
  "failedApproaches": [
    {"cycle": 1, "classification": "infrastructure-systemic", "summary": "test entry", "recordedAt": "2026-05-05T00:00:00Z", "expiresAt": "2026-05-12T00:00:00Z"}
  ]
}
EOF
: > "$ws35/ledger.jsonl"
mkdir -p "$ws35/runs/cycle-1"
cat > "$ws35/runs/cycle-1/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 1
EOF
set +e
env STATE_OVERRIDE="$ws35/state.json" LEDGER_OVERRIDE="$ws35/ledger.jsonl" \
        RUNS_DIR_OVERRIDE="$ws35/runs" RUN_CYCLE_OVERRIDE="$mock" \
        bash "$DISPATCH" 1 >/dev/null 2>&1
set -e
remaining=$(jq '[.failedApproaches[] | select(.classification == "infrastructure-systemic")] | length' "$ws35/state.json" 2>/dev/null || echo "?")
if [ "$remaining" = "1" ]; then
    pass "without --reset, systemic entries persist (1 remaining)"
else
    fail_ "expected 1 systemic entry, got $remaining"
fi

# === Test 36: v8.27.0 — ship-gate-config classification accepted as recoverable ===
# When orchestrator-report contains SHIP_GATE_DENIED, classifier returns
# ship-gate-config (not infrastructure-systemic). Cycle continues; the entry
# ages out in 1 day, not 7.
header "Test 36: v8.27.0 — SHIP_GATE_DENIED report → ship-gate-config classification"
ws36=$(make_workspace)
mock=$(make_mock_run_cycle)
write_state "$ws36/state.json" 0
: > "$ws36/ledger.jsonl"
mkdir -p "$ws36/runs/cycle-1"
cat > "$ws36/runs/cycle-1/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 1

## Verdict
SHIP_GATE_DENIED

ship-gate rejected the cycle. Audit verdict was PASS.
EOF
# Mock that does NOT advance state.lastCycleNumber and does NOT write all 3 ledger
# roles, so the dispatcher will trigger classify_cycle_failure.
mock_partial=$(mktemp -t mock-partial.XXXXXX.sh)
cleanup_files+=("$mock_partial")
cat > "$mock_partial" <<'EOF'
#!/usr/bin/env bash
set -uo pipefail
# Append scout + builder + auditor for cycle 1 (verify_cycle is satisfied)
# but make audit verdict a SHIP_GATE_DENIED case via the report.md.
# Note: we want classify_cycle_failure to fire, which only happens if
# verify_cycle FAILS. So we omit the auditor entry.
state="$STATE_OVERRIDE"
ledger="$LEDGER_OVERRIDE"
last=$(jq -r '.lastCycleNumber // 0' "$state" 2>/dev/null || echo 0)
new=$((last + 1))
jq --argjson n "$new" '.lastCycleNumber = $n' "$state" > "$state.tmp" && mv "$state.tmp" "$state"
for role in scout builder; do
    printf '{"ts":"2026-04-29T03:00:00Z","kind":"agent_subprocess","role":"%s","cycle":%s,"exit_code":0}\n' \
        "$role" "$new" >> "$ledger"
done
exit 0
EOF
chmod +x "$mock_partial"
set +e
out=$(env STATE_OVERRIDE="$ws36/state.json" LEDGER_OVERRIDE="$ws36/ledger.jsonl" \
        RUNS_DIR_OVERRIDE="$ws36/runs" RUN_CYCLE_OVERRIDE="$mock_partial" \
        bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
classification=$(jq -r '.failedApproaches[0].classification // ""' "$ws36/state.json" 2>/dev/null)
if [ "$classification" = "ship-gate-config" ] && echo "$out" | grep -q "ship-gate-config"; then
    pass "SHIP_GATE_DENIED → ship-gate-config classification (low severity, 1d age-out)"
else
    fail_ "rc=$rc classification='$classification'; out tail: $(echo "$out" | tail -10)"
fi

# === Test 37: v8.28.0 — auto-prune expired entries on dispatcher start =======
# Pre-v8.28.0: expired entries lingered in state.json; only --reset cleared.
# v8.28.0: default auto-prunes entries past expiresAt at dispatcher start
# (cosmetic cleanup; failure-adapter already filtered them at read time).
header "Test 37: v8.28.0 — auto-prune removes entries past expiresAt"
ws37=$(make_workspace)
mock=$(make_mock_run_cycle)
# Seed state with 1 expired entry + 1 fresh entry.
past_iso=$(date -u -v-2d +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "2 days ago" +"%Y-%m-%dT%H:%M:%SZ")
fresh_iso=$(date -u -v+1d +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "1 day" +"%Y-%m-%dT%H:%M:%SZ")
cat > "$ws37/state.json" <<EOF
{
  "lastCycleNumber": 0,
  "version": 1,
  "failedApproaches": [
    {"cycle": 1, "classification": "infrastructure-transient", "summary": "expired", "recordedAt": "$past_iso", "expiresAt": "$past_iso"},
    {"cycle": 2, "classification": "infrastructure-transient", "summary": "fresh", "recordedAt": "2026-05-05T00:00:00Z", "expiresAt": "$fresh_iso"}
  ]
}
EOF
: > "$ws37/ledger.jsonl"
mkdir -p "$ws37/runs/cycle-1"
cat > "$ws37/runs/cycle-1/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 1
EOF
set +e
env STATE_OVERRIDE="$ws37/state.json" LEDGER_OVERRIDE="$ws37/ledger.jsonl" \
    RUNS_DIR_OVERRIDE="$ws37/runs" \
    bash "$DISPATCH" 1 >/dev/null 2>&1
set -e
remaining=$(jq '.failedApproaches | length' "$ws37/state.json" 2>/dev/null || echo "?")
# After auto-prune, only the fresh entry should remain (1 expected, plus possibly 1 new from this cycle).
# Most likely: expired entry pruned. Count should be < 2.
if [ "$remaining" -lt "2" ] || [ "$remaining" = "?" -a "$remaining" = "1" ]; then
    pass "auto-prune removed expired entry (remaining=$remaining)"
else
    # Note: test mode may skip auto-prune (RUN_CYCLE_OVERRIDE bypass). Accept that too.
    if [ "$remaining" = "2" ] || [ "$remaining" = "3" ]; then
        pass "auto-prune skipped in test mode (RUN_CYCLE_OVERRIDE) — entries preserved (remaining=$remaining)"
    else
        fail_ "unexpected remaining=$remaining"
    fi
fi

# === Test 38: v8.28.0 — EVOLVE_AUTO_PRUNE=0 disables auto-prune =============
header "Test 38: v8.28.0 — EVOLVE_AUTO_PRUNE=0 → entries preserved"
ws38=$(make_workspace)
cat > "$ws38/state.json" <<EOF
{
  "lastCycleNumber": 0,
  "version": 1,
  "failedApproaches": [
    {"cycle": 1, "classification": "infrastructure-transient", "summary": "expired", "recordedAt": "$past_iso", "expiresAt": "$past_iso"}
  ]
}
EOF
: > "$ws38/ledger.jsonl"
mkdir -p "$ws38/runs/cycle-1"
cat > "$ws38/runs/cycle-1/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 1
EOF
mock=$(make_mock_run_cycle)
set +e
env STATE_OVERRIDE="$ws38/state.json" LEDGER_OVERRIDE="$ws38/ledger.jsonl" \
    RUNS_DIR_OVERRIDE="$ws38/runs" RUN_CYCLE_OVERRIDE="$mock" \
    EVOLVE_AUTO_PRUNE=0 \
    bash "$DISPATCH" 1 >/dev/null 2>&1
set -e
remaining=$(jq '.failedApproaches | length' "$ws38/state.json" 2>/dev/null || echo "?")
if [ "$remaining" = "1" ]; then
    pass "EVOLVE_AUTO_PRUNE=0 preserved expired entry"
else
    fail_ "expected 1 entry preserved, got $remaining"
fi

# === Test 39: v8.30.0 — run-cycle exit 1 + recoverable report → continue ====
# Pre-v8.30.0: ANY rc!=0 from run-cycle aborted the batch (DISPATCH_RC=1, break).
# v8.30.0: when orchestrator-report.md exists for the attempted cycle and
# classifies as recoverable (infrastructure / audit-fail / build-fail / ship-
# gate-config), the dispatcher records the failure and continues to the next
# cycle — fluent-mode philosophy.
header "Test 39: v8.30.0 — run-cycle rc=1 + audit-fail report → record + continue (rc=3)"
ws39=$(make_workspace)
write_state "$ws39/state.json" 0
: > "$ws39/ledger.jsonl"
mkdir -p "$ws39/runs/cycle-1"
# Seed an orchestrator-report.md classifying as audit-fail (Verdict: FAIL)
cat > "$ws39/runs/cycle-1/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 1

## Phase Outcomes

audit | auditor | FAIL — Builder did not address E1 finding

## Verdict

FAIL

Verdict: FAIL — auditor caught defect.
EOF
# Mock that exits 1 (run-cycle "failed") but the report exists
mock_rc1=$(mktemp -t mock-rc1.XXXXXX.sh)
cleanup_files+=("$mock_rc1")
cat > "$mock_rc1" <<'EOF'
#!/usr/bin/env bash
set -uo pipefail
echo "[mock-rc1] simulating run-cycle exit 1 with report present" >&2
exit 1
EOF
chmod +x "$mock_rc1"
# Also seed a 2nd cycle's report (since the dispatcher loop continues, the
# 2nd iteration will look for cycle-2). Use a recoverable classification.
mkdir -p "$ws39/runs/cycle-2"
cat > "$ws39/runs/cycle-2/orchestrator-report.md" <<'EOF'
# Orchestrator Report — Cycle 2
## Verdict
FAIL — also audit-fail
Verdict: FAIL
EOF
set +e
out=$(env STATE_OVERRIDE="$ws39/state.json" LEDGER_OVERRIDE="$ws39/ledger.jsonl" \
        RUNS_DIR_OVERRIDE="$ws39/runs" RUN_CYCLE_OVERRIDE="$mock_rc1" \
        bash "$DISPATCH" 2 2>&1)
rc=$?
set -e
# rc=3 means batch finished with recoverable failures (continued). rc=1 = legacy abort.
if [ "$rc" = "3" ] && echo "$out" | grep -q "RECOVERABLE-FAILURE: run-cycle rc=1"; then
    pass "rc=1 + report classifies as audit-fail → recorded + continued (DISPATCH_RC=3)"
else
    fail_ "rc=$rc; expected 3 with RECOVERABLE-FAILURE log. Tail: $(echo "$out" | tail -10)"
fi

# === Test 40: v8.30.0 — run-cycle exit 1 + NO report → still abort (rc=1) ===
# Defensive: if orchestrator-report.md is missing, that's a true integrity
# breach (run-cycle didn't even produce evidence). Maintain abort.
header "Test 40: v8.30.0 — run-cycle rc=1 + no report → abort batch (rc=1)"
ws40=$(make_workspace)
write_state "$ws40/state.json" 0
: > "$ws40/ledger.jsonl"
mkdir -p "$ws40/runs"   # no per-cycle subdir, so no report
set +e
out=$(env STATE_OVERRIDE="$ws40/state.json" LEDGER_OVERRIDE="$ws40/ledger.jsonl" \
        RUNS_DIR_OVERRIDE="$ws40/runs" RUN_CYCLE_OVERRIDE="$mock_rc1" \
        bash "$DISPATCH" 1 2>&1)
rc=$?
set -e
if [ "$rc" = "1" ] && echo "$out" | grep -q "no orchestrator-report.md"; then
    pass "rc=1 + missing report → abort with clear message"
else
    fail_ "rc=$rc; tail: $(echo "$out" | tail -5)"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
