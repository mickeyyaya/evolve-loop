#!/usr/bin/env bash
#
# rollback.sh — Auto-revert a failed release (v8.13.2).
#
# Reads a release-journal JSON written by release-pipeline.sh, then undoes the
# release in three independently-auditable steps:
#
#   1. Delete the GitHub Release (if present): `gh release delete vX.Y.Z`
#   2. Delete the tag from origin: `git push origin :refs/tags/vX.Y.Z`
#   3. Create a revert commit and push it via:
#        EVOLVE_BYPASS_SHIP_VERIFY=1 bash scripts/ship.sh "revert: ..."
#      The bypass is REQUIRED because the original audit was bound to the
#      now-reverted HEAD + tree_state_sha; ship.sh would refuse without it.
#      The bypass is logged as `[rollback] BYPASS_VERIFY` so the deviation is
#      visible in operator logs.
#
# Each step is logged to .evolve/release-rollbacks.jsonl (NDJSON ledger) for
# audit trail.
#
# Usage:
#   bash scripts/release/rollback.sh <journal.json> [--reason "..."] [--dry-run]
#
# Journal schema (written by release-pipeline.sh):
#   {
#     "version": "8.13.2",
#     "tag": "v8.13.2",
#     "commit_sha": "<sha>",
#     "branch": "main",
#     "release_url": "https://github.com/.../v8.13.2",
#     "started_at": "<iso>",
#     "completed_at": "<iso>"
#   }
#
# Exit codes:
#   0 — rollback succeeded (all 3 steps OR --dry-run completed)
#   1 — rollback partially failed (some step did not complete; ledger entry
#       still written so operator can resume manually)
#   2 — journal not found / malformed
#  10 — invalid arguments

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROLLBACK_LEDGER="$REPO_ROOT/.evolve/release-rollbacks.jsonl"
SHIP_SH="$REPO_ROOT/scripts/ship.sh"

log()  { echo "[rollback] $*" >&2; }
fail() { log "FAIL: $*"; exit 2; }

# ---- Args -----------------------------------------------------------------

DRY_RUN=0
REASON="release-pipeline failure"
JOURNAL=""

while [ $# -gt 0 ]; do
    case "$1" in
        --reason)  shift; REASON="$1" ;;
        --dry-run) DRY_RUN=1 ;;
        --help|-h) sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) log "unknown flag: $1"; exit 10 ;;
        *)
            if [ -z "$JOURNAL" ]; then JOURNAL="$1"
            else log "extra positional arg: $1"; exit 10
            fi ;;
    esac
    shift
done

[ -n "$JOURNAL" ] || { log "usage: rollback.sh <journal.json> [--reason \"...\"] [--dry-run]"; exit 10; }

# ---- Read journal ---------------------------------------------------------

[ -f "$JOURNAL" ] || fail "journal not found: $JOURNAL"
command -v jq >/dev/null 2>&1 || fail "jq required"

VERSION=$(jq -r '.version // empty' "$JOURNAL" 2>/dev/null) || fail "journal malformed: $JOURNAL"
TAG=$(jq -r '.tag // empty' "$JOURNAL")
COMMIT_SHA=$(jq -r '.commit_sha // empty' "$JOURNAL")
BRANCH=$(jq -r '.branch // empty' "$JOURNAL")

[ -n "$VERSION" ]    || fail "journal missing 'version': $JOURNAL"
[ -n "$TAG" ]        || fail "journal missing 'tag': $JOURNAL"
[ -n "$COMMIT_SHA" ] || fail "journal missing 'commit_sha': $JOURNAL"
[ -n "$BRANCH" ]     || fail "journal missing 'branch': $JOURNAL"

log "rolling back v$VERSION ($TAG @ $COMMIT_SHA on $BRANCH)"
log "reason: $REASON"

# ---- Helpers --------------------------------------------------------------

now_iso() { date -u +"%Y-%m-%dT%H:%M:%SZ"; }

append_ledger() {
    # NDJSON; atomic append per posix.
    local entry="$1"
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would append to ledger: $entry"
        return 0
    fi
    mkdir -p "$(dirname "$ROLLBACK_LEDGER")"
    printf '%s\n' "$entry" >> "$ROLLBACK_LEDGER"
}

dry_or_run() {
    local label="$1"; shift
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would $label"
        log "  command: $*"
        return 0
    fi
    log "$label"
    "$@"
}

# ---- Step 1: delete GitHub Release ----------------------------------------

step1_status="skipped"
if command -v gh >/dev/null 2>&1; then
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would gh release delete $TAG --yes"
        step1_status="dry-run-ok"
    else
        if gh release view "$TAG" >/dev/null 2>&1; then
            if gh release delete "$TAG" --yes >/dev/null 2>&1; then
                log "OK: deleted GitHub release $TAG"
                step1_status="deleted"
            else
                log "WARN: gh release delete $TAG failed (release may have already been removed)"
                step1_status="failed"
            fi
        else
            log "INFO: no GitHub release for $TAG (skipping)"
            step1_status="not-present"
        fi
    fi
else
    log "WARN: gh CLI not installed; skipping release deletion"
fi

# ---- Step 2: delete remote tag --------------------------------------------

step2_status="skipped"
if [ "$DRY_RUN" = "1" ]; then
    log "DRY-RUN: would git push origin :refs/tags/$TAG"
    step2_status="dry-run-ok"
else
    if git -C "$REPO_ROOT" ls-remote --tags origin "refs/tags/$TAG" 2>/dev/null | grep -q "$TAG"; then
        if git -C "$REPO_ROOT" push origin ":refs/tags/$TAG" >/dev/null 2>&1; then
            log "OK: deleted remote tag $TAG"
            step2_status="deleted"
        else
            log "WARN: git push origin :refs/tags/$TAG failed"
            step2_status="failed"
        fi
    else
        log "INFO: no remote tag $TAG (skipping)"
        step2_status="not-present"
    fi
    # Also delete local tag if it exists.
    git -C "$REPO_ROOT" tag -d "$TAG" >/dev/null 2>&1 || true
fi

# ---- Step 3: create revert commit and push it via ship.sh -----------------

step3_status="skipped"
if [ "$DRY_RUN" = "1" ]; then
    log "DRY-RUN: would git revert --no-edit $COMMIT_SHA"
    log "DRY-RUN: would EVOLVE_BYPASS_SHIP_VERIFY=1 bash scripts/ship.sh \"revert: $REASON [rollback of v$VERSION]\""
    step3_status="dry-run-ok"
else
    revert_msg="revert: $REASON [rollback of v$VERSION]"
    revert_err=$(git -C "$REPO_ROOT" revert --no-edit "$COMMIT_SHA" 2>&1)
    revert_rc=$?
    if [ "$revert_rc" -eq 0 ]; then
        log "OK: created revert commit"
        log "BYPASS_VERIFY for ship.sh push of revert commit"
        if EVOLVE_BYPASS_SHIP_VERIFY=1 bash "$SHIP_SH" "$revert_msg" >/dev/null 2>&1; then
            log "OK: pushed revert commit"
            step3_status="reverted"
        else
            log "WARN: ship.sh failed for revert push — local revert commit was made; manual push needed"
            step3_status="local-only"
        fi
    else
        log "WARN: git revert failed (rc=$revert_rc): $revert_err"
        step3_status="failed"
    fi
fi

# ---- Append ledger entry --------------------------------------------------

ledger_entry=$(jq -nc \
    --arg ts "$(now_iso)" \
    --arg version "$VERSION" \
    --arg tag "$TAG" \
    --arg sha "$COMMIT_SHA" \
    --arg reason "$REASON" \
    --arg s1 "$step1_status" \
    --arg s2 "$step2_status" \
    --arg s3 "$step3_status" \
    --arg dry "$DRY_RUN" \
    '{timestamp: $ts, version: $version, tag: $tag, commit_sha: $sha, reason: $reason,
      release_delete: $s1, tag_delete: $s2, revert: $s3, dry_run: ($dry == "1")}')

append_ledger "$ledger_entry"

# ---- Final exit code ------------------------------------------------------

if [ "$DRY_RUN" = "1" ]; then
    log "DONE: dry-run complete for v$VERSION"
    exit 0
fi

# Determine overall success. ALL three steps must have either succeeded or
# been legitimately skipped (not-present / not-installed). Any explicit
# "failed" status means a step encountered an error that requires manual
# remediation — surface as exit 1 so operators investigate.
# Audit cycle 8202 MEDIUM-1 fix: previously this only checked step3.
if [ "$step3_status" = "reverted" ] \
   && [ "$step1_status" != "failed" ] \
   && [ "$step2_status" != "failed" ]; then
    log "DONE: rollback complete for v$VERSION (release_delete=$step1_status, tag_delete=$step2_status, revert=$step3_status)"
    exit 0
else
    log "PARTIAL: rollback incomplete (release_delete=$step1_status, tag_delete=$step2_status, revert=$step3_status)"
    log "Manual remediation may be needed; ledger entry written to $ROLLBACK_LEDGER"
    exit 1
fi
