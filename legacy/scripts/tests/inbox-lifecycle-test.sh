#!/usr/bin/env bash
# inbox-lifecycle-test.sh — Inbox lifecycle (c37) unit tests.
# Tests inbox-mover.sh subcommands: claim, promote, recover-orphans.
# 7 scenarios covering happy path + error paths + multi-project isolation.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MOVER="$REPO_ROOT/scripts/utility/inbox-mover.sh"
SCRATCH=$(mktemp -d)

PASS=0; FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

cleanup() { rm -rf "$SCRATCH"; }
trap cleanup EXIT

# Create a minimal project root with state.json and ledger.jsonl
make_project() {
    local root="$SCRATCH/proj-$RANDOM"
    mkdir -p "$root/.evolve/inbox"
    printf '{"carryoverTodos":[],"failedApproaches":[]}\n' > "$root/.evolve/state.json"
    touch "$root/.evolve/ledger.jsonl"
    # Bootstrap a git repo for git-log checks
    git -C "$root" init -q 2>/dev/null
    git -C "$root" config user.email "test@test.com"
    git -C "$root" config user.name "Test"
    touch "$root/.evolve/.gitkeep"
    git -C "$root" add -A 2>/dev/null
    git -C "$root" commit -q -m "init" 2>/dev/null
    echo "$root"
}

# Write a minimal inbox JSON file for the given task_id
make_inbox_file() {
    local dir="$1" task_id="$2"
    local ts
    ts=$(date -u +%Y-%m-%dT%H-%M-%SZ 2>/dev/null || echo "2026-01-01T00-00-00Z")
    local rand="abcd1234"
    local fname="${ts}-${rand}.json"
    printf '{"id":"%s","action":"test action","priority":"MEDIUM","weight":0.5,"injected_at":"%s","injected_by":"test"}\n' \
        "$task_id" "$ts" > "$dir/$fname"
    echo "$dir/$fname"
}

# ── Test A: fresh claim → promote to processed/ ───────────────────────────────
header "Test A: claim → promote processed/"
PROJ=$(make_project)
INBOX="$PROJ/.evolve/inbox"
make_inbox_file "$INBOX" "c-test-A" >/dev/null

# Step 1: claim
EVOLVE_PROJECT_ROOT="$PROJ" bash "$MOVER" claim "c-test-A" 30 2>/dev/null
rc=$?
[ "$rc" -eq 0 ] && pass "A: claim exits 0" || fail "A: claim exited $rc"

# Assert: file is in processing/cycle-30/
found=$(find "$INBOX/processing/cycle-30" -name "*.json" 2>/dev/null | head -1)
[ -n "$found" ] && pass "A: file in processing/cycle-30/" || fail "A: file NOT in processing/cycle-30/"

# Assert: file is NOT in inbox/ anymore
remaining=$(find "$INBOX" -maxdepth 1 -name "*.json" 2>/dev/null | wc -l | tr -d ' ')
[ "$remaining" -eq 0 ] && pass "A: inbox/ empty after claim" || fail "A: inbox/ still has $remaining file(s)"

# Step 2: promote to processed with a commit sha
EVOLVE_PROJECT_ROOT="$PROJ" bash "$MOVER" promote "c-test-A" processed 30 --commit-sha "abc12345" 2>/dev/null
rc=$?
[ "$rc" -eq 0 ] && pass "A: promote exits 0" || fail "A: promote exited $rc"

# Assert: file in processed/cycle-30/ with sha8 prefix
proc=$(find "$INBOX/processed/cycle-30" -name "abc12345-*.json" 2>/dev/null | head -1)
[ -n "$proc" ] && pass "A: file in processed/cycle-30/ with sha8 prefix" || fail "A: file NOT in processed/cycle-30/ with sha8 prefix"

# Assert: ledger has inbox-lifecycle entry
ledger_count=$(grep -c '"class":"inbox-lifecycle"' "$PROJ/.evolve/ledger.jsonl" 2>/dev/null || echo 0)
[ "$ledger_count" -ge 2 ] && pass "A: ledger has ≥2 inbox-lifecycle entries" || fail "A: ledger has only $ledger_count inbox-lifecycle entries"

# ── Test B: git-log idempotency check pattern (unit) ─────────────────────────
header "Test B: git-log idempotency pattern matches shipped task"
PROJ=$(make_project)
TASK_ID="c-test-B-shipped"

# Commit a message that matches the feat: cycle N — <task_id>: pattern
git -C "$PROJ" commit -q --allow-empty \
    -m "feat: cycle 99 — ${TASK_ID}: some feature" 2>/dev/null

# Test the grep pattern that Triage Step 0a uses
shipped_sha=$(git -C "$PROJ" log \
    --grep="^feat: cycle [0-9]\+ — ${TASK_ID}\(:\| \)" \
    --format="%H" main 2>/dev/null | head -1 || true)
[ -n "$shipped_sha" ] && pass "B: git-log pattern finds shipped task sha" || fail "B: git-log pattern returned empty for shipped task"

# Also verify a non-shipped task returns empty
nonshipped=$(git -C "$PROJ" log \
    --grep="^feat: cycle [0-9]\+ — c-not-shipped\(:\| \)" \
    --format="%H" main 2>/dev/null | head -1 || true)
[ -z "$nonshipped" ] && pass "B: git-log returns empty for non-shipped task" || fail "B: git-log wrongly matched non-shipped task"

# ── Test C: promote to rejected/ ─────────────────────────────────────────────
header "Test C: promote to rejected/"
PROJ=$(make_project)
INBOX="$PROJ/.evolve/inbox"
make_inbox_file "$INBOX" "c-test-C" >/dev/null

EVOLVE_PROJECT_ROOT="$PROJ" bash "$MOVER" claim "c-test-C" 30 2>/dev/null
EVOLVE_PROJECT_ROOT="$PROJ" bash "$MOVER" promote "c-test-C" rejected 30 2>/dev/null
rc=$?
[ "$rc" -eq 0 ] && pass "C: promote rejected exits 0" || fail "C: promote rejected exited $rc"

rej=$(find "$INBOX/rejected/cycle-30" -name "*.json" 2>/dev/null | head -1)
[ -n "$rej" ] && pass "C: file in rejected/cycle-30/" || fail "C: file NOT in rejected/cycle-30/"

# ── Test D: promote failure (chmod 000 dest) → WARN + ship rc=0 ──────────────
header "Test D: mv failure → WARN + exit 0 (ship.sh compat)"
PROJ=$(make_project)
INBOX="$PROJ/.evolve/inbox"
make_inbox_file "$INBOX" "c-test-D" >/dev/null

EVOLVE_PROJECT_ROOT="$PROJ" bash "$MOVER" claim "c-test-D" 30 2>/dev/null

# Create dest dir but remove write permission to force mv failure
mkdir -p "$INBOX/processed/cycle-30"
chmod 000 "$INBOX/processed/cycle-30"

EVOLVE_PROJECT_ROOT="$PROJ" bash "$MOVER" promote "c-test-D" processed 30 --commit-sha "deadbeef" 2>/dev/null
rc=$?
# Must exit 0 (never block ship)
[ "$rc" -eq 0 ] && pass "D: promote exits 0 despite mv failure" || fail "D: promote exited $rc (expected 0)"

# Restore perms for cleanup
chmod 755 "$INBOX/processed/cycle-30"

# File should still exist in processing/ (not moved)
still_there=$(find "$INBOX/processing/cycle-30" -name "*.json" 2>/dev/null | wc -l | tr -d ' ')
[ "$still_there" -ge 1 ] && pass "D: file remains in processing/ after mv failure" || fail "D: file vanished from processing/ after mv failure"

# ── Test E: multi-project isolation ──────────────────────────────────────────
header "Test E: multi-project isolation"
PROJ1=$(make_project)
PROJ2=$(make_project)
make_inbox_file "$PROJ1/.evolve/inbox" "c-test-E" >/dev/null

# Claim in PROJ1 should not affect PROJ2
EVOLVE_PROJECT_ROOT="$PROJ1" bash "$MOVER" claim "c-test-E" 30 2>/dev/null

proj2_inbox=$(find "$PROJ2/.evolve/inbox" -maxdepth 1 -name "*.json" 2>/dev/null | wc -l | tr -d ' ')
[ "$proj2_inbox" -eq 0 ] && pass "E: PROJ2 inbox unaffected by PROJ1 claim" || fail "E: PROJ2 inbox has $proj2_inbox unexpected file(s)"

proj1_proc=$(find "$PROJ1/.evolve/inbox/processing/cycle-30" -name "*.json" 2>/dev/null | head -1)
[ -n "$proj1_proc" ] && pass "E: PROJ1 file correctly in processing/cycle-30/" || fail "E: PROJ1 file not found in processing/"

# ── Test F: crash recovery — orphan in dead cycle ────────────────────────────
header "Test F: recover-orphans moves dead-cycle files back to inbox/"
PROJ=$(make_project)
INBOX="$PROJ/.evolve/inbox"

# Simulate orphan: file stuck in processing/cycle-99/ but cycle-state shows cycle=30
mkdir -p "$INBOX/processing/cycle-99"
ts=$(date -u +%Y-%m-%dT%H-%M-%SZ 2>/dev/null || echo "2026-01-01T00-00-00Z")
printf '{"id":"c-orphan-99","action":"orphaned","priority":"LOW"}\n' \
    > "$INBOX/processing/cycle-99/${ts}-orphan.json"

# Write cycle-state.json showing active cycle = 30
printf '{"cycle_id":30,"phase":"build"}\n' > "$PROJ/.evolve/cycle-state.json"

EVOLVE_PROJECT_ROOT="$PROJ" bash "$MOVER" recover-orphans 2>/dev/null
rc=$?
[ "$rc" -eq 0 ] && pass "F: recover-orphans exits 0" || fail "F: recover-orphans exited $rc"

# Assert: file is back in inbox/
recovered=$(find "$INBOX" -maxdepth 1 -name "*orphan*.json" 2>/dev/null | head -1)
[ -n "$recovered" ] && pass "F: orphan file moved back to inbox/" || fail "F: orphan file NOT in inbox/"

# Assert: processing/cycle-99/ is empty
orphan_left=$(find "$INBOX/processing/cycle-99" -name "*.json" 2>/dev/null | wc -l | tr -d ' ')
[ "$orphan_left" -eq 0 ] && pass "F: processing/cycle-99/ is empty after recovery" || fail "F: processing/cycle-99/ still has $orphan_left file(s)"

# ── Test G: race-safety — active cycle's files untouched ─────────────────────
header "Test G: recover-orphans leaves active-cycle files alone"
PROJ=$(make_project)
INBOX="$PROJ/.evolve/inbox"

# File in processing/cycle-30/ with active cycle = 30
mkdir -p "$INBOX/processing/cycle-30"
ts=$(date -u +%Y-%m-%dT%H-%M-%SZ 2>/dev/null || echo "2026-01-01T00-00-00Z")
printf '{"id":"c-active-30","action":"in-progress","priority":"HIGH"}\n' \
    > "$INBOX/processing/cycle-30/${ts}-active.json"

printf '{"cycle_id":30,"phase":"build"}\n' > "$PROJ/.evolve/cycle-state.json"

EVOLVE_PROJECT_ROOT="$PROJ" bash "$MOVER" recover-orphans 2>/dev/null

# Assert: active file still in processing/cycle-30/ (not moved back)
active_there=$(find "$INBOX/processing/cycle-30" -name "*.json" 2>/dev/null | wc -l | tr -d ' ')
[ "$active_there" -ge 1 ] && pass "G: active cycle file untouched by recover-orphans" || fail "G: active cycle file was wrongly moved back to inbox/"

active_in_inbox=$(find "$INBOX" -maxdepth 1 -name "*active*.json" 2>/dev/null | wc -l | tr -d ' ')
[ "$active_in_inbox" -eq 0 ] && pass "G: inbox/ has no active-cycle file after recover-orphans" || fail "G: inbox/ has active-cycle file (should be in processing/)"

# ── Summary ───────────────────────────────────────────────────────────────────
echo
echo "inbox-lifecycle-test.sh — $PASS/$((PASS + FAIL)) PASS"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
