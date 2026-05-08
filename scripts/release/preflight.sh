#!/usr/bin/env bash
#
# preflight.sh — Pre-flight gate for the v8.13.2 release pipeline.
#
# Verifies the local environment is ready for a release before any mutating
# step runs. Fails fast with a clear message; never modifies anything.
#
# Usage:
#   bash scripts/release/preflight.sh <target-version> [--dry-run] [--skip-tests]
#
# Checks (in order — first failure is fatal):
#   1. Working tree clean (no unstaged or staged modifications).
#   2. Branch not detached (CURRENT_BRANCH is non-empty).
#   3. <target-version> parses as semver and is > current plugin.json version.
#   4. Auditor ledger has a recent (<7 days) PASS verdict for HEAD.
#   5. All four gate-test suites pass: guards-test, ship-integration-test,
#      role-gate-test, phase-gate-precondition-test.
#
# Flags:
#   --dry-run     — print "would check X" for each step but never execute.
#                   Useful for "what would this pipeline check?" inspection.
#   --skip-tests  — skip step 5 (gate-test suites). Reserved for hot-fix flows
#                   where tests have already run in CI; logged WARN.
#
# Exit codes:
#   0 — all checks pass; pipeline may proceed.
#   1 — some check failed (message on stderr explains which).
#  10 — invalid arguments.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PLUGIN_JSON="$REPO_ROOT/.claude-plugin/plugin.json"
LEDGER="$REPO_ROOT/.evolve/ledger.jsonl"
MAX_AUDIT_AGE_S=$((7 * 24 * 3600))

log()  { echo "[preflight] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

# ---- Args -----------------------------------------------------------------

DRY_RUN=0
SKIP_TESTS=0
TARGET=""

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run)    DRY_RUN=1 ;;
        --skip-tests) SKIP_TESTS=1 ;;
        --help|-h)
            sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --*) log "unknown flag: $1"; exit 10 ;;
        *)
            if [ -z "$TARGET" ]; then TARGET="$1"
            else log "extra positional arg: $1"; exit 10
            fi
            ;;
    esac
    shift
done

[ -n "$TARGET" ] || { log "usage: preflight.sh <target-version> [--dry-run] [--skip-tests]"; exit 10; }

# ---- Helpers --------------------------------------------------------------

extract_json_version() {
    sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$1" | head -1
}

# Parse "X.Y.Z" → echoes "X Y Z" (space-separated). Returns 1 on bad format.
parse_semver() {
    local v="$1"
    if [[ "$v" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)([+-].*)?$ ]]; then
        echo "${BASH_REMATCH[1]} ${BASH_REMATCH[2]} ${BASH_REMATCH[3]}"
        return 0
    fi
    return 1
}

# semver_gt A B → 0 if A > B, 1 otherwise.
semver_gt() {
    local a b
    a=$(parse_semver "$1") || return 1
    b=$(parse_semver "$2") || return 1
    set -- $a
    local a1=$1 a2=$2 a3=$3
    set -- $b
    local b1=$1 b2=$2 b3=$3
    [ "$a1" -gt "$b1" ] && return 0
    [ "$a1" -lt "$b1" ] && return 1
    [ "$a2" -gt "$b2" ] && return 0
    [ "$a2" -lt "$b2" ] && return 1
    [ "$a3" -gt "$b3" ] && return 0
    return 1
}

run_or_dry() {
    local label="$1"; shift
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would $label"
        return 0
    fi
    "$@"
}

# ---- Step 1: clean working tree -------------------------------------------

step_clean_tree() {
    log "step 1: working tree clean?"
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would check git diff --quiet HEAD"
        return 0
    fi
    if ! git -C "$REPO_ROOT" diff --quiet HEAD 2>/dev/null; then
        fail "working tree has uncommitted changes — commit or stash first"
    fi
    if [ -n "$(git -C "$REPO_ROOT" ls-files --others --exclude-standard 2>/dev/null)" ]; then
        log "WARN: untracked files present (not blocking; ship.sh will git add -A them)"
    fi
    log "OK: working tree clean"
}

# ---- Step 2: not detached HEAD --------------------------------------------

step_branch_attached() {
    log "step 2: branch attached?"
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would check git symbolic-ref --short HEAD"
        return 0
    fi
    local branch
    branch=$(git -C "$REPO_ROOT" symbolic-ref --short HEAD 2>/dev/null || echo "")
    [ -n "$branch" ] || fail "detached HEAD — checkout a branch first"
    log "OK: on branch $branch"
}

# ---- Step 3: target-version is a valid semver bump ------------------------

step_semver_bump() {
    log "step 3: target version $TARGET > current?"
    parse_semver "$TARGET" >/dev/null || fail "target version not semver: $TARGET"
    [ -f "$PLUGIN_JSON" ] || fail "plugin.json missing at $PLUGIN_JSON"
    local current
    current=$(extract_json_version "$PLUGIN_JSON")
    parse_semver "$current" >/dev/null || fail "current plugin.json version not semver: $current"
    if [ "$TARGET" = "$current" ]; then
        fail "target $TARGET equals current $current — nothing to bump"
    fi
    if ! semver_gt "$TARGET" "$current"; then
        fail "target $TARGET is not greater than current $current"
    fi
    log "OK: $current → $TARGET (valid bump)"
}

# ---- Step 4: recent audit ledger PASS -------------------------------------

step_audit_recent() {
    log "step 4: recent auditor PASS verdict?"
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would check $LEDGER for recent auditor PASS"
        return 0
    fi
    [ -f "$LEDGER" ] || fail "no ledger at $LEDGER — no Auditor has ever run"
    command -v jq >/dev/null 2>&1 || fail "jq required"
    # The ledger schema uses {role, ts} — historical preflight.sh searched
    # {agent, timestamp} which never matched any real entry. Three-bug fix
    # in v8.14.0: (1) role not agent, (2) ts not timestamp, (3) markdown-bold
    # PASS verdict (`**Verdict: PASS**`) is the common auditor output but the
    # old regex `^Verdict:` rejected it.
    local latest
    latest=$(grep '"role":"auditor"' "$LEDGER" 2>/dev/null | tail -1 || true)
    [ -n "$latest" ] || fail "no auditor entry in ledger"
    local artifact_path now ts age
    artifact_path=$(echo "$latest" | jq -r '.artifact_path // empty')
    [ -n "$artifact_path" ] || fail "ledger entry missing artifact_path"
    [ -f "$artifact_path" ] || fail "audit artifact missing on disk: $artifact_path"
    # Match ship.sh's accepted formats so `**Verdict: PASS**`, `Verdict: **PASS**`,
    # `Verdict: PASS`, AND heading form `## Verdict\n**PASS**` all qualify.
    # The heading-form fix (commit 8ced03a) was applied to ship.sh; this aligns
    # preflight with that contract.
    if ! { grep -qiE 'Verdict[[:space:]]*:[[:space:]]*\*?\*?[[:space:]]*PASS([[:space:]]|$|\*)' "$artifact_path" \
           || awk '
                /^#+[[:space:]]+([0-9]+\.[[:space:]]+)?Verdict[[:space:]]*$/ { saw=NR; next }
                saw && (NR - saw) <= 5 && /\*\*PASS\*\*/ { found=1; exit }
                END { exit !found }
              ' "$artifact_path"; }; then
        fail "most recent audit-report.md does not declare 'Verdict: PASS' ($artifact_path) — neither inline nor heading form"
    fi
    ts=$(echo "$latest" | jq -r '.ts // empty')
    [ -n "$ts" ] || fail "ledger entry missing ts"
    if command -v gdate >/dev/null 2>&1; then
        ts_s=$(gdate -d "$ts" +%s 2>/dev/null || echo "")
    else
        ts_s=$(date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$ts" +%s 2>/dev/null || date -d "$ts" +%s 2>/dev/null || echo "")
    fi
    now=$(date -u +%s)
    if [ -n "$ts_s" ]; then
        age=$((now - ts_s))
        [ "$age" -lt "$MAX_AUDIT_AGE_S" ] || fail "audit is ${age}s old (>${MAX_AUDIT_AGE_S}s); re-run Auditor"
    fi
    log "OK: latest audit PASS, artifact=$artifact_path"
}

# ---- Step 5: gate-test suites green ---------------------------------------

step_gate_tests() {
    log "step 5: gate-test suites green?"
    if [ "$SKIP_TESTS" = "1" ]; then
        log "WARN: --skip-tests set; skipping gate-test execution"
        return 0
    fi
    if [ "$DRY_RUN" = "1" ]; then
        log "DRY-RUN: would run guards-test, ship-integration-test, role-gate-test, phase-gate-precondition-test"
        return 0
    fi
    local suites=(
        "scripts/tests/guards-test.sh"
        "scripts/tests/ship-integration-test.sh"
        "scripts/tests/role-gate-test.sh"
        "scripts/tests/phase-gate-precondition-test.sh"
    )
    local s
    for s in "${suites[@]}"; do
        log "  running $s..."
        if ! bash "$REPO_ROOT/$s" >/dev/null 2>&1; then
            fail "gate-test suite failed: $s — re-run interactively to inspect"
        fi
    done
    log "OK: all 4 gate-test suites green"
}

# ---- Run ------------------------------------------------------------------

step_clean_tree
step_branch_attached
step_semver_bump
step_audit_recent
step_gate_tests

log "DONE: preflight passed for $TARGET${DRY_RUN:+ (dry-run)}"
exit 0
