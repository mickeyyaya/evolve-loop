#!/usr/bin/env bash
#
# audit-citation-check.sh — Verify audit-report.md citations stay inside the
# cycle's diff scope.
#
# WHY THIS EXISTS
#
# Cycle 61 demonstrated a class of failure where the Auditor cited a file:line
# (gemini.sh:206) that DID exist in HEAD but was NOT in the cycle's commit
# diff. The audit verdict was technically PASS, but the verdict's evidence was
# detached from the work being audited — audit-binding scope creep.
#
# This check parses audit-report.md for `path:line` citations and verifies
# each cited path appears in the cycle's diff. Citations outside the diff
# scope are surfaced as RED so phase-gate can refuse the audit-to-ship gate.
#
# CONTRACT
#
# Inputs:
#   1. <audit-report.md> (required)
#   2. --diff-files "a,b,c" (optional) — explicit comma-separated list of
#      paths considered "in scope". Used by fixture tests.
#   3. --git-head SHA (optional) — when --diff-files absent, compute scope
#      from `git diff --name-only <git-head> HEAD`.
#
# Exit codes:
#   0 — every path:line citation is in the diff scope (or skipped)
#   1 — at least one citation points outside the scope
#   2 — usage error
#
# DESIGN
#
# A "citation" is any token matching the regex:
#   [a-zA-Z0-9_/.-]+\.(sh|json|md|py|ts|js|yaml|yml|tf|rs|go|js|tsx|jsx):[0-9]+
#
# Bare file mentions (no `:line` suffix) are skipped — they're general
# references, not specific evidence claims. The strict path:line form is
# what indicates the auditor read a specific line.

set -uo pipefail

usage() {
    cat <<'EOF' >&2
audit-citation-check.sh <audit-report.md> [--diff-files "a,b,c"] [--git-head SHA]

Verifies audit-report.md path:line citations correspond to files in the
cycle's diff scope.

Exit: 0=all citations in scope, 1=at least one outside, 2=usage error.
EOF
}

AUDIT_REPORT=""
DIFF_FILES=""
GIT_HEAD=""
while [ $# -gt 0 ]; do
    case "$1" in
        --diff-files) shift; DIFF_FILES="${1:-}" ;;
        --git-head) shift; GIT_HEAD="${1:-}" ;;
        --help|-h) usage; exit 0 ;;
        --*) echo "[audit-citation] unknown flag: $1" >&2; usage; exit 2 ;;
        *)
            if [ -z "$AUDIT_REPORT" ]; then AUDIT_REPORT="$1"
            else echo "[audit-citation] extra arg: $1" >&2; usage; exit 2
            fi ;;
    esac
    shift
done

[ -n "$AUDIT_REPORT" ] || { usage; exit 2; }
[ -f "$AUDIT_REPORT" ] || { echo "[audit-citation] not a file: $AUDIT_REPORT" >&2; exit 2; }

# Resolve scope (the file set considered "in the cycle's diff").
SCOPE=""
if [ -n "$DIFF_FILES" ]; then
    # Convert comma to newline.
    SCOPE=$(echo "$DIFF_FILES" | tr ',' '\n')
elif [ -n "$GIT_HEAD" ]; then
    SCOPE=$(git diff --name-only "$GIT_HEAD" HEAD 2>/dev/null || true)
else
    # Default: HEAD~1..HEAD (most recent commit).
    SCOPE=$(git diff --name-only HEAD~1 HEAD 2>/dev/null || true)
fi

if [ -z "$SCOPE" ]; then
    echo "[audit-citation] WARN: empty diff scope — nothing to verify against" >&2
    echo "GREEN: no diff scope to enforce (vacuous)"
    exit 0
fi

# Extract path:line citations from audit-report.md.
# Bash regex limitations: use grep with extended regex.
CITATIONS=$(grep -oE '[a-zA-Z0-9_/.-]+\.(sh|json|md|py|ts|js|yaml|yml|tf|rs|go|tsx|jsx):[0-9]+' "$AUDIT_REPORT" | sort -u)

if [ -z "$CITATIONS" ]; then
    echo "GREEN: audit-report has no path:line citations (no claims to ground)"
    exit 0
fi

total=0
out_of_scope=0
in_scope_examples=""
out_of_scope_examples=""

while IFS= read -r cite; do
    [ -z "$cite" ] && continue
    total=$((total + 1))
    # Extract just the path (strip :line suffix).
    cite_path="${cite%:*}"
    if echo "$SCOPE" | grep -qFx "$cite_path"; then
        in_scope_examples="$in_scope_examples $cite"
    else
        out_of_scope=$((out_of_scope + 1))
        out_of_scope_examples="$out_of_scope_examples\n  - $cite"
    fi
done <<< "$CITATIONS"

if [ "$out_of_scope" = "0" ]; then
    echo "GREEN: $total/$total citations are in cycle diff scope"
    exit 0
else
    echo "RED: $out_of_scope/$total citations outside cycle diff scope:"
    printf '%b\n' "$out_of_scope_examples" >&2
    echo "  scope set ($(echo "$SCOPE" | wc -l | tr -d ' ') files): $(echo "$SCOPE" | head -3 | tr '\n' ',' | sed 's/,$//')..." >&2
    exit 1
fi
