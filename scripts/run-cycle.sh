#!/usr/bin/env bash
#
# run-cycle.sh — Convenience driver for the Evolve Loop (v8.13.1).
#
# Initializes per-cycle runtime state (.evolve/cycle-state.json) and spawns
# the orchestrator subagent (scripts/subagent-run.sh orchestrator ...). The
# orchestrator's profile (.evolve/profiles/orchestrator.json) restricts what
# it can do — combined with role-gate.sh (path-allowlist on Edit/Write) and
# phase-gate-precondition.sh (sequence-allowlist on subagent invocations),
# the trust boundary becomes structurally enforced rather than relying on
# LLM cooperation.
#
# Usage:
#   bash scripts/run-cycle.sh [GOAL]
#   bash scripts/run-cycle.sh --cycle 8200 [GOAL]
#   bash scripts/run-cycle.sh --dry-run   # print what would happen without spawning
#
# Lifecycle:
#   1. Resolve cycle ID (next-after-state OR explicit --cycle).
#   2. Create workspace .evolve/runs/cycle-N/.
#   3. cycle_state_init → cycle-state.json with phase=calibrate.
#   4. Build context block (instinct summary, ledger tail, failed approaches).
#   5. Spawn orchestrator: bash scripts/subagent-run.sh orchestrator $CYCLE $WORKSPACE.
#   6. On exit (PASS or FAIL), clear cycle-state.json and print summary.
#
# IMPORTANT — what this script does NOT do:
#   - It does NOT itself sequence phases. Phase sequencing lives inside the
#     orchestrator subagent (in agents/evolve-orchestrator.md). The runner
#     only writes the initial state file and spawns the orchestrator.
#   - It does NOT write to source code. role-gate.sh blocks that during cycles.
#   - It does NOT commit/push. Only scripts/ship.sh can (ship-gate enforces).
#
# Exit codes:
#   0   — orchestrator completed (verdict in orchestrator-report.md)
#   1   — runtime failure (couldn't spawn, missing prerequisites)
#   2   — integrity failure (cycle-state collision, etc.)

set -uo pipefail

# v8.16.2: explicitly export runtime knobs so they propagate through nested
# bash + sandbox-exec invocations to the deepest claude.sh call. Bash inherits
# env by default, but sandbox-exec on macOS may not propagate all variables
# reliably across nested invocations. Explicit export removes ambiguity.
[ -n "${EVOLVE_SANDBOX_FALLBACK_ON_EPERM:-}" ] && export EVOLVE_SANDBOX_FALLBACK_ON_EPERM
[ -n "${EVOLVE_DISPATCH_STOP_ON_FAIL:-}" ] && export EVOLVE_DISPATCH_STOP_ON_FAIL
[ -n "${EVOLVE_BYPASS_PHASE_GATE:-}" ] && export EVOLVE_BYPASS_PHASE_GATE

# v8.18.0: dual-root resolution. PLUGIN_ROOT for read-only scripts/agents,
# PROJECT_ROOT for writable state/ledger/runs/instincts. See resolve-roots.sh.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/resolve-roots.sh"
unset __rr_self

# v8.20.0: Prepend the plugin's scripts dir (and release/ subdir) to PATH so
# kernel scripts are findable by bare name from subagent subprocesses. This
# eliminates the install-layout-fragile `bash $EVOLVE_PLUGIN_ROOT/scripts/foo.sh`
# invocation pattern that required 4 path-variant allowlist entries per script
# (relative + ** glob + marketplace + cache absolute) — 140 patterns total in
# orchestrator.json. With PATH set, orchestrator invokes `cycle-state.sh advance`
# (bare name) and the allowlist needs ONE pattern: Bash(cycle-state.sh advance:*).
# Works in dev (cwd=repo), marketplace, cache, and any future install layout.
# Inherits to claude -p subprocess via standard env propagation.
export PATH="$EVOLVE_PLUGIN_ROOT/scripts:$EVOLVE_PLUGIN_ROOT/scripts/release:$PATH"

# Read-only resources from the plugin install
CYCLE_STATE_HELPER="$EVOLVE_PLUGIN_ROOT/scripts/cycle-state.sh"
SUBAGENT_RUN="$EVOLVE_PLUGIN_ROOT/scripts/subagent-run.sh"
ORCHESTRATOR_PROMPT="$EVOLVE_PLUGIN_ROOT/agents/evolve-orchestrator.md"

# Writable artifacts under the user's project (or evolve-loop repo in dev mode)
STATE_FILE="$EVOLVE_PROJECT_ROOT/.evolve/state.json"
LEDGER="$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl"
INSTINCT_SUMMARY="$EVOLVE_PROJECT_ROOT/.evolve/instincts/personal/summary.md"

log()  { echo "[run-cycle] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }
integrity_fail() { log "INTEGRITY-FAIL: $*"; exit 2; }

# ---- Argument parsing ------------------------------------------------------

DRY_RUN=0
CYCLE=""
GOAL=""

while [ $# -gt 0 ]; do
    case "$1" in
        --cycle)
            shift
            [ $# -gt 0 ] || fail "--cycle requires a value"
            CYCLE="$1"
            ;;
        --dry-run)
            DRY_RUN=1
            ;;
        --help|-h)
            sed -n '2,30p' "$0" | sed 's/^# //; s/^#//'
            exit 0
            ;;
        --*)
            fail "unknown flag: $1"
            ;;
        *)
            # First positional → goal.
            if [ -z "$GOAL" ]; then GOAL="$1"
            else GOAL="$GOAL $1"
            fi
            ;;
    esac
    shift
done

# ---- Prerequisites ---------------------------------------------------------

[ -f "$CYCLE_STATE_HELPER" ] || fail "missing $CYCLE_STATE_HELPER"
[ -f "$SUBAGENT_RUN" ]       || fail "missing $SUBAGENT_RUN"
[ -f "$ORCHESTRATOR_PROMPT" ] || fail "missing $ORCHESTRATOR_PROMPT"
command -v jq >/dev/null 2>&1 || fail "jq is required"

# ---- Resolve cycle ID ------------------------------------------------------

if [ -z "$CYCLE" ]; then
    if [ -f "$STATE_FILE" ]; then
        last=$(jq -r '.lastCycleNumber // 0' "$STATE_FILE" 2>/dev/null || echo 0)
    else
        last=0
    fi
    CYCLE=$((last + 1))
fi
[[ "$CYCLE" =~ ^[0-9]+$ ]] || fail "cycle must be integer, got: $CYCLE"

WORKSPACE="$EVOLVE_PROJECT_ROOT/.evolve/runs/cycle-$CYCLE"

# ---- Collision check -------------------------------------------------------

if bash "$CYCLE_STATE_HELPER" exists >/dev/null 2>&1; then
    existing=$(bash "$CYCLE_STATE_HELPER" get cycle_id || true)
    integrity_fail "cycle-state.json already exists for cycle $existing — refusing to clobber. Run: bash scripts/cycle-state.sh clear"
fi

# ---- Build context block ---------------------------------------------------

build_context() {
    local cycle="$1" workspace="$2" goal="$3"

    # Ledger tail (last 5 entries) — gives orchestrator awareness of recent activity.
    local ledger_tail=""
    if [ -f "$LEDGER" ]; then
        ledger_tail=$(tail -5 "$LEDGER" 2>/dev/null || echo "")
    fi

    # Instinct summary — accumulated lessons from prior cycles.
    local instinct=""
    if [ -f "$INSTINCT_SUMMARY" ]; then
        instinct=$(cat "$INSTINCT_SUMMARY" 2>/dev/null || echo "")
    fi

    # v8.22.0: deterministic failure-adapter decision. Replaces the prompt-only
    # markdown rule the orchestrator used to interpret. The adapter computes the
    # right action (PROCEED | RETRY-WITH-FALLBACK | BLOCK-CODE | BLOCK-OPERATOR-ACTION)
    # from non-expired failedApproaches and emits a structured JSON object.
    # The orchestrator reads this JSON and follows the action verbatim.
    local adapter_decision=""
    if [ -x "$EVOLVE_PLUGIN_ROOT/scripts/failure-adapter.sh" ]; then
        adapter_decision=$(bash "$EVOLVE_PLUGIN_ROOT/scripts/failure-adapter.sh" decide --state "$STATE_FILE" 2>/dev/null || echo '')
    fi

    # Read-side defense-in-depth: filter failedApproaches by expiresAt before
    # exposing as recentFailures to the orchestrator. Even if a write-time prune
    # missed an entry, expired ones never reach the LLM context.
    local now_s
    now_s=$(date -u +%s)
    local failed=""
    if [ -f "$STATE_FILE" ]; then
        failed=$(jq -r --argjson now "$now_s" '
            (.failedApproaches // [])
            | map(select(
                (.expiresAt // "") == "" or
                (.expiresAt | (try fromdateiso8601 catch ($now + 1))) > $now
              ))
            | .[-3:] | .[] | "- [" + (.classification // "legacy") + "] " + (.summary // .verdict // "unknown")
        ' "$STATE_FILE" 2>/dev/null || echo "")
    fi

    cat <<EOF

---
ORCHESTRATOR CONTEXT (injected by run-cycle.sh)
---

cycle: $cycle
workspace: $workspace
goal: ${goal:-<unspecified — pick from CLAUDE.md priorities>}
cycleState: $EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json (already initialized to phase=calibrate)
pluginRoot: $EVOLVE_PLUGIN_ROOT (read-only — scripts/, agents/, profiles/ live here)
projectRoot: $EVOLVE_PROJECT_ROOT (writable — state, ledger, runs, instincts go here)
intentRequired: ${EVOLVE_REQUIRE_INTENT:-0} (v8.19.0+: when 1, run intent persona before scout; cycle-state.intent_required is the authoritative source)
intentArtifactPath: $workspace/intent.md (only present if intent persona has run)

adaptiveFailureDecision (v8.22.0+ — deterministic kernel verdict — FOLLOW VERBATIM):
$adapter_decision

recentLedgerEntries:
$ledger_tail

recentFailures (non-expired, last 3):
$failed

instinctSummary:
$instinct

---
EOF
}

# v8.22.0: Honor failure-adapter's set_env directive at the run-cycle layer.
# The dispatcher already auto-sets EVOLVE_SANDBOX_FALLBACK_ON_EPERM for nested-
# claude (defense-in-depth). The adapter's set_env covers the case where the
# dispatcher path was skipped (direct run-cycle.sh invocation, tests, etc.).
honor_adapter_set_env() {
    [ -x "$EVOLVE_PLUGIN_ROOT/scripts/failure-adapter.sh" ] || return 0
    local decision
    decision=$(bash "$EVOLVE_PLUGIN_ROOT/scripts/failure-adapter.sh" decide --state "$STATE_FILE" 2>/dev/null || echo '')
    [ -n "$decision" ] || return 0
    if command -v jq >/dev/null 2>&1; then
        # Iterate set_env keys and export each.
        while IFS=$'\t' read -r k v; do
            [ -n "$k" ] || continue
            if [ -z "${!k:-}" ]; then
                log "adapter: setting $k=$v"
                export "$k=$v"
            fi
        done < <(echo "$decision" | jq -r '(.set_env // {}) | to_entries[] | "\(.key)\t\(.value)"' 2>/dev/null)
    fi
}

# ---- Setup workspace -------------------------------------------------------

mkdir -p "$WORKSPACE"
log "workspace=$WORKSPACE"

# Initialize cycle-state.json (phase=calibrate, no agent yet).
bash "$CYCLE_STATE_HELPER" init "$CYCLE" ".evolve/runs/cycle-$CYCLE" \
    || fail "cycle_state_init failed"
log "cycle-state.json initialized at phase=calibrate"

# v8.21.0: privileged-shell worktree provisioning — closes the trust-boundary
# gap where the orchestrator profile (correctly) denies `git worktree add` but
# nothing else provisioned the worktree, leaving cycle-state.active_worktree
# null and the builder's sandbox profile expanding {worktree_path} to empty.
# This block runs BEFORE the orchestrator subprocess so the worktree is ready
# by the time the build phase starts. The orchestrator and all phase agents
# may NOT call `git worktree add/remove` — only this privileged shell context.
WORKTREE_PATH=""
WORKTREE_BRANCH=""
WORKTREE_PROVISIONED=0

# v8.23.4 BUG-011 escape hatch: EVOLVE_SKIP_WORKTREE=1 disables worktree
# provisioning entirely and points cycle-state.active_worktree at the main
# project root. Use this when the parent Claude Code session's OS sandbox
# blocks writes to .evolve/worktrees/ even after the v8.23.3 cwd fix and
# v8.22.0 EPERM fallback. Tradeoff: builder edits land directly in the main
# repo (no isolation, no easy rollback). Operator must manually `git diff`
# and either commit or `git restore .` after each cycle.
#
# When NOT to use:
#   - Standalone shell (no parent Claude Code) — worktree provisioning works
#     normally; EVOLVE_SKIP_WORKTREE=1 just removes safety with no upside.
#   - When you can grant write access via .claude/settings.json instead.
# When TO use:
#   - Nested-claude environments where v8.23.3 still EPERMs at the build phase
#   - One-off recovery from cycles that need to land NOW
#
# Loud WARN log so the operator knows isolation is off.
if [ "${EVOLVE_SKIP_WORKTREE:-0}" = "1" ]; then
    log "WARN: EVOLVE_SKIP_WORKTREE=1 — bypassing worktree isolation"
    log "  → Builder will edit \$EVOLVE_PROJECT_ROOT directly (no worktree, no easy rollback)"
    log "  → After cycle: inspect \`git status\` and \`git diff\` manually"
    log "  → Set EVOLVE_SKIP_WORKTREE=0 (default) once the underlying sandbox issue is resolved"
    WORKTREE_PATH="$EVOLVE_PROJECT_ROOT"
    WORKTREE_BRANCH=""
    WORKTREE_PROVISIONED=0   # NOT provisioned — cleanup must skip worktree-remove
    bash "$CYCLE_STATE_HELPER" set-worktree "$WORKTREE_PATH" \
        || fail "set-worktree failed for $WORKTREE_PATH"
    export WORKTREE_PATH
    log "active_worktree=$WORKTREE_PATH (main repo, no isolation)"
elif [ "$DRY_RUN" = "0" ] || [ "${EVOLVE_DRY_RUN_PROVISION_WORKTREE:-1}" = "1" ]; then
    WORKTREE_BASE="$EVOLVE_PROJECT_ROOT/.evolve/worktrees"
    WORKTREE_PATH="$WORKTREE_BASE/cycle-$CYCLE"
    WORKTREE_BRANCH="evolve/cycle-$CYCLE"
    mkdir -p "$WORKTREE_BASE"

    # Idempotent: clean a stale worktree from a prior cycle with the same id
    # (typically a hard-killed run that didn't reach the cleanup trap).
    if git -C "$EVOLVE_PROJECT_ROOT" worktree list --porcelain 2>/dev/null \
         | grep -q "^worktree $WORKTREE_PATH$"; then
        log "removing stale worktree at $WORKTREE_PATH"
        git -C "$EVOLVE_PROJECT_ROOT" worktree remove --force "$WORKTREE_PATH" 2>/dev/null || true
    fi
    [ -d "$WORKTREE_PATH" ] && rm -rf "$WORKTREE_PATH"
    if git -C "$EVOLVE_PROJECT_ROOT" branch --list "$WORKTREE_BRANCH" 2>/dev/null \
         | grep -q "$WORKTREE_BRANCH"; then
        log "removing stale branch $WORKTREE_BRANCH"
        git -C "$EVOLVE_PROJECT_ROOT" branch -D "$WORKTREE_BRANCH" 2>/dev/null || true
    fi

    if git -C "$EVOLVE_PROJECT_ROOT" worktree add -b "$WORKTREE_BRANCH" "$WORKTREE_PATH" HEAD 2>&1 \
         | sed 's/^/[run-cycle:worktree] /' >&2; then
        WORKTREE_PROVISIONED=1
        bash "$CYCLE_STATE_HELPER" set-worktree "$WORKTREE_PATH" \
            || fail "set-worktree failed for $WORKTREE_PATH"
        export WORKTREE_PATH
        log "worktree provisioned at $WORKTREE_PATH (branch $WORKTREE_BRANCH)"
    else
        fail "worktree provisioning failed for cycle $CYCLE — see log above"
    fi
fi

# Always clean up cycle-state AND worktree on exit (success OR failure).
# v8.21.0: extended to tear down the worktree provisioned above. The
# WORKTREE_PROVISIONED flag prevents accidental deletion of a pre-existing
# branch in the rare race where the variable was set but the worktree never
# came online.
cleanup() {
    local rc=$?
    if [ "$WORKTREE_PROVISIONED" = "1" ]; then
        if [ -d "$WORKTREE_PATH" ]; then
            log "cleanup: removing worktree $WORKTREE_PATH"
            git -C "$EVOLVE_PROJECT_ROOT" worktree remove --force "$WORKTREE_PATH" 2>/dev/null \
                || rm -rf "$WORKTREE_PATH"
        fi
        if [ -n "$WORKTREE_BRANCH" ]; then
            git -C "$EVOLVE_PROJECT_ROOT" branch -D "$WORKTREE_BRANCH" 2>/dev/null || true
        fi
    elif [ "${EVOLVE_SKIP_WORKTREE:-0}" = "1" ]; then
        # v8.23.4: when worktree was bypassed, tell the operator what (if anything)
        # the builder left in the working tree. This is the only safety net for
        # the no-isolation path — operator must manually decide what to keep.
        local dirty_files
        dirty_files=$(git -C "$EVOLVE_PROJECT_ROOT" status --porcelain 2>/dev/null | wc -l | tr -d ' ')
        if [ "$dirty_files" -gt 0 ]; then
            log "WARN: EVOLVE_SKIP_WORKTREE=1 — main repo has $dirty_files changed file(s) from this cycle"
            log "  → Run \`git status\` and \`git diff\` to inspect"
            log "  → \`git restore .\` to discard, or commit/stash to keep"
        fi
    fi
    bash "$CYCLE_STATE_HELPER" clear 2>/dev/null || true
    log "cycle-state cleared (rc=$rc)"
    exit $rc
}
trap cleanup EXIT INT TERM

# v8.22.0: honor failure-adapter's set_env directive (e.g., auto-enable
# EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 when prior infra-transient failures are
# present and the dispatcher path didn't already set it).
honor_adapter_set_env

# ---- Build prompt ----------------------------------------------------------

PROMPT_FILE="$WORKSPACE/orchestrator-prompt.md"
{
    cat "$ORCHESTRATOR_PROMPT"
    build_context "$CYCLE" "$WORKSPACE" "$GOAL"
} > "$PROMPT_FILE"

log "prompt written to $PROMPT_FILE ($(wc -l < "$PROMPT_FILE") lines)"

# ---- Dry-run? --------------------------------------------------------------

if [ "$DRY_RUN" = "1" ]; then
    log "DRY RUN — would spawn:"
    log "  PROMPT_FILE_OVERRIDE=$PROMPT_FILE bash scripts/subagent-run.sh orchestrator $CYCLE $WORKSPACE"
    log "cycle-state snapshot before EXIT trap clears it:"
    bash "$CYCLE_STATE_HELPER" dump | jq . >&2 || true
    # v8.21.0: let the EXIT trap fire naturally — it tears down both the
    # worktree (if provisioned) and cycle-state.json. Set EVOLVE_DRY_RUN_PROVISION_WORKTREE=0
    # to skip worktree provisioning entirely in dry-run.
    exit 0
fi

# ---- Spawn orchestrator ----------------------------------------------------

log "spawning orchestrator subagent for cycle $CYCLE..."

set +e
PROMPT_FILE_OVERRIDE="$PROMPT_FILE" bash "$SUBAGENT_RUN" orchestrator "$CYCLE" "$WORKSPACE"
rc=$?
set -e

# ---- Summary ---------------------------------------------------------------

log "orchestrator subagent exited rc=$rc"

if [ -f "$WORKSPACE/orchestrator-report.md" ]; then
    log "orchestrator report at: $WORKSPACE/orchestrator-report.md"
    head -30 "$WORKSPACE/orchestrator-report.md" >&2 || true
else
    log "WARN: no orchestrator-report.md produced"
fi

exit "$rc"
