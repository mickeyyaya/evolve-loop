#!/usr/bin/env bash
# v9.1.0 Cycle 4: resume-cycle.sh validation tests.
#
# This exercises the structural correctness of:
#   - scripts/dispatch/resume-cycle.sh — locator, validator, dispatcher
#   - scripts/dispatch/evolve-loop-dispatch.sh — --resume flag parsing
#   - scripts/dispatch/run-cycle.sh — RESUME-MODE branches
#
# We can't run an actual end-to-end resume in CI (requires the claude binary
# and a working sub-shell), but we DO verify:
#   - resume-cycle.sh exists, is executable, has correct shebang
#   - --resume flag is parsed by the dispatcher and triggers resume-cycle.sh
#   - resume-cycle.sh validates checkpoint state correctly (exit 2 when none,
#     exit 1 on git HEAD drift, exit 1 on missing worktree)
#   - run-cycle.sh's RESUME-MODE branch skips init + worktree provision

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
RESUME_CYCLE="$PROJECT_ROOT/scripts/dispatch/resume-cycle.sh"
DISPATCH="$PROJECT_ROOT/scripts/dispatch/evolve-loop-dispatch.sh"
RUN_CYCLE="$PROJECT_ROOT/scripts/dispatch/run-cycle.sh"
CYCLE_STATE_HELPER="$PROJECT_ROOT/scripts/lifecycle/cycle-state.sh"

PASS=0
FAIL=0

expect() {
    local label="$1" actual="$2" expected="$3"
    if [ "$actual" = "$expected" ]; then
        printf "  PASS: %s\n" "$label"; PASS=$((PASS + 1))
    else
        printf "  FAIL: %s (expected=%s actual=%s)\n" "$label" "$expected" "$actual" >&2
        FAIL=$((FAIL + 1))
    fi
}

expect_match() {
    local label="$1" actual="$2" pattern="$3"
    if [[ "$actual" =~ $pattern ]]; then
        printf "  PASS: %s\n" "$label"; PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    pattern=%s\n" "$label" "$pattern" >&2
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Test 1: resume-cycle.sh exists and is executable ==="
[ -x "$RESUME_CYCLE" ] && expect "resume-cycle.sh executable" "yes" "yes" \
    || expect "resume-cycle.sh executable" "no" "yes"
head -1 "$RESUME_CYCLE" | grep -q '#!/usr/bin/env bash' \
    && expect "shebang correct" "yes" "yes" || expect "shebang correct" "no" "yes"

echo
echo "=== Test 2: resume-cycle.sh structural elements ==="
src=$(cat "$RESUME_CYCLE")
expect_match "uses is-checkpointed" "$src" "is-checkpointed"
expect_match "reads resumeFromPhase" "$src" "resumeFromPhase"
expect_match "validates git HEAD" "$src" "rev-parse HEAD"
expect_match "validates worktree directory" "$src" "WORKTREE.*-d"
expect_match "exports EVOLVE_RESUME_MODE=1" "$src" "EVOLVE_RESUME_MODE=1"
expect_match "spawns run-cycle.sh" "$src" "bash.*RUN_CYCLE"
expect_match "exit 2 when no checkpoint" "$src" "exit 2"
expect_match "EVOLVE_RESUME_ALLOW_HEAD_MOVED escape hatch" "$src" "EVOLVE_RESUME_ALLOW_HEAD_MOVED"

echo
echo "=== Test 3: dispatcher --resume flag parsing ==="
src=$(cat "$DISPATCH")
expect_match "--resume flag defined" "$src" "--resume\\)"
expect_match "RESUME_MODE=0 initialized" "$src" "RESUME_MODE=0"
expect_match "RESUME_MODE=1 set by flag" "$src" "RESUME_MODE=1"
expect_match "delegates to resume-cycle.sh" "$src" "RESUME_SCRIPT=.*resume-cycle.sh"

echo
echo "=== Test 4: run-cycle.sh RESUME-MODE branch ==="
src=$(cat "$RUN_CYCLE")
expect_match "EVOLVE_RESUME_MODE check" "$src" "EVOLVE_RESUME_MODE:-0"
expect_match "skips cycle_state_init" "$src" "skipping cycle_state_init"
expect_match "validates checkpoint exists" "$src" "RESUME-MODE.*checkpoint"
expect_match "SKIP_NORMAL_INIT flag set" "$src" "SKIP_NORMAL_INIT=1"
expect_match "worktree-provision gated" "$src" "SKIP_NORMAL_INIT.*= ?.1.?"

echo
echo "=== Test 5: resume-cycle.sh exit 2 when no cycle-state ==="
# Run resume-cycle.sh against a non-existent state dir.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM
EVOLVE_PROJECT_ROOT="$TMP" bash "$RESUME_CYCLE" >/dev/null 2>&1
rc=$?
expect "exit 2 when state missing" "$rc" "2"

echo
echo "=== Test 6: resume-cycle.sh exit 2 when state exists but no checkpoint ==="
mkdir -p "$TMP/.evolve"
cat > "$TMP/.evolve/cycle-state.json" <<'EOF'
{"cycle_id":99,"phase":"build","completed_phases":["calibrate"]}
EOF
EVOLVE_PROJECT_ROOT="$TMP" bash "$RESUME_CYCLE" >/dev/null 2>&1
rc=$?
expect "exit 2 when no checkpoint block" "$rc" "2"

echo
echo "=== Test 7: resume-cycle.sh exit 1 when git HEAD drifted ==="
# Stage a state with a checkpoint but a stale gitHead.
cat > "$TMP/.evolve/cycle-state.json" <<'EOF'
{
  "cycle_id": 99,
  "phase": "build",
  "completed_phases": ["calibrate","intent","research","triage"],
  "active_worktree": "/tmp/nonexistent-cycle-99",
  "checkpoint": {
    "enabled": true,
    "reason": "quota-likely",
    "savedAt": "2026-05-11T00:00:00Z",
    "resumeFromPhase": "build",
    "worktreePath": "/tmp/nonexistent-cycle-99",
    "completedPhases": ["calibrate","intent","research","triage"],
    "gitHead": "deadbeefcafebabe1234567890abcdef00000000",
    "costAtCheckpoint": 4.32
  }
}
EOF
# Set up a fake git repo (needed by rev-parse).
(cd "$TMP" && git init -q && git config user.email t@t.t && git config user.name t \
     && touch x && git add x && git commit -q -m i)
EVOLVE_PROJECT_ROOT="$TMP" bash "$RESUME_CYCLE" >/dev/null 2>&1
rc=$?
expect "exit 1 when git HEAD drifted" "$rc" "1"

echo
echo "=== Test 8: resume-cycle.sh exit 1 when worktree missing ==="
# Update the state to have a checkpoint with the actual git HEAD, but
# a worktree path that doesn't exist on disk.
CURRENT_HEAD=$(cd "$TMP" && git rev-parse HEAD)
jq --arg head "$CURRENT_HEAD" '.checkpoint.gitHead = $head' "$TMP/.evolve/cycle-state.json" \
    > "$TMP/.evolve/cycle-state.json.tmp" && mv "$TMP/.evolve/cycle-state.json.tmp" "$TMP/.evolve/cycle-state.json"
EVOLVE_PROJECT_ROOT="$TMP" bash "$RESUME_CYCLE" >/dev/null 2>&1
rc=$?
expect "exit 1 when worktree missing" "$rc" "1"

echo
echo "=== Test 9: syntax checks ==="
bash -n "$RESUME_CYCLE" 2>&1 && expect "resume-cycle.sh syntax" "ok" "ok" \
    || expect "resume-cycle.sh syntax" "FAIL" "ok"
bash -n "$DISPATCH" 2>&1 && expect "dispatcher syntax" "ok" "ok" \
    || expect "dispatcher syntax" "FAIL" "ok"
bash -n "$RUN_CYCLE" 2>&1 && expect "run-cycle.sh syntax" "ok" "ok" \
    || expect "run-cycle.sh syntax" "FAIL" "ok"

echo
echo "=== Summary ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"
if [ "$FAIL" -eq 0 ]; then
    echo "ALL TESTS PASSED"
    exit 0
else
    exit 1
fi
