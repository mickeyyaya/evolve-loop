#!/usr/bin/env bash
# inject-task.sh — Operator task-injection CLI for the evolve-loop inbox API (v9.5.0+).
#
# Validates schema, writes .evolve/inbox/<ts>-<rand>.json atomically.
# Triage ingests inbox files at phase start of the next cycle.
#
# Usage:
#   bash scripts/utility/inject-task.sh \
#     --priority HIGH \
#     --action "Fix the X issue" \
#     [--weight 0.85] \
#     [--evidence-pointer "url"] \
#     [--note "operator context"] \
#     [--id custom-task-id] \
#     [--injected-by operator|test|automation] \
#     [--dry-run]
#
# --injected-by: who is injecting this task (default: operator)
#
# Exit codes:
#   0   success
#   10  validation error (bad priority / weight / action)
#   11  id collision (already in state.json or inbox)
#   12  filesystem error (cannot write inbox)

set -uo pipefail

__self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$__self_dir/../.." && pwd)"
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}"

PRIORITY=""
ACTION=""
WEIGHT=""
EVIDENCE_POINTER=""
OPERATOR_NOTE=""
TASK_ID=""
DRY_RUN=0
INJECTED_BY="operator"
RESEARCH_POINTER=""
INVALIDATE_CACHE_FP=""

while [ $# -gt 0 ]; do
    case "$1" in
        --priority)          PRIORITY="$2";           shift 2 ;;
        --action)            ACTION="$2";              shift 2 ;;
        --weight)            WEIGHT="$2";              shift 2 ;;
        --evidence-pointer)  EVIDENCE_POINTER="$2";   shift 2 ;;
        --note)              OPERATOR_NOTE="$2";       shift 2 ;;
        --id)                TASK_ID="$2";             shift 2 ;;
        --dry-run)           DRY_RUN=1;                shift ;;
        --injected-by)       INJECTED_BY="$2";         shift 2 ;;
        --research-pointer)  RESEARCH_POINTER="$2";    shift 2 ;;
        --invalidate-cache)  INVALIDATE_CACHE_FP="$2"; shift 2 ;;
        --help)              awk '/^#!/{next} /^[^#]/{exit} /^#/{sub(/^# ?/,""); print}' "${BASH_SOURCE[0]}" >&2; exit 0 ;;
        --)                  shift; break ;;
        *) echo "ERROR: unknown argument: $1" >&2; exit 10 ;;
    esac
done

# --- Validation ---

[ -z "$PRIORITY" ] && { echo "ERROR: --priority is required (HIGH, MEDIUM, LOW)" >&2; exit 10; }
[ -z "$ACTION" ]   && { echo "ERROR: --action is required and must be non-empty" >&2; exit 10; }

PRIORITY_UP=$(echo "$PRIORITY" | tr '[:lower:]' '[:upper:]')
case "$PRIORITY_UP" in
    HIGH|MEDIUM|LOW) PRIORITY="$PRIORITY_UP" ;;
    *) echo "ERROR: --priority must be HIGH, MEDIUM, or LOW; got '$PRIORITY'" >&2; exit 10 ;;
esac

if [ -n "$WEIGHT" ]; then
    valid=$(awk -v w="$WEIGHT" 'BEGIN { if (w+0 == w && w >= 0.0 && w <= 1.0) print "ok"; else print "bad" }')
    [ "$valid" != "ok" ] && {
        echo "ERROR: --weight must be a float in [0.0, 1.0]; got '$WEIGHT'" >&2; exit 10
    }
fi

case "$INJECTED_BY" in
    operator|test|automation) ;;
    *) echo "ERROR: --injected-by must be operator, test, or automation; got '$INJECTED_BY'" >&2; exit 10 ;;
esac

# Generate timestamp + random suffix
NOW_ISO=$(date -u +"%Y-%m-%dT%H-%M-%SZ")
NOW_EPOCH=$(date -u +%s)
RAND_HEX=$(od -An -N4 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n' | head -c 8)

# Generate id if not supplied
[ -z "$TASK_ID" ] && TASK_ID="user-${NOW_EPOCH}-${RAND_HEX}"

# id uniqueness: check state.json
STATE_JSON="$PROJECT_ROOT/.evolve/state.json"
if [ -f "$STATE_JSON" ]; then
    existing=$(jq -r --arg id "$TASK_ID" '.carryoverTodos[]? | select(.id == $id) | .id' "$STATE_JSON" 2>/dev/null || true)
    [ -n "$existing" ] && {
        echo "ERROR: id '$TASK_ID' already exists in state.json:carryoverTodos[]" >&2; exit 11
    }
fi

# id uniqueness: check inbox
INBOX_DIR="$PROJECT_ROOT/.evolve/inbox"
if [ -d "$INBOX_DIR" ]; then
    for f in "$INBOX_DIR"/*.json; do
        [ -f "$f" ] || continue
        maybe=$(jq -r --arg id "$TASK_ID" 'select(.id == $id) | .id' "$f" 2>/dev/null || true)
        if [ -n "$maybe" ]; then
            echo "ERROR: id '$TASK_ID' already exists in inbox ($f)" >&2
            exit 11
        fi
    done
fi

# --invalidate-cache: emit a research_cache_invalidate event before injection.
if [ -n "$INVALIDATE_CACHE_FP" ]; then
    bash scripts/utility/research-cache.sh invalidate "$INVALIDATE_CACHE_FP" \
        --reason "operator-inject-$(date -u +%Y%m%d)" 2>/dev/null || true
    echo "[inject-task] research-cache: emitted invalidate event for fp=$INVALIDATE_CACHE_FP" >&2
fi

# Build JSON
INJECTED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
[ -z "$EVIDENCE_POINTER" ] && EVIDENCE_POINTER="inbox-injection://${INJECTED_AT}"

if [ -n "$WEIGHT" ]; then
    TASK_JSON=$(jq -cn \
        --arg id "$TASK_ID" \
        --arg action "$ACTION" \
        --arg priority "$PRIORITY" \
        --argjson weight "$WEIGHT" \
        --arg evidence_pointer "$EVIDENCE_POINTER" \
        --arg operator_note "$OPERATOR_NOTE" \
        --arg injected_at "$INJECTED_AT" \
        --arg injected_by "$INJECTED_BY" \
        --arg research_pointer "$RESEARCH_POINTER" \
        '{id:$id,action:$action,priority:$priority,weight:$weight,evidence_pointer:$evidence_pointer,operator_note:$operator_note,injected_at:$injected_at,injected_by:$injected_by}
         | if ($research_pointer | length) > 0 then . + {research_pointer: $research_pointer} else . end')
else
    TASK_JSON=$(jq -cn \
        --arg id "$TASK_ID" \
        --arg action "$ACTION" \
        --arg priority "$PRIORITY" \
        --arg evidence_pointer "$EVIDENCE_POINTER" \
        --arg operator_note "$OPERATOR_NOTE" \
        --arg injected_at "$INJECTED_AT" \
        --arg injected_by "$INJECTED_BY" \
        --arg research_pointer "$RESEARCH_POINTER" \
        '{id:$id,action:$action,priority:$priority,weight:null,evidence_pointer:$evidence_pointer,operator_note:$operator_note,injected_at:$injected_at,injected_by:$injected_by}
         | if ($research_pointer | length) > 0 then . + {research_pointer: $research_pointer} else . end')
fi

if [ "$DRY_RUN" -eq 1 ]; then
    echo "$TASK_JSON"
    echo "✓ dry-run OK; would have written to ${INBOX_DIR}/${NOW_ISO}-${RAND_HEX}.json" >&2
    exit 0
fi

# Write atomically to inbox
mkdir -p "$INBOX_DIR" || { echo "ERROR: cannot create $INBOX_DIR" >&2; exit 12; }

INBOX_FILE="${INBOX_DIR}/${NOW_ISO}-${RAND_HEX}.json"
INBOX_TMP="${INBOX_FILE}.tmp.$$"
printf '%s\n' "$TASK_JSON" > "$INBOX_TMP" || { echo "ERROR: cannot write $INBOX_TMP" >&2; exit 12; }
mv -f "$INBOX_TMP" "$INBOX_FILE" || { echo "ERROR: mv failed: $INBOX_TMP → $INBOX_FILE" >&2; exit 12; }

echo "✓ injected: $INBOX_FILE"
