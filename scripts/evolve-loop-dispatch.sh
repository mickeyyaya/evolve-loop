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

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_CYCLE="${RUN_CYCLE_OVERRIDE:-$REPO_ROOT/scripts/run-cycle.sh}"
LEDGER="${LEDGER_OVERRIDE:-$REPO_ROOT/.evolve/ledger.jsonl}"
STATE_FILE="${STATE_OVERRIDE:-$REPO_ROOT/.evolve/state.json}"
RUNS_DIR="${RUNS_DIR_OVERRIDE:-$REPO_ROOT/.evolve/runs}"

log()  { echo "[dispatch] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }
abort_args() { log "BAD-ARG: $*"; exit 10; }

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

if [ "${VALIDATE_ONLY:-0}" = "1" ] || [ "$DRY_RUN" = "1" ]; then
    log "VALIDATE_ONLY/DRY_RUN — not invoking run-cycle.sh"
    exit 0
fi

# --- Prerequisites ---------------------------------------------------------

[ -f "$RUN_CYCLE" ] || fail "missing run-cycle.sh at $RUN_CYCLE"
command -v jq >/dev/null 2>&1 || fail "jq is required for ledger verification"

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
verify_cycle() {
    local cycle="$1"
    local s b a
    s=$(count_role "$cycle" "scout")
    b=$(count_role "$cycle" "builder")
    a=$(count_role "$cycle" "auditor")
    log "ledger: cycle=$cycle scout=$s builder=$b auditor=$a"
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
    local cycle="$1" classification="$2"
    local report="$RUNS_DIR/cycle-${cycle}/orchestrator-report.md"

    [ -f "$STATE_FILE" ] || { log "WARN: state.json missing, cannot record failure"; return 0; }
    command -v jq >/dev/null 2>&1 || { log "WARN: jq missing, cannot record failure"; return 0; }

    local summary=""
    if [ -f "$report" ]; then
        # Pull the first 8 lines of the Failure Root Cause / Verdict block.
        summary=$(awk '
            /^##[[:space:]]+(Failure|Verdict|Phase Outcomes)/ { capture=1; lines=0; next }
            capture && lines<8 { print; lines++ }
            /^##[[:space:]]+/ && capture && lines>0 { exit }
        ' "$report" | tr '\n' ' ' | sed 's/  */ /g' | head -c 400)
    fi

    local now
    now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    local current updated tmp
    current=$(cat "$STATE_FILE")
    updated=$(echo "$current" | jq -c \
        --argjson cycle "$cycle" \
        --arg classification "$classification" \
        --arg summary "$summary" \
        --arg ts "$now" \
        '.failedApproaches = ((.failedApproaches // []) + [{
            cycle: $cycle,
            classification: $classification,
            summary: $summary,
            recordedAt: $ts
        }])')
    tmp="${STATE_FILE}.tmp.$$"
    printf '%s\n' "$updated" > "$tmp" && mv -f "$tmp" "$STATE_FILE"
    log "recorded failed approach: cycle=$cycle classification=$classification → state.json:failedApproaches"

    # ALSO advance lastCycleNumber so the next iteration uses a fresh cycle
    # number / workspace. Without this, every retry overwrites the previous
    # attempt's workspace artifacts, losing diagnostic evidence (the issue
    # exposed in the 2026-05-02 evolutionary-dispatcher run where 3 attempts
    # all wrote to .evolve/runs/cycle-17/).
    local current2 advanced
    current2=$(cat "$STATE_FILE")
    advanced=$(echo "$current2" | jq -c --argjson n "$cycle" '.lastCycleNumber = $n')
    tmp="${STATE_FILE}.tmp.$$"
    printf '%s\n' "$advanced" > "$tmp" && mv -f "$tmp" "$STATE_FILE"
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

for ((i=1; i<=CYCLES; i++)); do
    log "------------------ cycle $i / $CYCLES ------------------"

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
                    record_failed_approach "$ran_cycle" "$classification"
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
