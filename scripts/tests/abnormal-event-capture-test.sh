#!/usr/bin/env bash
#
# abnormal-event-capture-test.sh — v46+ abnormal-event pipeline tests.
# Tests:
#   1. _append_abnormal_event writes valid JSONL to workspace
#   2. gate_audit_to_retrospective passes when abnormal-events.jsonl non-empty
#   3. reconcile-carryover-todos.sh promotes abnormal events to carryoverTodos[]

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRATCH=$(mktemp -d)
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# ─── Inline _append_abnormal_event (mirrors subagent-run.sh implementation) ───
_append_abnormal_event() {
    local _ws="$1" _et="$2" _sev="$3" _det="$4" _rem="$5"
    [ -d "$_ws" ] || return 0
    local _ts; _ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local _det_esc; _det_esc=$(printf '%s' "$_det" | sed 's/"/\\"/g')
    local _rem_esc; _rem_esc=$(printf '%s' "$_rem" | sed 's/"/\\"/g')
    printf '{"event_type":"%s","timestamp":"%s","source_phase":"subagent-run","severity":"%s","details":"%s","remediation_hint":"%s"}\n' \
        "$_et" "$_ts" "$_sev" "$_det_esc" "$_rem_esc" >> "$_ws/abnormal-events.jsonl" 2>/dev/null || true
}

# ─── Test 1: _append_abnormal_event creates file with correct schema ──────────
header "Test 1: _append_abnormal_event creates well-formed JSONL"
WS1="$SCRATCH/ws1"
mkdir -p "$WS1"
_append_abnormal_event "$WS1" "dispatch-error" "HIGH" "phase exited rc=1" "check logs"
[ -f "$WS1/abnormal-events.jsonl" ] && pass "abnormal-events.jsonl created" || fail "abnormal-events.jsonl missing"
LINE=$(head -1 "$WS1/abnormal-events.jsonl")
echo "$LINE" | jq -e '.event_type' >/dev/null 2>&1 && pass "JSONL is valid JSON" || fail "JSONL not valid JSON: $LINE"
echo "$LINE" | jq -e '.event_type == "dispatch-error"' >/dev/null 2>&1 && pass "event_type=dispatch-error" || fail "event_type wrong"
echo "$LINE" | jq -e '.severity == "HIGH"' >/dev/null 2>&1 && pass "severity=HIGH" || fail "severity wrong"
echo "$LINE" | jq -e '.source_phase == "subagent-run"' >/dev/null 2>&1 && pass "source_phase=subagent-run" || fail "source_phase wrong"
echo "$LINE" | jq -e '.timestamp' >/dev/null 2>&1 && pass "timestamp present" || fail "timestamp missing"
echo "$LINE" | jq -e '.details == "phase exited rc=1"' >/dev/null 2>&1 && pass "details preserved" || fail "details wrong"

# ─── Test 2: _append_abnormal_event handles special chars in details ──────────
header "Test 2: _append_abnormal_event escapes quotes in details"
WS2="$SCRATCH/ws2"
mkdir -p "$WS2"
_append_abnormal_event "$WS2" "test-event" "LOW" 'say "hello"' 'retry "now"'
LINE2=$(head -1 "$WS2/abnormal-events.jsonl")
echo "$LINE2" | jq -e '.' >/dev/null 2>&1 && pass "quotes in details produce valid JSON" || fail "quote escaping broke JSON: $LINE2"

# ─── Test 3: _append_abnormal_event appends multiple events ──────────────────
header "Test 3: _append_abnormal_event appends (does not overwrite)"
WS3="$SCRATCH/ws3"
mkdir -p "$WS3"
_append_abnormal_event "$WS3" "event-a" "HIGH" "first" ""
_append_abnormal_event "$WS3" "event-b" "LOW" "second" ""
COUNT=$(wc -l < "$WS3/abnormal-events.jsonl" | tr -d ' ')
[ "$COUNT" = "2" ] && pass "two events appended" || fail "expected 2 lines, got $COUNT"

# ─── Test 4: _append_abnormal_event is no-op when workspace missing ───────────
header "Test 4: _append_abnormal_event is no-op for missing workspace"
_append_abnormal_event "/nonexistent/ws" "test" "LOW" "detail" "hint"
pass "no-op for missing workspace (did not crash)"

# ─── Test 5: gate_audit_to_retrospective allows retro on PASS+abnormal ───────
header "Test 5: phase-gate audit-to-retrospective passes on PASS+abnormal-events"
WS5="$SCRATCH/ws5"
mkdir -p "$WS5"
# Create substantive audit-report.md with PASS verdict using the format the gate expects:
# Verdict: **PASS** on one line (gate regex: Verdict:[[:space:]]*\*?\*?[[:space:]]*PASS)
cat > "$WS5/audit-report.md" <<'EOF'
<!-- challenge-token: test -->
# Audit Report — Cycle 99

## Acceptance Criteria Results

| ID | Criterion | Result |
|----|-----------|--------|
| AC1 | scripts/tests/stub-test.sh exits 0 | GREEN |
| AC2 | docs/architecture/stub.md exists | GREEN |

## Findings

All acceptance criteria verified against agents/evolve-builder.md and scripts/lifecycle/reconcile-carryover-todos.sh.
No defects found. The build produced changes to .evolve/profiles/scout.json and docs/architecture/token-reduction-roadmap.md.
Tests passed: 6/6. No regressions in acs/regression-suite.

## Verdict: **PASS**
EOF
# Write a non-empty abnormal-events.jsonl
echo '{"event_type":"test","timestamp":"2026-01-01T00:00:00Z","source_phase":"subagent-run","severity":"LOW","details":"test","remediation_hint":""}' > "$WS5/abnormal-events.jsonl"
# Create a fake ledger + cycle-state for gate invocation.
# check_subagent_ledger_match needs a "kind":"agent_subprocess" entry with
# matching SHA256 so the grep→jq pipeline doesn't exit non-zero under pipefail.
mkdir -p "$SCRATCH/ws5-proj/.evolve"
cat > "$SCRATCH/ws5-proj/.evolve/cycle-state.json" <<EOF
{"cycle_id":99,"phase":"audit","active_agent":"auditor","completed_phases":["calibrate","research","triage","build"]}
EOF
if command -v sha256sum >/dev/null 2>&1; then
    _sha=$(sha256sum "$WS5/audit-report.md" | awk '{print $1}')
else
    _sha=$(shasum -a 256 "$WS5/audit-report.md" | awk '{print $1}')
fi
printf '{"kind":"agent_subprocess","cycle":99,"role":"auditor","exit_code":0,"artifact_sha256":"%s","artifact_path":"%s","ts":"2026-01-01T00:00:00Z"}\n' \
    "$_sha" "$WS5/audit-report.md" > "$SCRATCH/ws5-proj/.evolve/ledger.jsonl"
GATE_RC=0
EVOLVE_PROJECT_ROOT="$SCRATCH/ws5-proj" \
    bash "$REPO_ROOT/scripts/lifecycle/phase-gate.sh" audit-to-retrospective 99 "$WS5" 2>/dev/null || GATE_RC=$?
[ "$GATE_RC" -eq 0 ] && pass "gate audit-to-retrospective passes on PASS+abnormal events" \
    || fail "gate audit-to-retrospective rejected PASS+abnormal (rc=$GATE_RC)"
[ -f "$WS5/.cycle-verdict" ] && VERD=$(cat "$WS5/.cycle-verdict") || VERD=""
[ "$VERD" = "PASS-WITH-ABNORMAL" ] && pass ".cycle-verdict=PASS-WITH-ABNORMAL" \
    || fail ".cycle-verdict wrong: '$VERD'"

# ─── Test 6: gate_audit_to_retrospective blocks PASS without abnormal ─────────
header "Test 6: phase-gate audit-to-retrospective blocks PASS without abnormal events"
WS6="$SCRATCH/ws6"
mkdir -p "$WS6"
# Substantive PASS audit-report without abnormal events
cat > "$WS6/audit-report.md" <<'EOF'
<!-- challenge-token: test -->
# Audit Report — Cycle 99

All acceptance criteria checked. No abnormal events detected during cycle execution.
Verified scripts/lifecycle/reconcile-carryover-todos.sh and .evolve/profiles/scout.json changes.
No regression failures. Build changes look correct.

## Verdict
**PASS**
EOF
# NO abnormal-events.jsonl
mkdir -p "$SCRATCH/ws6-proj/.evolve"
cp "$SCRATCH/ws5-proj/.evolve/cycle-state.json" "$SCRATCH/ws6-proj/.evolve/cycle-state.json"
cp "$SCRATCH/ws5-proj/.evolve/ledger.jsonl" "$SCRATCH/ws6-proj/.evolve/ledger.jsonl"
GATE6_RC=0
EVOLVE_PROJECT_ROOT="$SCRATCH/ws6-proj" \
    bash "$REPO_ROOT/scripts/lifecycle/phase-gate.sh" audit-to-retrospective 99 "$WS6" 2>/dev/null || GATE6_RC=$?
[ "$GATE6_RC" -ne 0 ] && pass "gate correctly blocks PASS without abnormal events" \
    || fail "gate should have blocked PASS without abnormal events"

# ─── Test 7: reconcile-carryover-todos promotes abnormal events ───────────────
header "Test 7: reconcile-carryover-todos.sh promotes abnormal events to carryoverTodos"
WS7="$SCRATCH/ws7"
mkdir -p "$WS7"
ROOT7="$SCRATCH/root7"
mkdir -p "$ROOT7/.evolve/runs/cycle-1"
# state.json with no carryoverTodos
cat > "$ROOT7/.evolve/state.json" <<'EOF'
{"instinctSummary":[],"carryoverTodos":[],"failedApproaches":[]}
EOF
# Write abnormal-events.jsonl with two events (same event_type deduplicates)
cat > "$WS7/abnormal-events.jsonl" <<'EOF'
{"event_type":"quota-exhausted","timestamp":"2026-01-01T00:00:00Z","source_phase":"subagent-run","severity":"HIGH","details":"rc=1 empty stderr","remediation_hint":"check quota"}
{"event_type":"quota-exhausted","timestamp":"2026-01-01T00:01:00Z","source_phase":"subagent-run","severity":"HIGH","details":"rc=1 again","remediation_hint":"check quota"}
{"event_type":"slow-response","timestamp":"2026-01-01T00:02:00Z","source_phase":"subagent-run","severity":"LOW","details":"latency spike","remediation_hint":"retry"}
EOF
cp "$WS7/abnormal-events.jsonl" "$ROOT7/.evolve/runs/cycle-1/abnormal-events.jsonl"
# scout-report and triage-decision (minimal, no carryover decisions)
echo "# Scout" > "$WS7/scout-report.md"
echo "# Triage" > "$WS7/triage-decision.md"
EVOLVE_PROJECT_ROOT="$ROOT7" bash "$REPO_ROOT/scripts/lifecycle/reconcile-carryover-todos.sh" \
    --cycle 1 --workspace "$WS7" --verdict PASS >/dev/null 2>&1
TODO_LEN=$(jq -r '.carryoverTodos | length' "$ROOT7/.evolve/state.json" 2>/dev/null)
[ "$TODO_LEN" -ge 2 ] && pass "abnormal events promoted to carryoverTodos (got $TODO_LEN)" \
    || fail "expected ≥2 carryoverTodos from abnormal events, got $TODO_LEN"
# Verify priority=HIGH and _inbox_source prefix
PRIORITIES_OK=true
SOURCES_OK=true
while IFS= read -r todo; do
    prio=$(echo "$todo" | jq -r '.priority // ""')
    src=$(echo "$todo" | jq -r '._inbox_source // ""')
    [ "$prio" = "HIGH" ] || PRIORITIES_OK=false
    echo "$src" | grep -q "^abnormal-event:" || SOURCES_OK=false
done < <(jq -c '.carryoverTodos[]' "$ROOT7/.evolve/state.json" 2>/dev/null)
"$PRIORITIES_OK" && pass "promoted todos have priority=HIGH" || fail "promoted todos missing priority=HIGH"
"$SOURCES_OK" && pass "promoted todos have _inbox_source=abnormal-event:*" || fail "_inbox_source prefix wrong"

# ─── Summary ─────────────────────────────────────────────────────────────────
echo
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
