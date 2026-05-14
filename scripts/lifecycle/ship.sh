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
#   bash scripts/lifecycle/ship.sh "<commit-message>"                   # default --class cycle
#   bash scripts/lifecycle/ship.sh --class manual "<commit-message>"    # interactive confirm
#   bash scripts/lifecycle/ship.sh --class release "<commit-message>"   # release-pipeline only
#   bash scripts/lifecycle/ship.sh --dry-run "<msg>"                    # simulate, no mutations
#
# --dry-run (v8.50.0+): runs all read-only checks (audit binding, TOFU SHA,
#   sequence) but skips git commit, git push, and gh release create. Writes a
#   .evolve/release-journal/dry-run-<ts>.json preview of the would-be ops. Safe
#   to run at any time. Combine with --class manual/release as needed.
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
integrity_fail(){
    log "INTEGRITY-FAIL: $*"
    # append abnormal event when ARTIFACT_PATH is known (workspace = dirname ARTIFACT_PATH)
    if [ -n "${ARTIFACT_PATH:-}" ]; then
        local _ws; _ws="$(dirname "$ARTIFACT_PATH")"
        local _ts; _ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
        local _det_esc; _det_esc=$(printf '%s' "$*" | sed 's/"/\\"/g')
        printf '{"event_type":"ship-refused","timestamp":"%s","source_phase":"ship","severity":"HIGH","details":"%s","remediation_hint":"Review audit-report.md and re-run auditor if cycle state has changed"}\n' \
            "$_ts" "$_det_esc" >> "$_ws/abnormal-events.jsonl" 2>/dev/null || true
    fi
    exit 2
}

# v8.50.0: --dry-run helpers. dry_log emits a "would" message; dry_skip is the
# semantic guard at every git/gh mutation site. Read-only checks (audit-binding,
# TOFU SHA verification, sequence checks) ALWAYS run regardless of DRY_RUN —
# the goal is to validate the entire pre-mutation pipeline, then halt before
# touching the working tree, the remote, or the GitHub release surface.
dry_log() { [ "${DRY_RUN:-0}" = "1" ] && log "[DRY-RUN] would: $*"; }

# DRY_RUN_OPS is appended to as each would-be mutation is short-circuited. It
# becomes the body of the journal preview written at exit.
DRY_RUN_OPS=""
dry_record() {
    DRY_RUN_OPS="${DRY_RUN_OPS}${DRY_RUN_OPS:+
}$1"
}

# Write the dry-run journal preview. Called before EVERY exit-0 path inside
# ship.sh so even early-exits ("no staged changes") leave evidence of what
# the simulator/operator was attempting. Idempotent — no-op if not DRY_RUN.
write_dry_run_preview() {
    [ "${DRY_RUN:-0}" = "1" ] || return 0
    local journal_dir="$EVOLVE_PROJECT_ROOT/.evolve/release-journal"
    mkdir -p "$journal_dir" 2>/dev/null || true
    local ts; ts=$(date -u +%Y%m%dT%H%M%SZ)
    local preview="$journal_dir/dry-run-${ts}.json"
    local head_sha; head_sha=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
    local branch; branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
    local ops_json="[]"
    if [ -n "$DRY_RUN_OPS" ]; then
        ops_json=$(printf '%s\n' "$DRY_RUN_OPS" | jq -R . | jq -s .)
    fi
    jq -n \
        --arg ts "$ts" \
        --arg class "${SHIP_CLASS:-cycle}" \
        --arg msg "${COMMIT_MSG:-}" \
        --arg branch "$branch" \
        --arg head "$head_sha" \
        --arg exit_reason "${1:-normal}" \
        --argjson ops "$ops_json" \
        '{ts:$ts, class:$class, branch:$branch, head_sha_at_dry_run:$head, commit_msg:$msg, exit_reason:$exit_reason, would_have:$ops}' \
        > "$preview" 2>/dev/null || true
    log "DRY-RUN: journal preview written to $preview"
}

# --- 0. Pre-flight -----------------------------------------------------------

# v8.25.0: Parse --class flag (and any future flags) before the positional
# commit message. Default class is "cycle" (audit-bound) for backward compat.
SHIP_CLASS="cycle"
COMMIT_MSG=""
DRY_RUN=0

while [ $# -gt 0 ]; do
    case "$1" in
        --class)
            shift
            [ $# -ge 1 ] || fail "--class requires a value (cycle|manual|release)"
            SHIP_CLASS="$1"
            shift
            ;;
        --dry-run)
            DRY_RUN=1
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
        log "  → Migrate to: bash scripts/lifecycle/ship.sh --class manual \"<msg>\""
        log "  → Or for CI:  EVOLVE_SHIP_AUTO_CONFIRM=1 bash scripts/lifecycle/ship.sh --class manual \"<msg>\""
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
    # C1: extract audit_bound_tree_sha for post-commit verification
    # `grep -m1` exits 1 on no-match; `|| true` prevents set -e from killing the script
    # when the Auditor predates C1 and didn't emit the field (graceful absent → empty string).
    AUDIT_BOUND_TREE_SHA=$(echo "$AUDIT_VERDICT_RAW" | grep -m1 'audit_bound_tree_sha:' \
        | awk '{print $NF}' | tr -d "[:space:]\`" || true)
    unset AUDIT_VERDICT_RAW

    # v10.0.0: EGPS predicate-suite gate (cycle-class only).
    # acs-verdict.json — produced by scripts/lifecycle/run-acs-suite.sh — IS the
    # verdict-bearing artifact going forward. If present, its red_count==0
    # requirement is the structural ship gate; this overrides the WARN-by-default
    # fluent posture (no WARN ship loophole). If absent, fluent posture from
    # audit-report.md still applies (bootstrap; cycles 1–39 have no predicates).
    # See docs/architecture/egps-v10.md for the contract.
    EGPS_VERDICT_FILE="$(dirname "$ARTIFACT_PATH")/acs-verdict.json"
    if [ -f "$EGPS_VERDICT_FILE" ] && command -v jq >/dev/null 2>&1; then
        _egps_red=$(jq -r '.red_count // 0' "$EGPS_VERDICT_FILE" 2>/dev/null)
        _egps_verdict=$(jq -r '.verdict // empty' "$EGPS_VERDICT_FILE" 2>/dev/null)
        _egps_total=$(jq -r '.predicate_suite.total // 0' "$EGPS_VERDICT_FILE" 2>/dev/null)
        if [ "${_egps_red:-0}" != "0" ]; then
            _egps_red_ids=$(jq -r '.red_ids | join(",")' "$EGPS_VERDICT_FILE" 2>/dev/null)
            integrity_fail "EGPS predicate suite has $_egps_red RED predicate(s): $_egps_red_ids (acs-verdict.json verdict=$_egps_verdict total=$_egps_total)"
        fi
        log "OK: EGPS predicate suite verdict=$_egps_verdict (green=$(jq -r '.green_count' "$EGPS_VERDICT_FILE" 2>/dev/null) total=$_egps_total)"
        unset _egps_red _egps_verdict _egps_total _egps_red_ids
    fi

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
# orchestrator session sees one allowed bash call (bash scripts/lifecycle/ship.sh ...)
# and the inner git operations don't fire the gate individually. Solves the
# v8.12.x D6 workflow regression.

# Determine current branch (block accidental ship from detached HEAD).
CURRENT_BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || echo "")
[ -n "$CURRENT_BRANCH" ] || fail "detached HEAD — refuse to ship; checkout a branch first"

# v8.43.0: worktree-aware shipping. Pre-v8.43, ship.sh ran git add+commit
# from main repo cwd; Builder edits live in active_worktree on branch
# evolve/cycle-N which is invisible from main's working tree, so ship.sh
# saw a clean tree and exited 0 with nothing shipped. Cycles correctly
# built improvements; ship was a no-op; lastCycleNumber didn't advance;
# 5-repeat circuit-breaker wasted ~$18.92 in the observed regression.
#
# Fix: when --class cycle and active_worktree is set, do the commit IN
# the worktree (where Builder's changes live), then fast-forward merge
# evolve/cycle-N into main, then push main. --ff-only refuses divergent
# history rather than silently auto-merging.
WORKTREE_COMMIT_DONE=0
if [ "$SHIP_CLASS" = "cycle" ]; then
    cycle_state_file="$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json"
    if [ -f "$cycle_state_file" ]; then
        active_worktree=$(jq -r '.active_worktree // empty' "$cycle_state_file" 2>/dev/null || echo "")
        if [ -n "$active_worktree" ] && [ -d "$active_worktree" ] \
           && [ "$active_worktree" != "$EVOLVE_PROJECT_ROOT" ]; then
            log "v8.43.0: worktree-aware ship — committing in active_worktree=$active_worktree"
            cycle_branch=$(git -C "$active_worktree" symbolic-ref --short HEAD 2>/dev/null || echo "")
            if [ -z "$cycle_branch" ]; then
                fail "could not resolve cycle branch from worktree $active_worktree"
            fi
            log "  cycle branch: $cycle_branch"
            git -C "$active_worktree" add -A
            if git -C "$active_worktree" diff --cached --quiet; then
                ahead=$(git rev-list --count "$CURRENT_BRANCH..$cycle_branch" 2>/dev/null || echo 0)
                if [ "$ahead" = "0" ]; then
                    # v8.58.0 Layer E4: warn before silent-exit-0 if main has
                    # uncommitted modifications. Cycle-6 verification showed
                    # Builder can escape the worktree (via Bash shell redirects
                    # or absolute paths) — when this happens the worktree stays
                    # clean but main accumulates the work without ever shipping.
                    main_dirty=$(git status --porcelain 2>/dev/null | head -10)
                    if [ -n "$main_dirty" ]; then
                        dirty_count=$(printf '%s\n' "$main_dirty" | wc -l | tr -d ' ')
                        log "WARN: worktree clean BUT main working tree has $dirty_count modified/untracked files (possible Builder isolation breach):"
                        printf '%s\n' "$main_dirty" | head -10 >&2
                        log "WARN: investigate before next cycle — see scripts/guards/role-gate.sh and recent .evolve/guards.log DENY entries"
                    fi
                    log "no changes in worktree AND branch not ahead of $CURRENT_BRANCH; exiting cleanly"
                    write_dry_run_preview "no-changes-no-ahead"
                    exit 0
                fi
                log "  no uncommitted worktree changes but branch is $ahead commit(s) ahead; will merge"
            else
                diff_files=$(git -C "$active_worktree" diff --cached --name-status 2>/dev/null || echo "")
                diff_stat=$(git -C "$active_worktree" diff --cached --shortstat 2>/dev/null || echo "")
                if [ -n "$diff_files" ]; then
                    file_count=$(printf '%s\n' "$diff_files" | grep -c '^' || echo 0)
                    diff_footer=$(cat <<FOOTER

---
## Actual diff (v8.34.0+)

Files modified ($file_count):
$(printf '%s' "$diff_files" | sed 's/^/- /')

$diff_stat
FOOTER
)
                    WORKTREE_COMMIT_MSG="$COMMIT_MSG$diff_footer"
                else
                    WORKTREE_COMMIT_MSG="$COMMIT_MSG"
                fi
                if [ "$DRY_RUN" = "1" ]; then
                    dry_log "git -C $active_worktree commit -m <msg>"
                    dry_record "worktree-commit:$cycle_branch"
                    log "  [DRY-RUN] would commit in worktree on $cycle_branch"
                else
                    git -C "$active_worktree" -c commit.gpgsign=false commit -m "$WORKTREE_COMMIT_MSG" \
                        || fail "git commit in worktree failed"
                    log "  OK: committed in worktree on $cycle_branch"
                fi
                unset WORKTREE_COMMIT_MSG diff_files diff_stat file_count diff_footer
            fi
            if [ "$DRY_RUN" = "1" ]; then
                dry_log "git merge --ff-only $cycle_branch"
                dry_log "git push origin $CURRENT_BRANCH"
                dry_record "merge-ff:$cycle_branch->$CURRENT_BRANCH"
                dry_record "push:$CURRENT_BRANCH"
                dry_record "tree-sha-binding:audit=${AUDIT_BOUND_TREE_SHA:-none}"
                log "  [DRY-RUN] would ff-merge + push $cycle_branch into $CURRENT_BRANCH"
            else
                git merge --ff-only "$cycle_branch" \
                    || fail "ff-merge $cycle_branch into $CURRENT_BRANCH failed (divergent history); re-run cycle from clean main"
                log "  OK: ff-merged $cycle_branch into $CURRENT_BRANCH"
                git push origin "$CURRENT_BRANCH" \
                    || fail "git push failed; main is at $(git rev-parse HEAD)"
                log "OK: pushed to origin/$CURRENT_BRANCH"
                # v10.x: Advance lastCycleNumber immediately after successful push, before
                # post-push integrity check. Counter tracks "push succeeded", not "integrity
                # verified". When integrity_fail fires below (e.g. tree-SHA mismatch from a
                # prior backtick bug), the commit is already on remote — the counter must
                # reflect that. Root cause of cycle-46 stuck-counter: integrity_fail at C1
                # exited rc=2 before the advance block at line ~757 was reached.
                _wt_state_file="$EVOLVE_PROJECT_ROOT/.evolve/state.json"
                _wt_cycle_id=$(jq -r '.cycle_id // empty' "$cycle_state_file" 2>/dev/null || echo "")
                if [ -f "$_wt_state_file" ] && [ -n "$_wt_cycle_id" ] && [ "$_wt_cycle_id" != "null" ]; then
                    _wt_tmp="${_wt_state_file}.tmp.$$"
                    if jq --argjson n "$_wt_cycle_id" '.lastCycleNumber = $n' "$_wt_state_file" > "$_wt_tmp" 2>/dev/null \
                       && mv -f "$_wt_tmp" "$_wt_state_file"; then
                        log "OK: advanced state.json:lastCycleNumber to $_wt_cycle_id (pre-integrity-check)"
                    else
                        rm -f "$_wt_tmp" 2>/dev/null
                        log "WARN: could not advance lastCycleNumber pre-integrity-check (state.json write failed)"
                    fi
                fi
                unset _wt_state_file _wt_cycle_id _wt_tmp
                # C1: tree-SHA binding verification + ship-binding.json sidecar
                TREE_SHA_COMMITTED=$(git rev-parse HEAD^{tree} 2>/dev/null || echo "")
                if [ -n "$AUDIT_BOUND_TREE_SHA" ] && [ -n "$TREE_SHA_COMMITTED" ]; then
                    if [ "$AUDIT_BOUND_TREE_SHA" != "$TREE_SHA_COMMITTED" ]; then
                        integrity_fail "INTEGRITY BREACH: audit-bound tree SHA $AUDIT_BOUND_TREE_SHA != committed tree SHA $TREE_SHA_COMMITTED — worktree-to-main tree drift detected (see docs/incidents/cycle-31-c38-orphan.md)"
                    fi
                    log "OK: tree-SHA binding verified (audit=$AUDIT_BOUND_TREE_SHA committed=$TREE_SHA_COMMITTED)"
                fi
                _sb_cycle=$(jq -r '.cycle_id // empty' "$cycle_state_file" 2>/dev/null || echo "")
                if [ -n "$_sb_cycle" ]; then
                    _sb_path="$EVOLVE_PROJECT_ROOT/.evolve/runs/cycle-${_sb_cycle}/ship-binding.json"
                    _sb_tmp="${_sb_path}.tmp.$$"
                    jq -n \
                        --arg audit "${AUDIT_BOUND_TREE_SHA:-}" \
                        --arg committed "${TREE_SHA_COMMITTED:-}" \
                        --arg commit "$(git rev-parse HEAD 2>/dev/null || echo '')" \
                        --argjson cycle "${_sb_cycle}" \
                        '{audit_bound_tree_sha:$audit, tree_sha_committed:$committed, commit_sha:$commit, cycle:$cycle}' \
                        > "$_sb_tmp" 2>/dev/null \
                        && mv -f "$_sb_tmp" "$_sb_path" \
                        || log "WARN: could not write ship-binding.json"
                fi
                unset TREE_SHA_COMMITTED _sb_cycle _sb_path _sb_tmp
            fi
            WORKTREE_COMMIT_DONE=1
        fi
    fi
fi

# Stage everything (intentional — audit was based on full diff HEAD).
# Skipped for v8.43.0 worktree-ship path (handled above).
if [ "$WORKTREE_COMMIT_DONE" = "0" ]; then
    git add -A
fi

# If nothing to commit, log and exit cleanly.
# Skipped if v8.43.0 worktree-ship already completed (commit+push handled above).
if [ "$WORKTREE_COMMIT_DONE" = "1" ]; then
    : # worktree path completed; skip remaining flow
elif git diff --cached --quiet; then
    # v8.58.0 Layer E4: warn on dirty main even on the non-worktree-cycle path.
    main_dirty=$(git status --porcelain 2>/dev/null | head -10)
    if [ -n "$main_dirty" ]; then
        dirty_count=$(printf '%s\n' "$main_dirty" | wc -l | tr -d ' ')
        log "WARN: nothing staged BUT main has $dirty_count uncommitted file(s) — operator review recommended:"
        printf '%s\n' "$main_dirty" | head -10 >&2
    fi
    log "no staged changes to ship; exiting cleanly (audit was for an empty diff)"
    write_dry_run_preview "no-staged-changes"
    exit 0
fi

# v8.34.0: Append actual-diff footer to commit message for cycle/manual classes.
# Records the real file list + line counts in `git log` so reviewers (and future
# audits) can compare the agent's narrative against what actually shipped. This
# is the "record, don't block" pattern: per-cycle audits in cycles 102+ shipped
# commits whose messages claimed major refactors but diffs were trivial 2-line
# moves. The auditor scored claims, not diff. Now `git log` carries both.
#
# Skipped for --class release (release commits are version bumps; the footer
# adds bulk without value because release scope is structurally well-defined).
if [ "$SHIP_CLASS" = "cycle" ] || [ "$SHIP_CLASS" = "manual" ]; then
    diff_files=$(git diff --cached --name-status 2>/dev/null || echo "")
    diff_stat=$(git diff --cached --shortstat 2>/dev/null || echo "")
    if [ -n "$diff_files" ]; then
        # Bash 3.2-safe count: pipe through wc -l (count of newlines).
        # diff_files always ends with a newline so this is the correct count.
        file_count=$(printf '%s\n' "$diff_files" | grep -c '^' || echo 0)
        # Build the footer. Two newlines before "---" so it's separate from any
        # existing trailing content in COMMIT_MSG (which may or may not end with
        # newline).
        diff_footer=$(cat <<FOOTER

---
## Actual diff (v8.34.0+)

Files modified ($file_count):
$(printf '%s' "$diff_files" | sed 's/^/- /')

$diff_stat
FOOTER
)
        COMMIT_MSG="$COMMIT_MSG$diff_footer"
    fi
    unset diff_files diff_stat file_count diff_footer
fi

# Commit + push. Skipped when v8.43.0 worktree-ship already pushed above.
if [ "$WORKTREE_COMMIT_DONE" = "0" ]; then
    if [ "$DRY_RUN" = "1" ]; then
        dry_log "git commit -m <msg>"
        dry_log "git push origin $CURRENT_BRANCH"
        dry_record "commit:$CURRENT_BRANCH"
        dry_record "push:$CURRENT_BRANCH"
        log "[DRY-RUN] would commit + push to $CURRENT_BRANCH"
    else
        # Commit. Use array form to avoid arg injection.
        COMMIT_ARGS=(commit -m "$COMMIT_MSG")
        git "${COMMIT_ARGS[@]}"
        log "OK: committed to $CURRENT_BRANCH"

        # Push.
        git push origin "$CURRENT_BRANCH"
        log "OK: pushed to origin/$CURRENT_BRANCH"
    fi
fi

# v8.34.0: Advance state.json:lastCycleNumber on successful cycle ship.
# Pre-v8.34, only failure paths (record_failed_approach in dispatcher) wrote
# lastCycleNumber. Successful ships left the counter unchanged → dispatcher's
# next iteration computed ran_cycle = last_before + 1 = the SAME cycle just
# shipped → 5-repeat circuit-breaker fired prematurely on legitimate runs.
# Fix: write lastCycleNumber from cycle-state.json:cycle_id atomically.
# Defensive — only for --class cycle, only when cycle-state.json exists with a
# valid cycle_id; otherwise no-op.
if [ "$SHIP_CLASS" = "cycle" ] && [ "$DRY_RUN" = "0" ]; then
    cycle_state_file="$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json"
    state_file="$EVOLVE_PROJECT_ROOT/.evolve/state.json"
    if [ -f "$cycle_state_file" ] && [ -f "$state_file" ]; then
        cycle_id=$(jq -r '.cycle_id // empty' "$cycle_state_file" 2>/dev/null || echo "")
        if [ -n "$cycle_id" ] && [ "$cycle_id" != "null" ]; then
            tmp_state="${state_file}.tmp.$$"
            if jq --argjson n "$cycle_id" '.lastCycleNumber = $n' "$state_file" > "$tmp_state" 2>/dev/null \
               && mv -f "$tmp_state" "$state_file"; then
                log "OK: advanced state.json:lastCycleNumber to $cycle_id"
            else
                rm -f "$tmp_state" 2>/dev/null
                log "WARN: could not advance lastCycleNumber (state.json write failed)"
            fi
        fi
    fi
    unset cycle_state_file state_file cycle_id tmp_state
elif [ "$SHIP_CLASS" = "cycle" ] && [ "$DRY_RUN" = "1" ]; then
    dry_log "advance state.json:lastCycleNumber"
    dry_record "state-bump:lastCycleNumber"
fi

# v9.6.0: inbox lifecycle — promote shipped inbox tasks to processed/ (c37).
# Reads triage-decision.json from the current cycle's workspace and moves
# top_n[] + skip_shipped[] files to .evolve/inbox/processed/cycle-N/.
# mv failure logs WARN to ledger but NEVER blocks ship (Layer 1 idempotency
# catches any residual on next cycle via git-log check in Triage Step 0a).
if [ "$SHIP_CLASS" = "cycle" ] && [ "$DRY_RUN" = "0" ]; then
    _inbox_cycle_id=""
    _inbox_cs="$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json"
    [ -f "$_inbox_cs" ] && \
        _inbox_cycle_id=$(jq -r '.cycle_id // empty' "$_inbox_cs" 2>/dev/null || true)
    if [ -n "$_inbox_cycle_id" ]; then
        _triage_json="$EVOLVE_PROJECT_ROOT/.evolve/runs/cycle-${_inbox_cycle_id}/triage-decision.json"
        if [ -f "$_triage_json" ]; then
            _commit_sha=$(git -C "$EVOLVE_PROJECT_ROOT" rev-parse --short=8 HEAD 2>/dev/null || echo "")
            # Promote top_n[] (freshly shipped this cycle)
            jq -r '.top_n[]?.id // empty' "$_triage_json" 2>/dev/null | while read -r tid; do
                [ -n "$tid" ] || continue
                inbox-mover.sh promote "$tid" processed "$_inbox_cycle_id" \
                    --commit-sha "$_commit_sha" \
                    2>&1 | sed 's/^/[ship-inbox] /' >&2 || true
            done
            # Promote skip_shipped[] (idempotency-matched; never re-execute)
            jq -r '.skip_shipped[]?.task_id // empty' "$_triage_json" 2>/dev/null | while read -r tid; do
                [ -n "$tid" ] || continue
                inbox-mover.sh promote "$tid" processed "$_inbox_cycle_id" \
                    --commit-sha "$_commit_sha" \
                    2>&1 | sed 's/^/[ship-inbox] /' >&2 || true
            done
            log "OK: inbox lifecycle promote complete for cycle ${_inbox_cycle_id}"
        else
            log "INFO: no triage-decision.json for cycle ${_inbox_cycle_id} — inbox promote skipped"
        fi
    fi
    unset _inbox_cycle_id _inbox_cs _triage_json _commit_sha
fi

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
        if [ "$DRY_RUN" = "1" ]; then
            dry_log "gh release create $TAG"
            dry_record "gh-release:$TAG"
            log "[DRY-RUN] would create GitHub release $TAG"
        elif command -v gh >/dev/null 2>&1; then
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

if [ "$DRY_RUN" = "1" ]; then
    write_dry_run_preview "normal"
    log "DRY-RUN DONE: no commit, no push, no release. tree clean."
    exit 0
fi

# v11.0 T1: Auto-heal expected_ship_sha after any successful cycle push.
# If ship.sh itself was included in the shipped commit, its on-disk SHA changed
# after the ff-merge. Re-read and re-pin so the next cycle's ship-gate does not
# integrity_fail on a legitimate self-update.
if [ "$SHIP_CLASS" = "cycle" ]; then
    _post_sha=$(SHA256 "${BASH_SOURCE[0]}")
    if [ "$_post_sha" != "$ACTUAL_SHIP_SHA" ]; then
        ACTUAL_SHIP_SHA="$_post_sha"
        _repin_ship_sha "post-cycle self-update (ship.sh changed in this commit)"
    fi
    unset _post_sha
fi

log "DONE: shipped $CURRENT_BRANCH at $(git rev-parse HEAD)"
exit 0
