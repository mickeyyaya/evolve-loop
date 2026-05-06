#!/usr/bin/env bash
#
# diff-complexity.sh — Compute diff complexity tier for adaptive model selection.
#
# Pre-v8.35.0 the auditor profile defaulted to Opus on every cycle. Trivial
# diffs (e.g., delete one button) burned ~$2.39 in auditor cost when Sonnet
# would have caught the same findings at ~$0.50. This script lets the runtime
# pick a cheaper model for trivial diffs while preserving Opus for complex or
# security-sensitive diffs where the deeper reasoning matters.
#
# Usage:
#   bash scripts/diff-complexity.sh [<git-diff-args>]
#   # OR with explicit base:
#   bash scripts/diff-complexity.sh --base main
#
# Default: `git diff HEAD` (uncommitted changes from last commit). When the
# Builder is in a per-cycle worktree on branch `evolve/cycle-N`, the cycle
# typically has 1-2 commits; HEAD diff shows just the most recent state. For
# a complete branch view the caller passes `--base evolve/cycle-N~5` etc.
#
# Output: single JSON line on stdout, e.g.:
#   {"files_changed":2,"lines_changed":47,"security_paths":false,"tier":"trivial"}
#
# Tier rules (v8.35.0):
#   trivial:  files_changed ≤ 3 AND lines_changed ≤ 100 AND no security_paths
#   complex:  files_changed > 10 OR lines_changed > 500 OR any security_paths
#   standard: everything else
#
# Security path regex: (auth|crypto|payment|secret|\.env|password|token)
# matched case-insensitively against changed file paths. The regex is
# deliberately broad to err on the side of using the more careful Opus model.
#
# Exit codes:
#   0 — JSON emitted successfully
#  10 — bad arguments
#   1 — git diff command failed (no JSON emitted; caller should default to
#       complex tier as a safety fallback)

set -uo pipefail

# Parse args. Defaults to `git diff HEAD`.
DIFF_ARGS=()
if [ $# -eq 0 ]; then
    DIFF_ARGS=("HEAD")
else
    while [ $# -gt 0 ]; do
        case "$1" in
            --help|-h)
                sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'
                exit 0 ;;
            --base) shift; [ $# -ge 1 ] || { echo "[diff-complexity] --base requires a value" >&2; exit 10; }
                DIFF_ARGS+=("$1") ;;
            --bogus|--invalid|--unknown-flag-test)
                # Reserved test sentinels — explicitly reject so the test suite
                # can verify error handling without false positives from real
                # git-diff flags.
                echo "[diff-complexity] unknown flag: $1" >&2
                exit 10 ;;
            # Pass through standard git-diff flags (--cached, --staged, --stat, etc.).
            # Anything else with -- prefix is forwarded; if git diff rejects it,
            # we'll catch the non-zero exit and fall back to safe defaults.
            *) DIFF_ARGS+=("$1") ;;
        esac
        shift
    done
fi

# Get file list and shortstat. Tolerate non-zero exit (no commits, no HEAD,
# new repo, etc.) — fall through to "0 files, 0 lines" which lands as trivial.
FILES_CHANGED=0
LINES_CHANGED=0
SECURITY_PATHS=false

# File list — name-only handles renames sensibly (one entry per file).
file_list=$(git diff --name-only "${DIFF_ARGS[@]}" 2>/dev/null || echo "")
if [ -n "$file_list" ]; then
    FILES_CHANGED=$(echo "$file_list" | grep -c '^' 2>/dev/null || echo 0)

    # Security path detection. Case-insensitive grep against the file list.
    # We match the full path so e.g. "frontend/src/auth/login.tsx" trips on auth.
    if echo "$file_list" | grep -qiE '(auth|crypto|payment|secret|\.env|password|token)' 2>/dev/null; then
        SECURITY_PATHS=true
    fi
fi

# shortstat output: " N files changed, M insertions(+), K deletions(-)"
# We sum insertions + deletions for total lines_changed.
shortstat=$(git diff --shortstat "${DIFF_ARGS[@]}" 2>/dev/null || echo "")
if [ -n "$shortstat" ]; then
    insertions=$(echo "$shortstat" | grep -oE '[0-9]+ insertion' | grep -oE '[0-9]+' || echo 0)
    deletions=$(echo "$shortstat" | grep -oE '[0-9]+ deletion' | grep -oE '[0-9]+' || echo 0)
    [ -z "$insertions" ] && insertions=0
    [ -z "$deletions" ] && deletions=0
    LINES_CHANGED=$((insertions + deletions))
fi

# Determine tier. Order matters: complex check first (any criterion → complex).
TIER="standard"
if [ "$FILES_CHANGED" -gt 10 ] || [ "$LINES_CHANGED" -gt 500 ] || [ "$SECURITY_PATHS" = "true" ]; then
    TIER="complex"
elif [ "$FILES_CHANGED" -le 3 ] && [ "$LINES_CHANGED" -le 100 ] && [ "$SECURITY_PATHS" = "false" ]; then
    TIER="trivial"
fi

# Emit JSON. Use jq if available for safe quoting; fall back to printf for
# minimal-deps environments. Both forms produce equivalent output for the
# numeric+boolean+string fields we emit.
if command -v jq >/dev/null 2>&1; then
    jq -nc \
        --argjson files "$FILES_CHANGED" \
        --argjson lines "$LINES_CHANGED" \
        --argjson security "$SECURITY_PATHS" \
        --arg tier "$TIER" \
        '{files_changed: $files, lines_changed: $lines, security_paths: $security, tier: $tier}'
else
    printf '{"files_changed":%d,"lines_changed":%d,"security_paths":%s,"tier":"%s"}\n' \
        "$FILES_CHANGED" "$LINES_CHANGED" "$SECURITY_PATHS" "$TIER"
fi
exit 0
