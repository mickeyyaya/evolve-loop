#!/usr/bin/env bash
#
# state-prune.sh — Operator utility for pruning state.json:failedApproaches (v8.22.0).
#
# WHY THIS EXISTS
#
# The orchestrator's adaptive-failure logic reads recentFailures (last 3
# failedApproaches entries). Pre-v8.22.0, infrastructure failures had no
# retention policy — stale entries from days/weeks ago permanently poisoned
# the lookback, causing the orchestrator to declare BLOCKED-SYSTEMIC even
# after the underlying issue (e.g., nested-sandbox EPERM) was resolved.
#
# v8.22.0 introduces:
#   - Structured classification taxonomy (infrastructure-transient, code-build-fail, etc.)
#   - Per-classification age-out windows (1d for transient, 30d for code)
#   - FIFO cap (50 entries)
#
# This utility lets operators prune entries by classification, age, or cycle
# without resorting to ad-hoc jq surgery. failure-adapter.sh also calls into
# the underlying cycle-state.sh prune-failed-approaches function for its
# automatic age-out behavior.
#
# Usage:
#   bash scripts/failure/state-prune.sh --classification <name> [--dry-run]
#       Remove all entries with that classification.
#   bash scripts/failure/state-prune.sh --age <duration>     [--dry-run]
#       Remove entries older than <duration> (e.g., 7d, 12h, 30m).
#   bash scripts/failure/state-prune.sh --cycle <number>     [--dry-run]
#       Remove the entry for that cycle id.
#   bash scripts/failure/state-prune.sh --all                [--dry-run]
#       Wipe failedApproaches entirely. Requires confirmation unless --yes.
#
# Output: JSON to stdout summarizing the operation:
#   {"before": N, "after": M, "removed": K, "matched_classification": [...]}
#
# Exit codes:
#   0  — pruned successfully (or dry-run preview emitted)
#   1  — argument error / state file missing
#   2  — confirmation declined for --all without --yes
# 127  — required binary missing (jq)

set -uo pipefail

# v8.18.0: dual-root. state.json lives in the user's project (writable side).
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"
unset __rr_self

STATE="${EVOLVE_STATE_FILE_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/state.json}"

log()  { echo "[state-prune] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

# ---- Arg parsing ----------------------------------------------------------

MODE=""
CLASSIFICATION=""
AGE=""
CYCLE=""
DRY_RUN=0
YES=0

while [ $# -gt 0 ]; do
    case "$1" in
        --classification)
            shift; [ $# -gt 0 ] || fail "--classification requires a value"
            MODE="classification"; CLASSIFICATION="$1"
            ;;
        --age)
            shift; [ $# -gt 0 ] || fail "--age requires a duration (e.g., 7d, 12h, 30m)"
            MODE="age"; AGE="$1"
            ;;
        --cycle)
            shift; [ $# -gt 0 ] || fail "--cycle requires a number"
            MODE="cycle"; CYCLE="$1"
            ;;
        --all)
            MODE="all"
            ;;
        --dry-run)
            DRY_RUN=1
            ;;
        --yes|-y)
            YES=1
            ;;
        --help|-h)
            sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            fail "unknown arg: $1 (try --help)"
            ;;
    esac
    shift
done

[ -n "$MODE" ] || fail "must specify one of: --classification, --age, --cycle, --all"

command -v jq >/dev/null 2>&1 || { log "missing required binary: jq"; exit 127; }

[ -f "$STATE" ] || {
    log "no state file at $STATE — nothing to prune"
    echo '{"before":0,"after":0,"removed":0,"reason":"no-state-file"}'
    exit 0
}

# ---- Compute predicate ----------------------------------------------------

# jq filter that returns true for entries to KEEP (false → prune).
case "$MODE" in
    classification)
        JQ_KEEP=". | select(.classification != \"$CLASSIFICATION\")"
        DESC="classification=$CLASSIFICATION"
        ;;
    age)
        # Convert duration to seconds. Supports: <N>d, <N>h, <N>m.
        case "$AGE" in
            *d) AGE_S=$(( ${AGE%d} * 86400 )) ;;
            *h) AGE_S=$(( ${AGE%h} * 3600 )) ;;
            *m) AGE_S=$(( ${AGE%m} * 60 )) ;;
            *)  fail "invalid duration '$AGE' (expected Nd, Nh, or Nm)" ;;
        esac
        NOW=$(date -u +%s)
        # recordedAt is ISO-8601 UTC. Convert via macOS or GNU date.
        # Skip entries whose recordedAt is *older* than (now - AGE_S).
        CUTOFF_S=$(( NOW - AGE_S ))
        # We need to compute each entry's epoch-seconds then compare. jq has
        # `fromdateiso8601` for that.
        JQ_KEEP=". | select((.recordedAt // \"\") | (try fromdateiso8601 catch 0) > $CUTOFF_S)"
        DESC="age<$AGE (cutoff_epoch=$CUTOFF_S)"
        ;;
    cycle)
        JQ_KEEP=". | select((.cycle | tostring) != \"$CYCLE\")"
        DESC="cycle=$CYCLE"
        ;;
    all)
        if [ "$YES" != "1" ] && [ "$DRY_RUN" != "1" ]; then
            log "REFUSED: --all wipes all failedApproaches. Re-run with --yes to confirm, or --dry-run to preview."
            exit 2
        fi
        JQ_KEEP="empty"   # keeps nothing
        DESC="all entries"
        ;;
esac

# ---- Compute before/after counts ------------------------------------------

BEFORE=$(jq '(.failedApproaches // []) | length' "$STATE")

# Use [.[] | filter] instead of .[] | filter to preserve array structure
# even when filter is `empty` (--all).
KEPT=$(jq --argjson n 0 "(.failedApproaches // []) | [.[] | $JQ_KEEP]" "$STATE")
AFTER=$(echo "$KEPT" | jq 'length')
REMOVED=$(( BEFORE - AFTER ))

# Build summary JSON.
SUMMARY=$(jq -nc \
    --argjson before "$BEFORE" \
    --argjson after "$AFTER" \
    --argjson removed "$REMOVED" \
    --arg mode "$MODE" \
    --arg desc "$DESC" \
    --argjson dry_run "$DRY_RUN" \
    '{before: $before, after: $after, removed: $removed, mode: $mode, predicate: $desc, dry_run: ($dry_run == 1)}')

log "$DESC: before=$BEFORE after=$AFTER removed=$REMOVED dry_run=$DRY_RUN"

if [ "$DRY_RUN" = "1" ]; then
    echo "$SUMMARY"
    exit 0
fi

# ---- Atomic write ---------------------------------------------------------

TMP="${STATE}.tmp.$$"
jq --argjson kept "$KEPT" '.failedApproaches = $kept' "$STATE" > "$TMP" || {
    rm -f "$TMP"
    fail "failed to compute pruned state"
}
mv -f "$TMP" "$STATE"

log "OK: pruned $REMOVED entries from $STATE"
echo "$SUMMARY"
exit 0
