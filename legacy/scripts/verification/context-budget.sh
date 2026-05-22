#!/usr/bin/env bash
# context-budget.sh — Per-cycle context budget gate.
#
# Design principle: each cycle is an INDEPENDENT plan-mode unit. The budget
# gate answers ONE question: "Is there room for one more cycle?" — NOT
# "How much total have we used across all cycles?"
#
# Why per-cycle works:
#   - Agents run in isolated subagent context (don't accumulate in parent)
#   - Critical state lives in files (state.json, reports, evals, ledger)
#   - Claude Code auto-compresses older conversation turns between cycles
#   - Only recent orchestrator conversation (1-2 cycles) is "live" context
#
# The effective context at any cycle start is roughly constant:
#   STATIC_OVERHEAD (system prompt, rules) + RECENT_CONTEXT (last 1-2 cycles
#   of orchestrator turns, not yet compacted) + ONE_CYCLE_COST (upcoming).
#   This is well within the 1M window regardless of how many cycles completed.
#
# Usage:
#   bash scripts/verification/context-budget.sh <cycle_number> <cycles_completed_this_session> [workspace_path]
#
# Exit codes:
#   0 = GREEN  — room for a normal cycle, continue
#   1 = YELLOW — lean mode for this cycle (reduces reads/writes/agent depth)
#   2 = RED    — not enough room for even a lean cycle; session break needed

set -euo pipefail

# --- Per-cycle cost estimates ---
# These represent the cost of ONE cycle in the parent orchestrator's context.
# Agent subagents are isolated — only their return summary enters parent context.
if [ -n "${LLM_CONTEXT_WINDOW:-}" ]; then
    CONTEXT_WINDOW=$LLM_CONTEXT_WINDOW
elif [ -n "${GEMINI_API_KEY:-}" ] || [ -d "$HOME/.gemini" ]; then
    CONTEXT_WINDOW=2000000      # 2M tokens (Gemini 1.5 Pro/Flash)
else
    CONTEXT_WINDOW=1000000      # 1M tokens (Claude)
fi
STATIC_OVERHEAD=25000           # System prompt, rules, SKILL.md — always present
RECENT_CONTEXT=50000            # ~2 recent cycles of orchestrator conversation (not yet compacted)
ONE_CYCLE_NORMAL=35000          # Full cycle: orchestration + agent summaries + phase gates
ONE_CYCLE_LEAN=20000            # Lean cycle: reduced reads, shorter agent prompts
SAFETY_BUFFER=50000             # Headroom for compaction lag, unexpected growth, handoff writes

# --- Lean mode and hard-stop thresholds ---
# These are cycle-count based, not cumulative-token based.
# After many cycles, even with compaction, there's residual context growth
# from orchestrator reasoning, handoff reads, and accumulated file state.
LEAN_MODE_AFTER=10              # Activate lean mode from this cycle onward
HARD_STOP_AFTER=30              # Safety valve for extremely long sessions

# --- Arguments ---
CYCLE_NUMBER=${1:?"Usage: context-budget.sh <cycle_number> <cycles_this_session> [workspace_path]"}
CYCLES_THIS_SESSION=${2:?"Usage: context-budget.sh <cycle_number> <cycles_this_session> [workspace_path]"}
WORKSPACE_PATH=${3:-".evolve/workspace"}

# --- Per-cycle headroom check ---
# Estimate effective context NOW (not cumulative — auto-compaction reclaims older cycles)
# then check if one more cycle fits.

if [ "$CYCLES_THIS_SESSION" -ge "$LEAN_MODE_AFTER" ]; then
    NEXT_CYCLE_COST=$ONE_CYCLE_LEAN
else
    NEXT_CYCLE_COST=$ONE_CYCLE_NORMAL
fi

# Effective context: static + recent (compacted older cycles are ~free) + next cycle
EFFECTIVE_USAGE=$(( STATIC_OVERHEAD + RECENT_CONTEXT ))
NEEDED_FOR_NEXT=$(( NEXT_CYCLE_COST + SAFETY_BUFFER ))
AVAILABLE=$(( CONTEXT_WINDOW - EFFECTIVE_USAGE ))
PROJECTED_AFTER_NEXT=$(( EFFECTIVE_USAGE + NEEDED_FOR_NEXT ))

# As cycles accumulate, add a small residual growth factor to account for
# compaction imperfection. Each past cycle leaves ~2-3K of residual context
# (compressed summaries, handoff fragments, orchestrator state references).
RESIDUAL_PER_CYCLE=3000
RESIDUAL_GROWTH=$(( CYCLES_THIS_SESSION * RESIDUAL_PER_CYCLE ))
EFFECTIVE_USAGE=$(( EFFECTIVE_USAGE + RESIDUAL_GROWTH ))
AVAILABLE=$(( CONTEXT_WINDOW - EFFECTIVE_USAGE ))
PROJECTED_AFTER_NEXT=$(( EFFECTIVE_USAGE + NEEDED_FOR_NEXT ))

# --- Compute percentages ---
CURRENT_PERCENT=$(( EFFECTIVE_USAGE * 100 / CONTEXT_WINDOW ))
PROJECTED_PERCENT=$(( PROJECTED_AFTER_NEXT * 100 / CONTEXT_WINDOW ))

# --- Determine status ---
STATUS="GREEN"
EXIT_CODE=0
RECOMMENDATION=""

# Hard stop: extremely long sessions where residual growth overwhelms compaction
if [ "$CYCLES_THIS_SESSION" -ge "$HARD_STOP_AFTER" ] || [ "$AVAILABLE" -lt "$ONE_CYCLE_LEAN" ]; then
    STATUS="RED"
    EXIT_CODE=2
    RECOMMENDATION="Session break recommended. $CYCLES_THIS_SESSION cycles completed — residual context growth may affect quality."
# Lean mode: optimize to extend session further
elif [ "$CYCLES_THIS_SESSION" -ge "$LEAN_MODE_AFTER" ] || [ "$AVAILABLE" -lt "$NEEDED_FOR_NEXT" ]; then
    STATUS="YELLOW"
    EXIT_CODE=1
    RECOMMENDATION="Lean mode activated. Per-cycle budget is tight — reducing agent depth and file reads."
else
    RECOMMENDATION="Per-cycle budget healthy. Room for a full cycle."
fi

# --- Compute remaining capacity ---
REMAINING_TOKENS=$AVAILABLE
if [ "$REMAINING_TOKENS" -lt 0 ]; then
    REMAINING_TOKENS=0
fi
REMAINING_CYCLES_ESTIMATE=0
if [ "$NEXT_CYCLE_COST" -gt 0 ]; then
    REMAINING_CYCLES_ESTIMATE=$(( REMAINING_TOKENS / (NEXT_CYCLE_COST + RESIDUAL_PER_CYCLE) ))
fi

# --- Output JSON ---
cat <<EOF
{
  "status": "$STATUS",
  "model": "per-cycle",
  "cycleNumber": $CYCLE_NUMBER,
  "cyclesThisSession": $CYCLES_THIS_SESSION,
  "effectiveUsage": $EFFECTIVE_USAGE,
  "projectedAfterNext": $PROJECTED_AFTER_NEXT,
  "currentPercent": $CURRENT_PERCENT,
  "projectedPercent": $PROJECTED_PERCENT,
  "available": $AVAILABLE,
  "nextCycleCost": $NEXT_CYCLE_COST,
  "residualGrowth": $RESIDUAL_GROWTH,
  "remainingCyclesEstimate": $REMAINING_CYCLES_ESTIMATE,
  "leanModeAfter": $LEAN_MODE_AFTER,
  "hardStopAfter": $HARD_STOP_AFTER,
  "recommendation": "$RECOMMENDATION"
}
EOF

exit $EXIT_CODE
