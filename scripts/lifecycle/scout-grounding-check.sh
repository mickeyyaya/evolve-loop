#!/usr/bin/env bash
#
# scout-grounding-check.sh — Verify Scout report's working-tree claims are real.
#
# WHY THIS EXISTS
#
# Cycle 61 demonstrated a class of failure where Scout (gemini-3.1-pro-preview)
# generated a Key Findings table referencing paths with quantitative claims
# ("+90 lines", "untracked"), and downstream phases (Builder, Auditor) trusted
# those claims without grounding them against git state. When Scout's claims
# are real, this check passes. When Scout fabricates a path or quantity, this
# check fails and the gate halts before Builder commits to a wrong understanding.
#
# CONTRACT
#
# Inputs:
#   1. Path to a scout-report.md (required)
#   2. Optional: --tree-sha SHA — override "current" git state with a historical
#      tree (for retrospective audits like cycle-61 regression replay)
#
# Output:
#   - stdout: per-finding GREEN/RED lines
#   - stderr: details on why a finding RED'd
#
# Exit codes:
#   0 — all Key Findings paths are real (in git tree or working-tree changes)
#   1 — at least one Key Findings claim cannot be grounded
#   2 — usage error (file missing, bad args)
#
# DESIGN
#
# A "groundable" Key Findings row is one that:
#   - Has a `## Key Findings` heading (we look only in that section)
#   - Contains a backtick-quoted path: `scripts/foo.sh`, `acs/some/dir/`
#   - Contains a quantitative or status claim: `+N lines`, `untracked`,
#     `Untracked`, `modified`, `flipped`, `new file`, `Working tree`
#
# For each groundable row, the path must appear in EITHER:
#   - `git status --porcelain` (working-tree changes, untracked, etc.)
#   - `git diff --stat HEAD` (committed-vs-working-tree diff)
#   - `git ls-files` (file existed at tree_sha)
#
# Empty Key Findings sections (or absent table) → exit 0 (vacuously grounded).

set -uo pipefail

usage() {
    cat <<'EOF' >&2
scout-grounding-check.sh <scout-report.md> [--tree-sha SHA]

Verifies that Scout's Key Findings entries reference real paths visible to
git at the time of scout (or at a specific tree_sha if provided).

Exit codes: 0=all grounded, 1=at least one ungrounded, 2=usage error.
EOF
}

SCOUT_REPORT=""
TREE_SHA=""
while [ $# -gt 0 ]; do
    case "$1" in
        --tree-sha) shift; TREE_SHA="${1:-}" ;;
        --help|-h) usage; exit 0 ;;
        --*) echo "[scout-grounding] unknown flag: $1" >&2; usage; exit 2 ;;
        *)
            if [ -z "$SCOUT_REPORT" ]; then SCOUT_REPORT="$1"
            else echo "[scout-grounding] extra arg: $1" >&2; usage; exit 2
            fi ;;
    esac
    shift
done

[ -n "$SCOUT_REPORT" ] || { usage; exit 2; }
[ -f "$SCOUT_REPORT" ] || { echo "[scout-grounding] not a file: $SCOUT_REPORT" >&2; exit 2; }

# Extract the Key Findings section (between `## Key Findings` and next `## `).
KF_TEXT=$(awk '
    /^## Key Findings/ { in_kf=1; next }
    in_kf && /^## / { in_kf=0 }
    in_kf { print }
' "$SCOUT_REPORT")

if [ -z "$KF_TEXT" ]; then
    echo "GREEN: scout-report has no Key Findings section — vacuously grounded"
    exit 0
fi

# Collect git visibility sets.
GIT_STATUS=$(git status --porcelain 2>/dev/null || true)
GIT_DIFF_STAT=$(git diff --stat HEAD 2>/dev/null || true)
if [ -n "$TREE_SHA" ]; then
    GIT_LS_FILES=$(git ls-tree -r --name-only "$TREE_SHA" 2>/dev/null || true)
else
    GIT_LS_FILES=$(git ls-files 2>/dev/null || true)
fi

# Parse each table row that has a backtick-quoted path AND a claim marker.
# Bash 3.2 compatible: no associative arrays, no readarray.
ungrounded_count=0
total_count=0

# Read the KF_TEXT line by line.
while IFS= read -r line; do
    # Match a backtick-quoted token. Use bash native regex.
    if ! [[ "$line" =~ \`([a-zA-Z0-9_./-]+)\` ]]; then
        continue
    fi
    candidate_path="${BASH_REMATCH[1]}"

    # Must also contain a claim marker.
    if ! echo "$line" | grep -qiE '\+[0-9]+ lines?|untracked|modified|flipped|new file|working[ -]tree'; then
        continue
    fi

    total_count=$((total_count + 1))

    # Strip trailing slash for directories.
    check_path="${candidate_path%/}"

    grounded=0
    # Check git status (handles untracked, modified, etc.)
    if echo "$GIT_STATUS" | grep -qF "$check_path"; then grounded=1; fi
    # Check diff stat
    if [ "$grounded" = "0" ] && echo "$GIT_DIFF_STAT" | grep -qF "$check_path"; then grounded=1; fi
    # Check tree (file present, even if unchanged)
    if [ "$grounded" = "0" ] && echo "$GIT_LS_FILES" | grep -qE "^${check_path}(/|$)"; then grounded=1; fi

    if [ "$grounded" = "1" ]; then
        echo "GREEN: '$candidate_path' grounded in git state"
    else
        echo "RED: '$candidate_path' NOT in git state (claim: $(echo "$line" | tr -d '|' | head -c 100))"
        ungrounded_count=$((ungrounded_count + 1))
    fi
done <<< "$KF_TEXT"

if [ "$total_count" = "0" ]; then
    echo "GREEN: no groundable Key Findings rows (no path+claim pairs found) — vacuously grounded"
    exit 0
fi

if [ "$ungrounded_count" = "0" ]; then
    echo "SUMMARY: $total_count/$total_count Key Findings grounded — GREEN"
    exit 0
else
    echo "SUMMARY: $ungrounded_count/$total_count Key Findings UNGROUNDED — RED" >&2
    exit 1
fi
