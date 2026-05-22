#!/usr/bin/env bash
#
# cycle-release.sh — Canonical per-cycle release on terminal exit.
#
# Plan reference: ~/.claude/plans/linked-meandering-lobster.md Step 6.
#
# WHY THIS EXISTS
#
# Pre-v10.x, run-cycle.sh's cleanup() trap did the right thing for happy-path
# terminal exits (PASS/FAIL/non-checkpointed) — worktree removed, cycle-state
# cleared. But:
#   - There was no auditable ledger marker saying "this cycle is truly done."
#     Reading the ledger, an operator couldn't distinguish "cycle 79 ended
#     cleanly" from "cycle 79's last entry happened to be auditor; orchestrator
#     hung and got SIGKILLed."
#   - The release operations were locked inside a trap; manual release after
#     an emergency abort required reproducing the trap logic by hand.
#   - The "promote terminal-exit trap" requirement of the plan (Step 4) needed
#     a callable script to invoke — not just inlined trap logic.
#
# This script provides that callable entry point. It's idempotent over the
# operations run-cycle.sh's cleanup also performs (worktree removal, state
# clear), and adds the new auditable thing: a `role:release` ledger entry
# that closes out the cycle on the SHA-chain.
#
# WHAT IT DOES (in order)
#
#   1. Resolve roots (EVOLVE_PLUGIN_ROOT for scripts; EVOLVE_PROJECT_ROOT for
#      state/ledger/runs).
#   2. If a checkpoint is active, skip release (cycle is paused, not done).
#   3. Remove worktree at cycle-state.active_worktree, unless
#      EVOLVE_KEEP_WORKTREE=1 or the dir is already gone.
#   4. Keep .evolve/runs/cycle-N/ workspace on disk (forensics).
#   5. Append a `role:release` entry to the ledger, maintaining the v8.37.0
#      SHA hash-chain (prev_hash + entry_seq) and updating .evolve/ledger.tip.
#   6. Clear .evolve/cycle-state.json (cycle-state.sh clear).
#
# Usage:
#   bash scripts/lifecycle/cycle-release.sh <cycle> <run_exit_code>
#
# Arguments:
#   cycle           — cycle number (integer). Required for the ledger entry.
#   run_exit_code   — exit code of run-cycle.sh (or 0 if released manually).
#                     Recorded in the ledger entry's exit_code field.
#
# Exit codes:
#   0   — release completed (or skipped because cycle is checkpointed).
#   1   — invalid arguments or missing prerequisites.
#   2   — partial release: ledger entry emitted but cycle-state clear failed.
#         Operator should inspect cycle-state.json manually.
#
# Environment:
#   EVOLVE_KEEP_WORKTREE=1  — skip worktree removal (forensics-preserving).
#   EVOLVE_LEDGER_OVERRIDE  — override ledger path for tests.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/resolve-roots.sh"

log() { echo "[cycle-release] $*" >&2; }

CYCLE="${1:-}"
RUN_RC="${2:-0}"

if [ -z "$CYCLE" ] || ! [[ "$CYCLE" =~ ^[0-9]+$ ]]; then
    log "ERROR: usage: cycle-release.sh <cycle> <run_exit_code>"
    exit 1
fi

LEDGER="${EVOLVE_LEDGER_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl}"
STATE_FILE="$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json"
CYCLE_STATE_HELPER="$EVOLVE_PLUGIN_ROOT/scripts/lifecycle/cycle-state.sh"
[ -f "$CYCLE_STATE_HELPER" ] || CYCLE_STATE_HELPER="$EVOLVE_PROJECT_ROOT/scripts/lifecycle/cycle-state.sh"

# --- Step 1: skip release if checkpointed ----------------------------------
#
# A checkpoint means the cycle is paused for --resume, NOT terminal. Releasing
# would destroy the worktree and clear the state that resume-cycle.sh depends
# on. The run-cycle.sh cleanup() trap already has this same check; this is
# defense-in-depth in case a future caller invokes us directly.
if [ -f "$STATE_FILE" ]; then
    if bash "$CYCLE_STATE_HELPER" is-checkpointed >/dev/null 2>&1; then
        log "checkpoint active for cycle $CYCLE — skipping release (paused, not done)"
        exit 0
    fi
fi

# --- Step 2: optionally remove worktree ------------------------------------
if [ "${EVOLVE_KEEP_WORKTREE:-0}" = "0" ] && [ -f "$STATE_FILE" ]; then
    WORKTREE=""
    if command -v jq >/dev/null 2>&1; then
        WORKTREE=$(jq -r '.active_worktree // empty' "$STATE_FILE" 2>/dev/null)
    fi
    if [ -n "$WORKTREE" ] && [ "$WORKTREE" != "null" ] && [ -d "$WORKTREE" ]; then
        # Defer to git worktree if it's a registered worktree of this repo;
        # otherwise rm -rf. The rm -rf path is the fallback for orphaned
        # /tmp/.../cycle-N/ dirs that lost their git admin entry.
        if git -C "$EVOLVE_PROJECT_ROOT" worktree list 2>/dev/null \
                | grep -q "$WORKTREE"; then
            git -C "$EVOLVE_PROJECT_ROOT" worktree remove --force "$WORKTREE" \
                2>/dev/null && log "removed worktree $WORKTREE" \
                || log "WARN: git worktree remove failed for $WORKTREE"
        else
            rm -rf "$WORKTREE" 2>/dev/null \
                && log "removed orphan worktree dir $WORKTREE" \
                || log "WARN: could not remove $WORKTREE"
        fi
    fi
    unset WORKTREE
fi

# --- Step 3: emit role:release ledger entry --------------------------------
#
# Maintains the v8.37.0 SHA hash-chain. Reading the previous entry's SHA and
# entry_seq, we compute prev_hash, write the new line, then update the tip.
# If jq or sha256sum/shasum is missing, we degrade to an unhashed entry that
# still makes the cycle terminal visible (the verify-ledger-chain script will
# flag the break loudly — better than silent skipping).
if [ -f "$LEDGER" ] && command -v jq >/dev/null 2>&1; then
    PREV_HASH=""
    ENTRY_SEQ=0
    if [ -s "$LEDGER" ]; then
        LAST_LINE=$(tail -1 "$LEDGER" 2>/dev/null || echo "")
        if [ -n "$LAST_LINE" ]; then
            if command -v sha256sum >/dev/null 2>&1; then
                PREV_HASH=$(printf '%s' "$LAST_LINE" | sha256sum | awk '{print $1}')
            elif command -v shasum >/dev/null 2>&1; then
                PREV_HASH=$(printf '%s' "$LAST_LINE" | shasum -a 256 | awk '{print $1}')
            fi
        fi
        ENTRY_SEQ=$(wc -l < "$LEDGER" 2>/dev/null | tr -d ' ' || echo 0)
    fi

    TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    GIT_HEAD=$(git -C "$EVOLVE_PROJECT_ROOT" rev-parse HEAD 2>/dev/null || echo "unknown")

    NEW_LINE=$(jq -nc \
        --arg ts "$TS" \
        --argjson cycle "$CYCLE" \
        --argjson exit_code "$RUN_RC" \
        --argjson entry_seq "$ENTRY_SEQ" \
        --arg prev_hash "$PREV_HASH" \
        --arg git_head "$GIT_HEAD" \
        '{ts: $ts, cycle: $cycle, role: "release", kind: "cycle_terminal",
          exit_code: $exit_code, entry_seq: $entry_seq, prev_hash: $prev_hash,
          git_head: $git_head}' 2>/dev/null) || NEW_LINE=""

    if [ -n "$NEW_LINE" ]; then
        printf '%s\n' "$NEW_LINE" >> "$LEDGER"
        # Update tip atomically.
        NEW_SHA=""
        if command -v sha256sum >/dev/null 2>&1; then
            NEW_SHA=$(printf '%s' "$NEW_LINE" | sha256sum | awk '{print $1}')
        elif command -v shasum >/dev/null 2>&1; then
            NEW_SHA=$(printf '%s' "$NEW_LINE" | shasum -a 256 | awk '{print $1}')
        fi
        if [ -n "$NEW_SHA" ]; then
            TIP_FILE="$(dirname "$LEDGER")/ledger.tip"
            TIP_TMP="${TIP_FILE}.tmp.$$"
            printf '%s:%s\n' "$ENTRY_SEQ" "$NEW_SHA" > "$TIP_TMP" \
                && mv -f "$TIP_TMP" "$TIP_FILE" \
                || rm -f "$TIP_TMP"
            unset TIP_FILE TIP_TMP NEW_SHA
        fi
        log "appended role:release ledger entry (cycle=$CYCLE seq=$ENTRY_SEQ rc=$RUN_RC)"
    else
        log "WARN: could not construct release ledger entry (jq failure)"
    fi
    unset PREV_HASH ENTRY_SEQ LAST_LINE TS GIT_HEAD NEW_LINE
fi

# --- Step 4: clear cycle-state ---------------------------------------------
#
# Idempotent — if run-cycle.sh's cleanup() already cleared the state, this
# is a no-op (cycle-state.sh clear is itself idempotent).
if [ -f "$STATE_FILE" ]; then
    if bash "$CYCLE_STATE_HELPER" clear 2>/dev/null; then
        log "cleared cycle-state.json"
    else
        log "WARN: cycle-state clear returned non-zero — operator inspect $STATE_FILE"
        exit 2
    fi
fi

log "release complete (cycle=$CYCLE rc=$RUN_RC)"
exit 0
