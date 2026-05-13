#!/usr/bin/env bash
#
# validate-predicate.sh — Lint EGPS predicate scripts for banned patterns.
#
# Predicates in acs/cycle-N/*.sh (or acs/regression-suite/cycle-N/*.sh) must
# follow strict rules to prevent the same gaming patterns that motivated EGPS:
#   - No grep-only verification (presence ≠ execution)
#   - No trivial echo PASS; exit 0
#   - No network calls (hermetic)
#   - No sleeps > 1s (predicates must be fast)
#   - No writes outside .evolve/runs/cycle-N/acs-output/
#   - Required metadata headers (AC-ID, Description, Evidence, etc.)
#   - Correct filename format (NNN-slug.sh)
#
# Usage:
#   bash scripts/verification/validate-predicate.sh <predicate-file>
#   bash scripts/verification/validate-predicate.sh --json <predicate-file>
#   bash scripts/verification/validate-predicate.sh --all <predicate-dir>
#
# Exit codes:
#   0  — predicate passes lint
#   3  — banned pattern detected (rejection)
#   10 — bad arguments

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# Resolve scripts/lib/ — works whether invoked directly or via EVOLVE_PLUGIN_ROOT.
if [ -f "$SCRIPT_DIR/../lib/acs-schema.sh" ]; then
    source "$SCRIPT_DIR/../lib/acs-schema.sh"
elif [ -f "${EVOLVE_PLUGIN_ROOT:-/dev/null}/scripts/lib/acs-schema.sh" ]; then
    source "$EVOLVE_PLUGIN_ROOT/scripts/lib/acs-schema.sh"
else
    echo "[validate-predicate] cannot locate scripts/lib/acs-schema.sh" >&2
    exit 1
fi

JSON=0
ALL=0
TARGET=""

while [ $# -gt 0 ]; do
    case "$1" in
        --json) JSON=1 ;;
        --all)  ALL=1 ;;
        --help|-h) sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[validate-predicate] unknown flag: $1" >&2; exit 10 ;;
        *)
            [ -z "$TARGET" ] && TARGET="$1" || { echo "[validate-predicate] too many args" >&2; exit 10; }
            ;;
    esac
    shift
done

[ -n "$TARGET" ] || { echo "[validate-predicate] usage: $0 [--json] [--all] <predicate-file-or-dir>" >&2; exit 10; }

# ── Validation ────────────────────────────────────────────────────────────
# Returns 0 if valid, 3 if banned-pattern, 2 if invalid (missing headers etc).
# Outputs JSON if --json; otherwise human-readable.
validate_one() {
    local file=$1
    local fname; fname=$(basename "$file")
    local violations=()

    # ── Check 1: filename format ──
    if ! acs_filename_valid "$fname"; then
        violations+=("FILENAME_FORMAT|filename must match ${ACS_PREDICATE_FILENAME_REGEX} (e.g., 001-foo.sh)")
    fi

    # ── Check 2: file is executable ──
    if [ ! -x "$file" ]; then
        violations+=("NOT_EXECUTABLE|predicate must have execute bit set (chmod +x)")
    fi

    # ── Check 3: required metadata headers ──
    for key in $ACS_REQUIRED_HEADERS; do
        local val
        val=$(acs_predicate_header "$key" "$file")
        if [ -z "$val" ]; then
            violations+=("MISSING_HEADER_${key}|predicate missing required header: $key")
        fi
    done

    # ── Check 4: banned patterns ──
    # Strip comments + heredoc bodies before scanning so doc comments don't
    # trip the matchers. Simple approach: filter out lines starting with #.
    local code
    code=$(grep -v '^[[:space:]]*#' "$file" 2>/dev/null || echo "")

    # 4a. Trivial echo PASS / exit 0
    if echo "$code" | grep -qE 'echo[[:space:]]+["'"'"']?(PASS|GREEN|OK)["'"'"']?[[:space:]]*$' \
       && echo "$code" | grep -qE '^[[:space:]]*exit[[:space:]]+0[[:space:]]*$' \
       && [ $(echo "$code" | grep -vE '^[[:space:]]*(echo|exit|set|source|\.|local|readonly)' | grep -cE '\S' || echo 0) -lt 3 ]; then
        violations+=("BANNED_TRIVIAL_PASS|predicate appears to be 'echo PASS; exit 0' tautology")
    fi

    # 4b. Network calls
    if echo "$code" | grep -qE "$ACS_BANNED_REGEX_NETWORK"; then
        violations+=("BANNED_NETWORK|predicate uses curl/wget/gh-api — hermetic violation")
    fi

    # 4c. Long sleeps
    if echo "$code" | grep -qE "$ACS_BANNED_REGEX_LONG_SLEEP"; then
        violations+=("BANNED_LONG_SLEEP|predicate has sleep >= 2 seconds — must be fast")
    fi

    # 4d. Writes outside permitted area
    if echo "$code" | grep -qE "$ACS_BANNED_REGEX_FS_WRITE"; then
        violations+=("BANNED_FS_WRITE|predicate writes outside .evolve/runs/cycle-N/acs-output/")
    fi

    # 4e. grep-only as last operation before exit
    # Pattern: predicate's penultimate non-trivial line is `grep -q ...` and
    # final line is `exit $?` or similar — no execution follows.
    local non_trivial
    non_trivial=$(echo "$code" | grep -vE '^[[:space:]]*(set|source|\.|local|readonly|export|#)' | grep -E '\S')
    local last_two
    last_two=$(echo "$non_trivial" | tail -2)
    if echo "$last_two" | head -1 | grep -qE "$ACS_BANNED_REGEX_GREP_ONLY" \
       && echo "$last_two" | tail -1 | grep -qE '^[[:space:]]*exit[[:space:]]'; then
        violations+=("BANNED_GREP_ONLY|predicate's verification is grep-only — must follow grep with execution")
    fi

    # ── Output ──
    if [ "${#violations[@]}" -eq 0 ]; then
        if [ "$JSON" = "1" ]; then
            jq -nc --arg file "$file" '{file: $file, result: "valid", violations: []}'
        else
            echo "[validate-predicate] OK: $file"
        fi
        return 0
    fi

    # Determine exit code: BANNED_* → 3; others → 2
    local rc=2
    for v in "${violations[@]}"; do
        case "$v" in BANNED_*) rc=3 ;; esac
    done

    if [ "$JSON" = "1" ]; then
        local violations_json="[]"
        local first=1
        violations_json="["
        for v in "${violations[@]}"; do
            [ "$first" = "1" ] || violations_json+=","
            first=0
            local code_part="${v%%|*}"
            local msg_part="${v#*|}"
            violations_json+=$(jq -nc --arg c "$code_part" --arg m "$msg_part" '{code: $c, message: $m}')
        done
        violations_json+="]"
        jq -nc --arg file "$file" --argjson v "$violations_json" --argjson rc "$rc" '{file: $file, result: "invalid", exit_code: $rc, violations: $v}'
    else
        echo "[validate-predicate] FAIL: $file" >&2
        for v in "${violations[@]}"; do
            local code_part="${v%%|*}"
            local msg_part="${v#*|}"
            echo "  - $code_part: $msg_part" >&2
        done
    fi
    return $rc
}

# ── Main ──────────────────────────────────────────────────────────────────
if [ "$ALL" = "1" ]; then
    [ -d "$TARGET" ] || { echo "[validate-predicate] --all requires a directory: $TARGET" >&2; exit 10; }
    overall_rc=0
    while IFS= read -r f; do
        validate_one "$f"
        rc=$?
        [ "$rc" -gt "$overall_rc" ] && overall_rc=$rc
    done < <(find "$TARGET" -name "*.sh" -type f 2>/dev/null | sort)
    exit "$overall_rc"
fi

[ -f "$TARGET" ] || { echo "[validate-predicate] file not found: $TARGET" >&2; exit 10; }
validate_one "$TARGET"
