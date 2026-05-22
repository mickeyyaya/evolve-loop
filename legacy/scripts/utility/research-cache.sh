#!/usr/bin/env bash
# research-cache.sh — Per-task research cache management CLI (v9.X.0+).
#
# Queries and manages the evolve-loop per-task research cache backed by
# state.json:researchCache.entries[] and .evolve/research/by-task/<fp>.md.
# Feature is gated by EVOLVE_RESEARCH_CACHE_ENABLED=1 (default OFF).
#
# Usage:
#   bash scripts/utility/research-cache.sh check <task_id>
#   bash scripts/utility/research-cache.sh invalidate <fp> [--reason TEXT]
#   bash scripts/utility/research-cache.sh list [--task <task_id>]
#   bash scripts/utility/research-cache.sh gc [--dry-run]
#
# Exit codes for 'check':
#   0  = HIT         — valid entry, not expired, files match
#  10  = STALE       — entry exists, age > EVOLVE_RESEARCH_CACHE_MAX_AGE
#  20  = MISS        — entry not in index (task_id not in researchCache)
#  30  = INVALIDATED — entry exists but invalidated=true
#  40  = NO_ENTRY    — task_id not found in carryoverTodos[] at all
#  50  = DISABLED    — EVOLVE_RESEARCH_CACHE_ENABLED != 1 (no-op mode)
#
# Exit codes for other commands:
#   0  = success
#   1  = error (bad args, missing jq, etc.)

set -uo pipefail

__self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Resolve project root via resolve-roots.sh if available
if [ -f "$__self_dir/../lifecycle/resolve-roots.sh" ]; then
    . "$__self_dir/../lifecycle/resolve-roots.sh" 2>/dev/null || true
fi
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(cd "$__self_dir/../.." && pwd)}"
STATE_JSON="$PROJECT_ROOT/.evolve/state.json"
CACHE_BASE="$PROJECT_ROOT/.evolve/research/by-task"
ARCHIVE_LOG="$PROJECT_ROOT/.evolve/archive/lessons/research-cache-archive.jsonl"
LEDGER="${EVOLVE_LEDGER_OVERRIDE:-$PROJECT_ROOT/.evolve/ledger.jsonl}"
MAX_AGE="${EVOLVE_RESEARCH_CACHE_MAX_AGE:-5}"
CURRENT_CYCLE="${EVOLVE_CURRENT_CYCLE:-0}"

log()  { echo "[research-cache] $*" >&2; }
fail() { log "ERROR: $*"; exit 1; }

# Check if feature is enabled
_cache_enabled() {
    [ "${EVOLVE_RESEARCH_CACHE_ENABLED:-0}" = "1" ]
}

# Compute sha256 of a string
_sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        printf '%s' "$1" | sha256sum | awk '{print $1}'
    else
        printf '%s' "$1" | shasum -a 256 | awk '{print $1}'
    fi
}

# Append a ledger event (best-effort, never fails pipeline)
_ledger_event() {
    local kind="$1" fp="$2" task_id="$3" cycle="$4" detail="${5:-}"
    [ -f "$LEDGER" ] || return 0
    local ts; ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local entry; entry=$(jq -cn \
        --arg ts "$ts" \
        --arg kind "$kind" \
        --arg fp "$fp" \
        --arg task_id "$task_id" \
        --argjson cycle "${cycle:-0}" \
        --arg detail "$detail" \
        '{ts: $ts, kind: $kind, fingerprint: $fp, task_id: $task_id,
          cycle: $cycle, detail: $detail}' 2>/dev/null) || return 0
    printf '%s\n' "$entry" >> "$LEDGER" 2>/dev/null || true
}

# --- cmd: check <task_id> ---
cmd_check() {
    local task_id="$1"
    [ -z "$task_id" ] && fail "check requires <task_id>"

    if ! _cache_enabled; then
        log "DISABLED (EVOLVE_RESEARCH_CACHE_ENABLED != 1)"
        exit 50
    fi

    [ -f "$STATE_JSON" ] || { log "state.json not found; treating as NO_ENTRY"; exit 40; }
    command -v jq >/dev/null 2>&1 || fail "jq is required"

    # Look up task in carryoverTodos[]
    local todo_json; todo_json=$(jq -r --arg id "$task_id" \
        '.carryoverTodos[]? | select(.id == $id)' "$STATE_JSON" 2>/dev/null || true)
    if [ -z "$todo_json" ]; then
        log "NO_ENTRY: task_id=$task_id not in carryoverTodos[]"
        exit 40
    fi

    # Check for research_fingerprint field
    local fp; fp=$(echo "$todo_json" | jq -r '.research_fingerprint // empty' 2>/dev/null || true)
    if [ -z "$fp" ]; then
        log "MISS: task_id=$task_id has no research_fingerprint"
        exit 20
    fi

    # Look up in researchCache.entries
    local entry_json; entry_json=$(jq -r --arg fp "$fp" \
        '.researchCache.entries[$fp] // empty' "$STATE_JSON" 2>/dev/null || true)
    if [ -z "$entry_json" ]; then
        log "MISS: fingerprint=$fp not in researchCache.entries"
        exit 20
    fi

    # Check invalidated flag
    local invalidated; invalidated=$(echo "$entry_json" | jq -r '.invalidated // false' 2>/dev/null || echo "false")
    if [ "$invalidated" = "true" ]; then
        local reason; reason=$(echo "$entry_json" | jq -r '.invalidation_reason // "unknown"' 2>/dev/null || echo "unknown")
        log "INVALIDATED: fingerprint=$fp reason=$reason"
        exit 30
    fi

    # Check age
    local produced_at_cycle; produced_at_cycle=$(echo "$entry_json" | jq -r '.produced_at_cycle // 0' 2>/dev/null || echo "0")
    local age=$(( ${CURRENT_CYCLE:-0} - produced_at_cycle ))
    # If CURRENT_CYCLE not set or 0, skip age check
    if [ "${CURRENT_CYCLE:-0}" -gt 0 ] && [ "$age" -gt "$MAX_AGE" ]; then
        log "STALE: fingerprint=$fp age=${age} cycles > max_age=${MAX_AGE}"
        exit 10
    fi

    # Check cache file + sidecar exist
    local cache_file="$CACHE_BASE/${fp}.md"
    local sidecar="$CACHE_BASE/${fp}.json"
    if [ ! -f "$cache_file" ] || [ ! -f "$sidecar" ]; then
        log "MISS: cache file or sidecar missing for fp=$fp"
        exit 20
    fi

    # Verify sidecar fingerprint matches index
    local sidecar_fp; sidecar_fp=$(jq -r '.fingerprint_sha // empty' "$sidecar" 2>/dev/null || true)
    if [ -n "$sidecar_fp" ] && [ "$sidecar_fp" != "$fp" ]; then
        log "MISS: sidecar fingerprint mismatch (index=$fp, sidecar=$sidecar_fp)"
        exit 20
    fi

    # Recompute fingerprint from current todo fields and verify match
    local action criteria files recomputed_fp
    action=$(echo "$todo_json" | jq -r '.action // empty' 2>/dev/null || true)
    criteria=$(echo "$todo_json" | jq -r '.acceptance_criteria // empty' 2>/dev/null || true)
    files=$(echo "$todo_json" | jq -r '.target_files // empty' 2>/dev/null || true)
    if [ -n "$action" ]; then
        recomputed_fp=$(bash "$__self_dir/task-fingerprint.sh" \
            --action "$action" \
            --criteria "$criteria" \
            --files "$files" 2>/dev/null || true)
        if [ -n "$recomputed_fp" ] && [ "$recomputed_fp" != "$fp" ]; then
            log "MISS: recomputed fingerprint $recomputed_fp != stored $fp (scope drift)"
            exit 20
        fi
    fi

    local research_path; research_path=$(echo "$entry_json" | jq -r '.research_path // empty' 2>/dev/null || true)
    _ledger_event "research_cache_hit" "$fp" "$task_id" "${CURRENT_CYCLE:-0}" "age=${age}"
    log "HIT: task_id=$task_id fp=$fp age=${age}c path=${research_path}"
    exit 0
}

# --- cmd: invalidate <fp> [--reason TEXT] ---
cmd_invalidate() {
    local fp="${1:-}"
    [ -z "$fp" ] && fail "invalidate requires <fingerprint>"
    shift
    local reason="manual"
    while [ $# -gt 0 ]; do
        case "$1" in
            --reason) reason="$2"; shift 2 ;;
            *) fail "invalidate: unknown argument: $1" ;;
        esac
    done

    command -v jq >/dev/null 2>&1 || fail "jq is required"
    [ -f "$STATE_JSON" ] || fail "state.json not found"

    local entry; entry=$(jq -r --arg fp "$fp" '.researchCache.entries[$fp] // empty' "$STATE_JSON" 2>/dev/null || true)
    if [ -z "$entry" ]; then
        log "WARN: fingerprint=$fp not found in researchCache.entries — no-op"
        exit 0
    fi

    local already; already=$(echo "$entry" | jq -r '.invalidated // false' 2>/dev/null || echo "false")
    if [ "$already" = "true" ]; then
        log "already invalidated (reason=$(echo "$entry" | jq -r '.invalidation_reason // "?"' 2>/dev/null))"
        exit 0
    fi

    local tmp="${STATE_JSON}.tmp.$$"
    jq --arg fp "$fp" --arg reason "$reason" \
        '.researchCache.entries[$fp].invalidated = true |
         .researchCache.entries[$fp].invalidation_reason = $reason' \
        "$STATE_JSON" > "$tmp" && mv -f "$tmp" "$STATE_JSON"

    local task_id; task_id=$(echo "$entry" | jq -r '.task_id // "unknown"' 2>/dev/null || echo "unknown")
    _ledger_event "research_cache_invalidate" "$fp" "$task_id" "${CURRENT_CYCLE:-0}" "reason=$reason"
    log "INVALIDATED: fp=$fp reason=$reason"
}

# --- cmd: list [--task <task_id>] ---
cmd_list() {
    local filter_task=""
    while [ $# -gt 0 ]; do
        case "$1" in
            --task) filter_task="$2"; shift 2 ;;
            *) fail "list: unknown argument: $1" ;;
        esac
    done

    command -v jq >/dev/null 2>&1 || fail "jq is required"
    [ -f "$STATE_JSON" ] || { log "state.json not found"; exit 0; }

    local entries; entries=$(jq -r '.researchCache.entries // {} | to_entries[]' "$STATE_JSON" 2>/dev/null || true)
    if [ -z "$entries" ]; then
        echo "(no cache entries)"
        return
    fi

    local filter_arg=""
    [ -n "$filter_task" ] && filter_arg="--arg filter_task $filter_task"

    jq -r --arg filter_task "$filter_task" \
        '.researchCache.entries // {} | to_entries[]
         | select(($filter_task == "") or (.value.task_id == $filter_task))
         | "\(.key[0:16])... task=\(.value.task_id) cycle=\(.value.produced_at_cycle) invalidated=\(.value.invalidated)"' \
        "$STATE_JSON" 2>/dev/null || true
}

# --- cmd: gc [--dry-run] ---
cmd_gc() {
    local dry_run=0
    while [ $# -gt 0 ]; do
        case "$1" in
            --dry-run) dry_run=1; shift ;;
            *) fail "gc: unknown argument: $1" ;;
        esac
    done

    command -v jq >/dev/null 2>&1 || fail "jq is required"
    [ -f "$STATE_JSON" ] || { log "state.json not found"; exit 0; }

    local removed=0
    local age_cutoff=$(( ${CURRENT_CYCLE:-0} - MAX_AGE ))

    # Collect FPs to remove: invalidated OR age-exceeded (when CURRENT_CYCLE known)
    local fps_to_remove; fps_to_remove=$(jq -r \
        --argjson cutoff "$age_cutoff" \
        --argjson cur "${CURRENT_CYCLE:-0}" \
        '.researchCache.entries // {} | to_entries[]
         | select(.value.invalidated == true
                  or ($cur > 0 and .value.produced_at_cycle < $cutoff))
         | .key' "$STATE_JSON" 2>/dev/null || true)

    if [ -z "$fps_to_remove" ]; then
        log "gc: nothing to remove"
        return
    fi

    # Archive entries and remove files
    mkdir -p "$(dirname "$ARCHIVE_LOG")" 2>/dev/null || true
    local ts; ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    while IFS= read -r fp; do
        [ -z "$fp" ] && continue
        local entry; entry=$(jq -r --arg fp "$fp" '.researchCache.entries[$fp]' "$STATE_JSON" 2>/dev/null || true)
        [ -z "$entry" ] && continue

        if [ "$dry_run" = "1" ]; then
            log "gc [dry-run]: would remove fp=${fp}"
        else
            # Archive
            printf '%s\n' "$entry" >> "$ARCHIVE_LOG" 2>/dev/null || true
            # Remove files
            rm -f "$CACHE_BASE/${fp}.md" "$CACHE_BASE/${fp}.json" 2>/dev/null || true
            # Remove from state.json
            local tmp="${STATE_JSON}.tmp.$$"
            jq --arg fp "$fp" 'del(.researchCache.entries[$fp])' "$STATE_JSON" > "$tmp" && mv -f "$tmp" "$STATE_JSON"
            log "gc: removed fp=${fp}"
            removed=$(( removed + 1 ))
        fi
    done <<EOF
$fps_to_remove
EOF

    [ "$dry_run" = "0" ] && log "gc: removed $removed entries"
    [ "$dry_run" = "1" ] && log "gc [dry-run]: would remove $(echo "$fps_to_remove" | grep -c .) entries"
}

# --- Main ---

[ $# -ge 1 ] || {
    sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
    exit 1
}

CMD="$1"; shift

case "$CMD" in
    check)      cmd_check "${1:-}" ;;
    invalidate) cmd_invalidate "$@" ;;
    list)       cmd_list "$@" ;;
    gc)         cmd_gc "$@" ;;
    --help|-h)
        sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
        exit 0
        ;;
    *) echo "[research-cache] unknown command: $CMD (check|invalidate|list|gc)" >&2; exit 1 ;;
esac
