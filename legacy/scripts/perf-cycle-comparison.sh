#!/usr/bin/env bash
#
# perf-cycle-comparison.sh — Operator-driven real-cycle perf comparison.
#
# Spends real money. Runs ONE live cycle through `bash run-cycle.sh`
# and ONE through `evolve cycle run`, capturing wall-time, peak RSS,
# and final cost. Writes a side-by-side markdown report.
#
# Budget: each side caps at $5 by default; override with PERF_BUDGET.
# Expected total: $5-15 depending on CLI verbosity and goal complexity.
#
# Usage:
#     bash legacy/scripts/perf-cycle-comparison.sh [--dry-run] [--goal "GOAL"]
#
# Output: perf-cycle-comparison-report.md
#
# Why structural benchmarks aren't enough: go/pkg/phaseproto's
# benchmarks measure orchestrator overhead (microseconds) — but LLM-
# bound paths (Scout, Builder, Auditor) dominate real cycles by
# orders of magnitude. This script's purpose is to confirm that the
# Go runtime doesn't ADD overhead on top of the LLM-bound paths.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${EVOLVE_GO_BIN:-$REPO_ROOT/go/bin/evolve}"
RUN_CYCLE="$REPO_ROOT/legacy/scripts/dispatch/run-cycle.sh"
REPORT="${PERF_REPORT:-$REPO_ROOT/perf-cycle-comparison-report.md}"
BUDGET="${PERF_BUDGET:-5}"
GOAL="perf-comparison fixture cycle"
DRY_RUN=0

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run)  DRY_RUN=1; shift ;;
        --goal)     GOAL="$2"; shift 2 ;;
        --help|-h)
            sed -n '3,30p' "$0"
            exit 0
            ;;
        *)
            echo "perf-cycle-comparison: unknown flag: $1" >&2
            exit 2
            ;;
    esac
done

log() { echo "[perf-comparison] $*"; }

if [ "$DRY_RUN" = "1" ]; then
    cat <<DRY
========== DRY RUN ==========

Mode:         dry-run (no money spent)
Repo root:    $REPO_ROOT
Go binary:    $GO_BIN
Bash entry:   $RUN_CYCLE
Budget:       \$$BUDGET per side
Goal:         "$GOAL"
Report:       $REPORT

What would happen in --full mode:
    /usr/bin/time -l bash $RUN_CYCLE "$GOAL"
    /usr/bin/time -l $GO_BIN cycle run --goal-hash=<sha> --budget-usd=$BUDGET
    diff key metrics: wall-time, peak RSS, total cost
DRY
    exit 0
fi

# --- pre-flight ---
for p in "$GO_BIN" "$RUN_CYCLE"; do
    [ -x "$p" ] || { log "FATAL: missing or not executable: $p"; exit 2; }
done

log "WARNING: spending real money via Claude CLI. Budget cap: \$$BUDGET per side."
log "Press Ctrl-C in the next 5s to abort..."
sleep 5

goal_hash=$(printf '%s' "$GOAL" | shasum | cut -c1-8)
log "goal_hash=$goal_hash"

# --- run bash side ---
log "bash side: bash $RUN_CYCLE \"$GOAL\""
bash_log=$(mktemp -t perf-bash.XXXXXX)
start_bash=$(date +%s)
/usr/bin/time -l bash "$RUN_CYCLE" "$GOAL" >"$bash_log" 2>&1
bash_rc=$?
end_bash=$(date +%s)
bash_wall=$((end_bash - start_bash))
bash_rss=$(grep -oE '[0-9]+\s+maximum resident set size' "$bash_log" | awk '{print $1}' | head -1)

# --- run Go side ---
log "Go side:   $GO_BIN cycle run --goal-hash=$goal_hash --budget-usd=$BUDGET"
go_log=$(mktemp -t perf-go.XXXXXX)
start_go=$(date +%s)
/usr/bin/time -l "$GO_BIN" cycle run \
    --goal-hash="$goal_hash" \
    --budget-usd="$BUDGET" >"$go_log" 2>&1
go_rc=$?
end_go=$(date +%s)
go_wall=$((end_go - start_go))
go_rss=$(grep -oE '[0-9]+\s+maximum resident set size' "$go_log" | awk '{print $1}' | head -1)

# --- report ---
{
    echo "# Perf cycle comparison report"
    echo
    echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "Goal: $GOAL"
    echo "Budget cap: \$$BUDGET per side"
    echo
    echo "## Results"
    echo
    echo "| Metric | Bash | Go | Δ |"
    echo "|---|---|---|---|"
    echo "| Wall time (s) | $bash_wall | $go_wall | $((go_wall - bash_wall)) |"
    echo "| Peak RSS (bytes) | ${bash_rss:-n/a} | ${go_rss:-n/a} | n/a |"
    echo "| Exit code | $bash_rc | $go_rc | — |"
    echo
    echo "## Bash log tail"
    echo
    echo '```'
    tail -10 "$bash_log"
    echo '```'
    echo
    echo "## Go log tail"
    echo
    echo '```'
    tail -10 "$go_log"
    echo '```'
} > "$REPORT"

rm -f "$bash_log" "$go_log"
log "report written: $REPORT"
log "bash: rc=$bash_rc wall=${bash_wall}s rss=${bash_rss:-?}"
log "go:   rc=$go_rc wall=${go_wall}s rss=${go_rss:-?}"
[ "$bash_rc" -eq 0 ] && [ "$go_rc" -eq 0 ]
