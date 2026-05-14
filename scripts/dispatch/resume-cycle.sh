#!/usr/bin/env bash
#
# resume-cycle.sh — Restart a checkpointed evolve-loop cycle (v9.1.0).
#
# WHY THIS EXISTS
#
# Pre-v9.1.0, a cycle that hit a Claude Code subscription quota wall (or any
# other rc=1 in a phase) lost ALL in-flight work: run-cycle.sh's EXIT trap
# unconditionally deleted the worktree, cleared cycle-state.json, and the
# dispatcher's batch loop aborted. v9.1.0 added checkpoint writes (Cycle 1),
# pre-emptive thresholds (Cycle 2), and reactive classification (Cycle 3) —
# this script (Cycle 4) closes the loop by allowing the operator to type
# `--resume` and pick up the paused cycle from its last clean phase boundary.
#
# Usage:
#   bash scripts/dispatch/resume-cycle.sh
#
# What it does (in order):
#   1. Locate the live checkpoint: $EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json
#      must have checkpoint.enabled == true.
#   2. Validate the checkpoint:
#      - git HEAD matches checkpoint.gitHead (no commits since pause)
#      - worktree path exists on disk
#      - cycle-state.json has all expected fields
#   3. Print a summary of what's being resumed (cycle N, phase X, cost Y).
#   4. Re-spawn the orchestrator subagent with EVOLVE_RESUME_MODE=1 in env.
#      The orchestrator persona (updated in v9.1.0 Cycle 5) reads this and
#      skips completed_phases[] before continuing.
#   5. On exit, the orchestrator decides whether to clear the checkpoint
#      block (if it ran to completion) or keep it (if it checkpointed again).
#
# Exit codes:
#   0  — resumed cycle ran to terminal phase (ship / retrospective / clear-block)
#   1  — validation failure (stale git HEAD, missing worktree, malformed state)
#   2  — no live checkpoint found
#   3  — orchestrator subagent crashed during resume
#  127  — required binary missing (jq, claude)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/../lifecycle/resolve-roots.sh"

log()  { echo "[resume-cycle] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

command -v jq >/dev/null 2>&1 || { log "missing jq"; exit 127; }

STATE_FILE="$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json"
CYCLE_STATE_HELPER="$EVOLVE_PLUGIN_ROOT/scripts/lifecycle/cycle-state.sh"
[ -f "$CYCLE_STATE_HELPER" ] || CYCLE_STATE_HELPER="$EVOLVE_PROJECT_ROOT/scripts/lifecycle/cycle-state.sh"
RUN_CYCLE="$EVOLVE_PLUGIN_ROOT/scripts/dispatch/run-cycle.sh"
[ -f "$RUN_CYCLE" ] || RUN_CYCLE="$EVOLVE_PROJECT_ROOT/scripts/dispatch/run-cycle.sh"

# --- Step 1: locate checkpoint ---------------------------------------------

if [ ! -f "$STATE_FILE" ]; then
    log "no cycle-state.json found at $STATE_FILE — nothing to resume"
    log "  (was the cycle ever checkpointed? did cleanup already clear it?)"
    exit 2
fi

if ! bash "$CYCLE_STATE_HELPER" is-checkpointed >/dev/null 2>&1; then
    log "cycle-state.json exists but checkpoint.enabled != true"
    log "  this cycle was either: (a) cleaned up normally, (b) never checkpointed,"
    log "  or (c) the checkpoint block was manually removed"
    log "  ad-hoc inspection: cat $STATE_FILE | jq .checkpoint"
    exit 2
fi

# --- Step 2: validate ------------------------------------------------------

CYCLE=$(jq -r '.cycle_id // .cycle // empty' "$STATE_FILE")
PHASE=$(jq -r '.checkpoint.resumeFromPhase // empty' "$STATE_FILE")
WORKTREE=$(jq -r '.checkpoint.worktreePath // empty' "$STATE_FILE")
GIT_HEAD_AT_PAUSE=$(jq -r '.checkpoint.gitHead // empty' "$STATE_FILE")
REASON=$(jq -r '.checkpoint.reason // "unknown"' "$STATE_FILE")
SAVED_AT=$(jq -r '.checkpoint.savedAt // "unknown"' "$STATE_FILE")
COST_AT_PAUSE=$(jq -r '.checkpoint.costAtCheckpoint // 0' "$STATE_FILE")
COMPLETED_PHASES=$(jq -r '.checkpoint.completedPhases | join(",") // ""' "$STATE_FILE")

[ -n "$CYCLE" ] || fail "checkpoint malformed: missing cycle_id"
[ -n "$PHASE" ] || fail "checkpoint malformed: missing resumeFromPhase"

# Validate git HEAD hasn't moved (resume only safe at the same commit).
# Operator override: EVOLVE_RESUME_ALLOW_HEAD_MOVED=1 forces the resume
# despite HEAD-drift; use sparingly and only when the operator understands
# the implications (e.g., a hot-fix was committed between pause and resume,
# and the worktree's tree still merges cleanly).
CURRENT_HEAD=$(git -C "$EVOLVE_PROJECT_ROOT" rev-parse HEAD 2>/dev/null || echo "unknown")
if [ "$GIT_HEAD_AT_PAUSE" != "unknown" ] && [ "$CURRENT_HEAD" != "$GIT_HEAD_AT_PAUSE" ]; then
    if [ "${EVOLVE_RESUME_ALLOW_HEAD_MOVED:-0}" != "1" ]; then
        log "STALE: git HEAD moved since checkpoint"
        log "  paused at: $GIT_HEAD_AT_PAUSE"
        log "  current  : $CURRENT_HEAD"
        log "  override: EVOLVE_RESUME_ALLOW_HEAD_MOVED=1 to proceed anyway (risky)"
        exit 1
    fi
    log "WARN: git HEAD moved — proceeding under EVOLVE_RESUME_ALLOW_HEAD_MOVED=1"
fi

# Validate worktree still exists. If not, the resume is doomed because
# Builder's edits live there, not in the main repo.
if [ -n "$WORKTREE" ] && [ "$WORKTREE" != "null" ] && [ ! -d "$WORKTREE" ]; then
    log "STALE: worktree no longer exists at $WORKTREE"
    log "  the v9.1 EXIT-trap preserves the worktree on checkpoint, so this"
    log "  means someone deleted it manually OR the disk got cleaned up"
    log "  recovery: cannot resume this cycle. Run /evolve-loop fresh."
    exit 1
fi

# --- Step 2b: auto-resume attempt cap (v10.6.0) ----------------------------
#
# Each /evolve-loop --resume invocation consumes one auto-resume attempt.
# The cap (checkpoint.autoResumeMaxAttempts, default 3) prevents infinite
# quota-resume-quota loops. If exhausted, refuse without running and leave
# the checkpoint marker for operator intervention. The counter is preserved
# across re-checkpoints (cycle_state_checkpoint copies the old value) so a
# chain of 3 consecutive quota hits hits the cap correctly. On full success
# the counter resets (Step 5b below).
bash "$CYCLE_STATE_HELPER" bump-auto-resume-attempts
bump_rc=$?
if [ "$bump_rc" -ne 0 ]; then
    if [ "$bump_rc" = "2" ]; then
        log "AUTO-RESUME EXHAUSTED: this cycle has used its autoResumeMaxAttempts budget."
        log "  the checkpoint remains intact. To override, increment the cap and re-invoke:"
        log "    jq '.checkpoint.autoResumeMaxAttempts = 5' $STATE_FILE > $STATE_FILE.tmp && mv $STATE_FILE.tmp $STATE_FILE"
        log "    bash scripts/dispatch/resume-cycle.sh"
        log "  or inspect what's going wrong:"
        log "    cat $STATE_FILE | jq .checkpoint"
        exit 2
    fi
    fail "bump-auto-resume-attempts failed unexpectedly (rc=$bump_rc)"
fi

# --- Step 3: summary -------------------------------------------------------

log "RESUME: cycle $CYCLE"
log "  paused phase    : $PHASE"
log "  completed phases: $COMPLETED_PHASES"
log "  pause reason    : $REASON"
log "  paused at       : $SAVED_AT"
log "  cost at pause   : \$$COST_AT_PAUSE"
log "  worktree        : $WORKTREE"
log "  git HEAD        : $CURRENT_HEAD"

# --- Step 4: re-spawn orchestrator ----------------------------------------

# Hand the orchestrator the resume signal via env vars. The orchestrator
# persona reads these and:
#   - skips any phase in EVOLVE_RESUME_COMPLETED_PHASES
#   - starts at EVOLVE_RESUME_PHASE
#   - clears the checkpoint block on its first successful phase write
#     (signaling the cycle is no longer paused)
export EVOLVE_RESUME_MODE=1
export EVOLVE_RESUME_PHASE="$PHASE"
export EVOLVE_RESUME_CYCLE="$CYCLE"
export EVOLVE_RESUME_COMPLETED_PHASES="$COMPLETED_PHASES"
# Clear any stale pre-emption signal so we don't immediately re-checkpoint.
unset EVOLVE_CHECKPOINT_REQUEST EVOLVE_CHECKPOINT_TRIGGERED 2>/dev/null || true

# Pull the original goal from state.json if available — pre-v9.1.0 cycles
# didn't store goal, so this is best-effort. Fallback: empty goal, the
# orchestrator persona will read state.json's stored proposals/intent.
GOAL_FROM_STATE=""
STATE_JSON="$EVOLVE_PROJECT_ROOT/.evolve/state.json"
if [ -f "$STATE_JSON" ]; then
    GOAL_FROM_STATE=$(jq -r '.lastGoal // .currentGoal // ""' "$STATE_JSON" 2>/dev/null || echo "")
fi

log "spawning run-cycle.sh in resume mode (cycle=$CYCLE phase=$PHASE)"
EVOLVE_RESUME_MODE=1 \
EVOLVE_RESUME_PHASE="$PHASE" \
EVOLVE_RESUME_CYCLE="$CYCLE" \
EVOLVE_RESUME_COMPLETED_PHASES="$COMPLETED_PHASES" \
bash "$RUN_CYCLE" "$GOAL_FROM_STATE"
rc=$?

if [ "$rc" -ne 0 ]; then
    log "orchestrator subagent exited rc=$rc during resume"
    # If the resume itself crashed, the checkpoint block survives (run-cycle's
    # EXIT trap honors it) so a second --resume invocation can retry. The
    # operator may need to investigate the root cause first. The
    # autoResumeAttempts counter is NOT reset on failure — the cap accumulates.
    exit 3
fi

# --- Step 5b: full success — reset auto-resume budget for future cycles ----
#
# The cycle ran to completion (orchestrator cleared the checkpoint on the
# terminal phase). If a fresh quota hit happens on a future cycle, it should
# get a clean retry budget. Reset is best-effort: if cycle-state was already
# cleared by the orchestrator, the helper exits non-zero and we ignore it.
bash "$CYCLE_STATE_HELPER" reset-auto-resume-attempts 2>/dev/null || true

log "RESUME: cycle $CYCLE completed successfully"
exit 0
