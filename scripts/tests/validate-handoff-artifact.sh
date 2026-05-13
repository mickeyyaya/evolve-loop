#!/usr/bin/env bash
#
# validate-handoff-artifact.sh — C2-handoff-schemas lint script.
#
# Validates a markdown artifact against a JSON schema definition stored in
# schemas/handoff/. Schema format is bash-native / jq-readable JSON (no
# external JSON Schema validator required).
#
# Usage:
#   bash scripts/tests/validate-handoff-artifact.sh \
#       --artifact <PATH>          # path to markdown artifact
#       --type scout|build|audit   # artifact type selects schema file
#       [--state <state.json>]     # required for conditional_sections checks
#
# Exit codes:
#   0  PASS  — no violations
#   1  FAIL  — one or more violations found (violations printed to stdout)
#   2  ERROR — usage error or missing dependency

set -uo pipefail

_self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCHEMA_BASE="${EVOLVE_SCHEMA_DIR:-$_self_dir/../../schemas/handoff}"

ARTIFACT=""
TYPE=""
STATE_FILE=""

while [ $# -gt 0 ]; do
    case "$1" in
        --artifact) ARTIFACT="$2"; shift 2 ;;
        --type)     TYPE="$2";     shift 2 ;;
        --state)    STATE_FILE="$2"; shift 2 ;;
        -h|--help)  sed -n '3,20p' "$0" >&2; exit 0 ;;
        *) echo "ERROR: unknown argument: $1" >&2; exit 2 ;;
    esac
done

[ -n "$ARTIFACT" ] || { echo "ERROR: --artifact required" >&2; exit 2; }
[ -n "$TYPE" ]     || { echo "ERROR: --type required (scout|build|audit)" >&2; exit 2; }
[ -f "$ARTIFACT" ] || { echo "ERROR: artifact not found: $ARTIFACT" >&2; exit 2; }

case "$TYPE" in
    scout|build|audit) ;;
    *) echo "ERROR: --type must be scout, build, or audit (got: $TYPE)" >&2; exit 2 ;;
esac

SCHEMA="$SCHEMA_BASE/${TYPE}-report.schema.json"
[ -f "$SCHEMA" ] || { echo "ERROR: schema not found: $SCHEMA" >&2; exit 2; }

command -v jq >/dev/null 2>&1 || { echo "ERROR: jq is required" >&2; exit 2; }

VIOLATIONS=0

_emit_violation() {
    local name="$1" msg="$2"
    echo "VIOLATION[$name]: $msg"
    VIOLATIONS=$((VIOLATIONS + 1))
}

# Check required_first_line
_check_first_line() {
    local pattern; pattern=$(jq -r '.required_first_line.pattern // empty' "$SCHEMA")
    [ -z "$pattern" ] && return 0
    local msg; msg=$(jq -r '.required_first_line.fail_message // "required_first_line pattern missing"' "$SCHEMA")
    local first; first=$(head -1 "$ARTIFACT")
    if ! echo "$first" | grep -qE "$pattern" 2>/dev/null; then
        _emit_violation "first_line" "$msg"
    fi
}

# Check a section object (name, patterns[], fail_message) — any pattern match satisfies.
_check_section() {
    local section_json="$1"
    local name; name=$(echo "$section_json" | jq -r '.name')
    local satisfied=0
    local pat
    while IFS= read -r pat; do
        [ -z "$pat" ] && continue
        grep -qE "$pat" "$ARTIFACT" 2>/dev/null && satisfied=1 && break
    done < <(echo "$section_json" | jq -r '.patterns[]')
    if [ "$satisfied" -eq 0 ]; then
        local msg; msg=$(echo "$section_json" | jq -r '.fail_message')
        _emit_violation "$name" "$msg"
    fi
}

# Check required_sections[]
_check_required_sections() {
    local count; count=$(jq -r '(.required_sections // []) | length' "$SCHEMA")
    local i=0
    while [ "$i" -lt "${count:-0}" ]; do
        local sec; sec=$(jq -c ".required_sections[$i]" "$SCHEMA")
        _check_section "$sec"
        i=$((i + 1))
    done
}

# Check conditional_sections[] — only when condition is met
_check_conditional_sections() {
    [ -z "$STATE_FILE" ] && return 0
    [ -f "$STATE_FILE" ] || return 0
    local count; count=$(jq -r '(.conditional_sections // []) | length' "$SCHEMA")
    local i=0
    while [ "$i" -lt "${count:-0}" ]; do
        local sec_json; sec_json=$(jq -c ".conditional_sections[$i]" "$SCHEMA")
        local condition; condition=$(echo "$sec_json" | jq -r '.condition')
        case "$condition" in
            has_carryover_todos)
                local ct; ct=$(jq -r '(.carryoverTodos // []) | length' "$STATE_FILE" 2>/dev/null || echo 0)
                if [ "${ct:-0}" -gt 0 ]; then
                    _check_section "$sec_json"
                fi
                ;;
        esac
        i=$((i + 1))
    done
}

# Check required_content[] — pattern must match somewhere in artifact
_check_required_content() {
    local count; count=$(jq -r '(.required_content // []) | length' "$SCHEMA")
    local i=0
    while [ "$i" -lt "${count:-0}" ]; do
        local item; item=$(jq -c ".required_content[$i]" "$SCHEMA")
        local name; name=$(echo "$item" | jq -r '.name')
        local pat; pat=$(echo "$item" | jq -r '.pattern')
        local msg; msg=$(echo "$item" | jq -r '.fail_message')
        if ! grep -qE "$pat" "$ARTIFACT" 2>/dev/null; then
            _emit_violation "$name" "$msg"
        fi
        i=$((i + 1))
    done
}

# Check min_words
_check_min_words() {
    local min; min=$(jq -r '.min_words // 0' "$SCHEMA")
    [ "${min:-0}" -le 0 ] && return 0
    local count; count=$(wc -w < "$ARTIFACT" | tr -d ' ')
    if [ "${count:-0}" -lt "$min" ]; then
        _emit_violation "min_words" "artifact has $count words, minimum is $min"
    fi
}

_check_first_line
_check_required_sections
_check_conditional_sections
_check_required_content
_check_min_words

[ "$VIOLATIONS" -eq 0 ] && exit 0 || exit 1
