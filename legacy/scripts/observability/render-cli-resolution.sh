#!/usr/bin/env bash
#
# render-cli-resolution.sh — Render a CLI Resolution markdown section from
# ledger entries for a given cycle.
#
# WHY THIS EXISTS
#
# Cycle 61 demonstrated that orchestrator-report.md could imply "Builder ran
# on Gemini" when the ledger recorded the actual phase as
# `target_cli=claude, source=llm_config_fallback`. The orchestrator persona
# can hallucinate CLI provenance, especially after context compaction.
#
# This script derives CLI provenance from the LEDGER (the trust-kernel-managed
# source of truth) and emits a markdown section that operators (and tests)
# can verify byte-identically. Orchestrator persona is instructed NOT to
# write CLI Resolution itself; phase-gate appends this script's output.
#
# CONTRACT
#
# Input: <cycle> integer
# Output: stdout — markdown section "## CLI Resolution" + table
# Exit codes: 0 (always — empty ledger yields empty-table placeholder)
#
# DESIGN
#
# Reads `.evolve/ledger.jsonl`, filters kind=agent_subprocess + cycle=N,
# extracts role + cli_resolution, emits one table row per phase.
# Output is byte-stable for a given ledger state — facilitates tamper detection.

set -uo pipefail

CYCLE="${1:?usage: render-cli-resolution.sh <cycle>}"
LEDGER="${EVOLVE_LEDGER:-${EVOLVE_PROJECT_ROOT:-.}/.evolve/ledger.jsonl}"

echo "## CLI Resolution"
echo
echo "_Auto-rendered from \`.evolve/ledger.jsonl\` by \`scripts/observability/render-cli-resolution.sh ${CYCLE}\`. Do NOT edit manually._"
echo

if [ ! -f "$LEDGER" ]; then
    echo "_(ledger missing; no entries to render)_"
    exit 0
fi

# Filter ledger entries for this cycle's agent_subprocess kind, sort by entry_seq,
# emit one row per role.
ROWS=$(jq -r --argjson cycle "$CYCLE" '
    select(.cycle == $cycle and .kind == "agent_subprocess") |
    [
        (.role // "?"),
        ((.cli_resolution.target_cli // .cli_resolution.cli) // "?"),
        ((.cli_resolution.model // .model) // "?"),
        (.cli_resolution.source // "?"),
        (.cli_resolution.mode // "")
    ] | @tsv
' "$LEDGER" 2>/dev/null)

if [ -z "$ROWS" ]; then
    echo "_(no agent_subprocess ledger entries for cycle ${CYCLE})_"
    exit 0
fi

echo "| Phase | Actual CLI | Actual Model | Source | Mode |"
echo "|-------|------------|--------------|--------|------|"
# Use printf to handle tab-separated values from jq -r '@tsv'.
echo "$ROWS" | awk -F'\t' '{ printf "| %s | %s | %s | %s | %s |\n", $1, $2, $3, $4, $5 }'
