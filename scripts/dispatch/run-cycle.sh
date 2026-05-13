#!/usr/bin/env bash
#
# run-cycle.sh — Convenience driver for the Evolve Loop (v8.13.1).
#
# Initializes per-cycle runtime state (.evolve/cycle-state.json) and spawns
# the orchestrator subagent (scripts/dispatch/subagent-run.sh orchestrator ...). The
# orchestrator's profile (.evolve/profiles/orchestrator.json) restricts what
# it can do — combined with role-gate.sh (path-allowlist on Edit/Write) and
# phase-gate-precondition.sh (sequence-allowlist on subagent invocations),
# the trust boundary becomes structurally enforced rather than relying on
# LLM cooperation.
#
# Usage:
#   bash scripts/dispatch/run-cycle.sh [GOAL]
#   bash scripts/dispatch/run-cycle.sh --cycle 8200 [GOAL]
#   bash scripts/dispatch/run-cycle.sh --dry-run   # print what would happen without spawning
#   bash scripts/dispatch/run-cycle.sh --simulate  # walk every phase via cycle-simulator.sh (no LLM)
#
# Lifecycle:
#   1. Resolve cycle ID (next-after-state OR explicit --cycle).
#   2. Create workspace .evolve/runs/cycle-N/.
#   3. cycle_state_init → cycle-state.json with phase=calibrate.
#   4. Build context block (instinct summary, ledger tail, failed approaches).
#   5. Spawn orchestrator: bash scripts/dispatch/subagent-run.sh orchestrator $CYCLE $WORKSPACE.
#   6. On exit (PASS or FAIL), clear cycle-state.json and print summary.
#
# IMPORTANT — what this script does NOT do:
#   - It does NOT itself sequence phases. Phase sequencing lives inside the
#     orchestrator subagent (in agents/evolve-orchestrator.md). The runner
#     only writes the initial state file and spawns the orchestrator.
#   - It does NOT write to source code. role-gate.sh blocks that during cycles.
#   - It does NOT commit/push. Only scripts/lifecycle/ship.sh can (ship-gate enforces).
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
[ -n "${EVOLVE_DISPATCH_POLICY:-}" ] && export EVOLVE_DISPATCH_POLICY
[ -n "${EVOLVE_BYPASS_PHASE_GATE:-}" ] && export EVOLVE_BYPASS_PHASE_GATE
# v9.1.0 Cycle 2: propagate checkpoint signals from the dispatcher to the
# orchestrator subprocess. EVOLVE_CHECKPOINT_REQUEST is set by the dispatcher
# at 95% batch-budget consumption; the orchestrator reads it from its
# invocation env and writes a checkpoint at the next phase boundary.
[ -n "${EVOLVE_CHECKPOINT_REQUEST:-}" ] && export EVOLVE_CHECKPOINT_REQUEST
[ -n "${EVOLVE_CHECKPOINT_REASON:-}" ] && export EVOLVE_CHECKPOINT_REASON
[ -n "${EVOLVE_CHECKPOINT_TRIGGERED:-}" ] && export EVOLVE_CHECKPOINT_TRIGGERED
[ -n "${EVOLVE_CHECKPOINT_AT_PCT:-}" ] && export EVOLVE_CHECKPOINT_AT_PCT
[ -n "${EVOLVE_CHECKPOINT_WARN_AT_PCT:-}" ] && export EVOLVE_CHECKPOINT_WARN_AT_PCT

# v8.18.0: dual-root resolution. PLUGIN_ROOT for read-only scripts/agents,
# PROJECT_ROOT for writable state/ledger/runs/instincts. See resolve-roots.sh.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"
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
CYCLE_STATE_HELPER="$EVOLVE_PLUGIN_ROOT/scripts/lifecycle/cycle-state.sh"
SUBAGENT_RUN="$EVOLVE_PLUGIN_ROOT/scripts/dispatch/subagent-run.sh"
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
SIMULATE=0
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
        --simulate)
            SIMULATE=1
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

# v9.1.0 Cycle 4 ADDENDUM (post-ship discovery, 2026-05-11): in resume mode
# the existing cycle-state.json is INTENTIONAL — that's the preserved state we
# are resuming from. The collision check must NOT fire here; if it does, the
# entire --resume path is broken (resume-cycle.sh validates state then spawns
# run-cycle.sh with EVOLVE_RESUME_MODE=1, but this gate would abort before
# the resume-aware branch at line ~412 has a chance to run).
#
# This bug was caught only by end-to-end positive resume testing — source-level
# tests (resume-cycle-test.sh) saw the new RESUME-MODE code at line ~412 and
# passed; they didn't simulate the actual handoff from resume-cycle.sh into
# run-cycle.sh, which is where the collision fires.
if [ "${EVOLVE_RESUME_MODE:-0}" != "1" ]; then
    if bash "$CYCLE_STATE_HELPER" exists >/dev/null 2>&1; then
        existing=$(bash "$CYCLE_STATE_HELPER" get cycle_id || true)
        integrity_fail "cycle-state.json already exists for cycle $existing — refusing to clobber. Run: bash scripts/lifecycle/cycle-state.sh clear"
    fi
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
    if [ -x "$EVOLVE_PLUGIN_ROOT/scripts/failure/failure-adapter.sh" ]; then
        adapter_decision=$(bash "$EVOLVE_PLUGIN_ROOT/scripts/failure/failure-adapter.sh" decide --state "$STATE_FILE" 2>/dev/null || echo '')
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

    # v8.33.0: emit only non-empty blocks. Pre-v8.33 emitted every block
    # unconditionally (including empty headers like "instinctSummary:\n\n"
    # which padded each prompt with ~500-1000 tokens of useless boilerplate).
    # Now: skip the block entirely when its data source is empty. No quality
    # impact (empty data is empty whether we ship the empty header or not).
    {
        echo
        echo "---"
        echo "ORCHESTRATOR CONTEXT (injected by run-cycle.sh)"
        echo "---"
        echo
        echo "cycle: $cycle"
        echo "workspace: $workspace"
        echo "goal: ${goal:-<unspecified — pick from CLAUDE.md priorities>}"
        echo "cycleState: $EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json (already initialized to phase=calibrate)"
        echo "pluginRoot: $EVOLVE_PLUGIN_ROOT (read-only — scripts/, agents/, profiles/ live here)"
        echo "projectRoot: $EVOLVE_PROJECT_ROOT (writable — state, ledger, runs, instincts go here)"
        echo "intentRequired: ${EVOLVE_REQUIRE_INTENT:-0} (v8.19.0+: when 1, run intent persona before scout; cycle-state.intent_required is the authoritative source)"
        echo "intentArtifactPath: $workspace/intent.md (only present if intent persona has run)"
        echo

        # Adaptive failure decision: always include (even empty-ish) — orchestrator
        # uses this as its FOLLOW-VERBATIM directive; absence is meaningful.
        echo "adaptiveFailureDecision (v8.22.0+ — deterministic kernel verdict — FOLLOW VERBATIM):"
        echo "${adapter_decision:-<no decision available — proceed normally>}"
        echo

        # v8.33.0: conditional blocks. Emit header + body only when body is non-empty.
        # v8.44.0: emit compact digest (ts cycle role exit sha8) instead of raw JSONL
        # to reduce recentLedgerEntries from ~783 tokens to ~100 tokens per block.
        if [ -n "$ledger_tail" ]; then
            echo "recentLedgerEntries (digest — ts cycle role exit sha8):"
            while IFS= read -r entry; do
                [ -z "$entry" ] && continue
                echo "$entry" | jq -r '"[" + (.ts[0:10]) + " cycle:" + (.cycle|tostring) + " role:" + .role + " exit:" + (.exit_code|tostring) + " sha:" + ((.artifact_sha256 // "-")[0:8]) + "]"' 2>/dev/null
            done <<< "$ledger_tail"
            echo
        fi

        if [ -n "$failed" ]; then
            echo "recentFailures (non-expired, last 3):"
            echo "$failed"
            echo
        fi

        if [ -n "$instinct" ]; then
            echo "instinctSummary:"
            echo "$instinct"
            echo
        fi

        echo "---"
    }
}

# v8.22.0: Honor failure-adapter's set_env directive at the run-cycle layer.
# The dispatcher already auto-sets EVOLVE_SANDBOX_FALLBACK_ON_EPERM for nested-
# claude (defense-in-depth). The adapter's set_env covers the case where the
# dispatcher path was skipped (direct run-cycle.sh invocation, tests, etc.).
honor_adapter_set_env() {
    [ -x "$EVOLVE_PLUGIN_ROOT/scripts/failure/failure-adapter.sh" ] || return 0
    local decision
    decision=$(bash "$EVOLVE_PLUGIN_ROOT/scripts/failure/failure-adapter.sh" decide --state "$STATE_FILE" 2>/dev/null || echo '')
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

# v8.30.1: clear stale cycle directory BEFORE cycle_state_init.
#
# Replaces v8.29.0's named-pattern wipe (find -name '*-report.md' ...) which
# missed files outside the pattern set: worker-*.md from fan-out, cache-
# prefix.md, test-patch.diff/txt, and any future artifact types. Downstream
# user analysis pinpointed the deeper issue: when cycle-N is reused across
# batches (lastCycleNumber didn't advance because audit failed), ANY file
# in $WORKSPACE could have been written by a prior worktree that's now
# destroyed. The orchestrator's "if artifact exists, reuse it" optimization
# then picks up cross-fingerprint stale data, audit signs it against the
# new worktree, ship.sh's worktree-content SHA check correctly refuses
# ("expected 773ab8d7… actual daa208e8…" — 5 retries → dispatcher abort).
#
# Right behavior per user: clear the cycle directory entirely at start of
# every fresh run. Collision check at line 137 already refused if cycle-
# state.json exists, so reaching this point guarantees fresh start. Safe
# to nuke. Subdirectories (workers/) recreated by phase agents as needed.
#
# v8.59.0 Layer O: BUT first archive any artifacts that the ledger references.
# v8.58 release was blocked by `preflight.sh:step_audit_recent` because cycle 6's
# audit-report.md had been deleted in a retry's cycle-init (this very rm -rf),
# but the ledger entry still pointed to it. Solution: if the ledger has any
# agent_subprocess entries with artifact_path inside this WORKSPACE, move the
# whole workspace to .evolve/runs/archive/cycle-N-TIMESTAMP/ before clearing.
# Forensic recovery is preserved; preflight finds the file at the recorded path.
_archive_if_needed() {
    local ws="$1" cycle="$2"
    local ledger="$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl"
    [ -d "$ws" ] || return 0
    [ -f "$ledger" ] || return 0
    # Cheap predicate: does any agent_subprocess line reference this workspace?
    if grep -q '"artifact_path":"'"$ws"'/' "$ledger" 2>/dev/null; then
        local archive_base="$EVOLVE_PROJECT_ROOT/.evolve/runs/archive"
        local ts
        ts=$(date -u +%Y%m%dT%H%M%SZ)
        local dest="$archive_base/cycle-${cycle}-${ts}"
        mkdir -p "$archive_base" 2>/dev/null || true
        if mv "$ws" "$dest" 2>/dev/null; then
            log "workspace archived to $dest (ledger references preserved)"
            return 0
        else
            log "WARN: archive mv failed; falling back to rm -rf (ledger references will be orphaned)"
        fi
    fi
    return 0
}
_archive_if_needed "$WORKSPACE" "$CYCLE"
rm -rf "$WORKSPACE"
mkdir -p "$WORKSPACE"
# Sentinel marker for cycle-scoped eval detection (cycle 21 fix).
# phase-gate.sh's mutation gate uses `find .evolve/evals -newer <marker>` to
# scope mutation testing to files created during this cycle. Without this,
# `git ls-files --others --exclude-standard` returned empty under .evolve/*
# gitignore (cycle-19 WARN); removing --exclude-standard returned all 277
# existing evals (rejected fix). The marker approach is precise and cheap.
touch "$WORKSPACE/.cycle-start-marker"
log "workspace=$WORKSPACE (cleared for fresh cycle-init)"

# v8.29.0: register cleanup trap BEFORE cycle_state_init.
# Pre-v8.29.0, the trap was set ~117 lines later, so any failure in worktree
# provisioning (lines 246-326) left cycle-state.json orphaned — the next
# dispatch would fail with "INTEGRITY-FAIL: cycle-state.json already exists
# for cycle N" until manual `cycle-state.sh clear`. Reproduced 3× this
# session when worktree-add hit "branch already exists".
WORKTREE_PATH=""
WORKTREE_BRANCH=""
WORKTREE_PROVISIONED=0
cleanup() {
    local rc=$?

    # v9.1.0 Cycle 1: checkpoint-aware cleanup. If a phase wrote a checkpoint
    # block to cycle-state.json (via cycle-state.sh checkpoint <reason>), or
    # the dispatcher signaled EVOLVE_CHECKPOINT_TRIGGERED=1 due to reactive
    # quota-likely failure classification, PRESERVE the worktree + state so
    # `resume-cycle.sh` can pick up exactly where this cycle paused. The
    # legacy cleanup path is the default — only an explicit checkpoint
    # flips us into preserve mode.
    local checkpointed=0
    if bash "$CYCLE_STATE_HELPER" is-checkpointed >/dev/null 2>&1; then
        checkpointed=1
    elif [ "${EVOLVE_CHECKPOINT_TRIGGERED:-0}" = "1" ]; then
        # The phase aborted via reactive classification but never got a chance
        # to write the checkpoint block; do it on its behalf so resume works.
        bash "$CYCLE_STATE_HELPER" checkpoint "quota-likely" 2>/dev/null && checkpointed=1
    fi

    # v8.60.0: hoist gitignored baseline artifacts from worktree to project root
    # BEFORE worktree removal. Worktree cleanup destroys .evolve/baselines/*.json
    # since they are gitignored (not committed). Skip when checkpointed —
    # the worktree stays around, so hoisting is unnecessary (and the next
    # resume run will see the same baselines in-place).
    if [ "$checkpointed" = "0" ] && [ "$WORKTREE_PROVISIONED" = "1" ] && [ -d "${WORKTREE_PATH:-}/.evolve/baselines" ]; then
        local dst="$EVOLVE_PROJECT_ROOT/.evolve/baselines"
        mkdir -p "$dst" 2>/dev/null || true
        for f in "$WORKTREE_PATH/.evolve/baselines/"*.json; do
            [ -f "$f" ] || continue
            base=$(basename "$f")
            if cp "$f" "$dst/$base.tmp.$$" 2>/dev/null && mv -f "$dst/$base.tmp.$$" "$dst/$base" 2>/dev/null; then
                log "baseline hoisted: $base → $dst/$base"
            else
                rm -f "$dst/$base.tmp.$$" 2>/dev/null
                log "WARN: could not hoist $base to $dst"
            fi
        done
    fi

    if [ "$checkpointed" = "1" ]; then
        log "[run-cycle] CHECKPOINT: worktree + state preserved at ${WORKTREE_PATH:-<none>}; resume with --resume"
        log "[run-cycle] preserved cycle-state at .evolve/cycle-state.json (phase=$(bash "$CYCLE_STATE_HELPER" resume-phase 2>/dev/null))"
        # Do NOT remove worktree, do NOT clear cycle-state, do NOT delete branch.
        # The dispatcher's caller (`resume-cycle.sh`) will own that lifecycle.
        exit $rc
    fi

    if [ "$WORKTREE_PROVISIONED" = "1" ]; then
        if [ -d "$WORKTREE_PATH" ]; then
            log "cleanup: removing worktree $WORKTREE_PATH"
            git -C "$EVOLVE_PROJECT_ROOT" worktree remove --force "$WORKTREE_PATH" 2>/dev/null \
                || rm -rf "$WORKTREE_PATH"
        fi
        # v8.36.0: defensive prune. If worktree-remove silently failed (e.g., the
        # directory was already gone but admin entry remained), this catches it
        # so the next cycle's pre-flight starts clean.
        git -C "$EVOLVE_PROJECT_ROOT" worktree prune 2>/dev/null || true
        if [ -n "$WORKTREE_BRANCH" ]; then
            git -C "$EVOLVE_PROJECT_ROOT" branch -D "$WORKTREE_BRANCH" 2>/dev/null || true
        fi
    elif [ "${EVOLVE_SKIP_WORKTREE:-0}" = "1" ]; then
        local dirty_files
        dirty_files=$(git -C "$EVOLVE_PROJECT_ROOT" status --porcelain 2>/dev/null | wc -l | tr -d ' ')
        if [ "$dirty_files" -gt 0 ]; then
            log "WARN: EVOLVE_SKIP_WORKTREE=1 — main repo has $dirty_files changed file(s) from this cycle"
            log "  → Run \`git status\` and \`git diff\` to inspect"
            log "  → \`git restore .\` to discard, or commit/stash to keep"
        fi
    fi
    # v9.5.1: stop the watchdog (if any) before clearing cycle-state.
    if [ -n "${WATCHDOG_PID:-}" ]; then
        kill -TERM "$WATCHDOG_PID" 2>/dev/null || true
        wait "$WATCHDOG_PID" 2>/dev/null || true
    fi
    bash "$CYCLE_STATE_HELPER" clear 2>/dev/null || true
    log "cycle-state cleared (rc=$rc)"
    # v9.5.1: handle watchdog-fired stall exit code.
    if [ "$rc" -eq 140 ]; then
        log "STALL-INACTIVITY: watchdog fired rc=140 — checkpoint written for --resume"
    fi
    exit $rc
}
trap cleanup EXIT INT TERM

# v9.1.0 Cycle 4: resume mode — if the dispatcher invoked us with
# EVOLVE_RESUME_MODE=1, the cycle-state.json + worktree from the paused
# cycle are already on disk. Skip the init and worktree-provision blocks
# entirely and hand off to the orchestrator with the resume env vars set.
if [ "${EVOLVE_RESUME_MODE:-0}" = "1" ]; then
    log "RESUME-MODE: skipping cycle_state_init (preserving paused cycle's state)"
    if ! bash "$CYCLE_STATE_HELPER" is-checkpointed >/dev/null 2>&1; then
        fail "RESUME-MODE: cycle-state.json has no checkpoint block — was the pause cleared?"
    fi
    # Re-read the cycle number from the preserved cycle-state.
    RESUMED_CYCLE=$(bash "$CYCLE_STATE_HELPER" get cycle_id 2>/dev/null || echo "")
    if [ -z "$RESUMED_CYCLE" ]; then
        # Fallback: read directly from the JSON in case the get subcommand
        # doesn't support cycle_id on older state.
        STATE_FILE_PATH="$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json"
        RESUMED_CYCLE=$(jq -r '.cycle_id // .cycle // empty' "$STATE_FILE_PATH" 2>/dev/null || echo "")
        unset STATE_FILE_PATH
    fi
    [ -n "$RESUMED_CYCLE" ] || fail "RESUME-MODE: cannot read cycle_id from cycle-state.json"
    CYCLE="$RESUMED_CYCLE"
    WORKSPACE=".evolve/runs/cycle-$CYCLE"
    log "RESUME-MODE: cycle=$CYCLE workspace=$WORKSPACE resume_phase=${EVOLVE_RESUME_PHASE:-?}"
    # Pull the preserved worktree path back into env so the orchestrator can
    # find it. Mark as NOT-provisioned-by-this-process so the EXIT trap won't
    # try to clean up something it didn't create (resume-cycle.sh owns the
    # lifecycle of the paused worktree).
    WORKTREE_PATH=$(jq -r '.active_worktree // empty' "$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json" 2>/dev/null || echo "")
    WORKTREE_BRANCH=""
    WORKTREE_PROVISIONED=0
    [ -n "$WORKTREE_PATH" ] && export WORKTREE_PATH
    # Skip the normal init + worktree provision below.
    SKIP_NORMAL_INIT=1
else
    SKIP_NORMAL_INIT=0
fi

if [ "$SKIP_NORMAL_INIT" = "0" ]; then
    # Initialize cycle-state.json (phase=calibrate, no agent yet).
    bash "$CYCLE_STATE_HELPER" init "$CYCLE" ".evolve/runs/cycle-$CYCLE" \
        || fail "cycle_state_init failed"
    log "cycle-state.json initialized at phase=calibrate"
fi

# v8.21.0: privileged-shell worktree provisioning — closes the trust-boundary
# gap where the orchestrator profile (correctly) denies `git worktree add` but
# nothing else provisioned the worktree, leaving cycle-state.active_worktree
# null and the builder's sandbox profile expanding {worktree_path} to empty.
# This block runs BEFORE the orchestrator subprocess so the worktree is ready
# by the time the build phase starts. The orchestrator and all phase agents
# may NOT call `git worktree add/remove` — only this privileged shell context.
#
# v9.1.0 Cycle 4 ADDENDUM (post-ship discovery, 2026-05-11): in resume mode the
# RESUME-MODE branch above already populated WORKTREE_PATH from the preserved
# cycle-state.json (line 451). Skipping the reset preserves that value so the
# downstream `re-using paused cycle's worktree at $WORKTREE_PATH` log is
# accurate AND the orchestrator subagent can find the worktree.
if [ "$SKIP_NORMAL_INIT" != "1" ]; then
    WORKTREE_PATH=""
    WORKTREE_BRANCH=""
    WORKTREE_PROVISIONED=0
fi

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
if [ "$SKIP_NORMAL_INIT" = "1" ]; then
    log "RESUME-MODE: skipping worktree provision (re-using paused cycle's worktree at $WORKTREE_PATH)"
elif [ "${EVOLVE_SKIP_WORKTREE:-0}" = "1" ]; then
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
    # v8.25.0: worktree base is selected by preflight-environment.sh and
    # exported as EVOLVE_WORKTREE_BASE by the dispatcher. Falls back to the
    # legacy in-project location if the env var is unset (direct run-cycle
    # invocation without a preflight pass, or test harnesses that don't
    # invoke the dispatcher). The fallback preserves backward compatibility.
    if [ -n "${EVOLVE_WORKTREE_BASE:-}" ]; then
        WORKTREE_BASE="$EVOLVE_WORKTREE_BASE"
    else
        WORKTREE_BASE="$EVOLVE_PROJECT_ROOT/.evolve/worktrees"
    fi
    WORKTREE_PATH="$WORKTREE_BASE/cycle-$CYCLE"
    WORKTREE_BRANCH="evolve/cycle-$CYCLE"
    mkdir -p "$WORKTREE_BASE" 2>/dev/null \
        || fail "cannot create worktree base $WORKTREE_BASE — set EVOLVE_WORKTREE_BASE to a writable path"

    # Idempotent: clean a stale worktree from a prior cycle with the same id
    # (typically a hard-killed run that didn't reach the cleanup trap).
    if git -C "$EVOLVE_PROJECT_ROOT" worktree list --porcelain 2>/dev/null \
         | grep -q "^worktree $WORKTREE_PATH$"; then
        log "removing stale worktree at $WORKTREE_PATH"
        git -C "$EVOLVE_PROJECT_ROOT" worktree remove --force "$WORKTREE_PATH" 2>/dev/null || true
    fi
    [ -d "$WORKTREE_PATH" ] && rm -rf "$WORKTREE_PATH"
    # v8.36.0: prune stale worktree admin entries BEFORE branch deletion. Pre-v8.36.0,
    # if a prior cycle was hard-killed at a different worktree path (typical in
    # nested-Claude where TMPDIR changes per session), .git/worktrees/<name>/ retained
    # an admin pointer whose directory no longer exists. `git branch -D` then silently
    # no-ops on the still-"checked-out" branch, and `git worktree add` fails with
    # "branch already exists". Pruning frees the branch.
    git -C "$EVOLVE_PROJECT_ROOT" worktree prune 2>/dev/null || true
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

# v8.29.0: cleanup() and trap moved to BEFORE cycle_state_init (line ~244).
# This block is intentionally empty — kept as a comment anchor for the audit
# trail of the pre-v8.29.0 location. Anything that needs to run between
# worktree provisioning and orchestrator spawn should be added below this
# anchor.

# v9.5.1: activity-based phase watchdog. Spawned here (after worktree
# provisioning, before orchestrator) so it covers all phase subagents from
# the first token. Killed in cleanup() to avoid orphaned background processes.
WATCHDOG_PID=""
if [ "${EVOLVE_INACTIVITY_DISABLE:-0}" != "1" ]; then
    RUN_PGID=$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ' || echo "")
    if [ -n "$RUN_PGID" ] && [[ "$RUN_PGID" =~ ^[0-9]+$ ]]; then
        CYCLE_STATE_PATH_FOR_WD="$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json"
        # v9.5.0: when EVOLVE_OBSERVER_ENFORCE=1, replace the legacy watchdog
        # with the phase-observer at --scope=cycle. Same kill semantics + the
        # unified envelope format. Default 0 preserves v9.4.0 behavior.
        if [ "${EVOLVE_OBSERVER_ENFORCE:-0}" = "1" ]; then
            bash "$EVOLVE_PLUGIN_ROOT/scripts/dispatch/phase-observer.sh" \
                --enforce --scope=cycle \
                "$WORKSPACE" "$RUN_PGID" "$CYCLE" "orchestrator" "orchestrator" \
                "$CYCLE_STATE_PATH_FOR_WD" &
            WATCHDOG_PID=$!
            log "phase-observer (cycle-scope, --enforce) spawned (pid=$WATCHDOG_PID pgid=$RUN_PGID threshold=${EVOLVE_OBSERVER_STALL_S:-${EVOLVE_INACTIVITY_THRESHOLD_S:-240}}s)"
        else
            bash "$EVOLVE_PLUGIN_ROOT/scripts/dispatch/phase-watchdog.sh" \
                "$WORKSPACE" "$RUN_PGID" "$CYCLE" "$CYCLE_STATE_PATH_FOR_WD" &
            WATCHDOG_PID=$!
            log "watchdog spawned (pid=$WATCHDOG_PID pgid=$RUN_PGID threshold=${EVOLVE_INACTIVITY_THRESHOLD_S:-240}s)"
        fi
    else
        log "WARN: could not determine PGID — watchdog not spawned"
    fi
fi

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
    log "  PROMPT_FILE_OVERRIDE=$PROMPT_FILE bash scripts/dispatch/subagent-run.sh orchestrator $CYCLE $WORKSPACE"
    log "cycle-state snapshot before EXIT trap clears it:"
    bash "$CYCLE_STATE_HELPER" dump | jq . >&2 || true
    # v8.21.0: let the EXIT trap fire naturally — it tears down both the
    # worktree (if provisioned) and cycle-state.json. Set EVOLVE_DRY_RUN_PROVISION_WORKTREE=0
    # to skip worktree provisioning entirely in dry-run.
    exit 0
fi

# ---- Simulate? -------------------------------------------------------------
# v8.50.0: --simulate uses cycle-simulator.sh in place of the real orchestrator.
# The simulator advances cycle-state through every phase, writes deterministic
# artifacts, appends ledger entries, and invokes ship.sh --dry-run. No LLM
# calls. This validates the cycle plumbing end-to-end without spending tokens.

if [ "$SIMULATE" = "1" ]; then
    SIMULATOR="$EVOLVE_PLUGIN_ROOT/scripts/dispatch/cycle-simulator.sh"
    [ -f "$SIMULATOR" ] || fail "missing $SIMULATOR (cycle-simulator.sh)"
    log "simulate mode: invoking cycle-simulator.sh (no LLM)"
    set +e
    bash "$SIMULATOR" "$CYCLE" "$WORKSPACE"
    rc=$?
    set -e
    log "simulator exited rc=$rc"
    exit "$rc"
fi

# ---- Spawn orchestrator ----------------------------------------------------

log "spawning orchestrator subagent for cycle $CYCLE..."

set +e
PROMPT_FILE_OVERRIDE="$PROMPT_FILE" bash "$SUBAGENT_RUN" orchestrator "$CYCLE" "$WORKSPACE"
rc=$?
set -e

# ---- Summary ---------------------------------------------------------------

log "orchestrator subagent exited rc=$rc"

if [ "$rc" -eq 140 ]; then
    log "STALL-INACTIVITY: watchdog fired during cycle $CYCLE — checkpoint written for --resume"
fi

if [ -f "$WORKSPACE/orchestrator-report.md" ]; then
    log "orchestrator report at: $WORKSPACE/orchestrator-report.md"
    head -30 "$WORKSPACE/orchestrator-report.md" >&2 || true
else
    log "WARN: no orchestrator-report.md produced"
fi

# v10.5.0: Phase-B cycle-end rollup — aggregate per-phase sidecars into a
# single cycle-metrics.json under .ephemeral/metrics/. Gated by
# EVOLVE_TRACKER_ENABLED (default OFF). Best-effort: a rollup fault never
# changes the cycle exit code. Runs even on FAIL/STALL exits because the
# observability data for a failed cycle is the most valuable diagnostic.
if [ "${EVOLVE_TRACKER_ENABLED:-0}" = "1" ]; then
    ROLLUP_SH="$EVOLVE_PLUGIN_ROOT/scripts/observability/rollup-cycle-metrics.sh"
    if [ -x "$ROLLUP_SH" ]; then
        bash "$ROLLUP_SH" "$CYCLE" >/dev/null 2>>"$WORKSPACE/rollup.stderr.log" \
            && log "OK: cycle-metrics rollup written" \
            || log "WARN: cycle-metrics rollup rc=$? (non-fatal)"
    fi
fi

exit "$rc"
