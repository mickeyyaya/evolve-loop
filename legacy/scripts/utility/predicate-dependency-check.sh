#!/usr/bin/env bash
#
# predicate-dependency-check.sh — Cross-cycle predicate-graph reachability check.
#
# Opt A (v10.19.0) — fail-safe for the "auto-skip Triage on trivial cycles"
# optimization. Cycle-91's lesson requires Triage to never rate trivial any
# cycle whose touched files intersect grep-callers of an existing regression-
# suite predicate. This script encodes that rule at the phase-gate level: if
# scout-projected file paths intersect any acs/regression-suite/**/*.sh
# basename grep, the cycle has a cross-cycle predicate dependency and Triage
# MUST run (so it can promote the cycle to MEDIUM+ rating).
#
# Pre-Triage projection: scout-report.md does not have a structured "files
# touched" projection field. We approximate by extracting backtick-quoted
# file paths from the report and checking each basename for reachability.
# This is conservative — false positives (running Triage when we could
# safely skip) are acceptable; false negatives (skipping when we should
# not) are forbidden.
#
# Usage:
#   bash legacy/scripts/utility/predicate-dependency-check.sh <cycle> <workspace>
#
# Inputs:
#   <workspace>/scout-report.md — parsed for backtick-quoted file paths
#
# Output: advisory lines on stderr
# Exit codes:
#   0 — no predicate-graph reachability (safe to auto-skip Triage)
#   1 — predicate-graph reachability detected (Triage MUST run)
#   2 — infra error (workspace missing, scout-report absent, etc.)
#
# Bash 3.2 compatible per CLAUDE.md (no declare -A, no mapfile, no GNU-only flags).

set -uo pipefail

CYCLE="${1:?usage: predicate-dependency-check.sh <cycle> <workspace>}"
WORKSPACE="${2:?usage: predicate-dependency-check.sh <cycle> <workspace>}"

# Resolve roots (PLUGIN_ROOT = read-only scripts; PROJECT_ROOT = writable state).
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESOLVE_ROOTS="$__rr_self/../lifecycle/resolve-roots.sh"
if [ -f "$RESOLVE_ROOTS" ]; then
    # shellcheck source=/dev/null
    . "$RESOLVE_ROOTS"
fi
unset __rr_self

SCOUT_REPORT="$WORKSPACE/scout-report.md"
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SUITE_DIR="$PROJECT_ROOT/acs/regression-suite"

log() { echo "[predicate-dependency-check] $*" >&2; }

if [ ! -f "$SCOUT_REPORT" ]; then
    log "ERROR: scout-report.md not found at $SCOUT_REPORT"
    exit 2
fi

if [ ! -d "$SUITE_DIR" ]; then
    log "no regression-suite dir at $SUITE_DIR — empty predicate-graph, safe to skip"
    exit 0
fi

# Extract file paths from backtick-quoted segments in scout-report.md.
# Pattern: `path/to/file.ext` — any backticked token containing a dot
# followed by 1+ alphanumeric chars (the file extension).
# Strip backticks, then take basename, then deduplicate.
basenames=$(grep -oE '`[^` ]+\.[a-zA-Z0-9]+`' "$SCOUT_REPORT" 2>/dev/null \
    | tr -d '`' \
    | awk -F/ '{ print $NF }' \
    | sort -u)

if [ -z "$basenames" ]; then
    log "no file paths found in scout-report — safe to skip"
    exit 0
fi

# For each basename, check if any acs/regression-suite/**/*.sh script
# references it (basename-level grep, matching the heuristic used by
# legacy/scripts/lifecycle/run-regression-suite-slice.sh).
deps=""
while IFS= read -r bn; do
    [ -z "$bn" ] && continue
    # -F = fixed string match (no regex interpretation on user-supplied basename).
    # -l = print only matching filenames. -r = recursive.
    if grep -rlF -- "$bn" "$SUITE_DIR" 2>/dev/null | head -1 | grep -q .; then
        deps="${deps}${bn} "
    fi
done <<EOF
$basenames
EOF

if [ -n "$deps" ]; then
    log "predicate-graph reachability detected via basenames: $deps"
    log "Triage MUST run (cycle-91 lesson: rate >= MEDIUM when grep-callers intersect)"
    exit 1
fi

log "no predicate-graph reachability — safe to auto-skip Triage"
exit 0
