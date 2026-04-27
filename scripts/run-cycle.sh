#!/usr/bin/env bash
#
# run-cycle.sh — Convenience driver for the Evolve Loop (v8.13.1).
#
# Initializes per-cycle runtime state (.evolve/cycle-state.json) and spawns
# the orchestrator subagent (scripts/subagent-run.sh orchestrator ...). The
# orchestrator's profile (.evolve/profiles/orchestrator.json) restricts what
# it can do — combined with role-gate.sh (path-allowlist on Edit/Write) and
# phase-gate-precondition.sh (sequence-allowlist on subagent invocations),
# the trust boundary becomes structurally enforced rather than relying on
# LLM cooperation.
#
# Usage:
#   bash scripts/run-cycle.sh [GOAL]
#   bash scripts/run-cycle.sh --cycle 8200 [GOAL]
#   bash scripts/run-cycle.sh --dry-run   # print what would happen without spawning
#
# Lifecycle:
#   1. Resolve cycle ID (next-after-state OR explicit --cycle).
#   2. Create workspace .evolve/runs/cycle-N/.
#   3. cycle_state_init → cycle-state.json with phase=calibrate.
#   4. Build context block (instinct summary, ledger tail, failed approaches).
#   5. Spawn orchestrator: bash scripts/subagent-run.sh orchestrator $CYCLE $WORKSPACE.
#   6. On exit (PASS or FAIL), clear cycle-state.json and print summary.
#
# IMPORTANT — what this script does NOT do:
#   - It does NOT itself sequence phases. Phase sequencing lives inside the
#     orchestrator subagent (in agents/evolve-orchestrator.md). The runner
#     only writes the initial state file and spawns the orchestrator.
#   - It does NOT write to source code. role-gate.sh blocks that during cycles.
#   - It does NOT commit/push. Only scripts/ship.sh can (ship-gate enforces).
#
# Exit codes:
#   0   — orchestrator completed (verdict in orchestrator-report.md)
#   1   — runtime failure (couldn't spawn, missing prerequisites)
#   2   — integrity failure (cycle-state collision, etc.)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE_FILE="$REPO_ROOT/.evolve/state.json"
CYCLE_STATE_HELPER="$REPO_ROOT/scripts/cycle-state.sh"
SUBAGENT_RUN="$REPO_ROOT/scripts/subagent-run.sh"
ORCHESTRATOR_PROMPT="$REPO_ROOT/agents/evolve-orchestrator.md"
LEDGER="$REPO_ROOT/.evolve/ledger.jsonl"
INSTINCT_SUMMARY="$REPO_ROOT/.evolve/instincts/personal/summary.md"

log()  { echo "[run-cycle] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }
integrity_fail() { log "INTEGRITY-FAIL: $*"; exit 2; }

# ---- Argument parsing ------------------------------------------------------

DRY_RUN=0
CYCLE=""
GOAL=""

while [ $# -gt 0 ]; do
    case "$1" in
        --cycle)
            shift
            [ $# -gt 0 ] || fail "--cycle requires a value"
            CYCLE="$1"
            ;;
        --dry-run)
            DRY_RUN=1
            ;;
        --help|-h)
            sed -n '2,30p' "$0" | sed 's/^# //; s/^#//'
            exit 0
            ;;
        --*)
            fail "unknown flag: $1"
            ;;
        *)
            # First positional → goal.
            if [ -z "$GOAL" ]; then GOAL="$1"
            else GOAL="$GOAL $1"
            fi
            ;;
    esac
    shift
done

# ---- Prerequisites ---------------------------------------------------------

[ -f "$CYCLE_STATE_HELPER" ] || fail "missing $CYCLE_STATE_HELPER"
[ -f "$SUBAGENT_RUN" ]       || fail "missing $SUBAGENT_RUN"
[ -f "$ORCHESTRATOR_PROMPT" ] || fail "missing $ORCHESTRATOR_PROMPT"
command -v jq >/dev/null 2>&1 || fail "jq is required"

# ---- Resolve cycle ID ------------------------------------------------------

if [ -z "$CYCLE" ]; then
    if [ -f "$STATE_FILE" ]; then
        last=$(jq -r '.lastCycleNumber // 0' "$STATE_FILE" 2>/dev/null || echo 0)
    else
        last=0
    fi
    CYCLE=$((last + 1))
fi
[[ "$CYCLE" =~ ^[0-9]+$ ]] || fail "cycle must be integer, got: $CYCLE"

WORKSPACE="$REPO_ROOT/.evolve/runs/cycle-$CYCLE"

# ---- Collision check -------------------------------------------------------

if bash "$CYCLE_STATE_HELPER" exists >/dev/null 2>&1; then
    existing=$(bash "$CYCLE_STATE_HELPER" get cycle_id || true)
    integrity_fail "cycle-state.json already exists for cycle $existing — refusing to clobber. Run: bash scripts/cycle-state.sh clear"
fi

# ---- Build context block ---------------------------------------------------

build_context() {
    local cycle="$1" workspace="$2" goal="$3"

    # Ledger tail (last 5 entries) — gives orchestrator awareness of recent activity.
    local ledger_tail=""
    if [ -f "$LEDGER" ]; then
        ledger_tail=$(tail -5 "$LEDGER" 2>/dev/null || echo "")
    fi

    # Instinct summary — accumulated lessons from prior cycles.
    local instinct=""
    if [ -f "$INSTINCT_SUMMARY" ]; then
        instinct=$(cat "$INSTINCT_SUMMARY" 2>/dev/null || echo "")
    fi

    # Recent failed approaches — orchestrator should avoid these.
    local failed=""
    if [ -f "$STATE_FILE" ]; then
        failed=$(jq -r '(.failedApproaches // []) | .[-3:] | .[] | "- " + (.summary // .verdict // "unknown")' "$STATE_FILE" 2>/dev/null || echo "")
    fi

    cat <<EOF

---
ORCHESTRATOR CONTEXT (injected by run-cycle.sh)
---

cycle: $cycle
workspace: $workspace
goal: ${goal:-<unspecified — pick from CLAUDE.md priorities>}
cycleState: $REPO_ROOT/.evolve/cycle-state.json (already initialized to phase=calibrate)

recentLedgerEntries:
$ledger_tail

recentFailures:
$failed

instinctSummary:
$instinct

---
EOF
}

# ---- Setup workspace -------------------------------------------------------

mkdir -p "$WORKSPACE"
log "workspace=$WORKSPACE"

# Initialize cycle-state.json (phase=calibrate, no agent yet).
bash "$CYCLE_STATE_HELPER" init "$CYCLE" ".evolve/runs/cycle-$CYCLE" \
    || fail "cycle_state_init failed"
log "cycle-state.json initialized at phase=calibrate"

# Always clear cycle-state on exit (success OR failure), unless dry-run.
cleanup() {
    local rc=$?
    if [ "$DRY_RUN" = "0" ]; then
        bash "$CYCLE_STATE_HELPER" clear 2>/dev/null || true
        log "cycle-state cleared (rc=$rc)"
    fi
    exit $rc
}
trap cleanup EXIT INT TERM

# ---- Build prompt ----------------------------------------------------------

PROMPT_FILE="$WORKSPACE/orchestrator-prompt.md"
{
    cat "$ORCHESTRATOR_PROMPT"
    build_context "$CYCLE" "$WORKSPACE" "$GOAL"
} > "$PROMPT_FILE"

log "prompt written to $PROMPT_FILE ($(wc -l < "$PROMPT_FILE") lines)"

# ---- Dry-run? --------------------------------------------------------------

if [ "$DRY_RUN" = "1" ]; then
    log "DRY RUN — would spawn:"
    log "  PROMPT_FILE_OVERRIDE=$PROMPT_FILE bash scripts/subagent-run.sh orchestrator $CYCLE $WORKSPACE"
    log "(cycle-state.json left in place for inspection)"
    bash "$CYCLE_STATE_HELPER" dump | jq . >&2 || true
    # Disable cleanup trap so dry-run leaves state visible.
    trap - EXIT INT TERM
    bash "$CYCLE_STATE_HELPER" clear >/dev/null 2>&1 || true
    exit 0
fi

# ---- Spawn orchestrator ----------------------------------------------------

log "spawning orchestrator subagent for cycle $CYCLE..."

set +e
PROMPT_FILE_OVERRIDE="$PROMPT_FILE" bash "$SUBAGENT_RUN" orchestrator "$CYCLE" "$WORKSPACE"
rc=$?
set -e

# ---- Summary ---------------------------------------------------------------

log "orchestrator subagent exited rc=$rc"

if [ -f "$WORKSPACE/orchestrator-report.md" ]; then
    log "orchestrator report at: $WORKSPACE/orchestrator-report.md"
    head -30 "$WORKSPACE/orchestrator-report.md" >&2 || true
else
    log "WARN: no orchestrator-report.md produced"
fi

exit "$rc"
