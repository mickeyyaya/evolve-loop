#!/usr/bin/env bash
# inbox-audit.sh — Operator audit of .evolve/inbox/ lifecycle state (v9.6.1+, c38)
#
# Usage:
#   bash scripts/utility/inbox-audit.sh [--json] [--help]
#
# Shows all inbox files by state (queued/in-flight/processed/rejected/pending-retry)
# with git-log proof for processed entries.
#
# Flags:
#   --json   Machine-readable JSON array output
#   --help   Print this usage
#
# Exit codes:
#   0  always (read-only)

set -uo pipefail

__self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$__self_dir/../.." && pwd)"
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || echo "$REPO_ROOT")}"
INBOX_DIR="$PROJECT_ROOT/.evolve/inbox"

JSON_MODE=0

while [ $# -gt 0 ]; do
    case "$1" in
        --json)  JSON_MODE=1; shift ;;
        --help)
            awk '/^#!/{next} /^[^#]/{exit} /^#/{sub(/^# ?/,""); print}' "${BASH_SOURCE[0]}" >&2
            exit 0
            ;;
        *) echo "ERROR: unknown argument: $1" >&2; exit 1 ;;
    esac
done

# git_proof <task_id> → "<sha8>|<subject>" or ""
git_proof() {
    local task_id="$1" line sha subject
    line=$(git -C "$PROJECT_ROOT" log \
        --grep="^feat: cycle [0-9]\+ — ${task_id}\(:\| \)" \
        --format="%H %s" main 2>/dev/null | head -1) || true
    if [ -n "$line" ]; then
        sha="${line%% *}"
        sha="${sha:0:8}"
        subject="${line#* }"
        printf '%s|%s' "$sha" "$subject"
    fi
}

ROW_COUNT=0

# print_row <state> <task_id> <cycle> <git_sha> <subject>
print_row() {
    local state="$1" task_id="$2" cycle="$3" sha="$4" subject="$5"
    if [ "$JSON_MODE" -eq 1 ]; then
        if [ "$ROW_COUNT" -gt 0 ]; then printf ',\n'; fi
        printf '  {"state":"%s","task_id":"%s","cycle":"%s","git_sha":"%s","commit_subject":"%s"}' \
            "$state" "$task_id" "$cycle" "$sha" "$subject"
    else
        printf '%-14s  %-36s  %-6s  %-9s  %s\n' "$state" "$task_id" "$cycle" "$sha" "$subject"
    fi
    ROW_COUNT=$((ROW_COUNT + 1))
}

scan_queued() {
    [ -d "$INBOX_DIR" ] || return 0
    local f task_id
    for f in "$INBOX_DIR"/*.json; do
        [ -f "$f" ] || continue
        task_id=$(jq -r '.id // "unknown"' "$f" 2>/dev/null || echo "unknown")
        print_row "queued" "$task_id" "-" "-" "-"
    done
}

scan_in_flight() {
    [ -d "$INBOX_DIR/processing" ] || return 0
    local d cycle f task_id
    for d in "$INBOX_DIR/processing"/cycle-*/; do
        [ -d "$d" ] || continue
        cycle="${d%/}"; cycle="${cycle##*/cycle-}"
        for f in "$d"*.json; do
            [ -f "$f" ] || continue
            task_id=$(jq -r '.id // "unknown"' "$f" 2>/dev/null || echo "unknown")
            print_row "in-flight" "$task_id" "$cycle" "-" "-"
        done
    done
}

scan_processed() {
    [ -d "$INBOX_DIR/processed" ] || return 0
    local d cycle f task_id proof sha subject
    for d in "$INBOX_DIR/processed"/cycle-*/; do
        [ -d "$d" ] || continue
        cycle="${d%/}"; cycle="${cycle##*/cycle-}"
        for f in "$d"*.json; do
            [ -f "$f" ] || continue
            task_id=$(jq -r '.id // "unknown"' "$f" 2>/dev/null || echo "unknown")
            proof=$(git_proof "$task_id")
            if [ -n "$proof" ]; then
                sha="${proof%%|*}"
                subject="${proof#*|}"
            else
                sha="-"; subject="-"
            fi
            print_row "processed" "$task_id" "$cycle" "$sha" "$subject"
        done
    done
}

scan_rejected() {
    [ -d "$INBOX_DIR/rejected" ] || return 0
    local d cycle f task_id
    for d in "$INBOX_DIR/rejected"/cycle-*/; do
        [ -d "$d" ] || continue
        cycle="${d%/}"; cycle="${cycle##*/cycle-}"
        for f in "$d"*.json; do
            [ -f "$f" ] || continue
            task_id=$(jq -r '.id // "unknown"' "$f" 2>/dev/null || echo "unknown")
            print_row "rejected" "$task_id" "$cycle" "-" "-"
        done
    done
}

scan_retry() {
    [ -d "$INBOX_DIR/retry" ] || return 0
    local f task_id
    for f in "$INBOX_DIR/retry"/*.json; do
        [ -f "$f" ] || continue
        task_id=$(jq -r '.id // "unknown"' "$f" 2>/dev/null || echo "unknown")
        print_row "pending-retry" "$task_id" "-" "-" "-"
    done
}

if [ "$JSON_MODE" -eq 1 ]; then
    printf '[\n'
else
    printf '%-14s  %-36s  %-6s  %-9s  %s\n' "STATE" "TASK_ID" "CYCLE" "GIT_SHA" "COMMIT_SUBJECT"
    printf -- '------------------------------------------------------------------------------\n'
fi

scan_queued
scan_in_flight
scan_processed
scan_rejected
scan_retry

if [ "$JSON_MODE" -eq 1 ]; then
    printf '\n]\n'
fi

exit 0
