#!/usr/bin/env bash
#
# reconcile-carryover-todos.sh — v8.57.0 Layer D: cycles_unpicked decay.
#
# Runs once per cycle, post-ship/post-retrospective. Reads the cycle's
# triage-decision.md (if Triage ran) AND/OR scout-report.md ## Carryover
# Decisions section (Layer S) to determine the disposition of each
# carryoverTodo, then mutates state.json:carryoverTodos[]:
#
#   * Picked (top_n / "include")     → reset cycles_unpicked=0; drop on PASS;
#                                       on FAIL/WARN retrospective re-emits via
#                                       merge-lesson-into-state.sh which
#                                       increments defer_count.
#   * Explicitly deferred ("defer")  → cycles_unpicked++; archive at threshold.
#   * Explicitly dropped ("drop")    → archive immediately (no decay needed).
#   * Not seen anywhere              → cycles_unpicked++ AND log WARN
#                                       (Layer S should make this rare).
#
# Archive sink: .evolve/archive/lessons/carryover-todos-archive.jsonl
# (gitignored under .evolve/*).
#
# Usage:
#   bash scripts/lifecycle/reconcile-carryover-todos.sh \
#       --cycle <N> --workspace <path> --verdict <PASS|WARN|FAIL>
#
# Env:
#   EVOLVE_CARRYOVER_TODO_MAX_UNPICKED  — threshold (default 3)
#   EVOLVE_PROJECT_ROOT                 — project root (auto from resolve-roots)
#
# Exit codes:
#   0 — reconcile complete (or no-op if no carryoverTodos)
#   1 — runtime failure (missing jq, malformed state)

set -uo pipefail

__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/resolve-roots.sh" 2>/dev/null || {
    EVOLVE_PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(pwd)}"
}
unset __rr_self

CYCLE=""
WORKSPACE=""
VERDICT="PASS"

while [ $# -gt 0 ]; do
    case "$1" in
        --cycle) CYCLE="$2"; shift 2 ;;
        --workspace) WORKSPACE="$2"; shift 2 ;;
        --verdict) VERDICT="$2"; shift 2 ;;
        -h|--help)
            sed -n '3,30p' "$0" >&2
            exit 0 ;;
        *) echo "[reconcile] WARN: unknown arg: $1" >&2; shift ;;
    esac
done

[ -n "$CYCLE" ] && [ -n "$WORKSPACE" ] || {
    echo "[reconcile] usage: --cycle N --workspace PATH [--verdict PASS|WARN|FAIL]" >&2
    exit 1
}

if [ -n "${EVOLVE_STATE_OVERRIDE:-}" ] && [ -z "${EVOLVE_STATE_FILE_OVERRIDE:-}" ]; then
    echo "[deprecation] EVOLVE_STATE_OVERRIDE is renamed to EVOLVE_STATE_FILE_OVERRIDE" >&2
    EVOLVE_STATE_FILE_OVERRIDE="$EVOLVE_STATE_OVERRIDE"
fi
STATE="${EVOLVE_STATE_FILE_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/state.json}"
MAX_UNPICKED="${EVOLVE_CARRYOVER_TODO_MAX_UNPICKED:-3}"
ARCHIVE_DIR="$EVOLVE_PROJECT_ROOT/.evolve/archive/lessons"
ARCHIVE_FILE="$ARCHIVE_DIR/carryover-todos-archive.jsonl"

log() { echo "[reconcile] $*" >&2; }

[ -f "$STATE" ] || { log "no state.json; nothing to reconcile"; exit 0; }
command -v jq >/dev/null 2>&1 || { log "ERROR: jq required"; exit 1; }

TODO_COUNT=$(jq -r '(.carryoverTodos // []) | length' "$STATE" 2>/dev/null)
[ "${TODO_COUNT:-0}" -eq 0 ] && { log "no carryoverTodos to reconcile"; exit 0; }

# ---- Parse this cycle's signals ------------------------------------------
# A carryoverTodo can appear in:
#   1. triage-decision.md ## top_n         → "include"
#   2. triage-decision.md ## deferred      → "defer"
#   3. triage-decision.md ## dropped       → "drop"
#   4. scout-report.md ## Carryover Decisions  → include|defer|drop (Layer S)
# The two sources can overlap; if BOTH disagree, triage wins (Triage is the
# authoritative scope-controller when it ran).

# Build a flat decisions map: id → include|defer|drop
DECISIONS_FILE=$(mktemp)
trap 'rm -f "$DECISIONS_FILE"' EXIT

# Scout's Carryover Decisions section parser. Format:
#   - <id>: include|defer|drop, reason: <text>
parse_scout_decisions() {
    local file="$1"
    [ -f "$file" ] || return 0
    awk '
        /^## *Carryover Decisions/   { in_section = 1; next }
        /^## /                       { in_section = 0 }
        in_section && /^- *[^:]+: *(include|defer|drop)/ {
            sub(/^- */, "")
            split($0, parts, ":")
            id = parts[1]; gsub(/^ +| +$/, "", id)
            decision = parts[2]; gsub(/^ +/, "", decision); gsub(/[, ].*$/, "", decision)
            print id "\t" decision
        }
    ' "$file"
}

# Triage section parser — items appear under ## top_n / ## deferred / ## dropped
parse_triage_section() {
    local file="$1" section="$2" decision="$3"
    [ -f "$file" ] || return 0
    awk -v sec="^## *${section}" -v dec="$decision" '
        $0 ~ sec                      { in_sec = 1; next }
        /^## /                        { in_sec = 0 }
        in_sec && /^- *[^:[:space:]]+/ {
            sub(/^- */, "")
            split($0, parts, ":")
            id = parts[1]; gsub(/^ +| +$/, "", id)
            print id "\t" dec
        }
    ' "$file"
}

# Collate decisions (triage takes precedence when both sources disagree).
{
    parse_scout_decisions "$WORKSPACE/scout-report.md"
    parse_triage_section  "$WORKSPACE/triage-decision.md" "top_n"    "include"
    parse_triage_section  "$WORKSPACE/triage-decision.md" "deferred" "defer"
    parse_triage_section  "$WORKSPACE/triage-decision.md" "dropped"  "drop"
} | awk -F'\t' 'NF==2 && $1 != "" { decisions[$1] = $2 } END { for (k in decisions) print k "\t" decisions[k] }' \
    > "$DECISIONS_FILE"

DECISION_LINES=$(wc -l < "$DECISIONS_FILE" | tr -d ' ')
log "parsed $DECISION_LINES decision(s) for cycle $CYCLE (verdict=$VERDICT, threshold=$MAX_UNPICKED)"

# ---- Apply decisions to each carryoverTodo --------------------------------
TMP_STATE=$(mktemp)
trap 'rm -f "$DECISIONS_FILE" "$TMP_STATE"' EXIT

mkdir -p "$ARCHIVE_DIR"

# Snapshot existing todos to a stable input.
EXISTING_FILE=$(mktemp)
jq -c '.carryoverTodos[]?' "$STATE" > "$EXISTING_FILE"

# Build the output array iteratively. We rewrite state.json with the resulting
# kept-todos at the end.
KEPT_FILE=$(mktemp)
: > "$KEPT_FILE"
WARN_IDS=()

while IFS= read -r todo_json; do
    [ -z "$todo_json" ] && continue
    id=$(echo "$todo_json" | jq -r '.id')
    decision=$(awk -F'\t' -v target="$id" '$1 == target { print $2; exit }' "$DECISIONS_FILE")

    case "$decision" in
        include)
            # Reset cycles_unpicked. On PASS, drop entirely. On WARN/FAIL, keep
            # so retrospective can re-emit (defer_count++). On WARN-NO-AUDIT or
            # other, keep with reset counter (operator review).
            if [ "$VERDICT" = "PASS" ]; then
                # Archive completion record (audit trail).
                echo "$todo_json" | jq -c --argjson cyc "$CYCLE" --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
                    '. + {archived_at: $ts, archived_at_cycle: $cyc, archive_reason: "completed-pass"}' \
                    >> "$ARCHIVE_FILE"
                log "DROP (PASS+include): $id → archive (completed)"
                continue
            else
                echo "$todo_json" | jq -c '.cycles_unpicked = 0' >> "$KEPT_FILE"
                log "RESET ($VERDICT+include): $id cycles_unpicked=0"
            fi
            ;;
        defer)
            # cycles_unpicked++. If at threshold, archive.
            new_cu=$(echo "$todo_json" | jq -r '(.cycles_unpicked // 0) + 1')
            if [ "$new_cu" -ge "$MAX_UNPICKED" ]; then
                echo "$todo_json" | jq -c --argjson cu "$new_cu" --argjson cyc "$CYCLE" --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
                    '.cycles_unpicked = $cu | . + {archived_at: $ts, archived_at_cycle: $cyc, archive_reason: "max-cycles-unpicked"}' \
                    >> "$ARCHIVE_FILE"
                log "ARCHIVE (defer hit threshold): $id cycles_unpicked=$new_cu >= $MAX_UNPICKED"
            else
                echo "$todo_json" | jq -c --argjson cu "$new_cu" '.cycles_unpicked = $cu' >> "$KEPT_FILE"
                log "DEFER: $id cycles_unpicked=$new_cu/$MAX_UNPICKED"
            fi
            ;;
        drop)
            echo "$todo_json" | jq -c --argjson cyc "$CYCLE" --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
                '. + {archived_at: $ts, archived_at_cycle: $cyc, archive_reason: "explicit-drop"}' \
                >> "$ARCHIVE_FILE"
            log "DROP (explicit): $id → archive"
            ;;
        "")
            # Not seen anywhere — defensive increment + WARN.
            new_cu=$(echo "$todo_json" | jq -r '(.cycles_unpicked // 0) + 1')
            if [ "$new_cu" -ge "$MAX_UNPICKED" ]; then
                echo "$todo_json" | jq -c --argjson cu "$new_cu" --argjson cyc "$CYCLE" --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
                    '.cycles_unpicked = $cu | . + {archived_at: $ts, archived_at_cycle: $cyc, archive_reason: "max-cycles-unpicked-unseen"}' \
                    >> "$ARCHIVE_FILE"
                log "ARCHIVE (unseen + threshold): $id cycles_unpicked=$new_cu"
            else
                echo "$todo_json" | jq -c --argjson cu "$new_cu" '.cycles_unpicked = $cu' >> "$KEPT_FILE"
            fi
            WARN_IDS+=("$id")
            log "WARN: $id not seen in scout/triage decisions; cycles_unpicked++ defensively (now $new_cu)"
            ;;
        *)
            log "WARN: $id has unknown decision '$decision'; treating as 'defer'"
            new_cu=$(echo "$todo_json" | jq -r '(.cycles_unpicked // 0) + 1')
            echo "$todo_json" | jq -c --argjson cu "$new_cu" '.cycles_unpicked = $cu' >> "$KEPT_FILE"
            ;;
    esac
done < "$EXISTING_FILE"

# Rebuild state.json with the kept todos.
KEPT_JSON=$(jq -cs '.' "$KEPT_FILE" 2>/dev/null || echo "[]")
jq --argjson kept "$KEPT_JSON" '.carryoverTodos = $kept' "$STATE" > "$TMP_STATE"
mv "$TMP_STATE" "$STATE"
rm -f "$EXISTING_FILE" "$KEPT_FILE"

NEW_COUNT=$(jq -r '.carryoverTodos | length' "$STATE")
log "DONE: $TODO_COUNT → $NEW_COUNT carryoverTodos (cycle $CYCLE, verdict $VERDICT)"

if [ "${#WARN_IDS[@]}" -gt 0 ]; then
    log "WARN: ${#WARN_IDS[@]} todo(s) not mentioned by Scout or Triage: ${WARN_IDS[*]}"
fi

exit 0
