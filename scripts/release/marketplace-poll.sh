#!/usr/bin/env bash
#
# marketplace-poll.sh — Post-publish marketplace propagation verifier (v8.13.2).
#
# Polls the local marketplace checkout (the path Claude Code reads at session
# startup) until the plugin.json there matches the target version, OR until
# the deadline. On success, re-invokes scripts/utility/release.sh <target> to refresh
# `installed_plugins.json` — this is the **cache-refresh ordering bug fix**:
# the original release.sh pulled marketplace AND checked version in one pass,
# which fails if the pull happens before origin/main has the new commit. By
# polling first, we ensure release.sh's check runs only when version-match is
# already true.
#
# Usage:
#   bash scripts/release/marketplace-poll.sh <target-version> \
#       [--max-wait-s 300] [--poll-interval-s 15] \
#       [--marketplace-dir <path>] [--dry-run]
#
# Default --marketplace-dir: $HOME/.claude/plugins/marketplaces/evolve-loop
# (override via env $EVOLVE_MARKETPLACE_DIR or this flag, mainly for tests).
#
# Exit codes:
#   0 — marketplace converged to target version; release.sh refresh succeeded.
#   1 — timeout: polled until --max-wait-s without matching version.
#   2 — runtime error (missing marketplace dir, malformed plugin.json, etc.)
#  10 — invalid arguments

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

log()  { echo "[marketplace-poll] $*" >&2; }
fail() { log "FAIL: $*"; exit 2; }

# ---- Args -----------------------------------------------------------------

DRY_RUN=0
MAX_WAIT_S=300
POLL_INTERVAL_S=15
MARKETPLACE_DIR="${EVOLVE_MARKETPLACE_DIR:-$HOME/.claude/plugins/marketplaces/evolve-loop}"
TARGET=""

while [ $# -gt 0 ]; do
    case "$1" in
        --max-wait-s)       shift; MAX_WAIT_S="$1" ;;
        --poll-interval-s)  shift; POLL_INTERVAL_S="$1" ;;
        --marketplace-dir)  shift; MARKETPLACE_DIR="$1" ;;
        --dry-run)          DRY_RUN=1 ;;
        --help|-h)          sed -n '2,28p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) log "unknown flag: $1"; exit 10 ;;
        *)
            if [ -z "$TARGET" ]; then TARGET="$1"
            else log "extra positional arg: $1"; exit 10
            fi ;;
    esac
    shift
done

[ -n "$TARGET" ] || { log "usage: marketplace-poll.sh <target-version> [flags]"; exit 10; }

if ! [[ "$TARGET" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    fail "target version not semver: $TARGET"
fi
[[ "$MAX_WAIT_S"      =~ ^[0-9]+$ ]] || { log "--max-wait-s must be integer";      exit 10; }
[[ "$POLL_INTERVAL_S" =~ ^[0-9]+$ ]] || { log "--poll-interval-s must be integer"; exit 10; }
[ "$POLL_INTERVAL_S" -gt 0 ] || { log "--poll-interval-s must be > 0"; exit 10; }

# ---- Helpers --------------------------------------------------------------

read_marketplace_version() {
    local dir="$1"
    local plugin_json="$dir/.claude-plugin/plugin.json"
    [ -f "$plugin_json" ] || return 1
    sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$plugin_json" | head -1
}

pull_marketplace() {
    local dir="$1"
    [ -d "$dir/.git" ] || return 0   # not a git checkout — silent no-op
    git -C "$dir" fetch origin main --quiet 2>/dev/null || return 0
    git -C "$dir" reset --hard origin/main --quiet 2>/dev/null || return 0
    return 0
}

# ---- Dry-run --------------------------------------------------------------

if [ "$DRY_RUN" = "1" ]; then
    log "DRY-RUN: would poll $MARKETPLACE_DIR for version=$TARGET"
    log "DRY-RUN: max_wait=${MAX_WAIT_S}s poll_interval=${POLL_INTERVAL_S}s"
    log "DRY-RUN: on success would invoke 'bash scripts/utility/release.sh $TARGET'"
    exit 0
fi

# ---- Validate marketplace dir ---------------------------------------------

[ -d "$MARKETPLACE_DIR" ] || fail "marketplace dir not found: $MARKETPLACE_DIR"
[ -f "$MARKETPLACE_DIR/.claude-plugin/plugin.json" ] || \
    fail "marketplace dir has no .claude-plugin/plugin.json: $MARKETPLACE_DIR"

# ---- Poll loop ------------------------------------------------------------

start_ts=$(date -u +%s)
deadline=$((start_ts + MAX_WAIT_S))
attempt=0

log "polling $MARKETPLACE_DIR for version=$TARGET (max_wait=${MAX_WAIT_S}s, interval=${POLL_INTERVAL_S}s)"

while :; do
    attempt=$((attempt + 1))
    pull_marketplace "$MARKETPLACE_DIR"
    current=$(read_marketplace_version "$MARKETPLACE_DIR" 2>/dev/null || echo "")
    if [ "$current" = "$TARGET" ]; then
        elapsed=$(( $(date -u +%s) - start_ts ))
        log "OK: marketplace converged to v$TARGET after ${elapsed}s (attempt $attempt)"
        break
    fi

    now=$(date -u +%s)
    if [ "$now" -ge "$deadline" ]; then
        log "TIMEOUT: marketplace still at v${current:-<unreadable>} after ${MAX_WAIT_S}s; expected v$TARGET"
        exit 1
    fi
    log "  attempt $attempt: marketplace at v${current:-<empty>}; waiting ${POLL_INTERVAL_S}s..."
    sleep "$POLL_INTERVAL_S"
done

# ---- Cache refresh (the ordering-bug fix) ---------------------------------
#
# Now that marketplace version matches target, run release.sh which will:
#   1. Verify all version markers consistent (already true).
#   2. Pull marketplace (already converged — no-op).
#   3. Refresh installed_plugins.json registry to point at the new version.
#
# Before v8.13.2 release.sh did pull-then-check in one pass; pull would no-op
# with a stale ref and the check would fail. With poll-first, this race is
# closed structurally.

log "running release.sh $TARGET to refresh installed_plugins.json..."
if ! bash "$REPO_ROOT/scripts/utility/release.sh" "$TARGET" >/dev/null 2>&1; then
    log "WARN: release.sh exited non-zero — manually verify installed_plugins.json"
    log "  bash scripts/utility/release.sh $TARGET"
    exit 2
fi

log "DONE: marketplace + installed_plugins.json refreshed to v$TARGET"
exit 0
