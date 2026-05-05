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
#   2. Loops `bash scripts/run-cycle.sh "<goal>"` once per cycle.
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
#   bash scripts/evolve-loop-dispatch.sh [CYCLES] [STRATEGY] [GOAL...]
#   bash scripts/evolve-loop-dispatch.sh --dry-run [args...]
#   bash scripts/evolve-loop-dispatch.sh --help
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
. "$__rr_self/resolve-roots.sh"
unset __rr_self

# v8.20.0: PATH-based kernel script invocation. Prepend plugin's scripts dir
# so subagents can invoke kernel scripts by bare name. Eliminates the
# install-layout-fragile path-pattern enumeration in orchestrator/auditor
# allowlists. Inherits to claude -p subprocess via env propagation.
export PATH="$EVOLVE_PLUGIN_ROOT/scripts:$EVOLVE_PLUGIN_ROOT/scripts/release:$PATH"

# Read-only: run-cycle.sh ships with the plugin
RUN_CYCLE="${RUN_CYCLE_OVERRIDE:-$EVOLVE_PLUGIN_ROOT/scripts/run-cycle.sh}"
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
        log "         Or:  EVOLVE_PROJECT_ROOT=/path/to/project bash <plugin>/scripts/evolve-loop-dispatch.sh ..."
        exit 10
        ;;
esac

# --- v8.22.0: nested-claude auto-detection ---------------------------------
#
# When /evolve-loop is invoked from inside Claude Code (the slash-command path
# AND direct CLI path covered here), the parent process is itself sandboxed.
# macOS Darwin 25.4+ refuses nested sandbox-exec apply, returning EPERM (rc=71)
# from sandbox_apply(). Without EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1, every
# sub-claude invocation (intent, scout, builder, auditor) hits this wall.
#
# SKILL.md auto-sets the flag for the slash-command entry point. This block
# adds defense-in-depth: any direct dispatcher invocation from inside Claude
# Code also gets the flag auto-enabled.
#
# The detection signal is the CLAUDECODE / CLAUDE_CODE_* env-var family
# (Claude Code's parent-env beacons). On non-Claude-Code shells (terminal,
# CI, scripts), the detector returns "standalone" and this block is a no-op.
#
# The flag is a no-op on Linux (bwrap supports nested namespaces). Symmetric
# detection still fires for log-uniformity but doesn't change behavior.
#
# Override: set EVOLVE_SANDBOX_FALLBACK_ON_EPERM=0 explicitly to skip the
# auto-set (e.g., when running an outer sandbox-exec wrapper that handles
# isolation differently).
if [ -x "$EVOLVE_PLUGIN_ROOT/scripts/detect-nested-claude.sh" ]; then
    if [ "$(bash "$EVOLVE_PLUGIN_ROOT/scripts/detect-nested-claude.sh")" = "nested" ]; then
        nested_announced=0
        if [ -z "${EVOLVE_SANDBOX_FALLBACK_ON_EPERM:-}" ]; then
            log "DETECTED: nested-claude (CLAUDECODE / CLAUDE_CODE_* env set)"
            nested_announced=1
            log "  → auto-enabling EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1"
            log "  → Darwin 25.4+ kernel forbids nested sandbox-exec; this flag retries"
            log "    sub-claude subprocesses unsandboxed when sandbox_apply hits EPERM."
            log "  → kernel hooks (role-gate, ship-gate, phase-gate) still enforce. To"
            log "    skip auto-set: EVOLVE_SANDBOX_FALLBACK_ON_EPERM=0 bash $0 ..."
            export EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1
        fi
        # v8.24.0: auto-relax Tier-2 OS isolation in nested-Claude.
        #
        # Even after the v8.22.0 EPERM-startup fallback (above) and the v8.23.3
        # cwd fix, the parent Claude Code's OS sandbox can still deny writes
        # *inside* .evolve/worktrees/cycle-N once the inner subprocess actually
        # tries to Edit/Write. That's an execution-time EPERM, not a startup
        # EPERM, so FALLBACK_ON_EPERM=1 doesn't help. Symptom: cycle-N runs
        # repeatedly, never advances state.json, dispatcher loops on the same
        # number and burns budget.
        #
        # The auto-relax follows the same precedent as the v8.22.0 block: the
        # operator can opt back in to strict isolation by setting
        # EVOLVE_SKIP_WORKTREE=0 explicitly. Tier-1 kernel hooks (phase-gate
        # ledger SHA, role-gate, ship-gate) still enforce structural integrity
        # without the worktree — this is the relaxation we can afford.
        if [ -z "${EVOLVE_SKIP_WORKTREE:-}" ]; then
            [ "$nested_announced" = "0" ] && log "DETECTED: nested-claude (CLAUDECODE / CLAUDE_CODE_* env set)"
            log "  → auto-enabling EVOLVE_SKIP_WORKTREE=1 (Tier-2 OS isolation auto-relax)"
            log "  → Reason: parent OS sandbox blocks writes to .evolve/worktrees/ in some"
            log "    nested-claude environments. Skipping worktree lets the builder edit"
            log "    \$EVOLVE_PROJECT_ROOT directly. Tier-1 kernel hooks still enforce."
            log "  → To opt back in to strict isolation: EVOLVE_SKIP_WORKTREE=0 bash $0 ..."
            export EVOLVE_SKIP_WORKTREE=1
        fi
        unset nested_announced
    fi
fi

# --- Argument parsing -------------------------------------------------------

DRY_RUN=0
CYCLES=""
STRATEGY=""
GOAL=""

# Pull off --flags first so positional parsing doesn't see them.
POSITIONAL=()
while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run)
            DRY_RUN=1
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
if [ "${#POSITIONAL[@]}" -gt 0 ]; then
    if [[ "${POSITIONAL[0]}" =~ ^[0-9]+$ ]]; then
        CYCLES="${POSITIONAL[0]}"
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

# Apply defaults.
[ -n "$CYCLES" ]   || CYCLES=2
[ -n "$STRATEGY" ] || STRATEGY=balanced

# Validate.
[[ "$CYCLES" =~ ^[0-9]+$ ]] || abort_args "CYCLES must be a non-negative integer (got: $CYCLES)"
[ "$CYCLES" -ge 1 ]         || abort_args "CYCLES must be >= 1 (got: $CYCLES)"
case "$STRATEGY" in
    balanced|innovate|harden|repair|ultrathink|autoresearch) ;;
    *) abort_args "STRATEGY must be one of: balanced|innovate|harden|repair|ultrathink|autoresearch (got: $STRATEGY)" ;;
esac

# --- Plan ---------------------------------------------------------------------

log "PLAN: cycles=$CYCLES strategy=$STRATEGY goal='${GOAL:-<autonomous>}'"
log "PLAN: run_cycle=$RUN_CYCLE"
log "PLAN: ledger=$LEDGER"
log "PLAN: verify=$([ "${EVOLVE_DISPATCH_VERIFY:-1}" = "1" ] && echo "on" || echo "OFF")"

# v8.24.0: export the reinvocation command so claude.sh's EPERM diagnostic
# can suggest a copy-paste recovery line. Quote args defensively.
export EVOLVE_REINVOKE_CMD="bash $0 $CYCLES $STRATEGY${GOAL:+ \"$GOAL\"}"

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
    return 0
}

# classify_cycle_failure <cycle> — reads the cycle's orchestrator-report.md
# and returns a classification on stdout:
#   infrastructure   — sandbox EPERM, rate limit, timeout, network
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
    . "$EVOLVE_PLUGIN_ROOT/scripts/failure-classifications.sh"
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
SAME_CYCLE_THRESHOLD="${EVOLVE_DISPATCH_REPEAT_THRESHOLD:-3}"

for ((i=1; i<=CYCLES; i++)); do
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
        log "FAIL: run-cycle.sh cycle $i exited rc=$rc — aborting batch"
        DISPATCH_RC=1
        break
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

    # Verify the pipeline ran end-to-end (scout, builder, auditor all present).
    # Skippable via env for legacy debugging only.
    if [ "${EVOLVE_DISPATCH_VERIFY:-1}" = "1" ]; then
        if ! verify_cycle "$ran_cycle"; then
            # Verification failed — classify before deciding STOP vs CONTINUE.
            # The orchestrator-report.md tells us if this was honest infrastructure
            # failure (recoverable, learn and adapt) or silent shortcut (STOP).
            classification=$(classify_cycle_failure "$ran_cycle")
            log "classified failure: $classification"

            # Legacy fail-fast can be restored explicitly (per CLAUDE.md autonomous rule,
            # the default is now to learn-and-continue).
            if [ "${EVOLVE_DISPATCH_STOP_ON_FAIL:-0}" = "1" ]; then
                log "EVOLVE_DISPATCH_STOP_ON_FAIL=1 — legacy fail-fast: aborting batch"
                DISPATCH_RC=2
                break
            fi

            case "$classification" in
                infrastructure|audit-fail|build-fail)
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
        log "WARN: EVOLVE_DISPATCH_VERIFY=0 — skipping ledger pipeline check (LEGACY)"
    fi
done

ELAPSED=$(( $(date -u +%s) - START_TS ))

log "------------------ summary ------------------"
log "elapsed: ${ELAPSED}s"
log "cycles_requested=$CYCLES"
log "exit_code=$DISPATCH_RC"

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
else
    log "DONE-WITH-FAILURE: run-cycle.sh failed; see logs above"
fi

exit "$DISPATCH_RC"
