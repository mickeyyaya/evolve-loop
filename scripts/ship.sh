#!/usr/bin/env bash
#
# ship.sh — Canonical atomic shipper for the evolve-loop project.
#
# This is the ONLY script the v8.13.0 ship-gate hook allowlists for git
# commit / git push / gh release create operations. It enforces the audit-
# first contract before doing any git work and runs all git operations
# atomically as a single Bash invocation from the orchestrator's perspective.
#
# Why this exists: cycles 8121, 8122 audits identified that a parser-based
# ship-gate that tries to detect ship-class commands inside arbitrary bash
# always loses the arms race (D1 bare-newline, D2 pipe-to-shell, D3
# here-string bypasses kept emerging). v8.13.0 reframes: instead of a smart
# parser, the gate allowlists exactly one canonical path (this script).
# ship.sh enforces the audit-first contract internally; the gate just checks
# "is this script the entry point?".
#
# Usage:
#   bash scripts/ship.sh "<commit-message>"                   # default --class cycle
#   bash scripts/ship.sh --class manual "<commit-message>"    # interactive confirm
#   bash scripts/ship.sh --class release "<commit-message>"   # release-pipeline only
#
# Commit classes (v8.25.0+):
#   cycle    (default) — Audit-bound. Most recent Auditor entry must be PASS,
#                        SHAs must match, HEAD/tree must be cycle-bound. This
#                        is the integrity-critical path used by /evolve-loop.
#   manual   — Operator-driven commit (manual feature work, hot-fix). Skips
#              audit verification but REQUIRES interactive y/N confirmation
#              after printing `git diff --cached --stat`. Refuses if stdin is
#              not a tty (an LLM agent cannot answer the prompt — that's the
#              boundary). Replaces the EVOLVE_BYPASS_SHIP_VERIFY=1 escape
#              hatch with an auditable lifecycle.
#   release  — Internal use by scripts/release-pipeline.sh. Skips audit
#              verification because version-bump.sh mutates files
#              post-audit. Logs RELEASE class loudly. NOT for general use.
#
# Environment overrides:
#   EVOLVE_SHIP_RELEASE_NOTES — if set, also creates a GitHub release tagged
#                               v<VERSION> with these notes after a successful
#                               commit + push. VERSION is read from
#                               .claude-plugin/plugin.json.
#   EVOLVE_BYPASS_SHIP_VERIFY — DEPRECATED in v8.25.0. Equivalent to --class
#                               manual but without the interactive prompt.
#                               Continues to work but emits a deprecation
#                               warning and treats commit as `manual` class
#                               with an "auto-confirmed" provenance marker.
#                               Migrate to `--class manual` (interactive) or
#                               `--class release` (pipeline-internal).
#   EVOLVE_SHIP_AUTO_CONFIRM  — if "1", skip the interactive y/N prompt for
#                               --class manual (CI use). Equivalent in effect
#                               to EVOLVE_BYPASS_SHIP_VERIFY but explicitly
#                               scoped to the manual class. Logged.
#
# Exit codes:
#   0  — shipped successfully
#   1  — runtime failure (missing arg, missing tools, git failure)
#   2  — integrity failure (no audit, audit not PASS, SHA mismatch, HEAD
#         moved since audit, ship.sh self-SHA mismatch, manual confirm denied)
# 127  — required binary missing (jq, git, gh)

set -euo pipefail

# v8.18.0: dual-root. git operations target the project repo (where the cycle's
# changes live); ledger/state are writable artifacts in the same project tree.
# REPO_ROOT alias is kept for the many existing references below — it points
# to PROJECT_ROOT (writable side, where git ops happen).
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/resolve-roots.sh"
unset __rr_self
REPO_ROOT="$EVOLVE_PROJECT_ROOT"
LEDGER="$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl"
STATE="$EVOLVE_PROJECT_ROOT/.evolve/state.json"

log()           { echo "[ship] $*" >&2; }
fail()          { log "FAIL: $*"; exit 1; }
integrity_fail(){ log "INTEGRITY-FAIL: $*"; exit 2; }

# --- 0. Pre-flight -----------------------------------------------------------

# v8.25.0: Parse --class flag (and any future flags) before the positional
# commit message. Default class is "cycle" (audit-bound) for backward compat.
SHIP_CLASS="cycle"
COMMIT_MSG=""

while [ $# -gt 0 ]; do
    case "$1" in
        --class)
            shift
            [ $# -ge 1 ] || fail "--class requires a value (cycle|manual|release)"
            SHIP_CLASS="$1"
            shift
            ;;
        --help|-h)
            sed -n '2,52p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --*)
            fail "unknown flag: $1"
            ;;
        *)
            if [ -z "$COMMIT_MSG" ]; then
                COMMIT_MSG="$1"
                shift
            else
                fail "extra positional arg: $1 (only one commit message expected)"
            fi
            ;;
    esac
done

[ -n "$COMMIT_MSG" ] || fail "usage: ship.sh [--class cycle|manual|release] <commit-message>"

case "$SHIP_CLASS" in
    cycle|manual|release) ;;
    *) fail "invalid --class '$SHIP_CLASS' (must be: cycle|manual|release)" ;;
esac

# v8.25.0: Translate legacy EVOLVE_BYPASS_SHIP_VERIFY=1 to --class manual with
# auto-confirm. This is the deprecation bridge: existing scripts/cron jobs
# using the env var continue to work but emit a one-time warning.
if [ "${EVOLVE_BYPASS_SHIP_VERIFY:-0}" = "1" ]; then
    if [ "$SHIP_CLASS" = "cycle" ]; then
        log "DEPRECATION: EVOLVE_BYPASS_SHIP_VERIFY=1 is deprecated in v8.25.0+"
        log "  → Migrate to: bash scripts/ship.sh --class manual \"<msg>\""
        log "  → Or for CI:  EVOLVE_SHIP_AUTO_CONFIRM=1 bash scripts/ship.sh --class manual \"<msg>\""
        log "  → Treating this invocation as: --class manual + EVOLVE_SHIP_AUTO_CONFIRM=1"
        SHIP_CLASS="manual"
        export EVOLVE_SHIP_AUTO_CONFIRM=1
    fi
fi

for bin in git jq; do
    command -v "$bin" >/dev/null 2>&1 || { log "missing required binary: $bin"; exit 127; }
done

cd "$REPO_ROOT"

# Determine sha256 binary.
if command -v sha256sum >/dev/null 2>&1; then
    SHA256() { sha256sum "$@" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
    SHA256() { shasum -a 256 "$@" | awk '{print $1}'; }
else
    log "missing sha256sum and shasum"; exit 127
fi
sha256_stdin() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}';
    else shasum -a 256 | awk '{print $1}'; fi
}

# --- 1. ship.sh self-SHA verification (version-aware TOFU, v8.32.0) ---------
#
# Defends against an attacker (or a confused LLM) editing ship.sh to bypass
# its own checks. Pre-v8.32.0 used naïve TOFU: pin the SHA on first run; any
# subsequent change → INTEGRITY-FAIL.
#
# THE TRAP pre-v8.32.0 fixed: every plugin update ships a new ship.sh with
# different content → SHA changes → pin no longer matches → cycles fail at
# ship-gate until operator manually deletes state.json:expected_ship_sha.
# This was friction without proportional protection — local tampering of
# ship.sh is a weak threat (attacker with write access to ~/.claude/plugins/
# can also tamper with role-gate.sh, ship-gate.sh, profiles, etc., none of
# which had TOFU).
#
# v8.32.0 makes TOFU VERSION-AWARE: pin BOTH the SHA AND the plugin version
# (read from $EVOLVE_PLUGIN_ROOT/.claude-plugin/plugin.json). On verification:
#   - Same version + same SHA → clean (pass)
#   - Same version + different SHA → REAL TAMPERING (fail)
#   - Different version (plugin updated) → re-pin both, log loudly, continue
#   - No pinned version (legacy state.json) → migrate: re-pin both
#
# Within-version SHA changes still trip the integrity check (the threat
# model that matters). Cross-version SHA changes auto-recover (the false-
# positive that tortured operators every release).

ACTUAL_SHIP_SHA=$(SHA256 "${BASH_SOURCE[0]}")
PLUGIN_VERSION=""
if [ -f "$EVOLVE_PLUGIN_ROOT/.claude-plugin/plugin.json" ]; then
    PLUGIN_VERSION=$(jq -r '.version // empty' "$EVOLVE_PLUGIN_ROOT/.claude-plugin/plugin.json" 2>/dev/null || echo "")
fi
EXPECTED_SHIP_SHA=""
EXPECTED_SHIP_VERSION=""
if [ -f "$STATE" ]; then
    EXPECTED_SHIP_SHA=$(jq -r '.expected_ship_sha // empty' "$STATE" 2>/dev/null)
    EXPECTED_SHIP_VERSION=$(jq -r '.expected_ship_version // empty' "$STATE" 2>/dev/null)
fi

# Reusable pinning helper (DRY: first-run, version-bump, and migration paths).
_repin_ship_sha() {
    local reason="$1"
    if [ ! -f "$STATE" ]; then
        echo '{}' > "$STATE"
    fi
    local _tmp; _tmp=$(mktemp)
    jq --arg sha "$ACTUAL_SHIP_SHA" --arg ver "$PLUGIN_VERSION" \
       '. + {expected_ship_sha: $sha, expected_ship_version: $ver}' "$STATE" > "$_tmp" \
       && mv "$_tmp" "$STATE"
    log "TOFU: $reason — pinned ship.sh SHA + plugin version='$PLUGIN_VERSION'"
}

if [ -z "$EXPECTED_SHIP_SHA" ]; then
    # First run — pin both SHA and version.
    _repin_ship_sha "first run"
elif [ "$EXPECTED_SHIP_SHA" = "$ACTUAL_SHIP_SHA" ]; then
    # SHA matches; ensure pinned version is current (auto-update on first
    # run after v8.32.0 if SHA happened not to change).
    if [ -z "$EXPECTED_SHIP_VERSION" ] && [ -n "$PLUGIN_VERSION" ]; then
        _repin_ship_sha "schema migration (no expected_ship_version recorded)"
    fi
elif [ -z "$EXPECTED_SHIP_VERSION" ]; then
    # Legacy state.json (pre-v8.32.0): SHA-only pin, no version recorded.
    # Auto-migrate: re-pin with current SHA + version. This is the path
    # that unblocks operators stuck on a stale pin from before v8.32.0.
    _repin_ship_sha "migrating legacy SHA-only pin to version-aware schema"
elif [ "$PLUGIN_VERSION" != "$EXPECTED_SHIP_VERSION" ]; then
    # Plugin version changed (update or downgrade). Treat as plugin-managed
    # update; re-pin SHA. Real tampering would have to also forge plugin.json.
    _repin_ship_sha "plugin version changed: '$EXPECTED_SHIP_VERSION' → '$PLUGIN_VERSION'"
else
    # Same version, different SHA — REAL local tampering.
    integrity_fail "ship.sh has been modified WITHIN plugin version $PLUGIN_VERSION (expected=$EXPECTED_SHIP_SHA actual=$ACTUAL_SHIP_SHA). This indicates real local tampering or plugin install corruption. To intentionally update: remove .evolve/state.json:expected_ship_sha and re-run."
fi

# --- 2. Class-aware verification (v8.25.0+) ----------------------------------
#
# Three commit classes determine which checks run:
#   cycle   → full audit-binding (the default, for /evolve-loop cycle commits)
#   manual  → interactive y/N confirmation (operator-driven manual commits)
#   release → no audit (release-pipeline mutates post-audit; logs RELEASE)
#
# The class is logged in the commit message footer so the lifecycle is
# auditable from `git log` alone — no need to consult ledger.jsonl to know
# whether a given commit was cycle-bound or manual.

CLASS_PROVENANCE=""

case "$SHIP_CLASS" in
    cycle)
        log "class: cycle (audit-bound)"
        CLASS_PROVENANCE="cycle (audit-verified)"
        ;;
    manual)
        log "class: manual (operator-driven)"
        # Stage everything first so `git diff --cached --stat` reflects what
        # will actually ship.
        git add -A
        if git diff --cached --quiet; then
            log "no staged changes; nothing to ship"
            exit 0
        fi
        echo "" >&2
        echo "=== git diff --cached --stat ===" >&2
        git diff --cached --stat >&2
        echo "" >&2
        echo "=== git diff --cached (first 80 lines) ===" >&2
        # `git diff | head -80` fires SIGPIPE on git when head closes; under
        # `set -euo pipefail` that aborts the script. Capture into a temp,
        # truncate manually, and emit. Avoids both the SIGPIPE and the
        # pipefail-vs-SIGPIPE interaction.
        diff_tmp=$(mktemp -t ship-diff.XXXXXX)
        git diff --cached > "$diff_tmp" 2>/dev/null || true
        awk 'NR<=80 {print} NR==81 {print "  ... (diff truncated; see git diff --cached for full)"; exit}' "$diff_tmp" >&2
        rm -f "$diff_tmp"
        echo "" >&2

        if [ "${EVOLVE_SHIP_AUTO_CONFIRM:-0}" = "1" ]; then
            log "EVOLVE_SHIP_AUTO_CONFIRM=1 — skipping interactive prompt (CI mode)"
            CLASS_PROVENANCE="manual (auto-confirmed via env)"
        else
            # Interactive boundary: an LLM agent cannot read the prompt.
            # If stdin is not a tty, refuse — this is the security boundary
            # that distinguishes manual class from a silent bypass.
            if [ ! -t 0 ]; then
                integrity_fail "--class manual requires interactive stdin (not a tty). Set EVOLVE_SHIP_AUTO_CONFIRM=1 for non-interactive use (CI), or run from a real terminal."
            fi
            printf '[ship] Confirm manual commit? Type EXACTLY "yes" to ship, anything else aborts: ' >&2
            read -r confirm
            if [ "$confirm" != "yes" ]; then
                log "manual confirmation declined — aborting"
                exit 2
            fi
            CLASS_PROVENANCE="manual (interactive-confirmed)"
        fi
        ;;
    release)
        log "class: release (pipeline-internal)"
        log "  → audit verification skipped: version-bump.sh mutates files post-audit"
        log "  → this commit must be created by scripts/release-pipeline.sh only"
        CLASS_PROVENANCE="release (pipeline-generated)"
        ;;
esac
log "provenance: $CLASS_PROVENANCE"

# Audit-binding only runs for cycle class; manual/release skip to section 7.
if [ "$SHIP_CLASS" = "cycle" ]; then
    # --- 3. Locate the most recent Auditor ledger entry --------------------------

    [ -f "$LEDGER" ] || integrity_fail "no ledger at $LEDGER — no Auditor has ever run"

    # `grep` on a ledger with no matching entries returns rc=1, which under
    # `set -e -o pipefail` would trip a script-exit before we get to the
    # explicit "no Auditor entry" check below. Wrap with `|| true` to defer
    # the missing-entry decision to the integrity_fail call.
    LATEST_AUDIT=$( { grep '"kind":"agent_subprocess"' "$LEDGER" 2>/dev/null || true; } \
        | jq -c 'select(.role == "auditor")' 2>/dev/null \
        | tail -1 )

    [ -n "$LATEST_AUDIT" ] || integrity_fail "no Auditor ledger entry found — independent review missing"

    # --- 4. Verify Auditor exit_code, artifact existence + SHA, verdict --------

    EXIT_CODE=$(echo "$LATEST_AUDIT" | jq -r '.exit_code')
    ARTIFACT_PATH=$(echo "$LATEST_AUDIT" | jq -r '.artifact_path')
    RECORDED_SHA=$(echo "$LATEST_AUDIT" | jq -r '.artifact_sha256')

    # v8.27.0: relaxed exit_code check.
    #
    # The auditor CLI follows Unix convention:
    #   exit 0 — no findings (cleanest possible audit)
    #   exit 1 — findings present (NORMAL — every realistic audit has findings)
    #   exit 2+ — true error (crash, signal, malformed invocation)
    #
    # Pre-v8.27.0, ship-gate rejected ANY non-zero exit, treating "findings
    # present" as integrity failure. This conflicted with the auditor's
    # actual semantics and produced a deterministic deadlock: every cycle
    # with findings (i.e., every realistic cycle) failed to ship even when
    # the artifact explicitly declared Verdict: PASS. The downstream user's
    # report (cycle-25 evidence): 5+ blocked invocations, 0 successful auto-
    # commits, kernel state locked at 4 infrastructure-systemic entries.
    #
    # The artifact-content checks below (SHA + Verdict text) are the actual
    # source of truth. They run regardless of exit_code. Anti-gaming is
    # preserved by SHA verification (artifact wasn't mutated post-audit) +
    # Verdict text grep (audit explicitly declared PASS). Cycle 102-111 /
    # 132-141 / Gemini-forgery were caught by these, not by the exit_code.
    case "$EXIT_CODE" in
        0|1) ;;
        *)   integrity_fail "most recent Auditor exited $EXIT_CODE (error state — not a Unix-convention findings signal)" ;;
    esac
    [ -f "$ARTIFACT_PATH" ] || integrity_fail "audit-report.md missing on disk: $ARTIFACT_PATH"

    ACTUAL_SHA=$(SHA256 "$ARTIFACT_PATH")
    [ "$ACTUAL_SHA" = "$RECORDED_SHA" ] || integrity_fail "audit-report.md SHA mismatch (ledger=$RECORDED_SHA actual=$ACTUAL_SHA) — artifact mutated post-audit"

    # v8.28.0: Verdict semantics relaxed to fluent-by-default.
    #
    # PASS  → ship (always)
    # WARN  → ship by default. Auditor expressed concerns but proceed.
    #         Operator opts back to strict blocking via EVOLVE_STRICT_AUDIT=1.
    # FAIL  → block. Auditor said "do not ship".
    #
    # Rationale: WARN typically means "minor findings to address in next
    # cycle". Pre-v8.28.0 treated WARN like FAIL, preventing iterative
    # improvement. The downstream user's analysis: "if we never accept WARN,
    # the audit-fail count climbs forever and the loop deadlocks."
    #
    # Anti-gaming preserved: the auditor still distinguishes PASS/WARN/FAIL
    # by writing different verdict text. The kernel can't prevent the LLM
    # from writing "PASS" when it should be "FAIL" — that's not a structural
    # threat, it's a model-quality concern. Tier-1 hooks (SHA, cycle binding,
    # ship-gate atomicity) still prevent FAKERY (orchestrator can't pretend
    # the auditor ran when it didn't).
    #
    # FAIL detection is FIRST (priority): if FAIL is declared, block
    # regardless of any PASS/WARN that might also appear (defensive against
    # malformed reports that include both).
    AUDIT_VERDICT_RAW="$(cat "$ARTIFACT_PATH")"
    has_fail() {
        echo "$AUDIT_VERDICT_RAW" | grep -qiE 'Verdict[[:space:]]*:[[:space:]]*\*?\*?[[:space:]]*FAIL([[:space:]]|$|\*)' \
            || echo "$AUDIT_VERDICT_RAW" | awk '
                /^#+[[:space:]]+Verdict[[:space:]]*$/ { saw=NR; next }
                saw && (NR - saw) <= 5 && /\*\*FAIL\*\*/ { found=1; exit }
                END { exit !found }
              '
    }
    has_pass() {
        echo "$AUDIT_VERDICT_RAW" | grep -qiE 'Verdict[[:space:]]*:[[:space:]]*\*?\*?[[:space:]]*PASS([[:space:]]|$|\*)' \
            || echo "$AUDIT_VERDICT_RAW" | awk '
                /^#+[[:space:]]+Verdict[[:space:]]*$/ { saw=NR; next }
                saw && (NR - saw) <= 5 && /\*\*PASS\*\*/ { found=1; exit }
                END { exit !found }
              '
    }
    has_warn() {
        echo "$AUDIT_VERDICT_RAW" | grep -qiE 'Verdict[[:space:]]*:[[:space:]]*\*?\*?[[:space:]]*WARN([[:space:]]|$|\*)' \
            || echo "$AUDIT_VERDICT_RAW" | awk '
                /^#+[[:space:]]+Verdict[[:space:]]*$/ { saw=NR; next }
                saw && (NR - saw) <= 5 && /\*\*WARN\*\*/ { found=1; exit }
                END { exit !found }
              '
    }

    # v8.30.0: detect dual-verdict (auditor wrote both PASS and FAIL).
    # Real audit-report observed in cycle-25: header "## Verdict\n**FAIL**"
    # with per-eval section showing all 4 evals PASS. Pre-v8.30.0, FAIL
    # detection would block (via the priority order below). v8.30.0 detects
    # the inconsistency and reports it as a malformed-artifact integrity
    # failure — auditor should commit to a single verdict per artifact.
    if has_fail && has_pass; then
        integrity_fail "audit-report.md declares BOTH 'Verdict: FAIL' AND 'Verdict: PASS' — auditor produced an inconsistent artifact. Re-run audit, or split into separate Verdict and per-eval-result sections."
    fi
    if has_fail; then
        integrity_fail "audit-report.md declares 'Verdict: FAIL' — auditor explicitly rejected this build"
    elif has_pass; then
        : # ship — clean PASS
    elif has_warn; then
        if [ "${EVOLVE_STRICT_AUDIT:-0}" = "1" ]; then
            integrity_fail "audit-report.md declares 'Verdict: WARN' and EVOLVE_STRICT_AUDIT=1 — strict mode rejects WARN"
        fi
        log "audit verdict: WARN — shipping per fluent-by-default policy (set EVOLVE_STRICT_AUDIT=1 to block on WARN)"
    else
        integrity_fail "audit-report.md declares no recognizable verdict (PASS/WARN/FAIL) — auditor output malformed"
    fi
    unset AUDIT_VERDICT_RAW

    # --- 5. Cycle binding: current state must match audited state -------------

    LEDGER_HEAD=$(echo "$LATEST_AUDIT" | jq -r '.git_head // empty')
    LEDGER_TREE=$(echo "$LATEST_AUDIT" | jq -r '.tree_state_sha // empty')

    if [ -z "$LEDGER_HEAD" ] || [ -z "$LEDGER_TREE" ]; then
        integrity_fail "Auditor ledger entry predates v8.13.0 cycle-binding (no git_head/tree_state_sha) — re-run audit"
    fi

    CURRENT_HEAD=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
    CURRENT_TREE=$(git diff HEAD 2>/dev/null | sha256_stdin)

    if [ "$CURRENT_HEAD" != "$LEDGER_HEAD" ]; then
        integrity_fail "git HEAD has moved since audit (audited=$LEDGER_HEAD current=$CURRENT_HEAD) — re-run Auditor on the new state"
    fi
    if [ "$CURRENT_TREE" != "$LEDGER_TREE" ]; then
        integrity_fail "uncommitted changes have been added since audit (tree-state mismatch) — re-run Auditor"
    fi

    # --- 6. Audit freshness (7d cap when cycle-binding present, else 24h) ----

    if [[ "$OSTYPE" == "darwin"* ]]; then
        ARTIFACT_MTIME=$(stat -f %m "$ARTIFACT_PATH")
    else
        ARTIFACT_MTIME=$(stat -c %Y "$ARTIFACT_PATH")
    fi
    ARTIFACT_AGE_S=$(( $(date +%s) - ARTIFACT_MTIME ))
    MAX_AGE_S=$((7 * 24 * 3600))   # 7 days when cycle-bound
    if [ "$ARTIFACT_AGE_S" -gt "$MAX_AGE_S" ]; then
        integrity_fail "audit-report.md is ${ARTIFACT_AGE_S}s old (>${MAX_AGE_S}s); re-run Auditor"
    fi

    log "OK: audit verified — verdict PASS, SHA matches, HEAD/tree bound to audit, age ${ARTIFACT_AGE_S}s"
fi

# --- 7. Atomic ship: stage + commit + push (+ optional release) -------------
#
# All git operations happen inside this single Bash invocation, so the
# orchestrator session sees one allowed bash call (bash scripts/ship.sh ...)
# and the inner git operations don't fire the gate individually. Solves the
# v8.12.x D6 workflow regression.

# Determine current branch (block accidental ship from detached HEAD).
CURRENT_BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || echo "")
[ -n "$CURRENT_BRANCH" ] || fail "detached HEAD — refuse to ship; checkout a branch first"

# Stage everything (intentional — audit was based on full diff HEAD).
git add -A

# If nothing to commit, log and exit cleanly.
if git diff --cached --quiet; then
    log "no staged changes to ship; exiting cleanly (audit was for an empty diff)"
    exit 0
fi

# Commit. Use array form to avoid arg injection.
COMMIT_ARGS=(commit -m "$COMMIT_MSG")
git "${COMMIT_ARGS[@]}"
log "OK: committed to $CURRENT_BRANCH"

# Push.
git push origin "$CURRENT_BRANCH"
log "OK: pushed to origin/$CURRENT_BRANCH"

# --- 8. Optional GitHub release (if EVOLVE_SHIP_RELEASE_NOTES set) -----------

if [ -n "${EVOLVE_SHIP_RELEASE_NOTES:-}" ]; then
    # v8.18.0: plugin.json is a read-only plugin resource, never a user-project
    # file. Must use EVOLVE_PLUGIN_ROOT, not the REPO_ROOT compat alias which
    # points at PROJECT_ROOT.
    PLUGIN_JSON="$EVOLVE_PLUGIN_ROOT/.claude-plugin/plugin.json"
    if [ ! -f "$PLUGIN_JSON" ]; then
        log "WARN: no .claude-plugin/plugin.json — skipping release"
    else
        VERSION=$(jq -r '.version' "$PLUGIN_JSON")
        TAG="v${VERSION}"
        if command -v gh >/dev/null 2>&1; then
            log "creating GitHub release $TAG..."
            # gh release create must be done as the same atomic event from the
            # gate's perspective. It runs after push so the tag points at the
            # just-pushed commit.
            if echo "$EVOLVE_SHIP_RELEASE_NOTES" | gh release create "$TAG" --title "$TAG" --notes-file - 2>&1; then
                log "OK: GitHub release $TAG created"
            else
                log "WARN: gh release create failed (release may already exist)"
            fi
        else
            log "WARN: gh CLI not available — skipping release"
        fi
    fi
fi

log "DONE: shipped $CURRENT_BRANCH at $(git rev-parse HEAD)"
exit 0
