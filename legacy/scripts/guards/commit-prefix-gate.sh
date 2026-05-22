#!/usr/bin/env bash
#
# commit-prefix-gate.sh — Layer 1 of Reward-Hacking Defense System (ADR-0012)
#
# Verifies that the commit-message prefix (e.g. "docs:", "feat(guards):", "fix:")
# matches the diff scope declared in .evolve/commit-prefix-scope.json. Rejects
# mislabeled commits with rc=2.
#
# Closes the cycle 70-72-75 mislabeling pattern:
#   - Cycle 71 feat(token-opt) commit was a role-gate bug fix (zero token-opt code)
#   - Cycle 72 feat(token-opt) commit was pure docs (zero production code)
#   - Cycle 75 docs: commit had zero docs in diff (fabricated AC verification)
#
# Bash 3.2 compatible: no declare -A, no mapfile, no GNU-only flags, no ${var^^}.
#
# Usage:
#   commit-prefix-gate.sh --msg "<commit-message>" [--repo-dir <path>] [--staged | --diff-ref <ref>]
#
#   --msg <STR>       Commit message (first line used; prefix extracted via regex).
#                     Required.
#   --repo-dir <DIR>  Git repo to inspect (default: current cwd). Used by ship.sh's
#                     worktree path: pass the worktree dir, gate uses git -C <DIR>.
#                     Closes the cwd defect identified in cycle 75 audit.
#   --staged          Inspect staged files via `git diff --cached --name-only` (DEFAULT).
#   --diff-ref <REF>  Inspect committed diff via `git diff <ref>..HEAD --name-only`.
#                     Useful for verifying existing commits.
#
# Bypass:
#   EVOLVE_BYPASS_PREFIX_GATE=1 + SHIP_CLASS=manual : allowed (logged WARN, emergency)
#   EVOLVE_BYPASS_PREFIX_GATE=1 + SHIP_CLASS=cycle  : DENIED (cycle integrity invariant)
#
# Exit codes:
#   0 = prefix matches scope (or unknown prefix = pass-through)
#   2 = scope violation (denied)
#   3 = bad arguments
#   4 = manifest missing or malformed

set -uo pipefail

# ── Configuration ───────────────────────────────────────────────────────────
GATE_NAME="commit-prefix-gate"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
DEFAULT_MANIFEST="$REPO_ROOT/.evolve/commit-prefix-scope.json"
GUARDS_LOG="$REPO_ROOT/.evolve/guards.log"

mkdir -p "$(dirname "$GUARDS_LOG")" 2>/dev/null || true

log() {
    local ts
    ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "[$ts] [$GATE_NAME] $*" >> "$GUARDS_LOG" 2>/dev/null || true
}

deny() {
    local msg="$1"
    log "DENY: $msg"
    echo "[$GATE_NAME] DENY: $msg" >&2
    echo "[$GATE_NAME] To bypass (emergency, manual class only): EVOLVE_BYPASS_PREFIX_GATE=1 SHIP_CLASS=manual" >&2
    exit 2
}

allow() {
    local msg="$1"
    log "ALLOW: $msg"
    exit 0
}

# ── Argument parsing ────────────────────────────────────────────────────────
COMMIT_MSG=""
REPO_DIR=""
MODE="staged"
DIFF_REF=""
MANIFEST="$DEFAULT_MANIFEST"

while [ $# -gt 0 ]; do
    case "$1" in
        --msg)
            COMMIT_MSG="$2"
            shift 2
            ;;
        --msg=*)
            COMMIT_MSG="${1#--msg=}"
            shift
            ;;
        --repo-dir)
            REPO_DIR="$2"
            shift 2
            ;;
        --repo-dir=*)
            REPO_DIR="${1#--repo-dir=}"
            shift
            ;;
        --staged)
            MODE="staged"
            shift
            ;;
        --diff-ref)
            MODE="ref"
            DIFF_REF="$2"
            shift 2
            ;;
        --diff-ref=*)
            MODE="ref"
            DIFF_REF="${1#--diff-ref=}"
            shift
            ;;
        --manifest)
            MANIFEST="$2"
            shift 2
            ;;
        --help|-h)
            sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "[$GATE_NAME] unknown arg: $1" >&2
            exit 3
            ;;
    esac
done

if [ -z "$COMMIT_MSG" ]; then
    echo "[$GATE_NAME] usage: $0 --msg \"<commit-message>\" [--repo-dir <path>] [--staged | --diff-ref <ref>]" >&2
    exit 3
fi

# Default repo-dir to current cwd
if [ -z "$REPO_DIR" ]; then
    REPO_DIR="$(pwd)"
fi

# Verify repo-dir is a git working tree
if ! git -C "$REPO_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "[$GATE_NAME] not a git working tree: $REPO_DIR" >&2
    exit 3
fi

# ── Bypass check ────────────────────────────────────────────────────────────
if [ "${EVOLVE_BYPASS_PREFIX_GATE:-0}" = "1" ]; then
    SHIP_CLASS="${SHIP_CLASS:-cycle}"
    case "$SHIP_CLASS" in
        manual)
            log "WARN: EVOLVE_BYPASS_PREFIX_GATE=1 + SHIP_CLASS=manual — bypass allowed"
            echo "[$GATE_NAME] WARN: bypass active (manual class); gate not enforcing" >&2
            exit 0
            ;;
        *)
            deny "bypass requested but SHIP_CLASS='$SHIP_CLASS' (only 'manual' permits bypass)"
            ;;
    esac
fi

# ── Manifest check ──────────────────────────────────────────────────────────
if [ ! -f "$MANIFEST" ]; then
    log "WARN: manifest missing at $MANIFEST — pass-through (gate not yet provisioned)"
    echo "[$GATE_NAME] WARN: manifest missing at $MANIFEST — pass-through" >&2
    exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
    log "WARN: jq missing — pass-through"
    echo "[$GATE_NAME] WARN: jq missing — pass-through" >&2
    exit 0
fi

if ! jq empty < "$MANIFEST" 2>/dev/null; then
    echo "[$GATE_NAME] ERROR: manifest is not valid JSON: $MANIFEST" >&2
    exit 4
fi

# ── Extract prefix from commit message ──────────────────────────────────────
# First line only; trim leading whitespace
FIRST_LINE="$(echo "$COMMIT_MSG" | head -1 | sed 's/^[[:space:]]*//')"

# Match patterns like "type:", "type(scope):", "type(scope)!:"
# We extract everything up to and including the first colon (excluding the colon)
PREFIX=""
if echo "$FIRST_LINE" | grep -qE '^[a-z][a-z-]*(\([a-z0-9-]+\))?!?:'; then
    PREFIX="$(echo "$FIRST_LINE" | sed -E 's/^([a-z][a-z-]*(\([a-z0-9-]+\))?)!?:.*/\1/')"
fi

if [ -z "$PREFIX" ]; then
    log "WARN: no conventional-commit prefix in '$FIRST_LINE' — pass-through"
    echo "[$GATE_NAME] WARN: commit message lacks conventional prefix; pass-through. Use 'type(scope): message' for gate coverage." >&2
    exit 0
fi

log "prefix='$PREFIX' commit_msg='$FIRST_LINE' repo_dir='$REPO_DIR' mode='$MODE'"

# ── Look up prefix in manifest ──────────────────────────────────────────────
PREFIX_ENTRY="$(jq --arg p "$PREFIX" '.prefixes[$p] // empty' "$MANIFEST")"

if [ -z "$PREFIX_ENTRY" ] || [ "$PREFIX_ENTRY" = "null" ]; then
    log "unknown prefix '$PREFIX' — pass-through"
    echo "[$GATE_NAME] INFO: unknown prefix '$PREFIX' — pass-through. Add to $MANIFEST if regulation needed." >&2
    exit 0
fi

# Permissive escape hatch
if echo "$PREFIX_ENTRY" | jq -e '.any_path == true' >/dev/null 2>&1; then
    allow "prefix '$PREFIX' has any_path=true (permissive)"
fi

# ── Gather diff paths ───────────────────────────────────────────────────────
DIFF_PATHS=""
case "$MODE" in
    staged)
        DIFF_PATHS="$(git -C "$REPO_DIR" diff --cached --name-only 2>/dev/null)"
        ;;
    ref)
        DIFF_PATHS="$(git -C "$REPO_DIR" diff "$DIFF_REF"..HEAD --name-only 2>/dev/null)"
        ;;
esac

if [ -z "$DIFF_PATHS" ]; then
    log "WARN: no diff paths found in $MODE mode — pass-through"
    echo "[$GATE_NAME] WARN: no diff (empty staged or empty ref-diff); pass-through" >&2
    exit 0
fi

# ── Glob matching (bash 3.2 — case statement) ──────────────────────────────
# Returns 0 if any path matches any pattern.
# CRITICAL: set -f disables filename globbing — without this, `for pat in $patterns` would
# glob-expand patterns like `docs/**` against the cwd's actual files (bug discovered post-Cycle A).
match_any_path() {
    local patterns="$1"
    local paths="$2"
    local pat path
    local restore_glob=""
    case $- in *f*) ;; *) restore_glob=1; set -f ;; esac
    for path in $paths; do
        # shellcheck disable=SC2086
        for pat in $patterns; do
            # Direct case-glob match (bash treats * loosely — matches across / too)
            case "$path" in
                $pat)
                    echo "$path matched $pat"
                    [ -n "$restore_glob" ] && set +f
                    return 0
                    ;;
            esac
            # Try with ** expanded to *
            local pat_flat="${pat//\*\*/\*}"
            # shellcheck disable=SC2254
            case "$path" in
                $pat_flat)
                    echo "$path matched $pat (via ** expansion)"
                    [ -n "$restore_glob" ] && set +f
                    return 0
                    ;;
            esac
            # Try prefix match for x/** patterns
            case "$pat" in
                *'/**')
                    local prefix="${pat%/**}"
                    case "$path" in
                        "$prefix"/*)
                            echo "$path matched $pat (prefix expansion)"
                            [ -n "$restore_glob" ] && set +f
                            return 0
                            ;;
                    esac
                    ;;
                '**/'*)
                    local suffix="${pat#**/}"
                    case "$path" in
                        */$suffix|$suffix)
                            echo "$path matched $pat (suffix expansion)"
                            [ -n "$restore_glob" ] && set +f
                            return 0
                            ;;
                    esac
                    ;;
            esac
        done
    done
    [ -n "$restore_glob" ] && set +f
    return 1
}

# Returns 0 if ALL paths match at least one pattern
all_paths_match() {
    local patterns="$1"
    local paths="$2"
    local path
    local restore_glob=""
    case $- in *f*) ;; *) restore_glob=1; set -f ;; esac
    for path in $paths; do
        if ! match_any_path "$patterns" "$path" >/dev/null 2>&1; then
            [ -n "$restore_glob" ] && set +f
            return 1
        fi
    done
    [ -n "$restore_glob" ] && set +f
    return 0
}

# Returns 0 if all paths are ENTIRELY under forbidden patterns
all_paths_forbidden() {
    local patterns="$1"
    local paths="$2"
    all_paths_match "$patterns" "$paths"
}

# ── Apply rules ─────────────────────────────────────────────────────────────
REQUIRED_PATHS="$(echo "$PREFIX_ENTRY" | jq -r '.required_paths[]? // empty' | tr '\n' ' ')"
FORBIDDEN_ONLY_PATHS="$(echo "$PREFIX_ENTRY" | jq -r '.forbidden_only_paths[]? // empty' | tr '\n' ' ')"
DIFF_MUST_BE_SUBSET="$(echo "$PREFIX_ENTRY" | jq -r '.diff_must_be_subset // false')"

# Rule 1: required_paths — at least ONE diff path must match at least one required pattern
if [ -n "$REQUIRED_PATHS" ]; then
    MATCH_RESULT="$(match_any_path "$REQUIRED_PATHS" "$DIFF_PATHS" 2>&1)"
    if [ -z "$MATCH_RESULT" ]; then
        deny "prefix '$PREFIX' requires at least one diff path under [$REQUIRED_PATHS], but diff contains only:
$DIFF_PATHS"
    fi
    log "required_paths check passed: $MATCH_RESULT"
fi

# Rule 2: forbidden_only_paths — diff must NOT be entirely under forbidden patterns
if [ -n "$FORBIDDEN_ONLY_PATHS" ]; then
    if all_paths_forbidden "$FORBIDDEN_ONLY_PATHS" "$DIFF_PATHS"; then
        deny "prefix '$PREFIX' diff is entirely under forbidden_only_paths [$FORBIDDEN_ONLY_PATHS]. This commit looks like docs/lessons/test work mislabeled as a feature. Use a different prefix (docs:, chore:, test:)."
    fi
    log "forbidden_only_paths check passed"
fi

# Rule 3: diff_must_be_subset — every diff path must match at least one required pattern
if [ "$DIFF_MUST_BE_SUBSET" = "true" ] && [ -n "$REQUIRED_PATHS" ]; then
    if ! all_paths_match "$REQUIRED_PATHS" "$DIFF_PATHS"; then
        local violators=""
        for path in $DIFF_PATHS; do
            if ! match_any_path "$REQUIRED_PATHS" "$path" >/dev/null 2>&1; then
                violators="$violators $path"
            fi
        done
        deny "prefix '$PREFIX' requires diff to be a subset of [$REQUIRED_PATHS], but these paths violate:$violators"
    fi
    log "diff_must_be_subset check passed"
fi

allow "prefix '$PREFIX' matches scope: required_paths=[$REQUIRED_PATHS] forbidden_only=[$FORBIDDEN_ONLY_PATHS] subset=$DIFF_MUST_BE_SUBSET"
