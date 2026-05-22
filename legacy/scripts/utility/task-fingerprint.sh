#!/usr/bin/env bash
# task-fingerprint.sh — Compute deterministic sha256 fingerprint for a task.
#
# Content-addresses per-task research cache entries keyed by normalized
# action + acceptance_criteria + target_files. Two tasks produce the same
# fingerprint iff their normalized content is identical (whitespace-collapsed,
# field-order fixed). Bash 3.2 compatible.
#
# Usage (stdin JSON):
#   echo '{"action":"Fix X","acceptance_criteria":"Y","target_files":"a.sh"}' \
#     | bash scripts/utility/task-fingerprint.sh
#
# Usage (flags):
#   bash scripts/utility/task-fingerprint.sh \
#     --action "Fix X" \
#     [--criteria "acceptance criteria text"] \
#     [--files "a.sh b.sh c.sh"]
#
# Exit codes:
#   0  — fingerprint (64-char hex) printed to stdout
#   1  — missing required input (no action field)
#   2  — sha256 tool unavailable

set -uo pipefail

ACTION=""
CRITERIA=""
FILES=""
FROM_STDIN=0

while [ $# -gt 0 ]; do
    case "$1" in
        --action)   ACTION="$2";   shift 2 ;;
        --criteria) CRITERIA="$2"; shift 2 ;;
        --files)    FILES="$2";    shift 2 ;;
        --help|-h)
            sed -n '2,19p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --)         shift; break ;;
        *)          echo "[task-fingerprint] unknown argument: $1" >&2; exit 1 ;;
    esac
done

# If no flags and stdin is not a tty, read JSON from stdin
if [ -z "$ACTION" ] && [ ! -t 0 ]; then
    FROM_STDIN=1
    INPUT=$(cat)
    if command -v jq >/dev/null 2>&1; then
        ACTION=$(echo "$INPUT" | jq -r '.action // empty' 2>/dev/null || true)
        CRITERIA=$(echo "$INPUT" | jq -r '.acceptance_criteria // empty' 2>/dev/null || true)
        FILES=$(echo "$INPUT" | jq -r '.target_files // empty' 2>/dev/null || true)
    else
        # Fallback: crude extraction without jq
        ACTION=$(echo "$INPUT" | grep -o '"action"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"action"[[:space:]]*:[[:space:]]*"\(.*\)"/\1/' | head -1 || true)
        CRITERIA=$(echo "$INPUT" | grep -o '"acceptance_criteria"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"acceptance_criteria"[[:space:]]*:[[:space:]]*"\(.*\)"/\1/' | head -1 || true)
        FILES=$(echo "$INPUT" | grep -o '"target_files"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"target_files"[[:space:]]*:[[:space:]]*"\(.*\)"/\1/' | head -1 || true)
    fi
fi

[ -z "$ACTION" ] && { echo "[task-fingerprint] ERROR: --action (or 'action' field in JSON) is required" >&2; exit 1; }

# Normalize: collapse all whitespace sequences to a single space, trim edges.
# Bash 3.2: use tr + sed (no ${var^^}, no mapfile, no declare -A).
_normalize() {
    printf '%s' "$1" | tr '\n\r\t' '   ' | tr -s ' ' | sed 's/^ //;s/ $//'
}

# Normalize target_files: sort unique, one-per-line, then join.
_normalize_files() {
    local f="$1"
    if [ -z "$f" ]; then
        echo ""
        return
    fi
    # Replace commas and common separators with newlines, sort, unique, rejoin
    printf '%s' "$f" | tr ',; \t' '\n' | tr -s '\n' | sed '/^[[:space:]]*$/d' | sort -u | tr '\n' ' ' | sed 's/ $//'
}

ACTION_NORM=$(_normalize "$ACTION")
CRITERIA_NORM=$(_normalize "$CRITERIA")
FILES_NORM=$(_normalize_files "$FILES")

# Canonical input: ACTION\nCRITERIA\nFILES (empty fields produce empty string)
CANONICAL=$(printf 'action:%s\ncriteria:%s\nfiles:%s\n' "$ACTION_NORM" "$CRITERIA_NORM" "$FILES_NORM")

# Compute sha256
if command -v sha256sum >/dev/null 2>&1; then
    FP=$(printf '%s' "$CANONICAL" | sha256sum | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    FP=$(printf '%s' "$CANONICAL" | shasum -a 256 | awk '{print $1}')
else
    echo "[task-fingerprint] ERROR: neither sha256sum nor shasum found" >&2
    exit 2
fi

printf '%s\n' "$FP"
