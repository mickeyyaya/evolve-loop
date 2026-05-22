#!/usr/bin/env bash
#
# lint-acs-predicates.sh — Static linter for ACS predicate quality.
#
# Classifies ACS predicates as GREP_ONLY or BEHAVIORAL.
# GREP_ONLY = predicate has grep -q calls but no subprocess invocations
#             (trivially-tautological pattern that can't test actual behavior).
# BEHAVIORAL = uses subprocess invocations ($(...), backtick, or pipe to shell)
#              or uses arithmetic/jq/awk/wc to verify actual system state.
#
# Usage:
#   bash scripts/verification/lint-acs-predicates.sh [--explain] [--predicates-dir DIR]
#
# Exit codes:
#   0 = all predicates are behavioral (PASS)
#   1 = one or more grep-only predicates detected (FAIL)

set -uo pipefail

EXPLAIN=0
PREDICATES_DIR=""

while [ $# -gt 0 ]; do
    case "$1" in
        --explain)          EXPLAIN=1; shift ;;
        --predicates-dir)   PREDICATES_DIR="$2"; shift 2 ;;
        --help|-h)
            sed -n '2,20p' "$0"
            exit 0 ;;
        *)
            echo "[lint-acs-predicates] unknown arg: $1" >&2
            exit 2 ;;
    esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# Collect predicate files
PRED_FILES=""
if [ -n "$PREDICATES_DIR" ]; then
    if [ ! -d "$PREDICATES_DIR" ]; then
        echo "[lint-acs-predicates] predicates-dir not found: $PREDICATES_DIR" >&2
        exit 2
    fi
    PRED_FILES=$(find "$PREDICATES_DIR" -maxdepth 1 -name "*.sh" -type f 2>/dev/null | sort || true)
else
    PRED_FILES=$(find "$REPO_ROOT/acs" -maxdepth 2 -name "*.sh" -type f 2>/dev/null | sort || true)
fi

if [ -z "$PRED_FILES" ]; then
    echo "[lint-acs-predicates] no predicate files found — nothing to check" >&2
    exit 0
fi

# Classify one predicate file.
# Outputs: GREP_ONLY or BEHAVIORAL
# Returns: 1 if grep-only, 0 if behavioral
classify_predicate() {
    local pred_file="$1"

    # Strip shebang, comments, and blank lines to get meaningful lines
    local meaningful_lines
    meaningful_lines=$(grep -v '^\s*#' "$pred_file" 2>/dev/null | grep -v '^\s*$' | grep -v '^#!' || true)

    if [ -z "$meaningful_lines" ]; then
        echo "BEHAVIORAL"
        return 0
    fi

    # Check if any line uses jq, awk, wc, cut, sed, arithmetic, or numeric
    # comparisons — these indicate real behavioral logic requiring actual
    # subprocess output to produce a meaningful comparison.
    local has_behavioral_indicators
    has_behavioral_indicators=$(echo "$meaningful_lines" | grep -E '(jq |awk |wc |cut |sed |\$\(\(|\[ "\$[a-zA-Z_]+ -[gletne]+ [0-9])' || true)

    if [ -n "$has_behavioral_indicators" ]; then
        echo "BEHAVIORAL"
        return 0
    fi

    # Count subprocess invocations: $(...), backtick, or pipe-to-bash.
    # Note: grep -c prints "0" even on no-match (exit 1) — use || true not
    # || echo 0 to avoid double output ("0\n0").
    local subprocess_count
    subprocess_count=$(echo "$meaningful_lines" | grep -cE '(\$\([^)]+\)|`[^`]+`|\| (bash|sh) )' 2>/dev/null || true)
    subprocess_count=$(echo "$subprocess_count" | tr -d ' \n')
    subprocess_count="${subprocess_count:-0}"

    # Count lines that call grep with -q flag (any variant: -q, -qi, -qE, -Eq, etc.)
    # Pattern: grep followed by any text containing -[letters]q[letters]
    local grep_only_count
    grep_only_count=$(echo "$meaningful_lines" | grep -cE '^\s*grep\s+.*-[a-zA-Z]*q[a-zA-Z]*' 2>/dev/null || true)
    grep_only_count=$(echo "$grep_only_count" | tr -d ' \n')
    grep_only_count="${grep_only_count:-0}"

    # FAIL if grep_only_count > 0 AND subprocess_invocation_count == 0
    if [ "$grep_only_count" -gt 0 ] && [ "$subprocess_count" -eq 0 ]; then
        local non_set_lines
        non_set_lines=$(echo "$meaningful_lines" | grep -v 'set -' || true)

        local last_line
        last_line=$(echo "$meaningful_lines" | tail -1)

        # Pattern 1: last line is `grep -q[flags] ... ; exit $?`
        if echo "$last_line" | grep -qE '^\s*grep\s+.*-[a-zA-Z]*q[a-zA-Z]*[^;]*;\s*exit\s+\$\?' ; then
            echo "GREP_ONLY"
            return 1
        fi

        # Pattern 2: only 0-1 meaningful non-set lines, all of which are grep -q
        local non_set_count
        non_set_count=$(echo "$non_set_lines" | grep -c '.' 2>/dev/null || true)
        non_set_count=$(echo "$non_set_count" | tr -d ' \n')
        non_set_count="${non_set_count:-0}"
        if [ "$non_set_count" -le 1 ] && [ -n "$non_set_lines" ]; then
            if echo "$non_set_lines" | grep -qE 'grep\s+.*-[a-zA-Z]*q[a-zA-Z]*'; then
                echo "GREP_ONLY"
                return 1
            fi
        fi
    fi

    echo "BEHAVIORAL"
    return 0
}

FAIL_COUNT=0
TOTAL=0

while IFS= read -r pred_file; do
    [ -f "$pred_file" ] || continue
    TOTAL=$((TOTAL + 1))

    classification=$(classify_predicate "$pred_file")
    classify_rc=$?

    if [ "$EXPLAIN" = "1" ]; then
        if [ "$classification" = "GREP_ONLY" ]; then
            echo "FAIL $pred_file — grep-only predicate (trivially-tautological; no subprocess invocation)"
        else
            echo "PASS $pred_file — behavioral predicate"
        fi
    fi

    if [ "$classify_rc" -ne 0 ]; then
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
done <<EOF
$PRED_FILES
EOF

if [ "$EXPLAIN" = "1" ]; then
    echo "[lint-acs-predicates] $TOTAL predicates checked, $FAIL_COUNT grep-only"
fi

if [ "$FAIL_COUNT" -gt 0 ]; then
    echo "[lint-acs-predicates] FAIL: $FAIL_COUNT of $TOTAL predicate(s) are grep-only (tautological risk)" >&2
    exit 1
fi

exit 0
