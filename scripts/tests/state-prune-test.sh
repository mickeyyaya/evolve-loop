#!/usr/bin/env bash
#
# state-prune-test.sh — Unit tests for state-prune.sh (v8.22.0).
#
# Tests the operator-utility against synthetic state.json fixtures via
# EVOLVE_STATE_FILE_OVERRIDE. Covers all modes: classification, age, cycle,
# all (refuse without --yes), dry-run preview, no-state-file edge case.
#
# Usage: bash scripts/state-prune-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/state-prune.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

cleanup_files=()
trap 'for f in "${cleanup_files[@]}"; do rm -f "$f"; done' EXIT

# Build a fixture state.json with N entries of varying classifications and ages.
make_fixture() {
    local f
    f=$(mktemp -t state-prune-test.XXXXXX.json)
    cleanup_files+=("$f")
    local now_s
    now_s=$(date -u +%s)
    local d2_s d10_s
    d2_s=$(( now_s - 2 * 86400 ))    # 2 days ago
    d10_s=$(( now_s - 10 * 86400 ))  # 10 days ago

    # macOS date and GNU date have different `-d`/`-r` semantics. Use python-free
    # ISO formatting via printf-of-jq's `todate` (after fromdateiso8601-roundtrip).
    iso_of() {
        local s="$1"
        echo "$s" | jq -r '. | todate'
    }
    local now_iso d2_iso d10_iso
    now_iso=$(iso_of "$now_s")
    d2_iso=$(iso_of "$d2_s")
    d10_iso=$(iso_of "$d10_s")

    cat > "$f" <<EOF
{
  "lastCycleNumber": 30,
  "failedApproaches": [
    {"cycle": 21, "classification": "infrastructure-transient", "summary": "old infra", "recordedAt": "$d10_iso"},
    {"cycle": 22, "classification": "infrastructure-transient", "summary": "less old infra", "recordedAt": "$d2_iso"},
    {"cycle": 23, "classification": "code-build-fail", "summary": "build broken", "recordedAt": "$d10_iso"},
    {"cycle": 24, "classification": "code-audit-fail", "summary": "audit failed", "recordedAt": "$d2_iso"},
    {"cycle": 25, "classification": "infrastructure-transient", "summary": "fresh infra", "recordedAt": "$now_iso"}
  ]
}
EOF
    echo "$f"
}

# === Test 1: --classification removes only matching entries ==================
header "Test 1: --classification infrastructure-transient removes 3 of 5"
sf=$(make_fixture)
out=$(EVOLVE_STATE_FILE_OVERRIDE="$sf" bash "$SCRIPT" --classification infrastructure-transient 2>/dev/null)
removed=$(echo "$out" | jq -r '.removed')
after=$(echo "$out" | jq -r '.after')
remaining=$(jq -r '.failedApproaches | map(.classification) | unique | sort | join(",")' "$sf")
if [ "$removed" = "3" ] && [ "$after" = "2" ] && [ "$remaining" = "code-audit-fail,code-build-fail" ]; then
    pass "removed 3 infra-transient entries; 2 code entries remain"
else
    fail_ "removed=$removed after=$after remaining=$remaining"
fi

# === Test 2: --age 7d keeps only entries < 7 days old =========================
header "Test 2: --age 7d removes 2 entries older than 7d, keeps 3 newer"
sf=$(make_fixture)
out=$(EVOLVE_STATE_FILE_OVERRIDE="$sf" bash "$SCRIPT" --age 7d 2>/dev/null)
removed=$(echo "$out" | jq -r '.removed')
after=$(echo "$out" | jq -r '.after')
remaining_cycles=$(jq -r '.failedApproaches | map(.cycle | tostring) | join(",")' "$sf")
# cycles 21 and 23 are 10d old → removed; cycles 22, 24, 25 → kept
if [ "$removed" = "2" ] && [ "$after" = "3" ] && [ "$remaining_cycles" = "22,24,25" ]; then
    pass "removed 2 old entries, kept 3 fresher (cycles 22,24,25)"
else
    fail_ "removed=$removed after=$after remaining_cycles=$remaining_cycles"
fi

# === Test 3: --cycle removes one specific entry ==============================
header "Test 3: --cycle 24 removes only that cycle's entry"
sf=$(make_fixture)
out=$(EVOLVE_STATE_FILE_OVERRIDE="$sf" bash "$SCRIPT" --cycle 24 2>/dev/null)
removed=$(echo "$out" | jq -r '.removed')
remaining_cycles=$(jq -r '.failedApproaches | map(.cycle | tostring) | join(",")' "$sf")
if [ "$removed" = "1" ] && [ "$remaining_cycles" = "21,22,23,25" ]; then
    pass "removed cycle 24 only; 21,22,23,25 remain"
else
    fail_ "removed=$removed remaining=$remaining_cycles"
fi

# === Test 4: --all without --yes is refused ==================================
header "Test 4: --all without --yes returns rc=2, no mutation"
sf=$(make_fixture)
before=$(jq '.failedApproaches | length' "$sf")
set +e
EVOLVE_STATE_FILE_OVERRIDE="$sf" bash "$SCRIPT" --all 2>/dev/null
rc=$?
set -e
after=$(jq '.failedApproaches | length' "$sf")
if [ "$rc" = "2" ] && [ "$before" = "$after" ]; then
    pass "rc=2, no mutation (before=$before == after=$after)"
else
    fail_ "rc=$rc before=$before after=$after"
fi

# === Test 5: --all --yes wipes all entries ===================================
header "Test 5: --all --yes wipes all 5 entries"
sf=$(make_fixture)
out=$(EVOLVE_STATE_FILE_OVERRIDE="$sf" bash "$SCRIPT" --all --yes 2>/dev/null)
after=$(jq '.failedApproaches | length' "$sf")
removed=$(echo "$out" | jq -r '.removed')
if [ "$after" = "0" ] && [ "$removed" = "5" ]; then
    pass "all 5 removed; failedApproaches now empty"
else
    fail_ "after=$after removed=$removed"
fi

# === Test 6: --dry-run never mutates =========================================
header "Test 6: --dry-run preserves state file (preview only)"
sf=$(make_fixture)
sha_before=$(shasum -a 256 "$sf" | awk '{print $1}')
out=$(EVOLVE_STATE_FILE_OVERRIDE="$sf" bash "$SCRIPT" --classification infrastructure-transient --dry-run 2>/dev/null)
sha_after=$(shasum -a 256 "$sf" | awk '{print $1}')
removed_preview=$(echo "$out" | jq -r '.removed')
dry_run_flag=$(echo "$out" | jq -r '.dry_run')
if [ "$sha_before" = "$sha_after" ] && [ "$removed_preview" = "3" ] && [ "$dry_run_flag" = "true" ]; then
    pass "dry-run reported 3 would-be-removed; state file SHA unchanged"
else
    fail_ "sha_match=$([ "$sha_before" = "$sha_after" ] && echo y || echo n) removed_preview=$removed_preview dry_run=$dry_run_flag"
fi

# === Test 7: missing state file → graceful no-op =============================
header "Test 7: missing state file emits empty summary, rc=0"
sf=$(mktemp -t state-prune-test-missing.XXXXXX.json)
rm -f "$sf"   # ensure absent
cleanup_files+=("$sf")
out=$(EVOLVE_STATE_FILE_OVERRIDE="$sf" bash "$SCRIPT" --classification infrastructure-transient 2>/dev/null)
rc=$?
before=$(echo "$out" | jq -r '.before')
reason=$(echo "$out" | jq -r '.reason // empty')
if [ "$rc" = "0" ] && [ "$before" = "0" ] && [ "$reason" = "no-state-file" ]; then
    pass "missing state file → before=0, reason='no-state-file', rc=0"
else
    fail_ "rc=$rc before=$before reason=$reason"
fi

# === Summary =================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
