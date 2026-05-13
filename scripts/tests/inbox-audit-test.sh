#!/usr/bin/env bash
# inbox-audit-test.sh — Smoke tests for inbox-audit.sh + inbox-reconcile.sh (v9.6.1+)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AUDIT="$REPO_ROOT/scripts/utility/inbox-audit.sh"
RECONCILE="$REPO_ROOT/scripts/utility/inbox-reconcile.sh"
SCRATCH=$(mktemp -d)

PASS=0; FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

cleanup() { rm -rf "$SCRATCH"; }
trap cleanup EXIT

make_project() {
    local root="$SCRATCH/proj-$RANDOM"
    mkdir -p "$root/.evolve/inbox"
    echo "$root"
}

# --- Test 1: inbox-audit.sh is executable ------------------------------------
header "Test 1: inbox-audit.sh exists and is executable"
[ -x "$AUDIT" ] && pass "inbox-audit.sh is executable" || fail "inbox-audit.sh missing or not executable"

# --- Test 2: --help exits 0 --------------------------------------------------
header "Test 2: inbox-audit.sh --help exits 0"
EVOLVE_PROJECT_ROOT="$SCRATCH" bash "$AUDIT" --help >/dev/null 2>&1
[ $? -eq 0 ] && pass "--help exits 0" || fail "--help did not exit 0"

# --- Test 3: exits 0 on empty inbox ------------------------------------------
header "Test 3: inbox-audit.sh exits 0 on empty inbox"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$AUDIT" >/dev/null 2>&1
[ $? -eq 0 ] && pass "exits 0 on empty inbox" || fail "non-zero exit on empty inbox"

# --- Test 4: --json flag produces valid JSON array ---------------------------
header "Test 4: --json produces valid JSON array"
PROJ=$(make_project)
out=$(EVOLVE_PROJECT_ROOT="$PROJ" bash "$AUDIT" --json 2>/dev/null)
echo "$out" | jq -e '. | type == "array"' >/dev/null 2>&1 && \
    pass "--json output is a JSON array" || \
    fail "--json output is not a valid JSON array; got: $out"

# --- Test 5: --json with queued items shows task_id --------------------------
header "Test 5: --json shows queued inbox items"
PROJ=$(make_project)
printf '{"id":"test-task-99","action":"do something","priority":"HIGH","injected_at":"2026-01-01T00:00:00Z","injected_by":"operator","weight":null,"evidence_pointer":null,"operator_note":null}\n' \
    > "$PROJ/.evolve/inbox/test-task-99.json"
out=$(EVOLVE_PROJECT_ROOT="$PROJ" bash "$AUDIT" --json 2>/dev/null)
task_id=$(echo "$out" | jq -r '.[0].task_id' 2>/dev/null)
state=$(echo "$out" | jq -r '.[0].state' 2>/dev/null)
[ "$task_id" = "test-task-99" ] && pass "task_id visible in --json output" || fail "expected test-task-99, got '$task_id'"
[ "$state" = "queued" ] && pass "state=queued for inbox item" || fail "expected state=queued, got '$state'"

# --- Test 6: inbox-reconcile.sh is executable --------------------------------
header "Test 6: inbox-reconcile.sh exists and is executable"
[ -x "$RECONCILE" ] && pass "inbox-reconcile.sh is executable" || fail "inbox-reconcile.sh missing or not executable"

# --- Test 7: inbox-reconcile.sh --help exits 0 -------------------------------
header "Test 7: inbox-reconcile.sh --help exits 0"
EVOLVE_PROJECT_ROOT="$SCRATCH" bash "$RECONCILE" --help >/dev/null 2>&1
[ $? -eq 0 ] && pass "--help exits 0" || fail "--help did not exit 0"

# --- Test 8: --recover-all-orphans exits 0 when no orphans -------------------
header "Test 8: --recover-all-orphans exits 0 when no orphans exist"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$RECONCILE" --recover-all-orphans >/dev/null 2>&1
[ $? -eq 0 ] && pass "--recover-all-orphans exits 0 with no orphans" || fail "unexpected non-zero exit"

# --- Summary ------------------------------------------------------------------
echo
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
