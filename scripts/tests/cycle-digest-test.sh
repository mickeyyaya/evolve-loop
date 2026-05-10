#!/usr/bin/env bash
#
# cycle-digest-test.sh — tests for build-cycle-digest.sh.
#
# v8.62.0 Campaign B Cycle B1.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/lifecycle/build-cycle-digest.sh"

PASS=0
FAIL=0
TESTS_TOTAL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# --- Setup: temporary workspace + state.json --------------------------------
TMP_ROOT=$(mktemp -d)
trap 'rm -rf "$TMP_ROOT"' EXIT
mkdir -p "$TMP_ROOT/.evolve"
mkdir -p "$TMP_ROOT/.evolve/runs/cycle-99"

# Minimal state.json fixture (covers both new + legacy failedApproaches shapes).
cat > "$TMP_ROOT/.evolve/state.json" <<'STATE'
{
  "lastCycleNumber": 99,
  "failedApproaches": [
    {"ts":"2026-05-01T00:00:00Z","cycle":97,"auditVerdict":"WARN","errorCategory":"legacy-shape","failedStep":"triage-ghost"},
    {"ts":"2026-05-02T00:00:00Z","cycle":98,"verdict":"WARN","classification":"new-shape","defects":[{"title":"defect-X"}]},
    {"ts":"2026-05-03T00:00:00Z","cycle":99,"verdict":"FAIL","classification":"new-shape-fail","defects":[]}
  ],
  "instinctSummary": [
    {"id":"inst-A","pattern":"avoid X","confidence":0.7},
    {"id":"inst-B","pattern":"prefer Y","confidence":0.85},
    {"id":"inst-C","pattern":"verify Z","confidence":0.92}
  ],
  "carryoverTodos": [
    {"id":"todo-1","action":"do thing","priority":"P1","cycles_unpicked":2},
    {"id":"todo-2","action":"another thing","priority":"P2","cycles_unpicked":4}
  ]
}
STATE

# Minimal intent.md with YAML goal + acceptance_checks.
cat > "$TMP_ROOT/.evolve/runs/cycle-99/intent.md" <<'INTENT'
<!-- challenge-token: testtoken99 -->
---
awn_class: test
goal: |
  Test goal: verify the digest writer handles the YAML intent format
  cleanly across both legacy and new shapes. Should produce a usable
  intent_anchor field.
non_goals:
  - "Do not break"
acceptance_checks:
  - check: "First acceptance check fires"
    how_verified: "manual"
  - check: "Second check also fires"
    how_verified: "manual"
constraints: []
INTENT

cat > "$TMP_ROOT/.evolve/runs/cycle-99/scout-report.md" <<'SCOUT'
<!-- challenge-token: testtoken99 -->
# Scout Report — Cycle 99

## Top Task

Test top task line for digest extraction.

## Other content here.
SCOUT

# --- Test 1: missing args exits 2 -------------------------------------------
header "Test 1: missing arguments exits 2"
set +e
EVOLVE_PROJECT_ROOT="$TMP_ROOT" bash "$SCRIPT" >/dev/null 2>&1
rc1=$?
EVOLVE_PROJECT_ROOT="$TMP_ROOT" bash "$SCRIPT" 99 >/dev/null 2>&1
rc2=$?
set -e
if [ "$rc1" = "2" ] && [ "$rc2" = "2" ]; then
    pass "missing args exit 2"
else
    fail_ "expected 2, got rc1=$rc1 rc2=$rc2"
fi

# --- Test 2: nonexistent workspace exits 1 ----------------------------------
header "Test 2: nonexistent workspace exits 1"
set +e
EVOLVE_PROJECT_ROOT="$TMP_ROOT" bash "$SCRIPT" 99 "$TMP_ROOT/nonexistent" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "1" ] && pass "missing workspace exits 1" || fail_ "expected 1, got $rc"

# --- Test 3: digest file is created ------------------------------------------
header "Test 3: digest file created at expected path"
EVOLVE_PROJECT_ROOT="$TMP_ROOT" bash "$SCRIPT" 99 "$TMP_ROOT/.evolve/runs/cycle-99" >/dev/null 2>&1
DIGEST="$TMP_ROOT/.evolve/runs/cycle-99/cycle-digest.json"
[ -f "$DIGEST" ] && pass "digest written" || fail_ "digest not at $DIGEST"

# --- Test 4: digest is valid JSON --------------------------------------------
header "Test 4: digest is valid JSON"
if jq . "$DIGEST" >/dev/null 2>&1; then
    pass "valid JSON"
else
    fail_ "invalid JSON; head: $(head -3 "$DIGEST")"
fi

# --- Test 5: required schema fields ------------------------------------------
# v9.0.1: built_at removed for determinism (HIGH-1 fix).
header "Test 5: digest has all required schema fields"
if jq -e '
    .schema_version and
    .cycle and
    .intent_anchor and
    .top_task and
    .acceptance_criteria and
    (.recent_failures | type == "array") and
    (.instinct_pointers | type == "array") and
    .todos_summary and
    .ledger_tip
' "$DIGEST" >/dev/null 2>&1; then
    pass "all required fields present"
else
    fail_ "missing required fields"
fi

# --- Test 5b (v9.0.1): digest is BYTE-IDENTICAL across re-runs ---------------
# Replaces the old test 11 (which excluded built_at). Now that built_at is
# gone, byte-identity must hold unconditionally. This is the determinism
# invariant the HIGH-1 fix delivers.
header "Test 5b (v9.0.1 HIGH-1): digest is byte-identical across re-runs"
sleep 1
EVOLVE_PROJECT_ROOT="$TMP_ROOT" bash "$SCRIPT" 99 "$TMP_ROOT/.evolve/runs/cycle-99" >/dev/null 2>&1
DIGEST_RERUN=$(cat "$DIGEST")
if [ "$DIGEST_RERUN" = "$(cat "$DIGEST")" ]; then
    # Read again and compare
    A=$(EVOLVE_PROJECT_ROOT="$TMP_ROOT" bash "$SCRIPT" 99 "$TMP_ROOT/.evolve/runs/cycle-99" 2>/dev/null; cat "$DIGEST")
    sleep 1
    B=$(EVOLVE_PROJECT_ROOT="$TMP_ROOT" bash "$SCRIPT" 99 "$TMP_ROOT/.evolve/runs/cycle-99" 2>/dev/null; cat "$DIGEST")
    if [ "$A" = "$B" ]; then
        pass "digest is byte-identical (no built_at drift)"
    else
        fail_ "digest differs across re-runs"
    fi
else
    fail_ "digest re-read mismatch"
fi

# --- Test 6: intent_anchor is YAML goal text --------------------------------
header "Test 6: intent_anchor extracted from YAML goal block"
intent_anchor=$(jq -r '.intent_anchor' "$DIGEST")
if echo "$intent_anchor" | grep -q "Test goal"; then
    pass "intent_anchor contains YAML goal text"
else
    fail_ "intent_anchor missing 'Test goal'; got: $intent_anchor"
fi

# --- Test 7: acceptance_criteria extracted from YAML acceptance_checks ------
header "Test 7: acceptance_criteria extracted from YAML"
ac=$(jq -r '.acceptance_criteria' "$DIGEST")
if echo "$ac" | grep -q "First acceptance check"; then
    pass "acceptance_criteria contains first check"
else
    fail_ "acceptance_criteria missing first check; got: $ac"
fi

# --- Test 8: recent_failures handles BOTH legacy and new shapes -------------
header "Test 8: recent_failures handles legacy + new schema"
legacy_count=$(jq '[.recent_failures[] | select(.classification == "legacy-shape")] | length' "$DIGEST")
new_count=$(jq '[.recent_failures[] | select(.classification == "new-shape")] | length' "$DIGEST")
fail_count=$(jq '[.recent_failures[] | select(.classification == "new-shape-fail")] | length' "$DIGEST")
if [ "$legacy_count" = "1" ] && [ "$new_count" = "1" ] && [ "$fail_count" = "1" ]; then
    pass "recent_failures correctly maps legacy + new shapes"
else
    fail_ "expected 1+1+1, got legacy=$legacy_count new=$new_count fail=$fail_count"
fi

# --- Test 9: instinct_pointers count + content -------------------------------
header "Test 9: instinct_pointers has 3 entries (state has 3)"
ip_count=$(jq '.instinct_pointers | length' "$DIGEST")
if [ "$ip_count" = "3" ]; then
    pass "instinct_pointers = 3"
else
    fail_ "expected 3, got $ip_count"
fi

# --- Test 10: todos_summary with 2 todos -----------------------------------
header "Test 10: todos_summary count = 2 (state has 2)"
todo_count=$(jq -r '.todos_summary.count' "$DIGEST")
if [ "$todo_count" = "2" ]; then
    pass "todos_summary.count = 2"
else
    fail_ "expected 2, got $todo_count"
fi

# --- Test 11: schema_version is 1.1 (v9.0.1: built_at removal) -------------
# Replaces the older idempotency test which now lives at 5b above (since
# byte-identity is the unconditional invariant after HIGH-1).
header "Test 11 (v9.0.1): schema_version is 1.1 (post HIGH-1 fix)"
schema_ver=$(jq -r '.schema_version' "$DIGEST")
if [ "$schema_ver" = "1.1" ]; then
    pass "schema_version = 1.1"
else
    fail_ "expected 1.1, got $schema_ver"
fi

# --- Test 12: digest is bounded (not bloated) -------------------------------
# We DON'T assert "smaller than raw" with a tiny fixture (overhead dominates).
# We DO assert the digest stays under 8KB regardless of input — the budget
# is a hard cap so the digest stays cheap to load into every phase prompt.
# Real workloads (cycle-10: ~28KB raw artifacts) yield 2-3KB digests, far
# under this ceiling.
header "Test 12: digest is bounded (under 8KB hard cap)"
digest_bytes=$(wc -c < "$DIGEST" | tr -d ' ')
if [ "$digest_bytes" -lt 8192 ]; then
    pass "digest=$digest_bytes bytes (under 8KB cap)"
else
    fail_ "digest=$digest_bytes bytes EXCEEDS 8KB cap"
fi

# --- Test 13: role-context-builder consumes digest under EVOLVE_CONTEXT_DIGEST=1
# Use a real-cycle workspace if available; check that digest mode produces
# a smaller scout context than legacy mode (the canonical Cycle B2 win).
header "Test 13 (Cycle B2): digest mode reduces scout context size"
RCB="$REPO_ROOT/scripts/lifecycle/role-context-builder.sh"
existing_cycle=$(ls "$REPO_ROOT/.evolve/runs/" 2>/dev/null | grep -E "^cycle-[0-9]+$" | grep -v "^cycle-[0-9]\{5,\}$" | head -1 | sed 's/cycle-//')
if [ -n "$existing_cycle" ] && [ -d "$REPO_ROOT/.evolve/runs/cycle-$existing_cycle" ]; then
    legacy_bytes=$(bash "$RCB" scout "$existing_cycle" "$REPO_ROOT/.evolve/runs/cycle-$existing_cycle" 2>/dev/null | wc -c | tr -d ' ')
    digest_bytes=$(EVOLVE_CONTEXT_DIGEST=1 bash "$RCB" scout "$existing_cycle" "$REPO_ROOT/.evolve/runs/cycle-$existing_cycle" 2>/dev/null | wc -c | tr -d ' ')
    if [ "$digest_bytes" -lt "$legacy_bytes" ]; then
        pct=$(( (legacy_bytes - digest_bytes) * 100 / legacy_bytes ))
        pass "digest mode shrinks scout context: $legacy_bytes -> $digest_bytes bytes (${pct}%% reduction)"
    else
        fail_ "digest=$digest_bytes >= legacy=$legacy_bytes (no compression)"
    fi
else
    pass "skipped (no real cycle workspace)"
fi

# --- Test 14: digest mode emits compact intent block ------------------------
header "Test 14 (Cycle B2): digest mode emits compact intent block"
if [ -n "$existing_cycle" ]; then
    out=$(EVOLVE_CONTEXT_DIGEST=1 bash "$RCB" scout "$existing_cycle" "$REPO_ROOT/.evolve/runs/cycle-$existing_cycle" 2>/dev/null)
    if echo "$out" | grep -q "Intent (compact" && echo "$out" | grep -q "Tier 2 digest"; then
        pass "compact intent block emitted with Tier 2 marker"
    else
        fail_ "compact intent block missing or unmarked"
    fi
else
    pass "skipped (no real cycle workspace)"
fi

# --- Test 15: legacy mode still emits full intent.md ------------------------
# Match the EXACT compact-mode marker "Intent (compact" with parenthesis,
# not the word "compact" alone (which can occur in real intent text —
# e.g. "compacted", "compaction", etc).
header "Test 15 (Cycle B2): legacy mode preserves full intent.md cat"
if [ -n "$existing_cycle" ]; then
    out=$(bash "$RCB" scout "$existing_cycle" "$REPO_ROOT/.evolve/runs/cycle-$existing_cycle" 2>/dev/null)
    if echo "$out" | grep -q "^## Intent$" && ! echo "$out" | grep -qF "Intent (compact"; then
        pass "legacy mode emits full Intent (no compact-mode marker)"
    else
        fail_ "legacy mode polluted with compact-mode marker OR missing Intent header"
    fi
else
    pass "skipped (no real cycle workspace)"
fi

# --- Summary -----------------------------------------------------------------
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="

[ "$FAIL" = "0" ]
