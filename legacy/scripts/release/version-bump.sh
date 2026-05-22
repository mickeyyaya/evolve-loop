#!/usr/bin/env bash
#
# version-bump.sh — Atomic version updater for the v8.13.2 release pipeline.
#
# Updates the canonical version markers consistently:
#   - .claude-plugin/plugin.json (always)
#   - .claude-plugin/marketplace.json (always)
#   - skills/evolve-loop/SKILL.md heading (only if major.minor changed)
#   - README.md "Current (vX.Y)" mention (only if major.minor changed)
#   - README.md history table — adds a new "v8.X | <date> | TBD" row only if
#     the row for the new major.minor doesn't already exist
#
# Atomicity: each file is written to a sibling .tmp file, then mv'd into place.
# A partial bump (e.g., interrupted between plugin.json and marketplace.json)
# would leave the JSON files mismatched and the next release.sh would catch it.
# This is acceptable; release-pipeline.sh runs version-bump in a single shell
# step before any commit.
#
# Idempotency: if every file is already at the target version, exits 0
# without any writes (and reports which files would have been touched).
#
# Usage:
#   bash scripts/release/version-bump.sh <target-version> [--dry-run]
#
# Output (stdout, JSON):
#   {"target":"<version>","modified":["plugin.json","marketplace.json",...]}
#
# Exit codes:
#   0  — success (or no-op idempotent)
#   1  — a write failed
#  10  — invalid arguments

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PLUGIN_JSON="$REPO_ROOT/.claude-plugin/plugin.json"
MARKETPLACE_JSON="$REPO_ROOT/.claude-plugin/marketplace.json"
SKILL_MD="$REPO_ROOT/skills/evolve-loop/SKILL.md"
README_MD="$REPO_ROOT/README.md"

log()  { echo "[version-bump] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

# ---- Args -----------------------------------------------------------------

DRY_RUN=0
TARGET=""

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run) DRY_RUN=1 ;;
        --help|-h) sed -n '2,28p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) log "unknown flag: $1"; exit 10 ;;
        *)
            if [ -z "$TARGET" ]; then TARGET="$1"
            else log "extra positional arg: $1"; exit 10
            fi ;;
    esac
    shift
done

[ -n "$TARGET" ] || { log "usage: version-bump.sh <target-version> [--dry-run]"; exit 10; }

if ! [[ "$TARGET" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    fail "target version not semver: $TARGET"
fi

TARGET_MAJOR_MINOR=$(echo "$TARGET" | sed 's/\([0-9][0-9]*\.[0-9][0-9]*\).*/\1/')

# ---- Helpers --------------------------------------------------------------

current_json_version() {
    sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$1" | head -1
}

# Atomic JSON version write via jq. Updates whichever of these paths exist:
#   - top-level `.version`         (plugin.json shape)
#   - `.plugins[].version`         (marketplace.json shape — every entry)
# Both updated when both exist; the schema dictates which is canonical.
bump_json_version() {
    local file="$1" target="$2"
    if [ ! -f "$file" ]; then fail "missing: $file"; fi
    local current
    current=$(current_json_version "$file")
    if [ "$current" = "$target" ]; then
        return 1   # no-op signal
    fi
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would bump $file: $current → $target"
        return 0
    fi
    if ! command -v jq >/dev/null 2>&1; then fail "jq required"; fi
    local tmp="${file}.tmp.$$"
    # The jq filter is path-aware: only touches paths that already exist.
    # `(.version? // empty) |= $v` updates top-level .version only if present.
    # `(.plugins // [])[].version |= $v` updates every plugins[].version.
    jq --arg v "$target" '
        if has("version") then .version = $v else . end
        | if has("plugins") and (.plugins | type == "array")
            then .plugins |= map(if has("version") then .version = $v else . end)
            else . end
    ' "$file" > "$tmp" || { rm -f "$tmp"; fail "jq write failed: $file"; }
    mv -f "$tmp" "$file"
    log "OK: bumped $(basename "$file"): $current → $target"
    return 0
}

# SKILL.md heading: "# Evolve Loop vX.Y" — only major.minor.
bump_skill_heading() {
    local file="$1" target_major_minor="$2"
    [ -f "$file" ] || return 1
    local current
    current=$(sed -n 's/^# Evolve Loop v\([0-9][0-9]*\.[0-9][0-9]*\).*/\1/p' "$file" | head -1)
    if [ "$current" = "$target_major_minor" ]; then
        return 1
    fi
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would bump SKILL.md heading: v$current → v$target_major_minor"
        return 0
    fi
    local tmp="${file}.tmp.$$"
    # Replace ONLY the first matching heading line (most repos have a single H1).
    awk -v target="$target_major_minor" '
        !done && /^# Evolve Loop v[0-9]+\.[0-9]+/ {
            sub(/v[0-9]+\.[0-9]+(\.[0-9]+)?/, "v" target)
            done = 1
        }
        { print }
    ' "$file" > "$tmp" || { rm -f "$tmp"; fail "awk SKILL.md write failed"; }
    mv -f "$tmp" "$file"
    log "OK: bumped SKILL.md heading: v$current → v$target_major_minor"
    return 0
}

# README "Current (vX.Y)" table cell — only major.minor.
bump_readme_current() {
    local file="$1" target_major_minor="$2"
    [ -f "$file" ] || return 1
    local current
    current=$(sed -n 's/.*Current (v\([0-9][0-9]*\.[0-9][0-9]*\)).*/\1/p' "$file" | head -1)
    if [ "$current" = "$target_major_minor" ]; then
        return 1
    fi
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would bump README.md Current: v$current → v$target_major_minor"
        return 0
    fi
    local tmp="${file}.tmp.$$"
    sed "s/Current (v[0-9][0-9]*\.[0-9][0-9]*)/Current (v${target_major_minor})/" "$file" > "$tmp" \
        || { rm -f "$tmp"; fail "sed README write failed"; }
    mv -f "$tmp" "$file"
    log "OK: bumped README.md Current: v$current → v$target_major_minor"
    return 0
}

# README history table: add a new row "| v8.X | <today> | TBD |" if the
# row for $target_major_minor doesn't already exist. Inserts BELOW the
# previous-major.minor row.
bump_readme_history() {
    local file="$1" target_major_minor="$2"
    [ -f "$file" ] || return 1
    if grep -qE "^\| v${target_major_minor} \|" "$file"; then
        return 1
    fi
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would add README.md history row for v$target_major_minor"
        return 0
    fi
    local today
    today=$(date +"%b %-d")
    local tmp="${file}.tmp.$$"
    awk -v target="$target_major_minor" -v today="$today" '
        BEGIN { added = 0 }
        {
            print
            if (!added && /^\| v[0-9]+\.[0-9]+ \|/) {
                # Capture this row; if next line is NOT also a v-row, insert after.
                last = $0
                getline next_line
                if (next_line !~ /^\| v[0-9]+\.[0-9]+ \|/) {
                    print "| v" target " | " today " | TBD — fill in via release-pipeline.sh + changelog-gen.sh |"
                    added = 1
                }
                print next_line
            }
        }
    ' "$file" > "$tmp" || { rm -f "$tmp"; fail "awk README history write failed"; }
    mv -f "$tmp" "$file"
    log "OK: added README.md history row for v$target_major_minor"
    return 0
}

# ---- Run ------------------------------------------------------------------

modified=()

if bump_json_version "$PLUGIN_JSON" "$TARGET"; then
    modified+=(".claude-plugin/plugin.json")
fi
if bump_json_version "$MARKETPLACE_JSON" "$TARGET"; then
    modified+=(".claude-plugin/marketplace.json")
fi
if bump_skill_heading "$SKILL_MD" "$TARGET_MAJOR_MINOR"; then
    modified+=("skills/evolve-loop/SKILL.md")
fi
if bump_readme_current "$README_MD" "$TARGET_MAJOR_MINOR"; then
    modified+=("README.md (Current)")
fi
if bump_readme_history "$README_MD" "$TARGET_MAJOR_MINOR"; then
    modified+=("README.md (history)")
fi

# Emit JSON summary on stdout.
if [ ${#modified[@]} -eq 0 ]; then
    log "no-op: all files already at $TARGET"
    printf '{"target":"%s","modified":[]}\n' "$TARGET"
    exit 0
fi

# Build JSON list (manual to avoid jq round-trip).
json_list=""
for m in "${modified[@]}"; do
    if [ -n "$json_list" ]; then json_list="$json_list,"; fi
    json_list="$json_list\"$m\""
done
printf '{"target":"%s","modified":[%s]}\n' "$TARGET" "$json_list"
exit 0
