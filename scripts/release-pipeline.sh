#!/usr/bin/env bash
#
# release-pipeline.sh — Self-healing release pipeline driver (v8.13.2).
#
# Single declarative entry point for "publish a new release." Owns the entire
# pre-flight → bump → changelog → audit-bound ship → marketplace-poll →
# rollback-on-failure flow. Every component sub-script supports --dry-run and
# is independently testable; this driver orchestrates them and writes a
# per-publish journal that rollback.sh can read.
#
# Usage:
#   bash scripts/release-pipeline.sh <target-version> \
#       [--dry-run]              # simulate, no mutations anywhere
#       [--no-rollback]          # don't auto-rollback on post-push failure
#       [--skip-tests]           # skip preflight gate-test execution (hot fixes)
#       [--max-poll-wait-s 300]  # marketplace propagation deadline
#       [--from-tag <tag>]       # changelog range start (default: previous tag)
#
# Lifecycle (each step is allowed to be a no-op when --dry-run):
#   1. preflight.sh           — clean tree, semver-valid, audit PASS, gate tests
#   2. changelog-gen.sh       — append [<version>] entry from commits since prev tag
#   3. version-bump.sh        — update plugin.json, marketplace.json, etc.
#   4. release.sh <target>    — verify all version markers consistent (pre-push)
#   5. ship.sh "<msg>"        — atomic commit + push + gh release create
#   6. marketplace-poll.sh    — poll up to 5 min, then re-run release.sh for cache
#   7. (on any post-push failure with --rollback-on-fail not disabled): rollback.sh
#
# Journal: .evolve/release-journal/<version>-<timestamp>.json — one file per
# attempt. rollback.sh reads this to know what to undo.
#
# Exit codes:
#   0  — published and propagated successfully
#   1  — a pre-publish step failed (preflight, bump, changelog, release.sh check)
#   2  — ship.sh failed (no rollback needed; nothing went out)
#   3  — post-publish (poll/refresh) failed; auto-rollback ran (or was skipped)
#  10  — invalid arguments

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="$REPO_ROOT/scripts/release"
JOURNAL_DIR="$REPO_ROOT/.evolve/release-journal"

PREFLIGHT="$RELEASE_DIR/preflight.sh"
CHANGELOG_GEN="$RELEASE_DIR/changelog-gen.sh"
VERSION_BUMP="$RELEASE_DIR/version-bump.sh"
MARKETPLACE_POLL="$RELEASE_DIR/marketplace-poll.sh"
ROLLBACK="$RELEASE_DIR/rollback.sh"
RELEASE_SH="$REPO_ROOT/scripts/release.sh"
SHIP_SH="$REPO_ROOT/scripts/ship.sh"

log()  { echo "[release-pipeline] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

# ---- Args -----------------------------------------------------------------

DRY_RUN=0
NO_ROLLBACK=0
SKIP_TESTS=0
MAX_POLL_WAIT_S=300
FROM_TAG=""
TARGET=""

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run)         DRY_RUN=1 ;;
        --no-rollback)     NO_ROLLBACK=1 ;;
        --skip-tests)      SKIP_TESTS=1 ;;
        --max-poll-wait-s) shift; MAX_POLL_WAIT_S="$1" ;;
        --from-tag)        shift; FROM_TAG="$1" ;;
        --help|-h)         sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) log "unknown flag: $1"; exit 10 ;;
        *)
            if [ -z "$TARGET" ]; then TARGET="$1"
            else log "extra positional arg: $1"; exit 10
            fi ;;
    esac
    shift
done

[ -n "$TARGET" ] || { log "usage: release-pipeline.sh <target-version> [flags]"; exit 10; }
if ! [[ "$TARGET" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    fail "target version not semver: $TARGET"
fi

# Resolve from-tag if not provided.
if [ -z "$FROM_TAG" ]; then
    FROM_TAG=$(git -C "$REPO_ROOT" describe --tags --abbrev=0 2>/dev/null || echo "")
    if [ -z "$FROM_TAG" ]; then
        log "WARN: no previous tag found; changelog range will start from initial commit"
        FROM_TAG=$(git -C "$REPO_ROOT" rev-list --max-parents=0 HEAD | head -1)
    fi
fi

log "target: v$TARGET"
log "changelog range: $FROM_TAG..HEAD"
log "dry-run: $DRY_RUN | no-rollback: $NO_ROLLBACK | skip-tests: $SKIP_TESTS"

# ---- Journal --------------------------------------------------------------

JOURNAL=""
init_journal() {
    if [ "$DRY_RUN" = "1" ]; then
        JOURNAL="/tmp/release-pipeline-dryrun-$$.json"
    else
        mkdir -p "$JOURNAL_DIR"
        local ts
        ts=$(date -u +"%Y%m%dT%H%M%SZ")
        JOURNAL="$JOURNAL_DIR/${TARGET}-${ts}.json"
    fi
    local started
    started=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local branch
    branch=$(git -C "$REPO_ROOT" symbolic-ref --short HEAD 2>/dev/null || echo "unknown")
    cat > "$JOURNAL" <<EOF
{
  "version": "$TARGET",
  "tag": "v$TARGET",
  "commit_sha": "",
  "branch": "$branch",
  "release_url": "",
  "started_at": "$started",
  "completed_at": "",
  "steps": []
}
EOF
    log "journal: $JOURNAL"
}

journal_step() {
    # journal_step <step-name> <status> [<note>]
    local step="$1" status="$2" note="${3:-}"
    [ -f "$JOURNAL" ] || return 0
    command -v jq >/dev/null 2>&1 || return 0
    local tmp="${JOURNAL}.tmp.$$"
    local now
    now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    jq --arg s "$step" --arg st "$status" --arg n "$note" --arg t "$now" \
       '.steps += [{step: $s, status: $st, note: $n, timestamp: $t}]' \
       "$JOURNAL" > "$tmp" && mv -f "$tmp" "$JOURNAL"
}

journal_set_field() {
    # journal_set_field <field> <value>
    [ -f "$JOURNAL" ] || return 0
    command -v jq >/dev/null 2>&1 || return 0
    local tmp="${JOURNAL}.tmp.$$"
    jq --arg f "$1" --arg v "$2" '.[$f] = $v' "$JOURNAL" > "$tmp" \
        && mv -f "$tmp" "$JOURNAL"
}

cleanup_dryrun_journal() {
    if [ "$DRY_RUN" = "1" ] && [ -n "$JOURNAL" ] && [ -f "$JOURNAL" ]; then
        rm -f "$JOURNAL"
    fi
}
trap cleanup_dryrun_journal EXIT

# ---- Run a sub-step, propagate failure ------------------------------------

run_step() {
    local label="$1"; shift
    log "step: $label"
    local rc
    if "$@"; then
        rc=0
        journal_step "$label" "ok"
    else
        rc=$?
        journal_step "$label" "fail" "exit=$rc"
        log "FAIL: step '$label' exited $rc"
        return $rc
    fi
}

# ---- Initialize journal --------------------------------------------------

init_journal

# ---- Step 1: preflight ----------------------------------------------------

pf_args=("$TARGET")
[ "$DRY_RUN" = "1" ]    && pf_args+=("--dry-run")
[ "$SKIP_TESTS" = "1" ] && pf_args+=("--skip-tests")
if ! run_step "preflight" bash "$PREFLIGHT" "${pf_args[@]}"; then
    log "preflight failed; aborting (no mutations have happened)"
    exit 1
fi

# ---- Step 2: changelog generation ----------------------------------------

cg_args=("$FROM_TAG" "HEAD" "$TARGET")
[ "$DRY_RUN" = "1" ] && cg_args+=("--dry-run")
if ! run_step "changelog-gen" bash "$CHANGELOG_GEN" "${cg_args[@]}"; then
    log "changelog generation failed; aborting"
    exit 1
fi

# ---- Step 3: version bump -------------------------------------------------

vb_args=("$TARGET")
[ "$DRY_RUN" = "1" ] && vb_args+=("--dry-run")
if ! run_step "version-bump" bash "$VERSION_BUMP" "${vb_args[@]}"; then
    log "version bump failed; aborting"
    exit 1
fi

# ---- Step 4: release.sh consistency check (pre-push) ---------------------

if [ "$DRY_RUN" = "1" ]; then
    log "step: release.sh-check (DRY-RUN — skipping; markers not actually bumped)"
    journal_step "release-sh-check" "skipped-dry-run"
else
    if ! run_step "release-sh-check" bash "$RELEASE_SH" "$TARGET"; then
        log "release.sh consistency check failed; aborting before push"
        exit 1
    fi
fi

# ---- Step 5: ship.sh atomic commit+push+gh-release -----------------------

# Build commit message: "release: v<version>" — short, scannable in git log.
COMMIT_MSG="release: v$TARGET"

if [ "$DRY_RUN" = "1" ]; then
    log "step: ship.sh (DRY-RUN — would commit & push & gh release create)"
    log "  commit msg: $COMMIT_MSG"
    journal_step "ship" "skipped-dry-run"
    log ""
    log "DRY RUN COMPLETE — no mutations were made."
    log "  preflight: simulated"
    log "  changelog-gen: simulated"
    log "  version-bump: simulated"
    log "  release-sh-check: skipped (markers unchanged in dry-run)"
    log "  ship: skipped"
    log "  marketplace-poll: skipped"
    exit 0
fi

# Generate release notes from the just-prepended CHANGELOG entry.
# Read everything between `## [<version>]` and the next `## [` heading.
release_notes=$(awk -v ver="$TARGET" '
    BEGIN { in_block = 0 }
    /^## \[/ {
        if (in_block) exit
        if ($0 ~ "\\[" ver "\\]") in_block = 1
        next
    }
    in_block { print }
' "$REPO_ROOT/CHANGELOG.md")

# ship.sh expects EVOLVE_SHIP_RELEASE_NOTES env to trigger gh release create.
log "step: ship.sh"
if EVOLVE_SHIP_RELEASE_NOTES="$release_notes" bash "$SHIP_SH" "$COMMIT_MSG"; then
    journal_step "ship" "ok"
    new_sha=$(git -C "$REPO_ROOT" rev-parse HEAD)
    journal_set_field "commit_sha" "$new_sha"
else
    rc=$?
    journal_step "ship" "fail" "exit=$rc"
    log "FAIL: ship.sh exited $rc — nothing pushed"
    exit 2
fi

# ---- Step 6: marketplace-poll (post-push) ---------------------------------

mp_args=("$TARGET" "--max-wait-s" "$MAX_POLL_WAIT_S")
log "step: marketplace-poll (max_wait=${MAX_POLL_WAIT_S}s)"
# Capture exit code WITHOUT `!` (which always yields 0 in the then-branch).
set +e
bash "$MARKETPLACE_POLL" "${mp_args[@]}"
mp_rc=$?
set -e
if [ "$mp_rc" -ne 0 ]; then
    journal_step "marketplace-poll" "fail" "exit=$mp_rc"
    log "FAIL: marketplace-poll exited $mp_rc"
    if [ "$NO_ROLLBACK" = "1" ]; then
        log "WARN: --no-rollback set; not rolling back. Manual remediation required."
        exit 3
    fi
    log "auto-rolling back v$TARGET..."
    journal_set_field "completed_at" "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    set +e
    bash "$ROLLBACK" "$JOURNAL" --reason "marketplace propagation failed (exit=$mp_rc)"
    rb_rc=$?
    set -e
    if [ "$rb_rc" -eq 0 ]; then
        log "rollback complete"
    else
        log "WARN: rollback exited $rb_rc; check .evolve/release-rollbacks.jsonl"
    fi
    exit 3
fi
journal_step "marketplace-poll" "ok"

# ---- Done -----------------------------------------------------------------

journal_set_field "completed_at" "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
log "DONE: v$TARGET shipped, propagated, and verified"
log "journal: $JOURNAL"
exit 0
