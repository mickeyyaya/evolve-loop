#!/usr/bin/env bash
#
# changelog-gen.sh — Conventional-commits parser for the v8.13.2 release pipeline.
#
# Parses `git log <from-tag>..<to-tag>` and groups commits into Keep-a-Changelog
# style sections, then prepends a `## [<version>] - <date>` block to CHANGELOG.md.
#
# Buckets (case-insensitive type prefix matching):
#   feat | feature                    → ### Added
#   fix | bugfix                      → ### Fixed
#   refactor | perf | performance     → ### Changed
#   docs | documentation              → ### Documentation
#   <no recognized prefix>            → ### Other     (the project's ~60% case)
#
# Skipped types (excluded from output by convention; mention in PR title only):
#   chore, ci, test, build, style, revert
#
# Idempotency:
#   - If CHANGELOG.md already has a `## [<version>]` heading, exit 0 without write.
#     A human or earlier run already curated this entry — preserve their work.
#   - The "preserved" decision is a header-presence check; we do NOT measure
#     section length. Manual edits of the latest entry are always preserved.
#
# Usage:
#   bash scripts/release/changelog-gen.sh <from-ref> <to-ref> <target-version> [--dry-run]
#
# <from-ref> and <to-ref> can be tags, branches, or SHAs. Conventional inputs:
#   <from-ref> = previous tag (e.g., v8.13.1)
#   <to-ref>   = HEAD or a branch tip
#
# Exit codes:
#   0 — wrote the entry, OR no-op idempotent skip, OR dry-run printed only
#   1 — git or CHANGELOG read/write error
#  10 — invalid arguments

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHANGELOG="$REPO_ROOT/CHANGELOG.md"

log()  { echo "[changelog-gen] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

# ---- Args -----------------------------------------------------------------

DRY_RUN=0
FROM_REF=""
TO_REF=""
TARGET_VERSION=""

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run) DRY_RUN=1 ;;
        --help|-h) sed -n '2,32p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) log "unknown flag: $1"; exit 10 ;;
        *)
            if   [ -z "$FROM_REF" ]; then FROM_REF="$1"
            elif [ -z "$TO_REF" ]; then TO_REF="$1"
            elif [ -z "$TARGET_VERSION" ]; then TARGET_VERSION="$1"
            else log "extra positional arg: $1"; exit 10
            fi ;;
    esac
    shift
done

[ -n "$FROM_REF" ] && [ -n "$TO_REF" ] && [ -n "$TARGET_VERSION" ] || {
    log "usage: changelog-gen.sh <from-ref> <to-ref> <target-version> [--dry-run]"
    exit 10
}

if ! [[ "$TARGET_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    fail "target version not semver: $TARGET_VERSION"
fi

cd "$REPO_ROOT"

# ---- Idempotency check ----------------------------------------------------

if [ -f "$CHANGELOG" ] && grep -qE "^## \[${TARGET_VERSION}\]" "$CHANGELOG"; then
    log "CHANGELOG.md already has [$TARGET_VERSION] entry — preserving (idempotent skip)"
    exit 0
fi

# ---- Verify refs exist ----------------------------------------------------

if ! git rev-parse --verify "$FROM_REF" >/dev/null 2>&1; then
    fail "from-ref does not exist: $FROM_REF"
fi
if ! git rev-parse --verify "$TO_REF" >/dev/null 2>&1; then
    fail "to-ref does not exist: $TO_REF"
fi

# ---- Read commits and bucket ----------------------------------------------

# Format: %H<TAB>%s for first-line subject. Body fetched separately if needed.
# We use a sentinel separator that subjects can't contain (control char).
SEP=$'\x1f'
LOG_OUTPUT=$(git log --pretty=format:"%H${SEP}%s" "${FROM_REF}..${TO_REF}" 2>/dev/null || true)

if [ -z "$LOG_OUTPUT" ]; then
    log "WARN: no commits between $FROM_REF..$TO_REF"
    log "writing minimal placeholder entry"
fi

declare_arrays() {
    # bash 3.2-safe: use plain string buffers per bucket.
    BUCKET_ADDED=""
    BUCKET_FIXED=""
    BUCKET_CHANGED=""
    BUCKET_DOCS=""
    BUCKET_OTHER=""
}
declare_arrays

# Lowercase the type prefix for matching. bash 3.2 compatible — uses sed/case.
classify() {
    local subject="$1"
    local type rest
    # Try "<word>(<scope>): <rest>" first, then "<word>: <rest>".
    type=$(echo "$subject" | sed -nE 's/^([A-Za-z]+)\(([^)]+)\):[[:space:]]*(.*)$/\1/p')
    if [ -n "$type" ]; then
        rest=$(echo "$subject" | sed -nE 's/^([A-Za-z]+)\(([^)]+)\):[[:space:]]*(.*)$/\3/p')
    else
        type=$(echo "$subject" | sed -nE 's/^([A-Za-z]+):[[:space:]]*(.*)$/\1/p')
        rest=$(echo "$subject" | sed -nE 's/^([A-Za-z]+):[[:space:]]*(.*)$/\2/p')
    fi
    if [ -z "$type" ]; then
        # No type prefix — the ~60% non-conformant case.
        BUCKET_OTHER+="- ${subject}"$'\n'
        return
    fi
    type=$(echo "$type" | tr 'A-Z' 'a-z')
    case "$type" in
        feat|feature)             BUCKET_ADDED+="- ${rest}"$'\n' ;;
        fix|bugfix)               BUCKET_FIXED+="- ${rest}"$'\n' ;;
        refactor|perf|performance|stability|techdebt) BUCKET_CHANGED+="- ${rest}"$'\n' ;;
        docs|documentation|doc)   BUCKET_DOCS+="- ${rest}"$'\n' ;;
        chore|ci|test|build|style|revert|meta|release) : ;;
        *)
            # Recognized as a typed prefix, but unknown type — bucket as Other.
            BUCKET_OTHER+="- (${type}) ${rest}"$'\n' ;;
    esac
}

if [ -n "$LOG_OUTPUT" ]; then
    while IFS= read -r line; do
        subject="${line#*${SEP}}"
        classify "$subject"
    done <<< "$LOG_OUTPUT"
fi

# ---- Render the new entry --------------------------------------------------

today=$(date +"%Y-%m-%d")
entry="## [${TARGET_VERSION}] - ${today}"$'\n\n'
entry+="_Generated by \`scripts/release/changelog-gen.sh\` from commits ${FROM_REF}..${TO_REF}._"$'\n\n'

append_bucket() {
    local heading="$1" body="$2"
    if [ -n "$body" ]; then
        entry+="### ${heading}"$'\n\n'"$body"$'\n'
    fi
}
append_bucket "Added"         "$BUCKET_ADDED"
append_bucket "Fixed"         "$BUCKET_FIXED"
append_bucket "Changed"       "$BUCKET_CHANGED"
append_bucket "Documentation" "$BUCKET_DOCS"
append_bucket "Other"         "$BUCKET_OTHER"

if [ -z "$BUCKET_ADDED$BUCKET_FIXED$BUCKET_CHANGED$BUCKET_DOCS$BUCKET_OTHER" ]; then
    entry+="### Other"$'\n\n'"- (no commits found in range; placeholder entry)"$'\n\n'
fi

entry+=$'\n---\n\n'

# ---- Dry-run prints; real run prepends ------------------------------------

if [ "$DRY_RUN" = "1" ]; then
    log "DRY-RUN: would prepend the following block to $CHANGELOG:"
    echo "----- BEGIN GENERATED -----"
    printf '%s' "$entry"
    echo "----- END GENERATED -----"
    exit 0
fi

# Prepend to CHANGELOG.md, preserving the file's existing top-of-file content
# (typically "# Changelog\n\nAll notable changes...\n\n").
if [ ! -f "$CHANGELOG" ]; then
    log "WARN: CHANGELOG.md not found; creating fresh file"
    {
        echo "# Changelog"
        echo
        echo "All notable changes to this project will be documented in this file."
        echo
        printf '%s' "$entry"
    } > "$CHANGELOG"
    log "OK: created $CHANGELOG with [$TARGET_VERSION]"
    exit 0
fi

# Insert the new entry AFTER the file header (everything up to the first
# `## [` line) and BEFORE the existing first version block. Pure shell so
# we don't trip on BSD awk's no-newlines-in-v rule.
tmp="${CHANGELOG}.tmp.$$"
first_block_line=$(grep -nE '^## \[' "$CHANGELOG" | head -1 | cut -d: -f1 || true)
if [ -z "$first_block_line" ]; then
    # No version blocks at all — append at end.
    cp "$CHANGELOG" "$tmp"
    printf '%s' "$entry" >> "$tmp"
else
    head -n $((first_block_line - 1)) "$CHANGELOG" > "$tmp"
    printf '%s' "$entry" >> "$tmp"
    tail -n +"$first_block_line" "$CHANGELOG" >> "$tmp"
fi
mv -f "$tmp" "$CHANGELOG"

log "OK: prepended [$TARGET_VERSION] entry to $CHANGELOG"
exit 0
