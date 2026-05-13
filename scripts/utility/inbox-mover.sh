#!/usr/bin/env bash
# inbox-mover.sh — Atomic inbox lifecycle transitions (v9.6.0+, c37)
#
# Subcommands:
#   claim <task_id> <cycle>                     inbox/ → processing/cycle-N/
#   promote <task_id> <new_state> [<cycle>] \   processing/ → processed/|rejected/|retry/
#           [--commit-sha <sha>]
#   recover-orphans                             processing/cycle-X/ → inbox/ (dead cycles)
#
# All state transitions use single atomic mv (same-FS). Ledger writes are
# best-effort — failure to write ledger never blocks a lifecycle operation.
# Exit codes: 0=success, 1=not-found (promote exits 0 for ship.sh compat),
#             2=mv-failed (claim only)

set -uo pipefail

# ── project root resolution ────────────────────────────────────────────────────
if [ -n "${EVOLVE_PROJECT_ROOT:-}" ]; then
    PROJECT_ROOT="$EVOLVE_PROJECT_ROOT"
else
    PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
fi

INBOX_DIR="$PROJECT_ROOT/.evolve/inbox"
LEDGER="$PROJECT_ROOT/.evolve/ledger.jsonl"

log_info()  { echo "[inbox-mover] $*" >&2; }
log_warn()  { echo "[inbox-mover] WARN: $*" >&2; }
log_error() { echo "[inbox-mover] ERROR: $*" >&2; }

# Append one ledger entry — best-effort, never fails the caller.
write_ledger() {
    local action="$1" task_id="$2" from_path="$3" to_path="$4"
    local cycle="${5:-}" git_sha="${6:-}" reason="${7:-}"
    local ts
    ts=$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "")
    # Build numeric cycle field (null when empty)
    local cycle_json="null"
    [ -n "$cycle" ] && cycle_json="$cycle"
    # Build quoted sha field (null when empty)
    local sha_json="null"
    [ -n "$git_sha" ] && sha_json="\"${git_sha}\""
    local json
    json=$(printf \
        '{"ts":"%s","class":"inbox-lifecycle","action":"%s","task_id":"%s","from":"%s","to":"%s","cycle":%s,"git_sha":%s,"reason":"%s"}' \
        "$ts" "$action" "$task_id" "$from_path" "$to_path" \
        "$cycle_json" "$sha_json" "$reason")
    # Best-effort: skip if ledger or its parent dir is not writable
    local ledger_dir
    ledger_dir=$(dirname "$LEDGER")
    if [ -w "$LEDGER" ] || [ -w "$ledger_dir" ]; then
        printf '%s\n' "$json" >> "$LEDGER" 2>/dev/null || true
    fi
}

# Return the path of the first file in <dir>/*.json whose .id == <task_id>,
# or exit 1 if not found.
find_file_by_task_id() {
    local dir="$1" task_id="$2"
    local f id
    for f in "$dir"/*.json; do
        [ -f "$f" ] || continue
        id=$(jq -r '.id // empty' "$f" 2>/dev/null || true)
        if [ "$id" = "$task_id" ]; then
            echo "$f"
            return 0
        fi
    done
    return 1
}

# ── subcommand: claim ──────────────────────────────────────────────────────────
# Move a file from inbox/ → processing/cycle-N/ atomically.
cmd_claim() {
    local task_id="${1:-}" cycle="${2:-}"
    if [ -z "$task_id" ] || [ -z "$cycle" ]; then
        log_error "usage: inbox-mover.sh claim <task_id> <cycle>"
        exit 1
    fi

    local src
    src=$(find_file_by_task_id "$INBOX_DIR" "$task_id" 2>/dev/null) || {
        log_warn "claim: task '$task_id' not found in $INBOX_DIR"
        exit 1
    }

    local base
    base=$(basename "$src")
    local dest_dir="$INBOX_DIR/processing/cycle-${cycle}"
    local dest="$dest_dir/$base"

    mkdir -p "$dest_dir" || {
        log_error "claim: mkdir -p '$dest_dir' failed"
        exit 2
    }

    if mv "$src" "$dest" 2>/dev/null; then
        log_info "claimed: $base → processing/cycle-${cycle}/"
        write_ledger "claim" "$task_id" \
            ".evolve/inbox/$base" \
            ".evolve/inbox/processing/cycle-${cycle}/$base" \
            "$cycle" "" "triage-claim"
        exit 0
    else
        log_warn "claim: mv failed for '$task_id' (may already be claimed by another cycle)"
        exit 2
    fi
}

# ── subcommand: promote ────────────────────────────────────────────────────────
# Move a file from processing/ → processed/|rejected/|retry/ atomically.
# Exits 0 even when source not found — ship.sh must never block on this.
cmd_promote() {
    local task_id="${1:-}" new_state="${2:-}"
    if [ -z "$task_id" ] || [ -z "$new_state" ]; then
        log_error "usage: inbox-mover.sh promote <task_id> <new_state> [<cycle>] [--commit-sha <sha>]"
        exit 1
    fi
    # Consume the two positional args; remaining are [<cycle>] [--commit-sha <sha>]
    shift 2 2>/dev/null || true

    local cycle="" commit_sha=""
    while [ $# -gt 0 ]; do
        case "$1" in
            --commit-sha)
                commit_sha="${2:-}"
                shift 2 2>/dev/null || shift 1 2>/dev/null || true
                ;;
            --*)
                shift
                ;;
            *)
                # First non-flag positional is cycle
                [ -z "$cycle" ] && cycle="$1"
                shift
                ;;
        esac
    done

    case "$new_state" in
        processed|rejected|retry) ;;
        *)
            log_error "promote: invalid state '$new_state'; must be processed|rejected|retry"
            exit 1
            ;;
    esac

    # Find source: processing/cycle-*/ first, then inbox/ as fallback
    local src="" src_rel="" d
    for d in "$INBOX_DIR/processing"/cycle-*/; do
        [ -d "$d" ] || continue
        src=$(find_file_by_task_id "$d" "$task_id" 2>/dev/null) && break || true
    done
    if [ -n "$src" ]; then
        src_rel="processing"
    else
        src=$(find_file_by_task_id "$INBOX_DIR" "$task_id" 2>/dev/null) || true
        [ -n "$src" ] && src_rel="inbox" || src_rel=""
    fi

    if [ -z "$src" ]; then
        log_warn "promote: task '$task_id' not found in processing/ or inbox/ — already moved?"
        exit 0
    fi

    local base
    base=$(basename "$src")
    local dest_dir dest

    case "$new_state" in
        processed)
            local eff_cycle="${cycle:-0}"
            dest_dir="$INBOX_DIR/processed/cycle-${eff_cycle}"
            if [ -n "$commit_sha" ]; then
                local sha8="${commit_sha:0:8}"
                dest="${dest_dir}/${sha8}-${base}"
            else
                dest="${dest_dir}/${base}"
            fi
            ;;
        rejected)
            dest_dir="$INBOX_DIR/rejected/cycle-${cycle:-0}"
            dest="${dest_dir}/${base}"
            ;;
        retry)
            dest_dir="$INBOX_DIR/retry"
            dest="${dest_dir}/${base}"
            ;;
    esac

    mkdir -p "$dest_dir" || {
        log_warn "promote: mkdir -p '$dest_dir' failed — leaving file in $src_rel/"
        write_ledger "promote-warn" "$task_id" \
            ".evolve/inbox/$src_rel/$base" "$dest" \
            "$cycle" "$commit_sha" "mkdir-failed"
        exit 0
    }

    if mv "$src" "$dest" 2>/dev/null; then
        log_info "promoted: $base → ${new_state}/"
        write_ledger "promote" "$task_id" \
            ".evolve/inbox/$src_rel/$base" "$dest" \
            "$cycle" "$commit_sha" "ship-promote-${new_state}"
        exit 0
    else
        log_warn "promote: mv failed for '$task_id' → $new_state (leaving in $src_rel/)"
        write_ledger "promote-warn" "$task_id" \
            ".evolve/inbox/$src_rel/$base" "$dest" \
            "$cycle" "$commit_sha" "mv-failed"
        exit 0
    fi
}

# ── subcommand: recover-orphans ────────────────────────────────────────────────
# Move files from processing/cycle-X/ back to inbox/ for any cycle X that is
# no longer active. Idempotent; safe to call on every dispatcher invocation.
cmd_recover_orphans() {
    local processing_dir="$INBOX_DIR/processing"
    if [ ! -d "$processing_dir" ]; then
        log_info "recover-orphans: no processing/ dir — nothing to do"
        exit 0
    fi

    # Determine active cycle from cycle-state.json
    local cycle_state="$PROJECT_ROOT/.evolve/cycle-state.json"
    local active_cycle="-1"
    if [ -f "$cycle_state" ]; then
        local cid
        cid=$(jq -r '.cycle_id // empty' "$cycle_state" 2>/dev/null || true)
        [ -n "$cid" ] && active_cycle="$cid"
    fi

    local recovered=0
    local d cycle_num f base task_id
    for d in "$processing_dir"/cycle-*/; do
        [ -d "$d" ] || continue
        # Extract cycle number from dir name (strip trailing slash, then prefix)
        cycle_num="${d%/}"
        cycle_num="${cycle_num##*/cycle-}"

        if [ "$cycle_num" = "$active_cycle" ]; then
            log_info "recover-orphans: cycle-${cycle_num}/ is active — skipping"
            continue
        fi

        for f in "$d"*.json; do
            [ -f "$f" ] || continue
            base=$(basename "$f")
            task_id=$(jq -r '.id // empty' "$f" 2>/dev/null || echo "unknown")
            if mv "$f" "$INBOX_DIR/$base" 2>/dev/null; then
                log_info "recovered: $base ← processing/cycle-${cycle_num}/"
                write_ledger "recover" "$task_id" \
                    ".evolve/inbox/processing/cycle-${cycle_num}/$base" \
                    ".evolve/inbox/$base" \
                    "$cycle_num" "" "orphan-recovery-cycle-not-active"
                recovered=$((recovered + 1))
            else
                log_warn "recover-orphans: mv failed for $base (leaving in processing/)"
            fi
        done
    done

    log_info "recover-orphans: $recovered file(s) recovered"
    exit 0
}

# ── dispatch ───────────────────────────────────────────────────────────────────
SUBCMD="${1:-}"
shift 2>/dev/null || true

case "$SUBCMD" in
    claim)           cmd_claim "$@" ;;
    promote)         cmd_promote "$@" ;;
    recover-orphans) cmd_recover_orphans ;;
    *)
        {
            echo "Usage: inbox-mover.sh <claim|promote|recover-orphans> [args]"
            echo "  claim <task_id> <cycle>"
            echo "  promote <task_id> <new_state> [<cycle>] [--commit-sha <sha>]"
            echo "    new_state: processed | rejected | retry"
            echo "  recover-orphans"
        } >&2
        exit 1
        ;;
esac
