#!/usr/bin/env bash
#
# prune-ephemeral.sh — Apply TTL retention to phase-tracker ephemeral data.
#
# Retention policy:
#   .evolve/runs/cycle-*/.ephemeral/   →  7 days (configurable via EVOLVE_TRACKER_TTL_DAYS)
#   .evolve/dispatch-logs/*.log         → 30 days (configurable via EVOLVE_DISPATCH_LOG_TTL_DAYS)
#   .evolve/runs/cycle-*/*.md           → never pruned
#   .evolve/runs/cycle-*/*.json         → never pruned
#   .evolve/ledger.jsonl                → never pruned (append-only, tamper-evident)
#
# Usage:
#   bash scripts/observability/prune-ephemeral.sh           # apply
#   bash scripts/observability/prune-ephemeral.sh --dry-run # show what would be removed
#   bash scripts/observability/prune-ephemeral.sh --quiet   # silent unless something pruned
#
# Idempotent. Safe to run repeatedly. Safe to run while a cycle is in progress
# (recently-modified files are protected by the mtime filter).
#
# Exit codes:
#   0 — success (whether anything was pruned or not)
#  10 — bad arguments

set -uo pipefail

DRY_RUN=0
QUIET=0
TRACKER_TTL_DAYS="${EVOLVE_TRACKER_TTL_DAYS:-7}"
DISPATCH_LOG_TTL_DAYS="${EVOLVE_DISPATCH_LOG_TTL_DAYS:-30}"

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run) DRY_RUN=1 ;;
        --quiet) QUIET=1 ;;
        --help|-h) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[prune-ephemeral] unknown flag: $1" >&2; exit 10 ;;
        *) echo "[prune-ephemeral] unexpected arg: $1" >&2; exit 10 ;;
    esac
    shift
done

[[ "$TRACKER_TTL_DAYS" =~ ^[0-9]+$ ]] || { echo "[prune-ephemeral] EVOLVE_TRACKER_TTL_DAYS must be integer" >&2; exit 10; }
[[ "$DISPATCH_LOG_TTL_DAYS" =~ ^[0-9]+$ ]] || { echo "[prune-ephemeral] EVOLVE_DISPATCH_LOG_TTL_DAYS must be integer" >&2; exit 10; }

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RUNS_DIR="${RUNS_DIR_OVERRIDE:-$PROJECT_ROOT/.evolve/runs}"
DISPATCH_LOGS_DIR="$PROJECT_ROOT/.evolve/dispatch-logs"

log() { [ "$QUIET" = "1" ] || echo "[prune-ephemeral] $*"; }

PRUNED_EPHEMERAL=0
PRUNED_LOGS=0

# Phase 1: .ephemeral/ subtrees under cycle-N/
if [ -d "$RUNS_DIR" ]; then
    while IFS= read -r d; do
        if [ "$DRY_RUN" = "1" ]; then
            log "DRY-RUN would remove $d"
        else
            rm -rf "$d" && log "removed $d"
        fi
        PRUNED_EPHEMERAL=$((PRUNED_EPHEMERAL + 1))
    done < <(find "$RUNS_DIR" -maxdepth 3 -type d -name '.ephemeral' -mtime +"$TRACKER_TTL_DAYS" 2>/dev/null)
fi

# Phase 2: batch-*.log files under .evolve/dispatch-logs/
if [ -d "$DISPATCH_LOGS_DIR" ]; then
    while IFS= read -r f; do
        if [ "$DRY_RUN" = "1" ]; then
            log "DRY-RUN would remove $f"
        else
            rm -f "$f" && log "removed $f"
        fi
        PRUNED_LOGS=$((PRUNED_LOGS + 1))
    done < <(find "$DISPATCH_LOGS_DIR" -maxdepth 1 -type f -name 'batch-*.log' -mtime +"$DISPATCH_LOG_TTL_DAYS" 2>/dev/null)
fi

if [ "$QUIET" = "1" ]; then
    [ "$PRUNED_EPHEMERAL" -gt 0 ] || [ "$PRUNED_LOGS" -gt 0 ] && echo "[prune-ephemeral] pruned $PRUNED_EPHEMERAL ephemeral dirs, $PRUNED_LOGS log files"
else
    log "summary: ephemeral=$PRUNED_EPHEMERAL log_files=$PRUNED_LOGS (dry_run=$DRY_RUN, ttl_days=$TRACKER_TTL_DAYS / $DISPATCH_LOG_TTL_DAYS)"
fi
