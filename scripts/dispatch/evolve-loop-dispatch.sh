#!/usr/bin/env bash
#
# evolve-loop-dispatch.sh — Strict dispatcher for /evolve-loop (v8.13.7).
#
# WHY THIS EXISTS
#
# Pre-v8.13.7, the /evolve-loop skill was a *prompt-driven* loop: SKILL.md
# described the phase sequence (Scout → Builder → Auditor → Ship → Learn) and
# relied on the assistant to honor it. The 2026-04-29 flow audit showed that
# under prompt-driven orchestration, the assistant routinely shortcuts:
# decomposes the goal into TodoWrite items, edits source directly, and skips
# the Scout/Builder subagents — bypassing the v8.13.x trust-boundary
# architecture entirely. The kernel hooks (role-gate, ship-gate, phase-gate-
# precondition) only fire on `subagent-run.sh` and forbidden Edit/Write paths;
# they cannot force the orchestrator to invoke `subagent-run.sh` in the first
# place.
#
# This dispatcher is the structural fix. When /evolve-loop is invoked, the
# skill executes EXACTLY ONE bash command: this script. The dispatcher then:
#
#   1. Parses cycles + strategy + goal from positional args.
#   2. Loops `bash scripts/dispatch/run-cycle.sh "<goal>"` once per cycle.
#   3. After each cycle, asserts the ledger contains a fresh
#      agent_subprocess entry for scout, builder, AND auditor.
#   4. If any cycle is missing entries, halts the batch with a loud error.
#
# Because run-cycle.sh spawns a profile-restricted orchestrator subagent
# (which itself goes through phase-gate-precondition for each subagent_run
# invocation), every cycle going through this dispatcher is structurally
# guaranteed to follow the Scout → Builder → Auditor sequence. The verify-
# ledger step is belt-and-suspenders: even if a future regression weakens
# run-cycle.sh, the dispatcher catches "the orchestrator shortcut" loud.
#
# USAGE
#
#   bash scripts/dispatch/evolve-loop-dispatch.sh [CYCLES] [STRATEGY] [GOAL...]
#   bash scripts/dispatch/evolve-loop-dispatch.sh --dry-run [args...]
#   bash scripts/dispatch/evolve-loop-dispatch.sh --help
#
# ARGS
#
#   CYCLES         integer >= 1 (default 2)
#   STRATEGY       one of: balanced|innovate|harden|repair|ultrathink|autoresearch
#                  (default: balanced; matched only if first non-numeric token
#                  matches one of these literals)
#   GOAL           remaining positional args, joined by spaces (default empty
#                  → autonomous, picks from CLAUDE.md priorities)
#
# ENV OVERRIDES (mostly for tests)
#
#   VALIDATE_ONLY=1            — parse args, print plan, exit 0 (no run-cycle invocation)
#   RUN_CYCLE_OVERRIDE=<path>  — alternative run-cycle.sh path (for tests)
#   LEDGER_OVERRIDE=<path>     — alternative ledger path (for tests)
#   STATE_OVERRIDE=<path>      — alternative state.json path (for tests)
#   EVOLVE_DISPATCH_VERIFY=0   — skip the per-cycle ledger verification
#                                 (legacy escape hatch; logs WARN; not for prod)
#
# EXIT CODES
#
#    0 — all cycles ran and verified clean
#    1 — runtime failure (missing prerequisite, bad arg, run-cycle.sh failed)
#    2 — INTEGRITY BREACH (orchestrator shortcut; STOP)
#    3 — batch completed but one or more cycles had recoverable failures
#         (infrastructure / audit-fail / build-fail) recorded to failedApproaches
#   10 — bad arguments (validation failure before any cycle ran)
#
# FAILURE CLASSIFICATION (v8.16.1+)
#
# When verify_cycle detects missing ledger entries, the dispatcher reads the
# cycle's orchestrator-report.md to classify the failure:
#
#   infrastructure  — sandbox-exec EPERM, rate limit, timeout, network
#   audit-fail      — cycle ran but Auditor verdict was FAIL/WARN
#   build-fail      — Builder couldn't turn tests GREEN
#   integrity-breach — orchestrator-report.md missing or doesn't explain why
#                      a phase didn't run (potential silent shortcut)
#
# Recoverable classifications: dispatcher records failure to
# state.json:failedApproaches[] and continues to the next cycle. The next
# cycle's orchestrator reads failedApproaches and adapts (smaller scope,
# alternative path, escalation).
#
# integrity-breach: STOP (exit 2) — requires human investigation.
#
# Override via EVOLVE_DISPATCH_STOP_ON_FAIL=1 to restore old fail-fast.

set -uo pipefail

# v8.18.0: dual-root resolution — see resolve-roots.sh.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"
unset __rr_self

# v8.20.0: PATH-based kernel script invocation. Prepend plugin's scripts dir
# so subagents can invoke kernel scripts by bare name. Eliminates the
# install-layout-fragile path-pattern enumeration in orchestrator/auditor
# allowlists. Inherits to claude -p subprocess via env propagation.
export PATH="$EVOLVE_PLUGIN_ROOT/scripts:$EVOLVE_PLUGIN_ROOT/scripts/dispatch:$EVOLVE_PLUGIN_ROOT/scripts/lifecycle:$EVOLVE_PLUGIN_ROOT/scripts/failure:$EVOLVE_PLUGIN_ROOT/scripts/observability:$EVOLVE_PLUGIN_ROOT/scripts/verification:$EVOLVE_PLUGIN_ROOT/scripts/utility:$EVOLVE_PLUGIN_ROOT/scripts/release:$EVOLVE_PLUGIN_ROOT/scripts/tests:$PATH"

# Read-only: run-cycle.sh ships with the plugin
RUN_CYCLE="${RUN_CYCLE_OVERRIDE:-$EVOLVE_PLUGIN_ROOT/scripts/dispatch/run-cycle.sh}"
# Writable: ledger, state, runs/ live in the user's project (or evolve-loop in dev)
LEDGER="${LEDGER_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/ledger.jsonl}"
STATE_FILE="${STATE_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/state.json}"
RUNS_DIR="${RUNS_DIR_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/runs}"

log()  { echo "[dispatch] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }
abort_args() { log "BAD-ARG: $*"; exit 10; }

# --- v8.18.1: pre-flight environment guards ---------------------------------
#
# Two failure modes exposed by the 2026-05-03 plugin-cwd incident:
#
# 1. Operator cd's into the plugin install directory before invoking. With
#    dual-root resolution (v8.18.0), EVOLVE_PROJECT_ROOT then resolves to the
#    plugin source tree itself, and cycles spin up against the wrong project.
#    Symptom: $0.57 spent on calibration before orchestrator notices the
#    goal-project mismatch.
#
# 2. claude -p drops --bare when ANTHROPIC_API_KEY is absent, which collapses
#    the orchestrator's profile-scoped permissions back to main-session prompts
#    and silently blocks subagent writes. Symptom: integrity-fail because no
#    artifact was persisted.
#
# Both are caught here at the dispatcher's earliest moment so the operator
# pays $0 instead of $0.57+.
#
# Tests bypass these via:
#   - RUN_CYCLE_OVERRIDE        — implies test mode (substitute mock run-cycle)
#   - EVOLVE_ALLOW_INTERACTIVE_FALLBACK=1 — explicit operator opt-in to running
#                                  in interactive Claude Code without API key
#                                  (degraded but supported scenario)
#
# The cwd guard fires unconditionally because it indicates an operator mistake
# even in dry-run / VALIDATE_ONLY mode (you don't want to validate a plan
# pointed at the wrong directory).

case "$EVOLVE_PROJECT_ROOT" in
    */plugins/cache/*|*/plugins/marketplaces/*)
        log "BAD-ARG: cwd is a plugin install directory ($EVOLVE_PROJECT_ROOT)"
        log "         Plugin installs are not valid project workspaces."
        log "         FIX: cd to your actual project, then run /evolve-loop"
        log "         Or:  EVOLVE_PROJECT_ROOT=/path/to/project bash <plugin>/scripts/dispatch/evolve-loop-dispatch.sh ..."
        exit 10
        ;;
esac

# --- v8.25.0: capability-detection pre-flight -------------------------------
#
# Replaces the prior nested-Claude auto-detection block (v8.22.0/v8.24.0).
# Runs scripts/dispatch/preflight-environment.sh which probes the host once and emits
# a structured JSON capability profile. The dispatcher reads auto_config and
# applies the flags it recommends, with operator override via direct edit
# of $EVOLVE_PROJECT_ROOT/.evolve/environment.json.
#
# Why this design:
#   - Discoverable: ONE file (.evolve/environment.json) replaces 6+ env vars
#   - Observable: profile is human-readable, version-controllable
#   - Anti-gaming preserved: probe runs in privileged shell; profile is read-
#     only to phase agents (deny-listed in profiles); Tier-1 kernel hooks
#     verify behavior post-execution regardless of profile contents
#
# Override mechanics:
#   1. If the operator already set EVOLVE_SANDBOX_FALLBACK_ON_EPERM /
#      EVOLVE_SKIP_WORKTREE explicitly (any value, including 0), the dispatcher
#      respects the operator's choice and does NOT overwrite from the profile.
#   2. Otherwise the profile's auto_config wins.
#   3. Power-users can edit .evolve/environment.json:auto_config directly.
PREFLIGHT_SCRIPT="$EVOLVE_PLUGIN_ROOT/scripts/dispatch/preflight-environment.sh"
if [ -x "$PREFLIGHT_SCRIPT" ]; then
    PROFILE_JSON=$(bash "$PREFLIGHT_SCRIPT" --write 2>/dev/null || echo "")
    if [ -n "$PROFILE_JSON" ]; then
        env_summary=$(echo "$PROFILE_JSON" | jq -r \
            '"\(.host.os) \(.host.os_version), nested-claude=\(.claude_code.nested), sandbox-works=\(.sandbox.expected_to_work)"' 2>/dev/null || echo "unparseable")
        log "ENVIRONMENT: $env_summary"
        env_reason=$(echo "$PROFILE_JSON" | jq -r '.auto_config.reasoning // ""' 2>/dev/null)
        [ -n "$env_reason" ] && log "  → $env_reason"

        auto_eperm=$(echo "$PROFILE_JSON" | jq -r '.auto_config.EVOLVE_SANDBOX_FALLBACK_ON_EPERM // "0"' 2>/dev/null)
        auto_wt_base=$(echo "$PROFILE_JSON" | jq -r '.auto_config.worktree_base // ""' 2>/dev/null)

        if [ -z "${EVOLVE_SANDBOX_FALLBACK_ON_EPERM:-}" ]; then
            if [ "$auto_eperm" = "1" ]; then
                log "  → auto-enabling EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 (from environment.json)"
                export EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1
            fi
        else
            log "  → operator set EVOLVE_SANDBOX_FALLBACK_ON_EPERM=$EVOLVE_SANDBOX_FALLBACK_ON_EPERM (override profile)"
        fi

        # v8.25.0: per-cycle worktree relocation (replaces auto-SKIP_WORKTREE).
        # Worktrees go to a sandbox-friendly path (typically $TMPDIR) so we
        # KEEP per-cycle isolation instead of skipping it. Operator override
        # via EVOLVE_WORKTREE_BASE.
        if [ -z "${EVOLVE_WORKTREE_BASE:-}" ]; then
            if [ -n "$auto_wt_base" ]; then
                log "  → worktree_base: $auto_wt_base"
                export EVOLVE_WORKTREE_BASE="$auto_wt_base"
            else
                log "FAIL: no writable worktree base could be selected. See .evolve/environment.json"
                log "      Operator must either:"
                log "        - Set EVOLVE_WORKTREE_BASE=<writable-dir> and re-run, OR"
                log "        - Fix permissions on \$TMPDIR / ~/.cache, OR"
                log "        - Run from a shell with broader permissions"
                log "      Last-resort (loses per-cycle isolation): EVOLVE_SKIP_WORKTREE=1 bash $0 ..."
                exit 1
            fi
        else
            log "  → operator set EVOLVE_WORKTREE_BASE=$EVOLVE_WORKTREE_BASE (override profile)"
        fi

        # SKIP_WORKTREE is no longer auto-enabled. v8.25.0 makes it a true
        # emergency operator-only flag. If operator explicitly set it, log
        # a loud warning so they know they're losing isolation.
        if [ "${EVOLVE_SKIP_WORKTREE:-0}" = "1" ]; then
            log "  → WARN: EVOLVE_SKIP_WORKTREE=1 (operator-set)"
            log "         Per-cycle worktree isolation DISABLED. Builder edits land directly"
            log "         in \$EVOLVE_PROJECT_ROOT. This is the v8.24-era behavior; v8.25.0+"
            log "         prefers worktree relocation (EVOLVE_WORKTREE_BASE) instead."
            log "         You probably want to UNSET this flag and let the new default work."
        fi

        unset auto_eperm auto_wt_base env_summary env_reason PROFILE_JSON
    else
        log "WARN: preflight-environment.sh failed; falling back to legacy detect-nested-claude.sh"
        # Legacy fallback: when preflight script is broken, set sandbox-fallback
        # only and pick a TMPDIR worktree base inline so cycles can still run.
        # This path is rare (only fires if jq is missing or preflight crashes).
        if [ -x "$EVOLVE_PLUGIN_ROOT/scripts/dispatch/detect-nested-claude.sh" ] && \
           [ "$(bash "$EVOLVE_PLUGIN_ROOT/scripts/dispatch/detect-nested-claude.sh")" = "nested" ]; then
            [ -z "${EVOLVE_SANDBOX_FALLBACK_ON_EPERM:-}" ] && export EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1
            if [ -z "${EVOLVE_WORKTREE_BASE:-}" ] && [ -n "${TMPDIR:-}" ]; then
                fallback_hash=$(printf '%s' "$EVOLVE_PROJECT_ROOT" | shasum -a 256 2>/dev/null | head -c 8 || echo "default")
                export EVOLVE_WORKTREE_BASE="${TMPDIR%/}/evolve-loop/$fallback_hash"
                log "  → legacy fallback: EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1, EVOLVE_WORKTREE_BASE=$EVOLVE_WORKTREE_BASE"
                unset fallback_hash
            else
                log "  → legacy fallback: EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 (no TMPDIR; worktree stays in-project)"
            fi
        fi
    fi
fi
unset PREFLIGHT_SCRIPT

# --- Argument parsing -------------------------------------------------------

DRY_RUN=0
RESET_FAILURES=0
RESUME_MODE=0
CYCLES=""
STRATEGY=""
GOAL=""

# Pull off --flags first so positional parsing doesn't see them.
POSITIONAL=()
# v8.60.0 Layer 1: budget-driven dispatch flags. --budget makes batch budget the
# primary stop condition; --cycles is the explicit cycle-count form. Positional
# integer (legacy) still parses as cycles but emits a deprecation WARN — v8.62
# will flip positional integer semantics to dollars.
BUDGET=""
LEGACY_POSITIONAL_USED=0
while [ $# -gt 0 ]; do
    case "$1" in
        --budget-usd|--budget)
            shift
            [ $# -gt 0 ] || abort_args "--budget-usd requires a dollar value"
            BUDGET="$1"
            shift
            ;;
        --cycles)
            shift
            [ $# -gt 0 ] || abort_args "--cycles requires an integer"
            CYCLES="$1"
            shift
            ;;
        --dry-run)
            DRY_RUN=1
            shift
            ;;
        --reset)
            # v8.27.0: operator-driven recovery from BLOCKED-SYSTEMIC and
            # accumulated transient entries. Prunes infrastructure-{systemic,
            # transient} and ship-gate-config from state.json:failedApproaches[]
            # before the cycle loop starts. Logs loudly so the operator's
            # choice is auditable. Does NOT bypass any kernel hook — Tier-1
            # phase-gate, ledger SHA, role-gate, ship-gate all stay enforced.
            RESET_FAILURES=1
            shift
            ;;
        --consensus-audit)
            # v8.54.0+: opt-in cross-CLI auditor consensus. Sets
            # EVOLVE_CONSENSUS_AUDIT=1 which the orchestrator reads to dispatch
            # the audit phase via consensus-dispatch.sh (cross-cli-vote merge
            # mode) instead of standard cmd_dispatch_parallel. Cost is N x
            # audit budget where N = profile.consensus.cli_voters length.
            # See docs/architecture/multi-llm-review.md for protocol.
            export EVOLVE_CONSENSUS_AUDIT=1
            shift
            ;;
        --resume)
            # v9.1.0 Cycle 4: locate the most recent checkpointed cycle and
            # pick up from its resumeFromPhase. Delegates to resume-cycle.sh
            # which runs ONE cycle from the resume point; control returns
            # here and the normal batch loop continues if more cycles remain
            # in the budget.
            RESUME_MODE=1
            shift
            ;;
        --help|-h)
            sed -n '2,55p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --*)
            abort_args "unknown flag: $1"
            ;;
        *)
            POSITIONAL+=("$1")
            shift
            ;;
    esac
done

# Now consume positional in the documented order:
# [CYCLES] [STRATEGY] [GOAL...]
# v8.60.0 Layer 1: positional integer is still parsed as CYCLES (legacy) when no
# --cycles or --budget flag is set. Emits a deprecation WARN; v8.62 will flip
# positional integer semantics to dollars.
if [ "${#POSITIONAL[@]}" -gt 0 ]; then
    if [[ "${POSITIONAL[0]}" =~ ^[0-9]+$ ]]; then
        if [ -z "$CYCLES" ] && [ -z "$BUDGET" ]; then
            CYCLES="${POSITIONAL[0]}"
            LEGACY_POSITIONAL_USED=1
        fi
        POSITIONAL=("${POSITIONAL[@]:1}")
    fi
fi
if [ "${#POSITIONAL[@]}" -gt 0 ]; then
    case "${POSITIONAL[0]}" in
        balanced|innovate|harden|repair|ultrathink|autoresearch)
            STRATEGY="${POSITIONAL[0]}"
            POSITIONAL=("${POSITIONAL[@]:1}")
            ;;
    esac
fi
# Anything left is the goal.
if [ "${#POSITIONAL[@]}" -gt 0 ]; then
    GOAL="${POSITIONAL[*]}"
fi

# v8.60.0 Layer 1: budget-mode setup. When --budget is set, CYCLES becomes a
# safety upper bound (default 50) and the cumulative-cost tripwire is the
# primary stop. Otherwise CYCLES applies its normal default of 2.
if [ -n "$BUDGET" ]; then
    # Validate budget is a positive dollar value (allow 50, 50.00, .50)
    case "$BUDGET" in
        ''|*[!0-9.]*) abort_args "--budget must be a positive number (got: $BUDGET)" ;;
    esac
    # Default CYCLES in budget mode is the safety upper bound.
    [ -n "$CYCLES" ] || CYCLES="${EVOLVE_BUDGET_MAX_CYCLES:-50}"
fi

# Apply defaults.
[ -n "$CYCLES" ]   || CYCLES=2
[ -n "$STRATEGY" ] || STRATEGY=balanced

# v8.60.0 Layer 1: emit deprecation WARN when legacy positional integer was
# used (no --cycles or --budget flag). v9.0.5: refreshed the deprecation
# target — the original v8.62 flip milestone got skipped when development
# jumped to v9.0.0. The flip is now a v10.0.0 candidate (breaking change;
# warrants a major-version-bump signal).
if [ "$LEGACY_POSITIONAL_USED" = "1" ]; then
    log "DEPRECATION: positional integer means cycles (since v8.60); v10.0.0 candidate will consider flipping to dollars — migrate to --cycles N or --budget-usd N now to be flip-safe"
fi

# Validate.
[[ "$CYCLES" =~ ^[0-9]+$ ]] || abort_args "CYCLES must be a non-negative integer (got: $CYCLES)"
[ "$CYCLES" -ge 1 ]         || abort_args "CYCLES must be >= 1 (got: $CYCLES)"
case "$STRATEGY" in
    balanced|innovate|harden|repair|ultrathink|autoresearch) ;;
    *) abort_args "STRATEGY must be one of: balanced|innovate|harden|repair|ultrathink|autoresearch (got: $STRATEGY)" ;;
esac

# --- Plan ---------------------------------------------------------------------

if [ -n "$BUDGET" ]; then
    log "PLAN: BUDGET=\$$BUDGET CYCLES=$CYCLES (safety upper bound) strategy=$STRATEGY mode=budget-mode goal='${GOAL:-<autonomous>}'"
else
    log "PLAN: CYCLES=$CYCLES strategy=$STRATEGY mode=cycles-mode goal='${GOAL:-<autonomous>}'"
fi
log "PLAN: run_cycle=$RUN_CYCLE"
log "PLAN: ledger=$LEDGER"
log "PLAN: dispatch_policy=${EVOLVE_DISPATCH_POLICY:-verify}$([ -n "${EVOLVE_DISPATCH_VERIFY:-}" ] && echo " (DEPRECATED EVOLVE_DISPATCH_VERIFY set)" || true)$([ -n "${EVOLVE_DISPATCH_STOP_ON_FAIL:-}" ] && echo " (DEPRECATED EVOLVE_DISPATCH_STOP_ON_FAIL set)" || true)"

# v8.24.0: export the reinvocation command so claude.sh's EPERM diagnostic
# can suggest a copy-paste recovery line. Quote args defensively.
if [ -n "$BUDGET" ]; then
    export EVOLVE_REINVOKE_CMD="bash $0 --budget $BUDGET $STRATEGY${GOAL:+ \"$GOAL\"}"
else
    export EVOLVE_REINVOKE_CMD="bash $0 $CYCLES $STRATEGY${GOAL:+ \"$GOAL\"}"
fi

if [ "${VALIDATE_ONLY:-0}" = "1" ] || [ "$DRY_RUN" = "1" ]; then
    log "VALIDATE_ONLY/DRY_RUN — not invoking run-cycle.sh"
    exit 0
fi

# --- Prerequisites ---------------------------------------------------------

[ -f "$RUN_CYCLE" ] || fail "missing run-cycle.sh at $RUN_CYCLE"
command -v jq >/dev/null 2>&1 || fail "jq is required for ledger verification"

# v8.24.0: pre-flight state.json writability check.
#
# Before v8.24.0, dispatcher silently lost cycle-progress writes when the OS
# sandbox blocked .evolve/state.json updates: record_failed_approach()
# attempted `printf > tmp && mv -f tmp $STATE_FILE`, swallowed the EPERM,
# and unconditionally logged success. lastCycleNumber never advanced; the
# loop kept guessing the same cycle number and burned 5 cycles' budget.
#
# This pre-flight catches the unwritable case at $0 cost: touch a sentinel
# in the state directory, abort with a copy-paste remediation if it fails.
# Skipped in test mode (RUN_CYCLE_OVERRIDE set) to avoid interfering with
# tests that mount STATE_OVERRIDE on read-only paths intentionally.
if [ -z "${RUN_CYCLE_OVERRIDE:-}" ]; then
    state_dir="$(dirname "$STATE_FILE")"
    mkdir -p "$state_dir" 2>/dev/null || true
    state_probe="${state_dir}/.writable-probe.$$"
    if ! { : > "$state_probe"; } 2>/dev/null; then
        log "FAIL: cannot write to state directory: $state_dir"
        log "      The dispatcher needs to update state.json:lastCycleNumber after"
        log "      each cycle. If this write fails silently, the loop deadlocks on"
        log "      the same cycle number and burns budget. Aborting before any cycle."
        log "REMEDIATION:"
        log "  - If running inside Claude Code's sandbox, the parent process may be"
        log "    blocking writes. Try: EVOLVE_SKIP_WORKTREE=1 bash $0 $* (worktree off)"
        log "  - Or run the dispatcher from a standalone terminal (not nested-claude)"
        log "  - Or check filesystem permissions: ls -la \"$state_dir\""
        exit 1
    fi
    rm -f "$state_probe"
    unset state_dir state_probe
fi

# v8.19.2: Auth path note (informational, not a hard block).
#
# Claude Code subscription auth (~/.claude.json) is the PRIMARY supported path
# for /evolve-loop. The claude-adapter automatically drops --bare when no
# ANTHROPIC_API_KEY is present so the subagent inherits OAuth credentials —
# which is the correct behavior for subscription users running under bypass-
# permissions. The kernel hooks (role-gate, ship-gate, phase-gate-precondition)
# fire at the file-system / Bash level regardless of --bare state.
#
# v8.18.1's hard block was over-protective: it assumed --bare was load-bearing
# for kernel isolation, but the hooks fire on the tool-call layer above
# Claude Code's session-permission layer. Subscription auth + bypass-permissions
# is sufficient for autonomous cycles.
#
# What this section now does: emit a one-line warning if NEITHER auth path
# is detectable, so users running without any auth see a clear message before
# claude exits with its own auth error. Skipped when RUN_CYCLE_OVERRIDE is
# set (test mode).
if [ -z "${RUN_CYCLE_OVERRIDE:-}" ] && \
   [ -z "${ANTHROPIC_API_KEY:-}" ] && \
   [ ! -f "$HOME/.claude.json" ] && \
   [ ! -f "$HOME/.config/claude/config.json" ]; then
    log "WARN: no subscription credentials (~/.claude.json) and no ANTHROPIC_API_KEY found."
    log "      The claude binary will likely fail to authenticate. Run \`claude auth\` to log in,"
    log "      or export ANTHROPIC_API_KEY=sk-... to use API-key auth. Continuing anyway."
fi

# --- Helpers ---------------------------------------------------------------

# count_role <cycle> <role> — counts agent_subprocess entries in the ledger
# for the given cycle and role with exit_code=0. Returns the count on stdout.
count_role() {
    local cycle="$1" role="$2"
    [ -f "$LEDGER" ] || { echo 0; return 0; }
    # `set -e` would explode on grep "no match"; absorb via || true.
    local hits
    hits=$( { grep '"kind":"agent_subprocess"' "$LEDGER" 2>/dev/null || true; } \
        | jq -c --argjson c "$cycle" --arg r "$role" \
            'select(.cycle == $c and .role == $r and .exit_code == 0)' 2>/dev/null \
        | wc -l \
        | tr -d ' ')
    echo "${hits:-0}"
}

# verify_cycle <cycle> — exits 0 if the cycle has scout, builder, AND auditor
# entries; emits a diagnostic and returns 2 if any are missing.
# v8.19.0: when cycle was init'd with intent_required=true, also assert
# the cycle has an `intent` agent_subprocess entry. Read intent_required from
# state.json (NOT env) so mid-stream env flips don't change verification.
verify_cycle() {
    local cycle="$1"
    local s b a
    s=$(count_role "$cycle" "scout")
    b=$(count_role "$cycle" "builder")
    a=$(count_role "$cycle" "auditor")
    local i_required="false" i=0
    if command -v jq >/dev/null 2>&1; then
        # v8.19.0 (audit MEDIUM-1 fix): prefer the per-cycle workspace's own
        # cycle-state.json if it survives in the workspace dir, then fall back
        # to global state.json. The global state only holds the most-recent
        # cycle's intent_required, so for historical cycles it can be wrong.
        local per_cycle_state="$RUNS_DIR/cycle-${cycle}/cycle-state.json"
        if [ -f "$per_cycle_state" ]; then
            i_required=$(jq -r '.intent_required // false' "$per_cycle_state" 2>/dev/null || echo false)
        elif [ -f "$STATE_FILE" ]; then
            i_required=$(jq -r '.intent_required // false' "$STATE_FILE" 2>/dev/null || echo false)
        fi
    fi
    if [ "$i_required" = "true" ]; then
        i=$(count_role "$cycle" "intent")
        log "ledger: cycle=$cycle intent=$i scout=$s builder=$b auditor=$a"
        if [ "$i" -lt 1 ]; then
            log "VERIFY-INCOMPLETE: cycle $cycle missing intent entry (intent_required=true; intent=$i scout=$s builder=$b auditor=$a)"
            return 2
        fi
    else
        log "ledger: cycle=$cycle scout=$s builder=$b auditor=$a"
    fi
    if [ "$s" -lt 1 ] || [ "$b" -lt 1 ] || [ "$a" -lt 1 ]; then
        log "VERIFY-INCOMPLETE: cycle $cycle pipeline incomplete (scout=$s builder=$b auditor=$a)"
        return 2
    fi
    # v8.58.0 Layer E3: PASS-cycle memo enforcement (defense-in-depth alongside
    # gate_ship_to_learn). If the cycle's audit was PASS (read .cycle-verdict
    # written by gate_audit_to_ship), the memo subagent MUST have produced a
    # ledger entry. Without this, the v8.57 contract is silently violated.
    local verdict_file="$RUNS_DIR/cycle-${cycle}/.cycle-verdict"
    if [ -f "$verdict_file" ]; then
        local cycle_verdict
        cycle_verdict=$(cat "$verdict_file" 2>/dev/null)
        if [ "$cycle_verdict" = "PASS" ]; then
            local m
            m=$(count_role "$cycle" "memo")
            if [ "$m" -lt 1 ]; then
                log "VERIFY-INCOMPLETE: PASS cycle $cycle has zero memo ledger entries (Layer P contract violated; orchestrator skipped memo)"
                return 2
            fi
            log "ledger: cycle=$cycle memo=$m (PASS contract met)"
        fi
    fi
    return 0
}

# classify_cycle_failure <cycle> — reads the cycle's orchestrator-report.md
# and returns a classification on stdout:
#   infrastructure   — sandbox EPERM, rate limit, timeout, network
#   ship-gate-config — audit declared PASS but ship-gate refused (v8.27.0,
#                      e.g., auditor exit-code semantics mismatch). Distinct
#                      from audit-fail because the audit itself succeeded;
#                      the rejection is in the post-audit gate config/logic.
#   audit-fail       — cycle ran but Auditor verdict was FAIL/WARN
#   build-fail       — Builder couldn't turn tests GREEN
#   integrity-breach — report missing or unclassifiable (treat as STOP)
classify_cycle_failure() {
    local cycle="$1"
    local report="$RUNS_DIR/cycle-${cycle}/orchestrator-report.md"
    if [ ! -f "$report" ]; then
        echo "integrity-breach"
        return
    fi
    # Infrastructure markers (recoverable, often deterministic).
    if grep -qiE 'INFRASTRUCTURE FAILURE|sandbox-exec.*Operation not permitted|sandbox_apply.*permitted|EPERM|rate.?limit|429.*Too Many|connection.refused|ETIMEDOUT|operation timed out' "$report"; then
        echo "infrastructure"
        return
    fi
    # v8.27.0: ship-gate rejected an audit-PASS cycle. Tested BEFORE audit-fail
    # because a SHIP_GATE_DENIED report can also mention the verdict in passing,
    # and we want to classify it as ship-gate-config (1d age-out, low severity)
    # rather than code-audit-fail (30d, high). The marker is intentionally
    # specific to avoid false-positives: SHIP_GATE_DENIED phrase OR ship-gate
    # rejection patterns from ship.sh's integrity_fail messages.
    if grep -qiE 'SHIP_GATE_DENIED|ship-?gate.*(rejected|denied|exited)|integrity.?fail.*Auditor exited' "$report"; then
        echo "ship-gate-config"
        return
    fi
    # Audit verdict failures (cycle ran but didn't pass the gate).
    if grep -qiE 'Verdict.*FAIL|Verdict.*WARN|verdict.*: *fail' "$report"; then
        echo "audit-fail"
        return
    fi
    # Builder couldn't make the build green.
    if grep -qiE 'Build status.*FAIL|tests.*RED|builder.*failed' "$report"; then
        echo "build-fail"
        return
    fi
    # v8.N: Two-factor orchestrator-hang detector (EVOLVE_HANG_CLASSIFIER=1, default OFF).
    # When the orchestrator writes its report (SHIPPED verdict) and commits, but then
    # hangs and is killed (rc=1, empty stderr), the report exists with a SHIPPED verdict
    # but no error markers — falling through to integrity-breach is a false positive.
    # Two-factor: SHIPPED verdict in report AND cycle commit present on main.
    if [ "${EVOLVE_HANG_CLASSIFIER:-0}" = "1" ]; then
        if awk '/^##[[:space:]]+Verdict/{f=1;next} f && NF{print tolower($0);exit}' "$report" 2>/dev/null \
                | grep -qiE 'shipped'; then
            local _git_commit
            _git_commit=$(git -C "${EVOLVE_PROJECT_ROOT:-.}" log --oneline \
                --grep="cycle $cycle" main 2>/dev/null | head -1 || true)
            if [ -n "$_git_commit" ]; then
                local ts
                ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
                local _ledger="${LEDGER:-${EVOLVE_PROJECT_ROOT:-.}/.evolve/ledger.jsonl}"
                if command -v jq >/dev/null 2>&1 && [ -f "$_ledger" ]; then
                    local entry
                    entry=$(jq -nc \
                        --arg ts "$ts" \
                        --argjson cycle "${cycle}" \
                        --arg commit "$_git_commit" \
                        '{ts:$ts,cycle:$cycle,kind:"classification",classification:"exit-transport-hang",commit:$commit,note:"reclassified from integrity-breach; rc=1 with SHIPPED verdict + commit on main"}')
                    printf '%s\n' "$entry" >> "$_ledger" 2>/dev/null || true
                fi
                echo "exit-transport-hang"
                return
            fi
        fi
    fi
    # Report exists but doesn't transparently explain the gap → treat as breach.
    echo "integrity-breach"
}

# record_failed_approach <cycle> <classification> — appends the failed cycle's
# summary to state.json:failedApproaches[] so the next cycle's orchestrator
# can read and adapt. State schema is per skills/evolve-loop/SKILL.md "Shared
# Agent Values" block.
record_failed_approach() {
    local cycle="$1" raw_classification="$2"
    local report="$RUNS_DIR/cycle-${cycle}/orchestrator-report.md"

    [ -f "$STATE_FILE" ] || { log "WARN: state.json missing, cannot record failure"; return 0; }
    command -v jq >/dev/null 2>&1 || { log "WARN: jq missing, cannot record failure"; return 0; }

    # v8.22.0: source the classification helpers + normalize legacy strings to
    # the structured taxonomy. expiresAt is computed per-classification so the
    # orchestrator's recentFailures lookback (and failure-adapter.sh) can
    # filter out aged-out entries automatically.
    . "$EVOLVE_PLUGIN_ROOT/scripts/failure/failure-classifications.sh"
    local classification
    classification=$(failure_normalize_legacy "$raw_classification")

    local summary=""
    if [ -f "$report" ]; then
        # Pull the first 8 lines of the Failure Root Cause / Verdict block.
        summary=$(awk '
            /^##[[:space:]]+(Failure|Verdict|Phase Outcomes)/ { capture=1; lines=0; next }
            capture && lines<8 { print; lines++ }
            /^##[[:space:]]+/ && capture && lines>0 { exit }
        ' "$report" | tr '\n' ' ' | sed 's/  */ /g' | head -c 400)
    fi

    local now expires_at
    now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    expires_at=$(failure_compute_expires_at "$classification" "$now")

    # FIFO cap: max 50 entries. Append, trim from front if over.
    local current updated tmp
    current=$(cat "$STATE_FILE")
    updated=$(echo "$current" | jq -c \
        --argjson cycle "$cycle" \
        --arg classification "$classification" \
        --arg summary "$summary" \
        --arg ts "$now" \
        --arg exp "$expires_at" \
        '.failedApproaches = (((.failedApproaches // []) + [{
            cycle: $cycle,
            classification: $classification,
            summary: $summary,
            recordedAt: $ts,
            expiresAt: $exp
        }]) | (if length > 50 then .[length-50:] else . end))')
    tmp="${STATE_FILE}.tmp.$$"
    if ! { printf '%s\n' "$updated" > "$tmp" && mv -f "$tmp" "$STATE_FILE"; } 2>/dev/null; then
        rm -f "$tmp" 2>/dev/null
        log "FATAL: state.json write failed (EPERM?) at: $STATE_FILE"
        log "       Cannot record failed approach; cycle progress cannot be persisted."
        log "       This is the silent-deadlock case from pre-v8.24.0. Aborting batch."
        return 1
    fi
    log "recorded failed approach: cycle=$cycle classification=$classification (raw=$raw_classification) expires=$expires_at"

    # ALSO advance lastCycleNumber so the next iteration uses a fresh cycle
    # number / workspace. Without this, every retry overwrites the previous
    # attempt's workspace artifacts, losing diagnostic evidence (the issue
    # exposed in the 2026-05-02 evolutionary-dispatcher run where 3 attempts
    # all wrote to .evolve/runs/cycle-17/).
    local current2 advanced
    current2=$(cat "$STATE_FILE")
    advanced=$(echo "$current2" | jq -c --argjson n "$cycle" '.lastCycleNumber = $n')
    tmp="${STATE_FILE}.tmp.$$"
    if ! { printf '%s\n' "$advanced" > "$tmp" && mv -f "$tmp" "$STATE_FILE"; } 2>/dev/null; then
        rm -f "$tmp" 2>/dev/null
        log "FATAL: state.json write failed (EPERM?) when advancing lastCycleNumber"
        log "       Without this advance, every retry hits the same cycle workspace and"
        log "       overwrites prior diagnostic evidence. Aborting batch."
        return 1
    fi
    log "advanced state.json:lastCycleNumber to $cycle (so next attempt uses cycle-$((cycle + 1)))"
}

# Pick the cycle number used by the most recent run-cycle invocation.
# run-cycle.sh writes lastCycleNumber to state.json on a successful ship.
# In tests we may inject this via STATE_OVERRIDE.
read_last_cycle() {
    if [ -f "$STATE_FILE" ]; then
        jq -r '.lastCycleNumber // 0' "$STATE_FILE" 2>/dev/null || echo 0
    else
        echo 0
    fi
}

# --- v8.28.0: auto-prune expired entries on dispatcher start ----------------
#
# Pre-v8.28.0: failedApproaches entries past their expiresAt remained on disk
# (the failure-adapter filtered them at READ time, but operators saw the
# accumulation in state.json and got confused). v8.28.0 cleans them up
# proactively at dispatcher start.
#
# This is purely cosmetic — the adapter's read-time expiresAt filter already
# prevented expired entries from influencing decisions. But "why are there 4
# entries in failedApproaches?" was operator-archeology friction.
#
# Operator opt-out: EVOLVE_AUTO_PRUNE=0 disables this pre-cycle prune.
if [ "${EVOLVE_AUTO_PRUNE:-1}" = "1" ] && [ -z "${RUN_CYCLE_OVERRIDE:-}" ] && [ -f "$STATE_FILE" ]; then
    CSH="$EVOLVE_PLUGIN_ROOT/scripts/lifecycle/cycle-state.sh"
    if [ -x "$CSH" ]; then
        bash "$CSH" prune-expired-failures "$STATE_FILE" 2>&1 | sed 's/^/[auto-prune] /' >&2 \
            || true   # non-fatal — cosmetic cleanup
    fi
    unset CSH
fi

# --- v8.27.0: --reset operator unblock --------------------------------------
#
# When the failure-adapter has accumulated infrastructure-systemic entries,
# every new cycle BLOCKs at calibrate before any phase agent runs. The
# downstream user's report (cycle-25 evidence): 4 systemic entries, 5+
# blocked invocations, kernel state locked.
#
# --reset prunes infrastructure-systemic + infrastructure-transient + ship-
# gate-config entries before the cycle loop starts. The operator's choice
# is auditable in the log. Pre-existing kernel rules continue to enforce
# anti-gaming on the actual code work this cycle does.
if [ "$RESET_FAILURES" = "1" ]; then
    PRUNE="$EVOLVE_PLUGIN_ROOT/scripts/failure/state-prune.sh"
    if [ -x "$PRUNE" ]; then
        log "--reset: pruning infrastructure-{systemic,transient} + ship-gate-config from $STATE_FILE"
        log "  → operator-driven recovery; DO NOT use to mask repeating real systemic failures"
        # state-prune.sh honors EVOLVE_STATE_FILE_OVERRIDE (different name than
        # the dispatcher's STATE_OVERRIDE). Pass STATE_FILE explicitly so
        # tests using STATE_OVERRIDE see the prune target the correct file.
        for cls in infrastructure-systemic infrastructure-transient ship-gate-config; do
            EVOLVE_STATE_FILE_OVERRIDE="$STATE_FILE" \
                bash "$PRUNE" --classification "$cls" --yes 2>&1 | sed 's/^/[--reset] /' >&2 || \
                log "  WARN: state-prune for $cls failed (non-fatal; continuing)"
        done
        log "--reset: complete — Tier-1 hooks (phase-gate, role-gate, ledger SHA, ship-gate) remain enforced"
    else
        log "WARN: --reset requested but state-prune.sh not executable at $PRUNE (skipping)"
    fi
    unset PRUNE cls
fi

# v9.6.0: inbox crash-recovery — move any orphaned processing/cycle-X/ files
# back to inbox/ for cycles that are no longer active. Idempotent and safe to
# run on every dispatcher invocation. A SIGKILL between Triage's claim call and
# ship.sh's promote call leaves files in processing/; this hook recovers them.
if command -v inbox-mover.sh >/dev/null 2>&1; then
    inbox-mover.sh recover-orphans 2>&1 | sed 's/^/[inbox-recover] /' >&2 || true
else
    log "INFO: inbox-mover.sh not on PATH — skipping recover-orphans (expected before c37 ships)"
fi

# --- Main loop -------------------------------------------------------------

START_TS=$(date -u +%s)
DISPATCH_RC=0

# v8.24.0: circuit-breaker for the "cycle-N runs M× without progress" deadlock.
# Track the last ran_cycle and a streak counter; abort the batch when N
# consecutive iterations report the same cycle number. Threshold is 3 — that
# leaves room for legitimate retries (one infra blip + one recovery + a third
# clean run) while catching the systemic-failure case before it burns a full
# 10-cycle budget. Tunable via env if a use case ever justifies it.
PREV_RAN_CYCLE=""
SAME_CYCLE_STREAK=0
SAME_CYCLE_THRESHOLD="${EVOLVE_DISPATCH_REPEAT_THRESHOLD:-5}"

# v8.58.0 Layer B: per-batch cumulative budget cap. Per-cycle profile caps
# default to $999999 (effectively unlimited) per v8.26's friction-free reasoning
# choice. The 10-cycle v8.57 verification batch cost ~$30 with no tripwire.
# This adds a batch-level ceiling (default $20) — opt-out via
# EVOLVE_BATCH_BUDGET_DISABLE=1. The cap counts only successfully-costed cycles
# (those with valid show-cycle-cost.sh JSON); cycles that failed before
# producing usage sidecars contribute $0 to the batch total.
BATCH_TOTAL_COST="0.00"
# v8.60.0 Layer 1: --budget flag overrides EVOLVE_BATCH_BUDGET_CAP. In budget
# mode, the cap IS the budget — explicit and primary. In cycle mode, cap stays
# at the v8.58 default backstop (or env override).
if [ -n "$BUDGET" ]; then
    BATCH_CAP="$BUDGET"
    DISPATCH_MODE="budget"
else
    BATCH_CAP="${EVOLVE_BATCH_BUDGET_CAP:-20.00}"
    DISPATCH_MODE="cycles"
fi
BATCH_BUDGET_DISABLE="${EVOLVE_BATCH_BUDGET_DISABLE:-0}"
if [ "$BATCH_BUDGET_DISABLE" = "1" ]; then
    log "BATCH-BUDGET: disabled (EVOLVE_BATCH_BUDGET_DISABLE=1)"
else
    if [ "$DISPATCH_MODE" = "budget" ]; then
        log "BATCH-BUDGET: cap=\$${BATCH_CAP} (budget-mode — primary stop condition, safety upper bound CYCLES=$CYCLES)"
    else
        log "BATCH-BUDGET: cap=\$${BATCH_CAP} (override via EVOLVE_BATCH_BUDGET_CAP, disable via EVOLVE_BATCH_BUDGET_DISABLE=1)"
    fi
fi

# v8.60.0 Layer 1: structured output counters — track cycles actually run
# and reason for stopping (budget|batch_cap|cycles|unknown).
CYCLES_RAN=0
STOP_REASON=""

# v9.1.0 Cycle 4: --resume flag handling. When set, the dispatcher delegates
# the first cycle of the batch to resume-cycle.sh, which locates the most
# recent checkpointed cycle and picks up from its resumeFromPhase. After
# that one resume cycle completes, control returns here and the normal
# batch loop runs the remaining cycles (subject to budget caps).
if [ "$RESUME_MODE" = "1" ]; then
    RESUME_SCRIPT="$EVOLVE_PLUGIN_ROOT/scripts/dispatch/resume-cycle.sh"
    if [ ! -f "$RESUME_SCRIPT" ]; then
        log "FAIL: --resume requested but resume-cycle.sh not found at $RESUME_SCRIPT"
        DISPATCH_RC=1
    else
        log "------------------ RESUME ------------------"
        log "Resuming the most recent checkpointed cycle via $RESUME_SCRIPT"
        bash "$RESUME_SCRIPT"
        resume_rc=$?
        if [ "$resume_rc" -eq 0 ]; then
            CYCLES_RAN=$((CYCLES_RAN + 1))
            log "RESUME: completed — continuing with normal batch loop (if any cycles remain)"
        else
            log "RESUME: failed (rc=$resume_rc) — see resume-cycle.sh output above"
            DISPATCH_RC=1
            # Bypass the normal batch loop on resume failure: there's nothing
            # to fall through to safely (the resumed cycle's state is in an
            # ambiguous half-state).
            STOP_REASON="resume_failed"
        fi
        unset RESUME_SCRIPT resume_rc
    fi
fi

for ((i=1; i<=CYCLES; i++)); do
    # Skip the loop entirely if resume mode failed (DISPATCH_RC already set).
    if [ "$RESUME_MODE" = "1" ] && [ "${STOP_REASON:-}" = "resume_failed" ]; then
        break
    fi
    log "------------------ cycle $i / $CYCLES ------------------"

    # v8.21.0: harden against cwd drift between iterations and against
    # plugin-update mid-batch. Subagent subprocesses can leave cwd shifted
    # (sandboxed cd in claude.sh, etc.), and a plugin upgrade between
    # cycles N and N+1 could move/delete RUN_CYCLE. Pinning + re-validation
    # at iteration start makes both classes of failure loud (rc=1 with a
    # specific log line) rather than silently propagating as rc=127.
    if ! cd "$EVOLVE_PROJECT_ROOT" 2>/dev/null; then
        log "FAIL: cannot cd to \$EVOLVE_PROJECT_ROOT=$EVOLVE_PROJECT_ROOT — aborting batch"
        DISPATCH_RC=1
        break
    fi
    if [ ! -x "$RUN_CYCLE" ]; then
        log "FAIL: RUN_CYCLE not executable: $RUN_CYCLE — aborting batch"
        DISPATCH_RC=1
        break
    fi

    # Capture cycle number BEFORE run-cycle.sh so we can verify the right one.
    last_before=$(read_last_cycle)

    # Run the cycle. Pass strategy via env (run-cycle.sh accepts $EVOLVE_STRATEGY,
    # though current run-cycle.sh ignores it — the orchestrator subagent reads
    # state.json's strategy field). Goal is the first positional. Per CLAUDE.md
    # bash convention this script uses `set -uo pipefail` (no `set -e`), so the
    # `rc=$?` capture is sufficient — no `set +e`/`set -e` toggling needed.
    EVOLVE_STRATEGY="$STRATEGY" bash "$RUN_CYCLE" "$GOAL"
    rc=$?

    if [ "$rc" -ne 0 ]; then
        # v10.6.0 auto-resume Layer 3: detect a fresh quota-likely checkpoint
        # FIRST (before the recoverable-failure classifier). If the cycle
        # paused because of a Claude Code subscription quota hit, emit a
        # structured QUOTA-PAUSE marker and exit DISPATCH_RC=5 so the SKILL
        # layer (and any wrapping /loop) can call ScheduleWakeup until the
        # wake-at timestamp and re-invoke /evolve-loop --resume.
        cycle_state_file="$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json"
        if [ -f "$cycle_state_file" ] && command -v jq >/dev/null 2>&1; then
            cp_enabled=$(jq -r '.checkpoint.enabled // false' "$cycle_state_file" 2>/dev/null)
            cp_reason=$(jq -r '.checkpoint.reason // ""' "$cycle_state_file" 2>/dev/null)
            if [ "$cp_enabled" = "true" ] && [ "$cp_reason" = "quota-likely" ]; then
                cp_cycle=$(jq -r '.cycle_id // .cycle // "?"' "$cycle_state_file" 2>/dev/null)
                cp_wake=$(jq -r '.checkpoint.quotaResetAt // ""' "$cycle_state_file" 2>/dev/null)
                cp_source=$(jq -r '.checkpoint.quotaResetSource // "unknown"' "$cycle_state_file" 2>/dev/null)
                cp_attempts=$(jq -r '.checkpoint.autoResumeAttempts // 0' "$cycle_state_file" 2>/dev/null)
                cp_max=$(jq -r '.checkpoint.autoResumeMaxAttempts // 3' "$cycle_state_file" 2>/dev/null)
                log "QUOTA-PAUSE: cycle=$cp_cycle wake-at=$cp_wake source=$cp_source attempts=$cp_attempts/$cp_max"
                log "  to auto-resume in-session: SKILL.md / /loop wrapper calls ScheduleWakeup until $cp_wake then /evolve-loop --resume"
                log "  to resume manually: bash scripts/dispatch/resume-cycle.sh"
                DISPATCH_RC=5
                STOP_REASON="quota-pause"
                break
            fi
        fi

        # v8.30.0: classify before aborting — fluent-mode philosophy.
        #
        # Pre-v8.30.0: ANY non-zero exit from run-cycle.sh aborted the entire
        # batch. But run-cycle.sh exits 1 for many reasons:
        #   - orchestrator subagent crashed mid-cycle (transient)
        #   - claude-adapter timeout / API issue (transient)
        #   - worktree provisioning hit a race (now mostly fixed in v8.29.0)
        # Aborting the whole batch on a single transient cycle failure is
        # exactly the friction the fluent-default philosophy wants to remove.
        #
        # v8.30.0: when orchestrator-report.md exists for the attempted cycle
        # and classifies as recoverable (infrastructure / audit-fail / build-
        # fail / ship-gate-config), record the failure and continue to the
        # next cycle. Only abort when no report exists (genuine breach) OR
        # classification is integrity-breach.
        attempted_cycle=$(read_last_cycle)
        attempted_cycle=$((attempted_cycle + 1))
        attempted_report="$RUNS_DIR/cycle-${attempted_cycle}/orchestrator-report.md"
        if [ -f "$attempted_report" ]; then
            classification=$(classify_cycle_failure "$attempted_cycle")
            log "run-cycle.sh exited rc=$rc; classifying via orchestrator-report.md: $classification"
            case "$classification" in
                infrastructure|audit-fail|build-fail|ship-gate-config|exit-transport-hang)
                    log "RECOVERABLE-FAILURE: run-cycle rc=$rc but report classifies as $classification"
                    log "  → recording to state.json:failedApproaches; continuing batch"
                    if ! record_failed_approach "$attempted_cycle" "$classification"; then
                        log "ABORT: state.json unwritable mid-batch (FATAL above)"
                        DISPATCH_RC=1
                        break
                    fi
                    DISPATCH_RC=3
                    CYCLES_RAN=$((CYCLES_RAN + 1))
                    continue   # next cycle iteration
                    ;;
                integrity-breach|*)
                    log "INTEGRITY-BREACH: run-cycle rc=$rc + orchestrator-report unclassifiable"
                    DISPATCH_RC=2
                    break
                    ;;
            esac
        else
            log "FAIL: run-cycle.sh cycle $i exited rc=$rc with no orchestrator-report.md — aborting batch"
            DISPATCH_RC=1
            break
        fi
    fi

    # Identify which cycle ran. run-cycle.sh increments lastCycleNumber on
    # successful ship; if the orchestrator FAILED audit, lastCycleNumber may
    # not have advanced. We use last_before+1 as the conservative guess —
    # tests can verify either the numeric next-step or use a synthetic ledger.
    last_after=$(read_last_cycle)
    if [ "$last_after" -gt "$last_before" ]; then
        ran_cycle="$last_after"
    else
        ran_cycle=$((last_before + 1))
        log "NOTE: lastCycleNumber did not advance; verifying cycle $ran_cycle (likely WARN/FAIL audit verdict — that is acceptable, but pipeline must still have been complete)"
    fi

    # v8.33.0: per-cycle cost summary. Reuse show-cycle-cost.sh's --json mode
    # to aggregate per-phase usage.json sidecars and emit one log line per
    # cycle. Surfaces the optimization (cache hits) AND the cost-driver phases
    # without operators needing to grep sidecar JSON manually. Best-effort —
    # if the cycle didn't produce sidecar files (early failure), this is a
    # no-op.
    #
    # Field path note: show-cycle-cost.sh's --json output nests totals under
    # `.total.{cost_usd,cache_read_input_tokens,cache_creation_input_tokens,input_tokens}`.
    # All field accesses use `// 0` defaults so missing fields don't break jq.
    SCC="$EVOLVE_PLUGIN_ROOT/scripts/observability/show-cycle-cost.sh"
    if [ -x "$SCC" ] && [ -d "$RUNS_DIR/cycle-${ran_cycle}" ]; then
        # Pass RUNS_DIR through so show-cycle-cost.sh finds the workspace even
        # in test isolation (RUNS_DIR_OVERRIDE) or plugin-install (project_root
        # ≠ plugin_root) scenarios.
        cost_json=$(RUNS_DIR_OVERRIDE="$RUNS_DIR" bash "$SCC" "$ran_cycle" --json 2>/dev/null || echo "")
        if [ -n "$cost_json" ]; then
            cost_line=$(echo "$cost_json" | jq -r '
                . as $c
                | (.total.cost_usd // 0) as $tc
                | ((.total.cache_read_input_tokens // 0) + (.total.cache_creation_input_tokens // 0)) as $cache_in
                | ((.total.input_tokens // 0) + $cache_in) as $all_input
                | (if $all_input > 0 then ($cache_in / $all_input * 100) | floor else 0 end) as $hit_pct
                | ($c.phases | map("\(.phase)=$\((.cost_usd // 0) | (. * 10000 | round / 10000) | tostring)") | join(", ")) as $phase_str
                | "cycle \($c.cycle) cost: $\($tc | (. * 10000 | round / 10000) | tostring) (\($phase_str)) cache_hit=\($hit_pct)%"
            ' 2>/dev/null || echo "")
            [ -n "$cost_line" ] && log "$cost_line"
            # v8.58.0 Layer B: accumulate this cycle's cost into the batch total.
            # Use bc for fractional arithmetic; fall back to integer-cents on systems
            # without bc. Best-effort — invalid/missing cost contributes $0.
            _cycle_cost=$(echo "$cost_json" | jq -r '.total.cost_usd // 0' 2>/dev/null || echo "0")
            if command -v bc >/dev/null 2>&1; then
                BATCH_TOTAL_COST=$(echo "$BATCH_TOTAL_COST + $_cycle_cost" | bc -l 2>/dev/null || echo "$BATCH_TOTAL_COST")
            fi
            if [ "$BATCH_BUDGET_DISABLE" != "1" ]; then
                log "BATCH-BUDGET: cumulative \$${BATCH_TOTAL_COST} / \$${BATCH_CAP}"
            fi
            unset _cycle_cost
        fi
    fi
    unset SCC cost_json cost_line

    # v9.1.0 Cycle 2: pre-emptive checkpoint thresholds.
    # Two thresholds fire BEFORE the hard tripwire below:
    #
    #   EVOLVE_CHECKPOINT_WARN_AT_PCT (default 80) — log a WARN advising
    #     the operator that they're approaching the cap. No state change.
    #
    #   EVOLVE_CHECKPOINT_AT_PCT (default 95) — set EVOLVE_CHECKPOINT_REQUEST=1
    #     in the dispatcher's env so the NEXT cycle's orchestrator sees the
    #     signal in its invocation context. The orchestrator persona reads
    #     this and writes a checkpoint at the next phase boundary instead
    #     of continuing. The cycle that's already in progress completes; the
    #     pre-emption is for the cycle that hasn't started yet. This avoids
    #     forcing a mid-cycle abort (which would lose phase-coherent state).
    #
    # Rationale: the v8.58 tripwire is binary (over/under). When you cross
    # the cap mid-batch, work in flight is lost. The 95% pre-emptive signal
    # gives the orchestrator a chance to graceful-pause at the next clean
    # phase boundary, where resume is straightforward.
    CHECKPOINT_WARN_AT_PCT="${EVOLVE_CHECKPOINT_WARN_AT_PCT:-80}"
    CHECKPOINT_AT_PCT="${EVOLVE_CHECKPOINT_AT_PCT:-95}"
    if [ "$BATCH_BUDGET_DISABLE" != "1" ] && command -v bc >/dev/null 2>&1 \
            && [ "${EVOLVE_CHECKPOINT_DISABLE:-0}" != "1" ]; then
        # Compute integer percentage (truncated). bc's `scale=0` gives floor.
        _pct=$(echo "scale=2; $BATCH_TOTAL_COST / $BATCH_CAP * 100" | bc -l 2>/dev/null | awk -F. '{print $1+0}')
        _pct="${_pct:-0}"
        if [ "$_pct" -ge "$CHECKPOINT_AT_PCT" ] && [ "${EVOLVE_CHECKPOINT_REQUEST:-0}" != "1" ]; then
            log "BATCH-BUDGET CRITICAL: cumulative \$${BATCH_TOTAL_COST} (${_pct}%) >= ${CHECKPOINT_AT_PCT}% — signaling next cycle to checkpoint at phase boundary"
            log "  next cycle's orchestrator will receive EVOLVE_CHECKPOINT_REQUEST=1 in its invocation context"
            log "  to resume after this batch ends: bash scripts/dispatch/evolve-loop-dispatch.sh --resume"
            export EVOLVE_CHECKPOINT_REQUEST=1
            export EVOLVE_CHECKPOINT_REASON="batch-cap-near"
        elif [ "$_pct" -ge "$CHECKPOINT_WARN_AT_PCT" ]; then
            log "BATCH-BUDGET WARN: cumulative \$${BATCH_TOTAL_COST} (${_pct}%) >= ${CHECKPOINT_WARN_AT_PCT}% — consider operator review"
            log "  pre-emptive checkpoint fires at ${CHECKPOINT_AT_PCT}% (\$$(echo "scale=2; $BATCH_CAP * $CHECKPOINT_AT_PCT / 100" | bc -l 2>/dev/null))"
        fi
        unset _pct
    fi

    # v8.58.0 Layer B: tripwire check. After the cycle's cost is added, if the
    # cumulative total exceeds the cap, stop the batch — remaining cycles are
    # skipped. This is the operator-facing tripwire that the v8.57 verification
    # didn't have. Set EVOLVE_BATCH_BUDGET_DISABLE=1 to opt out.
    if [ "$BATCH_BUDGET_DISABLE" != "1" ] && command -v bc >/dev/null 2>&1; then
        if [ "$(echo "$BATCH_TOTAL_COST > $BATCH_CAP" | bc -l 2>/dev/null)" = "1" ]; then
            # v8.60.0 Layer 1: in budget-mode, hitting the budget IS success
            # (we ran cycles until budget consumed — the user-requested behavior).
            # In cycles-mode, hitting the cap is a backstop (operator override
            # needed for more cycles), so rc=4 distinguishes overrun.
            if [ "$DISPATCH_MODE" = "budget" ]; then
                log "BUDGET-EXHAUSTED: cumulative \$${BATCH_TOTAL_COST} >= budget \$${BATCH_CAP} (after cycle $ran_cycle)"
                log "  ran $i of safety_max=$CYCLES cycles; budget-driven completion"
                log "  override: --budget-usd <higher> on next invocation"
                STOP_REASON="budget"
                DISPATCH_RC=0
            else
                log "BATCH-BUDGET-EXCEEDED: cumulative \$${BATCH_TOTAL_COST} > cap \$${BATCH_CAP} (after cycle $ran_cycle)"
                log "  override: EVOLVE_BATCH_BUDGET_CAP=<higher> or EVOLVE_BATCH_BUDGET_DISABLE=1"
                log "  remaining cycles ($((CYCLES - i)) of $CYCLES) will be skipped"
                STOP_REASON="batch_cap"
                DISPATCH_RC=4
            fi
            CYCLES_RAN=$((CYCLES_RAN + 1))
            break
        fi
    fi

    # v8.24.0: same-cycle circuit-breaker. If iteration after iteration reports
    # the same cycle number, the dispatcher is deadlocked — either state.json
    # writes silently fail (pre-v8.24.0 bug, now caught by record_failed_approach
    # FATAL guards) or run-cycle.sh is failing in a way that blocks progress
    # before the cycle even begins. Either way, looping further wastes budget.
    if [ "$ran_cycle" = "$PREV_RAN_CYCLE" ]; then
        SAME_CYCLE_STREAK=$((SAME_CYCLE_STREAK + 1))
    else
        SAME_CYCLE_STREAK=1
        PREV_RAN_CYCLE="$ran_cycle"
    fi
    if [ "$SAME_CYCLE_STREAK" -ge "$SAME_CYCLE_THRESHOLD" ]; then
        log "ABORT: same cycle number ($ran_cycle) reported $SAME_CYCLE_STREAK consecutive times (threshold=$SAME_CYCLE_THRESHOLD)"
        log "       The dispatcher cannot make progress. Aborting batch to avoid wasting budget."
        log "REMEDIATION:"
        log "  - Most likely: state.json writes are blocked by the parent OS sandbox."
        log "    Set EVOLVE_SKIP_WORKTREE=1 (or run from a standalone terminal)."
        log "  - If state.json IS writable, inspect $RUNS_DIR/cycle-${ran_cycle}/orchestrator-report.md"
        log "    for the underlying failure reason."
        log "  - To raise the threshold for legitimate-retry scenarios:"
        log "    EVOLVE_DISPATCH_REPEAT_THRESHOLD=N bash $0 ..."
        DISPATCH_RC=1
        break
    fi

    # v8.60+: resolve canonical dispatch policy from EVOLVE_DISPATCH_POLICY or
    # legacy deprecated flags (EVOLVE_DISPATCH_VERIFY / EVOLVE_DISPATCH_STOP_ON_FAIL).
    # Resolution runs once per cycle (idempotent via _DISPATCH_POLICY_WARNED guard).
    _DISPATCH_POLICY="${EVOLVE_DISPATCH_POLICY:-verify}"
    if [ -z "${_DISPATCH_POLICY_WARNED:-}" ]; then
        export _DISPATCH_POLICY_WARNED=1
        _legacy_verify="${EVOLVE_DISPATCH_VERIFY:-}"
        _legacy_stop="${EVOLVE_DISPATCH_STOP_ON_FAIL:-}"
        if [ -n "$_legacy_stop" ] && [ "$_legacy_stop" = "1" ] && [ -n "$_legacy_verify" ] && [ "$_legacy_verify" = "0" ]; then
            echo "WARN: Both EVOLVE_DISPATCH_STOP_ON_FAIL=1 and EVOLVE_DISPATCH_VERIFY=0 set — STOP_ON_FAIL wins (most restrictive). Use EVOLVE_DISPATCH_POLICY=stop. Both flags deprecated (v8.60+)." >&2
            _DISPATCH_POLICY="stop"
        elif [ -n "$_legacy_stop" ] && [ "$_legacy_stop" = "1" ]; then
            echo "WARN: EVOLVE_DISPATCH_STOP_ON_FAIL is deprecated (v8.60+); use EVOLVE_DISPATCH_POLICY=stop (bridging now)." >&2
            _DISPATCH_POLICY="stop"
        elif [ -n "$_legacy_verify" ] && [ "$_legacy_verify" = "0" ]; then
            echo "WARN: EVOLVE_DISPATCH_VERIFY=0 is deprecated (v8.60+); use EVOLVE_DISPATCH_POLICY=off (bridging now)." >&2
            _DISPATCH_POLICY="off"
        fi
    fi

    # Verify the pipeline ran end-to-end (scout, builder, auditor all present).
    # Controlled by EVOLVE_DISPATCH_POLICY: off skips, verify/stop both check.
    if [ "$_DISPATCH_POLICY" != "off" ]; then
        if ! verify_cycle "$ran_cycle"; then
            # Verification failed — classify before deciding STOP vs CONTINUE.
            # The orchestrator-report.md tells us if this was honest infrastructure
            # failure (recoverable, learn and adapt) or silent shortcut (STOP).
            classification=$(classify_cycle_failure "$ran_cycle")
            log "classified failure: $classification"

            # Legacy fail-fast can be restored explicitly (per CLAUDE.md autonomous rule,
            # the default is now to learn-and-continue).
            if [ "$_DISPATCH_POLICY" = "stop" ]; then
                log "EVOLVE_DISPATCH_POLICY=stop — legacy fail-fast: aborting batch"
                DISPATCH_RC=2
                break
            fi

            case "$classification" in
                infrastructure|audit-fail|build-fail|ship-gate-config|exit-transport-hang)
                    log "RECOVERABLE-FAILURE: cycle $ran_cycle classification=$classification"
                    log "  → recording to state.json:failedApproaches; next cycle's orchestrator will read this and adapt"
                    if ! record_failed_approach "$ran_cycle" "$classification"; then
                        # v8.24.0: state.json itself is unwritable. The pre-flight
                        # should have caught this, but if a mid-batch permission
                        # change happens, fail loud rather than silently looping.
                        log "ABORT: state.json unwritable mid-batch. See FATAL above."
                        DISPATCH_RC=1
                        break
                    fi
                    DISPATCH_RC=3   # batch will end with rc=3 if any cycle fails recoverably
                    # IMPORTANT: do NOT break; continue to next cycle (evolutionary behavior)
                    ;;
                integrity-breach|*)
                    log "INTEGRITY-BREACH: cycle $ran_cycle — orchestrator shortcut detected (orchestrator-report.md missing or unclassifiable)"
                    log "  → this is a kernel breach signal (silent skip); STOP and require human investigation"
                    DISPATCH_RC=2
                    break
                    ;;
            esac
        fi
    else
        log "WARN: EVOLVE_DISPATCH_POLICY=off — skipping ledger pipeline check (LEGACY)"
    fi
    CYCLES_RAN=$((CYCLES_RAN + 1))
done

# Set stop reason for normal cycles-exhausted exit (STOP_REASON not set by tripwire).
[ -z "$STOP_REASON" ] && STOP_REASON="cycles"

ELAPSED=$(( $(date -u +%s) - START_TS ))

log "------------------ summary ------------------"
log "elapsed: ${ELAPSED}s"
if [ -n "$BUDGET" ]; then
    log "mode=budget budget=\$$BUDGET cycles_safety_max=$CYCLES"
else
    log "mode=cycles cycles_requested=$CYCLES"
fi
log "exit_code=$DISPATCH_RC"
# v8.58.0 Layer B: surface batch total in summary so operators can verify
# spend at a glance without grepping show-cycle-cost.sh per cycle.
if [ "${BATCH_BUDGET_DISABLE:-0}" = "1" ]; then
    log "batch_total_cost=\$${BATCH_TOTAL_COST:-0.00} (cap disabled)"
else
    log "batch_total_cost=\$${BATCH_TOTAL_COST:-0.00} / cap=\$${BATCH_CAP:-20.00}"
fi
# v8.60.0 Layer 1: structured stop summary — machine-parseable fields for
# downstream tooling and acceptance-check assertions.
log "stop_reason=${STOP_REASON:-unknown} cycles_run=${CYCLES_RAN} total_cost_usd=\$${BATCH_TOTAL_COST:-0.00}"

if [ "$DISPATCH_RC" = "0" ]; then
    log "DONE: all $CYCLES cycles completed AND verified end-to-end"
elif [ "$DISPATCH_RC" = "3" ]; then
    log "DONE-WITH-RECOVERABLE-FAILURES: batch completed; some cycles had infrastructure/audit/build failures"
    log "  → state.json:failedApproaches now contains the failure modes for review"
    log "  → next /evolve-loop dispatch's orchestrator will read these and adapt approach"
elif [ "$DISPATCH_RC" = "2" ]; then
    log "INTEGRITY-BREACH: a cycle bypassed Scout/Builder/Auditor and the orchestrator-report didn't disclose it"
    log "  → inspect $LEDGER for the cycle in question; the orchestrator may"
    log "  → have invoked the in-process Agent tool, edited files inline, or"
    log "  → used an unauthorized escape hatch. Treat this as a CRITICAL finding."
elif [ "$DISPATCH_RC" = "5" ]; then
    log "QUOTA-PAUSE: batch paused on a Claude Code subscription quota hit (v10.6.0 auto-resume)"
    log "  → the cycle's worktree + cycle-state.json are preserved on disk"
    log "  → the SKILL layer (or wrapping /loop) should call ScheduleWakeup until the wake-at timestamp"
    log "     emitted above (QUOTA-PAUSE: wake-at=...), then re-invoke /evolve-loop --resume"
    log "  → to resume manually right now: bash scripts/dispatch/resume-cycle.sh"
    log "  → to inspect: cat $EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json | jq .checkpoint"
else
    log "DONE-WITH-FAILURE: run-cycle.sh failed; see logs above"
fi

exit "$DISPATCH_RC"
