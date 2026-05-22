#!/usr/bin/env bash
# promote-research-cache.sh — Single-writer promotion of staged research to
# the canonical per-task research cache (v9.X.0+).
#
# Runs after Scout+Triage, before Builder dispatch (called by orchestrator).
# Reads .evolve/runs/cycle-N/workers/research-cache-staging/<fp>.md files
# staged by Scout and promotes them to the canonical location:
#   .evolve/research/by-task/<fp>.md      — Markdown research artifact
#   .evolve/research/by-task/<fp>.json    — sidecar state-index mirror
#
# Also atomically updates:
#   state.json:researchCache.entries[fp]         — index entry
#   state.json:carryoverTodos[*].research_pointer/_fingerprint/_cycle  — backlink
#   .evolve/ledger.jsonl                          — kind=research_cache_write
#
# NOOP when EVOLVE_RESEARCH_CACHE_ENABLED != 1.
# Idempotent: skips already-promoted entries.
#
# Usage:
#   bash scripts/lifecycle/promote-research-cache.sh <cycle> <workspace_path>
#   bash scripts/lifecycle/promote-research-cache.sh <cycle> <workspace_path> --dry-run
#
# Exit codes:
#   0  — success (or no-op)
#   1  — fatal error (missing jq, bad state.json)

set -uo pipefail

__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$__rr_self/resolve-roots.sh" ]; then
    . "$__rr_self/resolve-roots.sh" 2>/dev/null || true
fi
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(cd "$__rr_self/../.." && pwd)}"
REPO_ROOT="$PROJECT_ROOT"

CYCLE=""
WORKSPACE=""
DRY_RUN=0

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run) DRY_RUN=1; shift ;;
        --help|-h) sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*)       echo "[promote-research-cache] unknown flag: $1" >&2; exit 1 ;;
        *)
            if [ -z "$CYCLE" ]; then
                CYCLE="$1"
            elif [ -z "$WORKSPACE" ]; then
                WORKSPACE="$1"
            else
                echo "[promote-research-cache] too many positional args" >&2; exit 1
            fi
            shift
            ;;
    esac
done

[ -n "$CYCLE" ]     || { echo "[promote-research-cache] usage: $0 <cycle> <workspace_path>" >&2; exit 1; }
[ -n "$WORKSPACE" ] || { echo "[promote-research-cache] usage: $0 <cycle> <workspace_path>" >&2; exit 1; }

log()  { echo "[promote-research-cache] $*" >&2; }
fail() { log "ERROR: $*"; exit 1; }

# Feature gate
if [ "${EVOLVE_RESEARCH_CACHE_ENABLED:-0}" != "1" ]; then
    log "NOOP (EVOLVE_RESEARCH_CACHE_ENABLED != 1)"
    exit 0
fi

command -v jq >/dev/null 2>&1 || fail "jq is required"

STATE_JSON="$PROJECT_ROOT/.evolve/state.json"
CACHE_BASE="$PROJECT_ROOT/.evolve/research/by-task"
LEDGER="${EVOLVE_LEDGER_OVERRIDE:-$PROJECT_ROOT/.evolve/ledger.jsonl}"
STAGING_DIR="$WORKSPACE/workers/research-cache-staging"

# Ensure state.json exists and has a researchCache field
[ -f "$STATE_JSON" ] || fail "state.json not found at $STATE_JSON"

# Initialize researchCache if absent
has_rc=$(jq -r '.researchCache // "null"' "$STATE_JSON" 2>/dev/null || echo "null")
if [ "$has_rc" = "null" ]; then
    if [ "$DRY_RUN" = "0" ]; then
        local_tmp="${STATE_JSON}.tmp.$$"
        jq '.researchCache = {"version": 1, "entries": {}}' "$STATE_JSON" > "$local_tmp" \
            && mv -f "$local_tmp" "$STATE_JSON"
        log "initialized researchCache in state.json"
    else
        log "[dry-run] would initialize researchCache in state.json"
    fi
fi

# Ensure canonical directory exists
if [ "$DRY_RUN" = "0" ]; then
    mkdir -p "$CACHE_BASE" 2>/dev/null || fail "cannot create $CACHE_BASE"
fi

# Check for staged files
if [ ! -d "$STAGING_DIR" ]; then
    log "no staging dir at $STAGING_DIR — nothing to promote"
    exit 0
fi

staged_count=$(find "$STAGING_DIR" -maxdepth 1 -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')
if [ "${staged_count:-0}" = "0" ]; then
    log "no staged files in $STAGING_DIR"
    exit 0
fi

log "found $staged_count staged research file(s) in $STAGING_DIR"

# Ledger helpers (matches subagent-run.sh chain pattern)
_sha256_stdin() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}';
    else shasum -a 256 | awk '{print $1}'; fi
}

_ledger_chain_link() {
    local prev_hash="0000000000000000000000000000000000000000000000000000000000000000"
    local entry_seq=0
    if [ -f "$LEDGER" ] && [ -s "$LEDGER" ]; then
        local last_line; last_line=$(tail -1 "$LEDGER" 2>/dev/null || echo "")
        [ -n "$last_line" ] && prev_hash=$(printf '%s' "$last_line" | _sha256_stdin)
        entry_seq=$(wc -l < "$LEDGER" 2>/dev/null | tr -d ' ' || echo 0)
        [ -z "$entry_seq" ] && entry_seq=0
    fi
    printf '%s %s\n' "$prev_hash" "$entry_seq"
}

_ledger_update_tip() {
    local seq="$1" sha="$2"
    local tip_file; tip_file="$(dirname "$LEDGER")/ledger.tip"
    local tmp="${tip_file}.tmp.$$"
    printf '%s:%s\n' "$seq" "$sha" > "$tmp" 2>/dev/null \
        && mv -f "$tmp" "$tip_file" 2>/dev/null \
        || rm -f "$tmp" 2>/dev/null
}

_emit_ledger_event() {
    local fp="$1" task_id="$2" research_path="$3"
    [ -f "$LEDGER" ] || return 0
    local ts; ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local chain_link prev_hash entry_seq
    chain_link=$(_ledger_chain_link)
    prev_hash="${chain_link%% *}"
    entry_seq="${chain_link##* }"
    local new_line; new_line=$(jq -cn \
        --arg ts "$ts" \
        --argjson cycle "${CYCLE:-0}" \
        --arg fp "$fp" \
        --arg task_id "$task_id" \
        --arg research_path "$research_path" \
        --argjson entry_seq "$entry_seq" \
        --arg prev_hash "$prev_hash" \
        '{ts: $ts, cycle: $cycle, kind: "research_cache_write",
          fingerprint: $fp, task_id: $task_id, research_path: $research_path,
          entry_seq: $entry_seq, prev_hash: $prev_hash}' 2>/dev/null) || return 0
    printf '%s\n' "$new_line" >> "$LEDGER" 2>/dev/null || return 0
    local new_sha; new_sha=$(printf '%s' "$new_line" | _sha256_stdin)
    _ledger_update_tip "$entry_seq" "$new_sha"
}

# Process each staged file
PROMOTED=0
SKIPPED=0
TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Iterate staged .md files using find + while loop (bash 3.2 compat, no mapfile)
while IFS= read -r staged_file; do
    [ -z "$staged_file" ] && continue
    [ -f "$staged_file" ] || continue

    # Extract fp from filename (basename without .md)
    staged_basename="$(basename "$staged_file")"
    fp="${staged_basename%.md}"

    # Validate fp looks like a sha256 (64 hex chars)
    if ! printf '%s' "$fp" | grep -qE '^[0-9a-f]{64}$'; then
        log "SKIP: $staged_file — filename is not a sha256 (got: $fp)"
        SKIPPED=$(( SKIPPED + 1 ))
        continue
    fi

    dest_md="$CACHE_BASE/${fp}.md"
    dest_json="$CACHE_BASE/${fp}.json"

    # Idempotency: skip if already promoted
    if [ -f "$dest_md" ] && [ -f "$dest_json" ]; then
        log "SKIP (already promoted): fp=${fp}"
        SKIPPED=$(( SKIPPED + 1 ))
        continue
    fi

    # Read frontmatter from staged file to extract task_id and scope_summary
    task_id=$(grep -m1 '^task_id:' "$staged_file" | sed 's/^task_id:[[:space:]]*//' | sed 's/^"//' | sed 's/"$//' || echo "unknown")
    scope_summary=$(grep -m1 '^scope_summary:' "$staged_file" | sed 's/^scope_summary:[[:space:]]*//' | sed 's/^"//' | sed 's/"$//' || echo "")
    topic_capsules=$(grep -m1 '^topic_capsules:' "$staged_file" | sed 's/^topic_capsules:[[:space:]]*//' || echo "[]")

    research_path=".evolve/research/by-task/${fp}.md"

    if [ "$DRY_RUN" = "1" ]; then
        log "[dry-run] would promote fp=${fp} task_id=${task_id}"
        PROMOTED=$(( PROMOTED + 1 ))
        continue
    fi

    # Step 1: Copy staged file to canonical location
    cp "$staged_file" "$dest_md" || { log "ERROR: copy failed for $staged_file"; continue; }

    # Step 2: Write sidecar JSON
    sidecar_tmp="${dest_json}.tmp.$$"
    jq -cn \
        --arg fp "$fp" \
        --arg task_id "$task_id" \
        --arg research_path "$research_path" \
        --argjson cycle "${CYCLE:-0}" \
        --arg ts "$TS" \
        --arg scope_summary "$scope_summary" \
        '{fingerprint_sha: $fp, task_id: $task_id, research_path: $research_path,
          produced_at_cycle: $cycle, produced_at_ts: $ts,
          scope_summary: $scope_summary}' > "$sidecar_tmp" \
        && mv -f "$sidecar_tmp" "$dest_json" \
        || { log "ERROR: sidecar write failed for fp=$fp"; rm -f "$sidecar_tmp"; continue; }

    # Step 3: Update state.json:researchCache.entries[fp]
    state_tmp="${STATE_JSON}.tmp.$$"
    jq --arg fp "$fp" \
       --arg task_id "$task_id" \
       --arg research_path "$research_path" \
       --argjson cycle "${CYCLE:-0}" \
       --arg ts "$TS" \
       --arg scope_summary "$scope_summary" \
       '.researchCache.entries[$fp] = {
           task_id: $task_id,
           fingerprint_sha: $fp,
           research_path: $research_path,
           produced_at_cycle: $cycle,
           produced_at_ts: $ts,
           scope_summary: $scope_summary,
           hits: 0,
           last_hit_cycle: null,
           invalidated: false,
           invalidation_reason: null
        }' "$STATE_JSON" > "$state_tmp" \
        && mv -f "$state_tmp" "$STATE_JSON" \
        || { log "ERROR: state.json update failed for fp=$fp"; rm -f "$state_tmp"; continue; }

    # Step 4: Add research_pointer/research_fingerprint/research_cycle to matching carryoverTodos
    state_tmp2="${STATE_JSON}.tmp.$$"
    jq --arg task_id "$task_id" \
       --arg fp "$fp" \
       --arg research_path "$research_path" \
       --argjson cycle "${CYCLE:-0}" \
       '(.carryoverTodos //= []) |
        .carryoverTodos |= map(
            if .id == $task_id
            then . + {
                research_pointer: $research_path,
                research_fingerprint: $fp,
                research_cycle: $cycle
            }
            else .
            end
        )' "$STATE_JSON" > "$state_tmp2" \
        && mv -f "$state_tmp2" "$STATE_JSON" \
        || { log "WARN: carryoverTodos backlink update failed for task_id=$task_id"; rm -f "$state_tmp2"; }

    # Step 5: Emit ledger event
    _emit_ledger_event "$fp" "$task_id" "$research_path"

    log "PROMOTED: fp=${fp} task_id=${task_id} → ${research_path}"
    PROMOTED=$(( PROMOTED + 1 ))

done < <(find "$STAGING_DIR" -maxdepth 1 -name "*.md" -type f 2>/dev/null | sort)

log "done: promoted=$PROMOTED skipped=$SKIPPED"
exit 0
