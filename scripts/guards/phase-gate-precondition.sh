#!/usr/bin/env bash
#
# phase-gate-precondition.sh — PreToolUse hook for Claude Code Bash calls (v8.13.1).
#
# Blocks out-of-order subagent invocations. Triggers ONLY for commands of the
# form `bash scripts/dispatch/subagent-run.sh <agent> <cycle> <workspace>` — every other
# command falls through with ALLOW.
#
# Decision tree:
#   1. Read JSON payload from stdin → extract command.
#   2. If command does not match `bash scripts/dispatch/subagent-run.sh ...` → ALLOW.
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

# v8.18.0: dual-root. guards.log + cycle-state.json are writable artifacts under
# the user's project. team-context.sh is read-only and lives with the plugin.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"
unset __rr_self

GUARDS_LOG="$EVOLVE_PROJECT_ROOT/.evolve/guards.log"
CYCLE_STATE_FILE="${EVOLVE_CYCLE_STATE_FILE:-$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json}"

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
    # COMMAND is only populated for Bash tool calls. Other tools (Edit, Write,
    # Agent, etc.) reach here with empty COMMAND. Continue past this guard so
    # the v8.21.0 Agent-tool check below can still inspect the payload.
    :
fi

# ---- Bypass switch ---------------------------------------------------------

if [ "${EVOLVE_BYPASS_PHASE_GATE:-0}" = "1" ]; then
    log "WARN: EVOLVE_BYPASS_PHASE_GATE=1 — bypassing for: ${COMMAND:0:100}"
    echo "[phase-gate-pre] WARN: bypass active; gate not enforcing" >&2
    exit 0
fi

# ---- v8.21.0: Agent tool denial during cycles ------------------------------
# CLAUDE.md rule #5 documents that the in-process `Agent` tool is forbidden
# in production cycles — phase agents must be invoked via subagent-run.sh so
# the kernel ledger captures every dispatch. The orchestrator profile denies
# Agent, but a misconfigured/missing profile would let the bypass slip
# through. This kernel hook is defense-in-depth: even if the profile fails
# open, the Agent tool cannot be invoked while a cycle-state.json exists.
#
# Triggers ONLY when (a) the tool is Agent and (b) a cycle is in flight.
# Ad-hoc Agent invocations outside cycles fall through.
TOOL_NAME=""
if command -v jq >/dev/null 2>&1; then
    TOOL_NAME=$(echo "$PAYLOAD" | jq -r '.tool_name // empty' 2>/dev/null || true)
fi
if [ -z "$TOOL_NAME" ]; then
    TOOL_NAME=$(echo "$PAYLOAD" | sed -n 's/.*"tool_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
fi

if [ "$TOOL_NAME" = "Agent" ]; then
    if [ -f "$CYCLE_STATE_FILE" ] && [ -s "$CYCLE_STATE_FILE" ]; then
        deny "Agent tool forbidden during evolve-loop cycles — use \`bash scripts/dispatch/subagent-run.sh <agent> <cycle> <workspace>\` so the kernel ledger captures dispatch"
    else
        log "Agent tool: no cycle-state — ALLOW (ad-hoc)"
        exit 0
    fi
fi

# Below this point, only Bash-tool subagent-run.sh invocations are checked.
# Empty COMMAND on a non-Agent tool means we have nothing further to inspect.
if [ -z "$COMMAND" ]; then
    log "no command in payload (non-Bash tool); ALLOW"
    exit 0
fi

# ---- Trigger detection -----------------------------------------------------
# Match `bash scripts/dispatch/subagent-run.sh ...` (with optional ./, /full/path/, and
# leading whitespace). Anything else: passthrough.

TRIMMED="${COMMAND#"${COMMAND%%[![:space:]]*}"}"
case "$TRIMMED" in
    bash*scripts/dispatch/subagent-run.sh*|sh*scripts/dispatch/subagent-run.sh*|*/subagent-run.sh*)
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

# Recognized agents (canonical phase agents and Sprint 1 fan-out workers).
# Worker names are of the form `<role>-worker-<subtask>`. For workers, derive
# the parent role and check sequence as if the parent role were invoked.
# Unrecognized agents fall through (subagent-run.sh prints its own error).
DERIVED_ROLE=""
case "$REQUESTED_AGENT" in
    intent|scout|builder|auditor|evaluator|inspirer|orchestrator|retrospective|tdd-engineer|plan-reviewer|triage)
        DERIVED_ROLE="$REQUESTED_AGENT"
        ;;
    intent-worker-*|scout-worker-*|builder-worker-*|auditor-worker-*|evaluator-worker-*|inspirer-worker-*|retrospective-worker-*|tdd-engineer-worker-*|plan-reviewer-worker-*|triage-worker-*)
        # Strip "-worker-<subtask>" to get parent role.
        DERIVED_ROLE="${REQUESTED_AGENT%%-worker-*}"
        log "worker-pattern: agent='$REQUESTED_AGENT' derived_role='$DERIVED_ROLE'"
        ;;
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
#   discover   → scout (continuing) | tdd-engineer (TDD phase) | builder (skip-tdd path)
#   tdd        → tdd-engineer (re-spawn) | builder (next phase, after RED tests written)
#   build      → builder (re-spawn) | auditor (next phase)
#   audit      → auditor (re-spawn) | retrospective (on FAIL/WARN — orchestrator decides) | evaluator
#   ship       → orchestrator (re-spawn) | retrospective
#   learn      → retrospective | inspirer
#
# Re-spawning the same active_agent is always allowed (recovery flows).

EXPECTED=""
case "$PHASE" in
    calibrate)    EXPECTED="intent scout orchestrator" ;;
    intent)       EXPECTED="intent orchestrator" ;;
    research)     EXPECTED="scout inspirer orchestrator" ;;
    discover)     EXPECTED="scout triage plan-reviewer tdd-engineer builder orchestrator" ;;
    triage)       EXPECTED="triage plan-reviewer orchestrator" ;;
    plan-review)  EXPECTED="plan-reviewer scout tdd-engineer orchestrator" ;;
    tdd)          EXPECTED="tdd-engineer builder orchestrator" ;;
    build)        EXPECTED="builder auditor orchestrator" ;;
    audit)        EXPECTED="auditor evaluator retrospective orchestrator" ;;
    ship)         EXPECTED="orchestrator retrospective" ;;
    learn)        EXPECTED="retrospective inspirer orchestrator" ;;
    *)            EXPECTED="" ;;
esac

# Re-spawn always OK. For workers, the prefix (parent role) is what counts.
if [ -n "$ACTIVE_AGENT" ]; then
    if [ "$REQUESTED_AGENT" = "$ACTIVE_AGENT" ] || [ "$DERIVED_ROLE" = "$ACTIVE_AGENT" ]; then
        log "ALLOW (re-spawn) phase=$PHASE cycle=$CYCLE_ID agent=$REQUESTED_AGENT (derived=$DERIVED_ROLE)"
        exit 0
    fi
fi

# Check expected set against the derived role (so workers inherit their
# parent role's phase eligibility).
ALLOWED=0
for cand in $EXPECTED; do
    if [ "$cand" = "$REQUESTED_AGENT" ] || [ "$cand" = "$DERIVED_ROLE" ]; then
        ALLOWED=1
        break
    fi
done

if [ "$ALLOWED" = "0" ]; then
    deny "phase=$PHASE cycle=$CYCLE_ID active=$ACTIVE_AGENT completed=[$COMPLETED] requested=$REQUESTED_AGENT not in expected={$EXPECTED}"
fi

# ---- Intent-phase precondition (state-gated, v8.19.0) ---------------------
# When the cycle was initialized with intent_required=true (from
# EVOLVE_REQUIRE_INTENT=1 at init time), scout invocations require that the
# intent persona has produced intent.md AND a corresponding agent_subprocess
# ledger entry exists for cycle+role=intent. Otherwise scout would be running
# without a structured intent, defeating the skill's purpose. Other agents
# and other phases are unaffected.
#
# Reads intent_required from cycle-state.json (NOT env) — this means a
# mid-stream env flip cannot retroactively block in-flight cycles, and a
# cycle that was init'd with intent_required=true will continue to enforce
# even if env is later unset.
#
# Default off: cycles initialized without EVOLVE_REQUIRE_INTENT=1 have
# intent_required=false in cycle-state.json, and this block is a no-op.
if [ "$REQUESTED_AGENT" = "scout" ] && command -v jq >/dev/null 2>&1; then
    INTENT_REQUIRED=$(jq -r '.intent_required // false' "$CYCLE_STATE_FILE" 2>/dev/null)
    if [ "$INTENT_REQUIRED" = "true" ]; then
        WORKSPACE_PATH=$(jq -r '.workspace_path // empty' "$CYCLE_STATE_FILE" 2>/dev/null)
        LEDGER_PATH="${EVOLVE_LEDGER_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl}"
        if [ ! -f "$LEDGER_PATH" ]; then
            deny "intent_required=true but ledger missing at $LEDGER_PATH; intent persona must run first"
        fi
        # Look for a recent intent ledger entry for this cycle.
        INTENT_ENTRY=$(grep '"kind":"agent_subprocess"' "$LEDGER_PATH" 2>/dev/null \
            | jq -c --argjson cycle "$CYCLE_ID" 'select(.cycle == $cycle and .role == "intent")' 2>/dev/null \
            | tail -1)
        if [ -z "$INTENT_ENTRY" ]; then
            deny "intent_required=true (cycle $CYCLE_ID) but no intent ledger entry found; run \`bash \$EVOLVE_PLUGIN_ROOT/scripts/dispatch/subagent-run.sh intent $CYCLE_ID $WORKSPACE_PATH\` before scout"
        fi
        log "intent precondition OK: ledger has intent entry for cycle $CYCLE_ID"
    fi
fi

# ---- Team-context bus precondition (env-flag-gated, default off) ----------
# When EVOLVE_REQUIRE_TEAM_CONTEXT=1, builder invocations require that the
# team-context.md bus has Scout's findings AND TDD-Engineer's contract
# populated — otherwise builder would be implementing without the test
# contract. Other agents and other phases are unaffected. Default off so
# existing cycles that predate the bus continue to work.
if [ "${EVOLVE_REQUIRE_TEAM_CONTEXT:-0}" = "1" ] && [ "$REQUESTED_AGENT" = "builder" ]; then
    WORKSPACE_PATH=$(jq -r '.workspace_path // empty' "$CYCLE_STATE_FILE" 2>/dev/null)
    if [ -z "$WORKSPACE_PATH" ]; then
        log "team-context check: cycle-state has no workspace_path — skipping"
    else
        TEAM_CTX_SH="$EVOLVE_PLUGIN_ROOT/scripts/utility/team-context.sh"
        if [ ! -x "$TEAM_CTX_SH" ]; then
            log "team-context check: $TEAM_CTX_SH not executable — skipping"
        elif ! "$TEAM_CTX_SH" verify "$CYCLE_ID" "$WORKSPACE_PATH" --require scout,tdd-engineer >/dev/null 2>&1; then
            deny "EVOLVE_REQUIRE_TEAM_CONTEXT=1 active; builder blocked: team-context.md missing scout or tdd-engineer section in $WORKSPACE_PATH"
        else
            log "team-context check OK: scout + tdd-engineer sections populated"
        fi
    fi
fi

log "ALLOW phase=$PHASE cycle=$CYCLE_ID requested=$REQUESTED_AGENT (in expected={$EXPECTED})"
exit 0
