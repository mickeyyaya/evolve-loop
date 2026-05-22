#!/usr/bin/env bash
#
# backfill-lessons.sh — Restore instinctSummary from on-disk lesson YAMLs and
# from YAML blocks embedded in retrospective-report.md files.
#
# Two operations (both idempotent):
#
#   1. EXTRACT: parse ```yaml``` blocks from retrospective-report.md files in
#      .evolve/runs/cycle-N/ and write each `- id:` item as a discrete YAML
#      file to .evolve/instincts/lessons/<id>.yaml. Skips IDs that already
#      have a file on disk.
#
#   2. SYNC: append to state.json:instinctSummary[] any on-disk .yaml file in
#      .evolve/instincts/lessons/ whose `id` field is not already present.
#      Also updates instinctCount.
#
# Usage:
#   bash scripts/utility/backfill-lessons.sh [--dry-run] [--cycle N]
#
# Options:
#   --dry-run   Print what would change; do not modify files.
#   --cycle N   Restrict EXTRACT phase to retrospective-report.md from cycle N.
#               SYNC phase always scans all on-disk YAMLs.
#
# Exit codes:
#   0  — success (or dry-run with changes found)
#   1  — runtime error
#   2  — nothing to do (no missing lessons found)

set -uo pipefail

__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"
unset __rr_self

LESSONS_DIR="$EVOLVE_PROJECT_ROOT/.evolve/instincts/lessons"
RUNS_DIR="$EVOLVE_PROJECT_ROOT/.evolve/runs"
STATE="${EVOLVE_STATE_FILE_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/state.json}"

DRY_RUN=0
RESTRICT_CYCLE=""

for arg in "$@"; do
    case "$arg" in
        --dry-run) DRY_RUN=1 ;;
        --cycle) ;;
        --cycle=*) RESTRICT_CYCLE="${arg#--cycle=}" ;;
        [0-9]*) RESTRICT_CYCLE="$arg" ;;
    esac
done
# Handle "--cycle N" (space-separated)
prev=""
for arg in "$@"; do
    if [ "$prev" = "--cycle" ]; then
        RESTRICT_CYCLE="$arg"
    fi
    prev="$arg"
done

log() { echo "[backfill-lessons] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

command -v jq >/dev/null 2>&1 || fail "jq required"
[ -d "$LESSONS_DIR" ] || fail "lessons dir not found: $LESSONS_DIR"
[ -f "$STATE" ] || fail "state.json not found: $STATE"

mkdir -p "$LESSONS_DIR"

# ---------------------------------------------------------------------------
# Phase 1: EXTRACT — parse YAML blocks from retrospective-report.md files
# ---------------------------------------------------------------------------

_extract_yaml_blocks() {
    local report_path="$1"
    [ -f "$report_path" ] || return 0

    # Use awk to extract content between ```yaml and ``` markers.
    # Handles multiple blocks in one file.
    local yaml_block
    yaml_block=$(awk '
        /^```yaml/ { in_block=1; block=""; next }
        in_block && /^```/ { print block; in_block=0; block=""; next }
        in_block { block = block "\n" $0 }
    ' "$report_path")

    [ -z "$yaml_block" ] && return 0

    # Split block at "- id:" boundaries (top-level list items).
    # Each item starts with "- id: " at the beginning of a line.
    # We collect lines until the next "- id:" or end of block.
    local current_id="" current_content="" line
    local extracted=0

    while IFS= read -r line; do
        if [[ "$line" =~ ^-[[:space:]]id:[[:space:]]*(.*) ]]; then
            # Flush previous item
            if [ -n "$current_id" ]; then
                _write_lesson_yaml "$current_id" "$current_content"
                extracted=$((extracted + 1))
            fi
            current_id="${BASH_REMATCH[1]}"
            current_content="id: ${BASH_REMATCH[1]}"
        elif [ -n "$current_id" ]; then
            # Strip the leading 2-space YAML list indentation for top-level fields
            if [[ "$line" =~ ^[[:space:]][[:space:]](.*) ]]; then
                current_content="${current_content}
${BASH_REMATCH[1]}"
            elif [ -n "$line" ]; then
                current_content="${current_content}
${line}"
            fi
        fi
    done <<< "$yaml_block"

    # Flush last item
    if [ -n "$current_id" ]; then
        _write_lesson_yaml "$current_id" "$current_content"
        extracted=$((extracted + 1))
    fi

    [ "$extracted" -gt 0 ] && log "extracted $extracted lesson(s) from $report_path"
}

_write_lesson_yaml() {
    local id="$1" content="$2"
    local dest="$LESSONS_DIR/${id}.yaml"

    if [ -f "$dest" ]; then
        log "skip (already exists): $id"
        return 0
    fi
    if [ "$DRY_RUN" = "1" ]; then
        log "dry-run: would write $dest"
        return 0
    fi
    local tmp="${dest}.tmp.$$"
    printf '%s\n' "$content" > "$tmp" && mv -f "$tmp" "$dest"
    log "wrote $dest"
}

# Find retrospective-report.md files to process
if [ -n "$RESTRICT_CYCLE" ]; then
    retro_files=("$RUNS_DIR/cycle-${RESTRICT_CYCLE}/retrospective-report.md")
else
    retro_files=()
    for dir in "$RUNS_DIR"/cycle-*/; do
        f="${dir}retrospective-report.md"
        [ -f "$f" ] && retro_files+=("$f")
    done
fi

for retro_file in "${retro_files[@]:+${retro_files[@]}}"; do
    _extract_yaml_blocks "$retro_file"
done

# ---------------------------------------------------------------------------
# Phase 2: SYNC — append on-disk YAMLs missing from instinctSummary
# ---------------------------------------------------------------------------

jq empty "$STATE" 2>/dev/null || fail "state.json is not valid JSON: $STATE"

# Collect IDs already in instinctSummary
existing_ids=""
while IFS= read -r id; do
    [ -n "$id" ] && existing_ids="${existing_ids} ${id} "
done < <(jq -r '.instinctSummary[]?.id // empty' "$STATE" 2>/dev/null)

appended=0

for yaml_file in "$LESSONS_DIR"/*.yaml; do
    [ -f "$yaml_file" ] || continue

    # Extract id from YAML file (try python3 first, fall back to grep)
    file_id=""
    if command -v python3 >/dev/null 2>&1; then
        file_id=$(python3 -c "
try:
    with open('$yaml_file') as f:
        for line in f:
            s = line.strip()
            # Handle dict format: 'id: value' and list format: '- id: value'
            if s.startswith('- id:'):
                print(s[len('- id:'):].strip())
                break
            elif s.startswith('id:'):
                print(s[len('id:'):].strip())
                break
except Exception:
    pass
" 2>/dev/null)
    fi
    if [ -z "$file_id" ]; then
        # Handle both 'id:' and '- id:' formats
        file_id=$(grep -E '^(- )?id:' "$yaml_file" | head -1 | sed 's/^-[[:space:]]*//' | sed 's/^id:[[:space:]]*//' | tr -d '"')
    fi
    [ -z "$file_id" ] && { log "WARN: cannot extract id from $yaml_file; skipping"; continue; }

    # Check if already in state
    if [[ "$existing_ids" == *" ${file_id} "* ]]; then
        continue
    fi

    # Extract pattern and confidence for the instinctSummary entry
    pattern=""
    confidence="0.90"
    error_cat=""
    if command -v python3 >/dev/null 2>&1; then
        pattern=$(python3 -c "
try:
    with open('$yaml_file') as f:
        content = f.read()
    for line in content.splitlines():
        s = line.strip()
        if s.startswith('pattern:'):
            v = s.split(':', 1)[1].strip().strip('\"')
            print(v)
            break
except Exception:
    pass
" 2>/dev/null)
        error_cat=$(python3 -c "
try:
    with open('$yaml_file') as f:
        content = f.read()
    for line in content.splitlines():
        s = line.strip()
        if s.startswith('classification:'):
            v = s.split(':', 1)[1].strip().strip('\"')
            print(v)
            break
except Exception:
    pass
" 2>/dev/null)
    fi
    if [ -z "$pattern" ]; then
        pattern=$(grep -E '^[[:space:]]*pattern:' "$yaml_file" | head -1 | sed 's/^[[:space:]]*pattern:[[:space:]]*//' | tr -d '"' || echo "")
    fi
    if [ -z "$error_cat" ]; then
        error_cat=$(grep -E '^[[:space:]]*classification:' "$yaml_file" | head -1 | sed 's/^[[:space:]]*classification:[[:space:]]*//' | tr -d '"' || echo "")
    fi
    [ -z "$pattern" ] && pattern="(see $yaml_file)"

    if [ "$DRY_RUN" = "1" ]; then
        log "dry-run: would append to instinctSummary: $file_id"
        appended=$((appended + 1))
        continue
    fi

    # Append to instinctSummary atomically
    local_id="$file_id"
    local_pattern="$pattern"
    local_conf="$confidence"
    local_cat="$error_cat"
    TMP_STATE=$(mktemp)
    jq \
        --arg id "$local_id" \
        --arg pattern "$local_pattern" \
        --argjson confidence "$local_conf" \
        --arg errorCategory "$local_cat" \
        '.instinctSummary += [{
            id: $id,
            pattern: $pattern,
            confidence: $confidence,
            type: "failure-lesson",
            errorCategory: $errorCategory
        }]
        | .instinctCount = (.instinctSummary | length)' \
        "$STATE" > "$TMP_STATE" && mv -f "$TMP_STATE" "$STATE"

    existing_ids="${existing_ids} ${file_id} "
    appended=$((appended + 1))
    log "appended to instinctSummary: $file_id"
done

if [ "$appended" -eq 0 ] && [ "$DRY_RUN" = "0" ]; then
    log "nothing to sync — instinctSummary already up to date"
    exit 2
fi

log "done: $appended lesson(s) synced to state.json"
exit 0
