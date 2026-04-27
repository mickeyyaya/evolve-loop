#!/usr/bin/env bash
#
# phase-gate-precondition.sh — PreToolUse hook for Claude Code Bash calls (v8.13.1).
#
# Blocks out-of-order subagent invocations. Triggers ONLY for commands of the
# form `bash scripts/subagent-run.sh <agent> <cycle> <workspace>` — every other
# command falls through with ALLOW.
#
# Decision tree:
#   1. Read JSON payload from stdin → extract command.
#   2. If command does not match `bash scripts/subagent-run.sh ...` → ALLOW.
#   3. If .evolve/cycle-state.json missing → ALLOW (manual ad-hoc invocation).
#   4. Read cycle_state.phase + completed_phases.
#   5. Compute expected next agent for current phase. If requested agent is in
#      the expected set OR matches active_agent (re-spawn) → ALLOW.
#   6. Otherwise → DENY.
#
# Bypass: EVOLVE_BYPASS_PHASE_GATE=1 (logged WARN; emergency only).
#
# Exit codes:
#   0 — allow
#   2 — deny
#
# This gate is intentionally a SECOND hook on the Bash matcher (ship-gate is
# the first). Claude Code runs PreToolUse hooks in array order; if any deny,
# the call is blocked. ship-gate handles ship-class commands; this gate
# handles subagent-run sequencing.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GUARDS_LOG="$REPO_ROOT/.evolve/guards.log"
CYCLE_STATE_FILE="${EVOLVE_CYCLE_STATE_FILE:-$REPO_ROOT/.evolve/cycle-state.json}"

mkdir -p "$(dirname "$GUARDS_LOG")"

log() {
    local ts
    ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "[$ts] [phase-gate-pre] $*" >> "$GUARDS_LOG"
}

deny() {
    local msg="$1"
    log "DENY: $msg"
    echo "[phase-gate-pre] DENY: $msg" >&2
    echo "[phase-gate-pre] To bypass (emergency only): export EVOLVE_BYPASS_PHASE_GATE=1" >&2
    exit 2
}

# ---- Read payload ----------------------------------------------------------

PAYLOAD="$(cat || true)"
if [ -z "$PAYLOAD" ]; then
    log "no-payload (manual invocation?); ALLOW"
    exit 0
fi

COMMAND=""
if command -v jq >/dev/null 2>&1; then
    COMMAND=$(echo "$PAYLOAD" | jq -r '.tool_input.command // empty' 2>/dev/null || true)
fi
if [ -z "$COMMAND" ]; then
    COMMAND=$(echo "$PAYLOAD" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
fi

if [ -z "$COMMAND" ]; then
    log "no command in payload; ALLOW"
    exit 0
fi

# ---- Bypass switch ---------------------------------------------------------

if [ "${EVOLVE_BYPASS_PHASE_GATE:-0}" = "1" ]; then
    log "WARN: EVOLVE_BYPASS_PHASE_GATE=1 — bypassing for: ${COMMAND:0:100}"
    echo "[phase-gate-pre] WARN: bypass active; gate not enforcing" >&2
    exit 0
fi

# ---- Trigger detection -----------------------------------------------------
# Match `bash scripts/subagent-run.sh ...` (with optional ./, /full/path/, and
# leading whitespace). Anything else: passthrough.

TRIMMED="${COMMAND#"${COMMAND%%[![:space:]]*}"}"
case "$TRIMMED" in
    bash*scripts/subagent-run.sh*|sh*scripts/subagent-run.sh*|*/subagent-run.sh*)
        : # match — fall through to checks
        ;;
    *)
        log "non-subagent-run command; ALLOW"
        exit 0
        ;;
esac

# Extract the agent argument (first positional after subagent-run.sh).
# Use awk to split on whitespace robustly.
REQUESTED_AGENT=$(echo "$TRIMMED" | awk '
    {
        for (i = 1; i <= NF; i++) {
            if ($i ~ /subagent-run\.sh$/) {
                if (i + 1 <= NF) {
                    print $(i+1)
                }
                exit
            }
        }
    }
')

if [ -z "$REQUESTED_AGENT" ]; then
    log "could not parse agent argument; ALLOW (let subagent-run.sh handle malformed)"
    exit 0
fi

# Strip trailing punctuation that shells might put there.
REQUESTED_AGENT="${REQUESTED_AGENT%[\"\']}"
REQUESTED_AGENT="${REQUESTED_AGENT#[\"\']}"

# Recognized agents — anything else falls through (let subagent-run.sh
# print its own error message).
case "$REQUESTED_AGENT" in
    scout|builder|auditor|evaluator|inspirer|orchestrator|retrospective)
        : ;;
    *)
        log "unrecognized agent '$REQUESTED_AGENT'; ALLOW (delegating to subagent-run.sh)"
        exit 0
        ;;
esac

# ---- No cycle-state → ALLOW (manual / ad-hoc) ------------------------------

if [ ! -f "$CYCLE_STATE_FILE" ]; then
    log "no cycle-state; ALLOW (ad-hoc) agent=$REQUESTED_AGENT"
    exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
    log "WARN: jq missing; cannot parse cycle-state — ALLOW $REQUESTED_AGENT"
    exit 0
fi

PHASE=$(jq -r '.phase // empty' "$CYCLE_STATE_FILE" 2>/dev/null || true)
ACTIVE_AGENT=$(jq -r '.active_agent // empty' "$CYCLE_STATE_FILE" 2>/dev/null || true)
COMPLETED=$(jq -r '(.completed_phases // []) | join(",")' "$CYCLE_STATE_FILE" 2>/dev/null || true)
CYCLE_ID=$(jq -r '.cycle_id // empty' "$CYCLE_STATE_FILE" 2>/dev/null || true)

if [ -z "$PHASE" ]; then
    log "cycle-state malformed; ALLOW (fail-open) agent=$REQUESTED_AGENT"
    exit 0
fi

# ---- Compute expected next agent set ---------------------------------------
# Mapping (current phase → set of acceptable next agents):
#   calibrate  → scout (research starts)
#   research   → scout (still researching) | inspirer (heuristic injection)
#   discover   → scout (continuing) | builder (next phase)
#   build      → builder (re-spawn) | auditor (next phase)
#   audit      → auditor (re-spawn) | retrospective (on FAIL/WARN — orchestrator decides) | evaluator
#   ship       → orchestrator (re-spawn) | retrospective
#   learn      → retrospective | inspirer
#
# Re-spawning the same active_agent is always allowed (recovery flows).

EXPECTED=""
case "$PHASE" in
    calibrate)  EXPECTED="scout orchestrator" ;;
    research)   EXPECTED="scout inspirer orchestrator" ;;
    discover)   EXPECTED="scout builder orchestrator" ;;
    build)      EXPECTED="builder auditor orchestrator" ;;
    audit)      EXPECTED="auditor evaluator retrospective orchestrator" ;;
    ship)       EXPECTED="orchestrator retrospective" ;;
    learn)      EXPECTED="retrospective inspirer orchestrator" ;;
    *)          EXPECTED="" ;;
esac

# Re-spawn always OK.
if [ -n "$ACTIVE_AGENT" ] && [ "$REQUESTED_AGENT" = "$ACTIVE_AGENT" ]; then
    log "ALLOW (re-spawn) phase=$PHASE cycle=$CYCLE_ID agent=$REQUESTED_AGENT"
    exit 0
fi

# Check expected set.
for cand in $EXPECTED; do
    if [ "$cand" = "$REQUESTED_AGENT" ]; then
        log "ALLOW phase=$PHASE cycle=$CYCLE_ID requested=$REQUESTED_AGENT (in expected={$EXPECTED})"
        exit 0
    fi
done

deny "phase=$PHASE cycle=$CYCLE_ID active=$ACTIVE_AGENT completed=[$COMPLETED] requested=$REQUESTED_AGENT not in expected={$EXPECTED}"
