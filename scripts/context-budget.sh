#!/usr/bin/env bash
# context-budget.sh — Estimate context window usage and signal session breaks.
#
# Research basis:
#   - "Lost in the Middle" (Stanford 2023): U-shaped recall, worst in middle
#   - Chroma "Context Rot" (2025): degradation visible at 25% capacity
#   - Effective context ≈ 25-50% of theoretical max (arXiv:2410.18745)
#   - Principle: minimum tokens, maximum signal-to-noise ratio
#
# Usage:
#   bash scripts/context-budget.sh <cycle_number> <cycles_completed_this_session> [workspace_path]
#
# Exit codes:
#   0 = GREEN  — context budget healthy, continue
#   1 = YELLOW — approaching threshold, activate lean mode
#   2 = RED    — session break required before next cycle

set -euo pipefail

# --- Configuration ---
# For 1M context window, target 20-30% usage (200-300K tokens)
CONTEXT_WINDOW=1000000          # 1M tokens (Claude Code)
TARGET_PERCENT=25               # Ideal operating range center
YELLOW_THRESHOLD_PERCENT=20     # Lean mode trigger
RED_THRESHOLD_PERCENT=30        # Session break trigger

YELLOW_THRESHOLD=$(( CONTEXT_WINDOW * YELLOW_THRESHOLD_PERCENT / 100 ))  # 200K
RED_THRESHOLD=$(( CONTEXT_WINDOW * RED_THRESHOLD_PERCENT / 100 ))        # 300K

# --- Token estimates per component ---
# Based on measured evolve-loop cycle costs
STATIC_OVERHEAD=35000           # CLAUDE.md, SKILL.md, system prompt, rules
CYCLE_NORMAL=50000              # Full cycle (Scout + Builder + Auditor + Ship + Learn)
CYCLE_LEAN=30000                # Lean mode cycle
CYCLE_INLINE_S=15000            # S-task inline (no Builder agent)
ORCHESTRATOR_PER_CYCLE=12000    # Orchestrator reasoning, handoffs, phase gates
COMPACTION_SAVINGS=0            # Set dynamically if auto-compact detected

# --- Arguments ---
CYCLE_NUMBER=${1:?"Usage: context-budget.sh <cycle_number> <cycles_this_session> [workspace_path]"}
CYCLES_THIS_SESSION=${2:?"Usage: context-budget.sh <cycle_number> <cycles_this_session> [workspace_path]"}
WORKSPACE_PATH=${3:-".evolve/workspace"}

# --- Estimate current usage ---
# Formula: static_overhead + (cycles_completed * avg_cost_per_cycle) + orchestrator_overhead
# Lean mode kicks in at cycle 4+, so we split the estimate

estimate_tokens() {
    local cycles=$1
    local total=$STATIC_OVERHEAD

    if [ "$cycles" -le 0 ]; then
        echo "$total"
        return
    fi

    # First 3 cycles: normal mode
    local normal_cycles=$cycles
    local lean_cycles=0
    if [ "$cycles" -gt 3 ]; then
        normal_cycles=3
        lean_cycles=$(( cycles - 3 ))
    fi

    total=$(( total + normal_cycles * CYCLE_NORMAL ))
    total=$(( total + lean_cycles * CYCLE_LEAN ))
    total=$(( total + cycles * ORCHESTRATOR_PER_CYCLE ))

    echo "$total"
}

ESTIMATED_TOKENS=$(estimate_tokens "$CYCLES_THIS_SESSION")

# --- Estimate next cycle cost ---
# Check if next cycle would likely be lean or normal
if [ "$CYCLES_THIS_SESSION" -ge 3 ]; then
    NEXT_CYCLE_COST=$CYCLE_LEAN
else
    NEXT_CYCLE_COST=$CYCLE_NORMAL
fi

PROJECTED_AFTER_NEXT=$(( ESTIMATED_TOKENS + NEXT_CYCLE_COST + ORCHESTRATOR_PER_CYCLE ))

# --- Compute percentages ---
CURRENT_PERCENT=$(( ESTIMATED_TOKENS * 100 / CONTEXT_WINDOW ))
PROJECTED_PERCENT=$(( PROJECTED_AFTER_NEXT * 100 / CONTEXT_WINDOW ))

# --- Determine status ---
STATUS="GREEN"
EXIT_CODE=0
RECOMMENDATION=""

if [ "$PROJECTED_AFTER_NEXT" -ge "$RED_THRESHOLD" ]; then
    STATUS="RED"
    EXIT_CODE=2
    RECOMMENDATION="Session break required. Write handoff.md and start new session."
elif [ "$ESTIMATED_TOKENS" -ge "$YELLOW_THRESHOLD" ] || [ "$PROJECTED_AFTER_NEXT" -ge "$YELLOW_THRESHOLD" ]; then
    STATUS="YELLOW"
    EXIT_CODE=1
    RECOMMENDATION="Activate lean mode. Consider session break after this cycle."
else
    RECOMMENDATION="Context budget healthy. Continue normally."
fi

# --- Compute remaining capacity ---
REMAINING_TOKENS=$(( RED_THRESHOLD - ESTIMATED_TOKENS ))
if [ "$REMAINING_TOKENS" -lt 0 ]; then
    REMAINING_TOKENS=0
fi
REMAINING_CYCLES_ESTIMATE=0
if [ "$NEXT_CYCLE_COST" -gt 0 ]; then
    REMAINING_CYCLES_ESTIMATE=$(( REMAINING_TOKENS / (NEXT_CYCLE_COST + ORCHESTRATOR_PER_CYCLE) ))
fi

# --- Output JSON ---
cat <<EOF
{
  "status": "$STATUS",
  "cycleNumber": $CYCLE_NUMBER,
  "cyclesThisSession": $CYCLES_THIS_SESSION,
  "estimatedTokens": $ESTIMATED_TOKENS,
  "projectedAfterNext": $PROJECTED_AFTER_NEXT,
  "currentPercent": $CURRENT_PERCENT,
  "projectedPercent": $PROJECTED_PERCENT,
  "yellowThreshold": $YELLOW_THRESHOLD,
  "redThreshold": $RED_THRESHOLD,
  "remainingTokens": $REMAINING_TOKENS,
  "remainingCyclesEstimate": $REMAINING_CYCLES_ESTIMATE,
  "nextCycleCost": $NEXT_CYCLE_COST,
  "recommendation": "$RECOMMENDATION"
}
EOF

exit $EXIT_CODE
