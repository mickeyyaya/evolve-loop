#!/usr/bin/env bash
#
# _capability-compose.sh — Compose N quality tiers, returning the lowest.
#
# Usage: _capability-compose.sh <tier1> [<tier2> ...]
# Stdout: lowest tier — none < degraded < hybrid < full
#
# Used by orchestrator-report.md generation and mixed-CLI cycle tests to
# determine a single cycle-level quality_tier from per-phase adapter tiers.
# Companion to _capability-check.sh's per-adapter aggregation.
#
# v8.51.0+ (ships with multi-LLM review cycle)
# Bash 3.2 compatible.

set -uo pipefail

mode_rank() {
    case "$1" in
        full)     echo 3 ;;
        hybrid)   echo 2 ;;
        degraded) echo 1 ;;
        *)        echo 0 ;;
    esac
}

rank_to_mode() {
    case "$1" in
        3) echo full ;;
        2) echo hybrid ;;
        1) echo degraded ;;
        *) echo none ;;
    esac
}

[ $# -ge 1 ] || { echo "none"; exit 0; }

LOW=3
for t in "$@"; do
    r=$(mode_rank "$t")
    if [ "$r" -lt "$LOW" ]; then
        LOW="$r"
    fi
done

rank_to_mode "$LOW"
