#!/usr/bin/env bash
#
# build-report-ac-verify.sh — AC-TABLE harness (v1.0)
#
# Reads acceptance_checks: from intent.md, runs acs/cycle-N/ predicate scripts,
# and writes a tamper-evident AC-TABLE region into build-report.md atomically.
#
# Usage: bash scripts/lifecycle/build-report-ac-verify.sh <intent.md> <build-report.md>
#
# Output region in build-report.md:
#   <!-- AC-TABLE-BEGIN -->
#   | Result | Acceptance Check | Exit |
#   |--------|-----------------|------|
#   | PASS   | `...`           | 0    |
#   <!-- harness-stamp: build-report-ac-verify.sh v1.0 cycle=N ts=... -->
#   <!-- AC-TABLE-END -->
#
# Idempotent: replaces existing AC-TABLE region on re-run.
# Exit codes: 0=all checks passed (or no acs scripts found), 1=one or more checks failed.

set -uo pipefail

# ---- Args -------------------------------------------------------------------

INTENT_MD="${1:?Usage: build-report-ac-verify.sh <intent.md> <build-report.md>}"
BUILD_REPORT="${2:?Missing build-report.md path}"

# ---- Helpers ----------------------------------------------------------------

log() { echo "[ac-verify] $*" >&2; }

# ---- Locate acs directory ---------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Infer cycle from build-report path or workspace name
CYCLE=""
# Try to extract from the workspace path (build-report lives in .evolve/runs/cycle-N/)
_br_dir="$(dirname "$BUILD_REPORT")"
_br_dir_base="$(basename "$_br_dir")"
case "$_br_dir_base" in
    cycle-*)
        CYCLE="${_br_dir_base#cycle-}"
        ;;
esac

# Fallback: try CYCLE env var or infer from intent.md path
if [ -z "$CYCLE" ] && [ -n "${CYCLE:-}" ]; then
    : # already set
fi
if [ -z "$CYCLE" ]; then
    # Try EVOLVE env vars
    CYCLE="${EVOLVE_CYCLE:-}"
fi

if [ -z "$CYCLE" ]; then
    log "WARN: cannot infer cycle number from build-report path '$BUILD_REPORT' — skipping acs predicate run"
    # Write an empty AC-TABLE with a warning
    _write_table "WARN: cycle number unknown — no predicates run" "" 0
    exit 0
fi

ACS_DIR="${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}/acs/cycle-${CYCLE}"
log "ACS dir: $ACS_DIR (cycle=$CYCLE)"

# ---- Parse acceptance_checks: from intent.md --------------------------------

# Extracts check: values from YAML block:
#   acceptance_checks:
#     - check: "text"
#       how_verified: ...
parse_checks() {
    local file="$1"
    [ -f "$file" ] || return 0
    local in_block=0
    while IFS= read -r line; do
        case "$line" in
            acceptance_checks:*)
                in_block=1
                ;;
            "  - check:"*|"  - check :"*)
                [ "$in_block" -eq 1 ] || continue
                # Extract value after check:
                local val
                val="${line#*check:}"
                val="${val#*check :}"
                # Strip leading/trailing spaces and quotes
                val="$(echo "$val" | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//' | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")"
                [ -n "$val" ] && echo "$val"
                ;;
            "  - "*|"    "*|"")
                # Still in block
                ;;
            *)
                # New top-level key — end of block
                [ "$in_block" -eq 1 ] && in_block=0
                ;;
        esac
    done < "$file"
}

# Read check descriptions (may be empty if intent.md absent or has no checks)
CHECK_DESCS=""
if [ -f "$INTENT_MD" ]; then
    CHECK_DESCS="$(parse_checks "$INTENT_MD")"
fi

# ---- Collect predicate scripts ----------------------------------------------

PRED_SCRIPTS=""
if [ -d "$ACS_DIR" ]; then
    PRED_SCRIPTS="$(find "$ACS_DIR" -maxdepth 1 -name "*.sh" -type f 2>/dev/null | sort)"
fi

if [ -z "$PRED_SCRIPTS" ]; then
    log "No predicate scripts found in $ACS_DIR — writing empty AC-TABLE"
fi

# ---- Run predicates and build table rows ------------------------------------

TABLE_ROWS=""
FAIL_COUNT=0
IDX=0

if [ -n "$PRED_SCRIPTS" ]; then
    while IFS= read -r script_path; do
        [ -f "$script_path" ] || continue
        IDX=$((IDX + 1))

        # Get description: from acceptance_checks by index, or use script name
        DESC=""
        if [ -n "$CHECK_DESCS" ]; then
            DESC="$(echo "$CHECK_DESCS" | sed -n "${IDX}p")"
        fi
        if [ -z "$DESC" ]; then
            DESC="$(basename "$script_path" .sh)"
        fi

        # Run predicate
        _rc=0
        if [ -x "$script_path" ]; then
            _out=$(bash "$script_path" 2>&1) || _rc=$?
        else
            _out="NOT_EXECUTABLE"
            _rc=127
        fi

        if [ "$_rc" -eq 0 ]; then
            RESULT="PASS"
        else
            RESULT="FAIL"
            FAIL_COUNT=$((FAIL_COUNT + 1))
            log "FAIL: $script_path (exit=$_rc): $_out"
        fi

        # Escape backticks in description
        _safe_desc="$(echo "$DESC" | sed 's/`/\\`/g')"
        TABLE_ROWS="${TABLE_ROWS}| ${RESULT} | \`${_safe_desc}\` | ${_rc} |
"
    done <<EOF
$PRED_SCRIPTS
EOF
fi

# ---- Compose AC-TABLE region ------------------------------------------------

TS="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

AC_TABLE="<!-- AC-TABLE-BEGIN -->
| Result | Acceptance Check | Exit |
|--------|-----------------|------|
${TABLE_ROWS}<!-- harness-stamp: build-report-ac-verify.sh v1.0 cycle=${CYCLE} ts=${TS} -->
<!-- AC-TABLE-END -->"

# ---- Write to build-report.md atomically ------------------------------------

if [ ! -f "$BUILD_REPORT" ]; then
    log "WARN: build-report.md not found at $BUILD_REPORT — creating"
    touch "$BUILD_REPORT"
fi

TMP="${BUILD_REPORT}.tmp.$$"

# If existing AC-TABLE region present, replace it; otherwise append
if grep -q "<!-- AC-TABLE-BEGIN -->" "$BUILD_REPORT" 2>/dev/null; then
    # Remove existing region between AC-TABLE-BEGIN and AC-TABLE-END (inclusive)
    # Use a sed approach that works on bash 3.2 / macOS BSD sed
    _in_block=0
    _new_content=""
    while IFS= read -r line; do
        case "$line" in
            "<!-- AC-TABLE-BEGIN -->")
                _in_block=1
                ;;
            "<!-- AC-TABLE-END -->")
                _in_block=0
                ;;
            *)
                [ "$_in_block" -eq 0 ] && _new_content="${_new_content}${line}
"
                ;;
        esac
    done < "$BUILD_REPORT"
    printf '%s' "$_new_content" > "$TMP"
    printf '\n%s\n' "$AC_TABLE" >> "$TMP"
else
    # Append region
    cp "$BUILD_REPORT" "$TMP"
    printf '\n%s\n' "$AC_TABLE" >> "$TMP"
fi

mv "$TMP" "$BUILD_REPORT"
log "AC-TABLE written to $BUILD_REPORT (checks=$IDX, fail=$FAIL_COUNT)"

# ---- Exit -------------------------------------------------------------------

if [ "$FAIL_COUNT" -gt 0 ]; then
    log "FAIL: $FAIL_COUNT/$IDX acceptance check(s) failed"
    exit 1
fi

exit 0
