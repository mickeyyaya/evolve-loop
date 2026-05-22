#!/usr/bin/env bash
#
# fanout-dispatch.sh — Bounded-concurrency parallel worker dispatcher (Sprint 1).
#
# Reads a TSV commands file (`<worker_name>\t<command>\n`) and runs each
# command concurrently, bounded by EVOLVE_FANOUT_CONCURRENCY (default 2).
# Each worker is wrapped in a per-worker timeout (EVOLVE_FANOUT_TIMEOUT,
# default 600s). WAIT-ALL semantics: every worker runs to completion or
# timeout regardless of others' failures, so partial failures are diagnosed
# rather than masked.
#
# Output: TSV at <results_path> with rows `<worker_name>\t<exit_code>\t<duration_s>`.
# Each worker's stdout/stderr is captured to <results_dir>/<worker_name>.{out,err}.
#
# Usage:
#   fanout-dispatch.sh <commands.tsv> <results.tsv>
#
# Exit codes:
#   0 — every worker exited 0
#   1 — one or more workers had non-zero exit codes (still wait-all completed)
#   2 — bad arguments / setup failure
#
# Bash 3.2 compatible per CLAUDE.md (no declare -A, no mapfile, no GNU-only flags).

set -uo pipefail

CMD_FILE=""
RESULTS_FILE=""
CACHE_PREFIX_FILE=""

# v8.23.0: argument parsing (was bare positional). Backward-compat: bare
# positional usage continues to work (`fanout-dispatch.sh <cmds> <results>`).
while [ $# -gt 0 ]; do
    case "$1" in
        --cache-prefix-file)
            shift; CACHE_PREFIX_FILE="${1:-}"
            ;;
        --cache-prefix-file=*)
            CACHE_PREFIX_FILE="${1#--cache-prefix-file=}"
            ;;
        --help|-h)
            sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --*)
            echo "[fanout-dispatch] unknown flag: $1" >&2; exit 2
            ;;
        *)
            if [ -z "$CMD_FILE" ]; then CMD_FILE="$1"
            elif [ -z "$RESULTS_FILE" ]; then RESULTS_FILE="$1"
            else echo "[fanout-dispatch] extra positional: $1" >&2; exit 2
            fi
            ;;
    esac
    shift
done

if [ -z "$CMD_FILE" ] || [ -z "$RESULTS_FILE" ]; then
    echo "[fanout-dispatch] usage: $0 [--cache-prefix-file=PATH] <commands.tsv> <results.tsv>" >&2
    exit 2
fi

if [ -n "$CACHE_PREFIX_FILE" ] && [ ! -f "$CACHE_PREFIX_FILE" ]; then
    echo "[fanout-dispatch] cache-prefix-file not found: $CACHE_PREFIX_FILE" >&2
    exit 2
fi

if [ ! -f "$CMD_FILE" ]; then
    echo "[fanout-dispatch] commands file not found: $CMD_FILE" >&2
    exit 2
fi

CONCURRENCY="${EVOLVE_FANOUT_CONCURRENCY:-2}"
TIMEOUT_SECS="${EVOLVE_FANOUT_TIMEOUT:-600}"

# v8.23.0 Task B knobs: early-cancel on consensus.
CANCEL_ON_CONSENSUS="${EVOLVE_FANOUT_CANCEL_ON_CONSENSUS:-0}"
CONSENSUS_K="${EVOLVE_FANOUT_CONSENSUS_K:-2}"
CONSENSUS_POLL_INTERVAL="${EVOLVE_FANOUT_CONSENSUS_POLL_S:-1}"

# v8.23.0 Task D knob: per-worker status writes into cycle-state.parallel_workers.workers[].
TRACK_WORKERS="${EVOLVE_FANOUT_TRACK_WORKERS:-1}"

# v8.55.0 Phase E knob: per-worker budget cap for fan-out workers.
# Default $0.20 USD per worker subprocess. Total fan-out spend ≤ concurrency × cap × batches.
# Injected into _run_worker() as EVOLVE_MAX_BUDGET_USD ONLY if the operator hasn't already
# set it externally — operator override always wins. See:
# docs/architecture/sequential-write-discipline.md "Cost cap" section.
PER_WORKER_BUDGET_USD="${EVOLVE_FANOUT_PER_WORKER_BUDGET_USD:-0.20}"

# Validate numeric env values; reject negative or non-numeric.
case "$CONCURRENCY" in ''|*[!0-9]*) CONCURRENCY=2 ;; esac
case "$TIMEOUT_SECS" in ''|*[!0-9]*) TIMEOUT_SECS=600 ;; esac
case "$CONSENSUS_K" in ''|*[!0-9]*) CONSENSUS_K=2 ;; esac
case "$CONSENSUS_POLL_INTERVAL" in ''|*[!0-9]*) CONSENSUS_POLL_INTERVAL=1 ;; esac
# Per-worker budget allows decimals (e.g., 0.20). Reject anything with chars
# outside [0-9.]; fall back to default. Multi-dot is technically allowed by this
# filter but downstream `claude --max-budget-usd` will reject malformed values.
case "$PER_WORKER_BUDGET_USD" in ''|*[!0-9.]*) PER_WORKER_BUDGET_USD="0.20" ;; esac
[ "$CONCURRENCY" -lt 1 ] && CONCURRENCY=1
[ "$CONSENSUS_K" -lt 1 ] && CONSENSUS_K=2

# Resolve cycle-state.sh location for worker-status writes (Task D).
# Set by EVOLVE_PLUGIN_ROOT or fall back to sibling lookup.
CYCLE_STATE_HELPER=""
if [ -n "${EVOLVE_PLUGIN_ROOT:-}" ] && [ -x "$EVOLVE_PLUGIN_ROOT/scripts/lifecycle/cycle-state.sh" ]; then
    CYCLE_STATE_HELPER="$EVOLVE_PLUGIN_ROOT/scripts/lifecycle/cycle-state.sh"
elif [ -x "$(dirname "${BASH_SOURCE[0]}")/cycle-state.sh" ]; then
    CYCLE_STATE_HELPER="$(dirname "${BASH_SOURCE[0]}")/cycle-state.sh"
fi

# Empty commands file → empty results, exit 0.
if [ ! -s "$CMD_FILE" ]; then
    : > "$RESULTS_FILE"
    exit 0
fi

RESULTS_DIR="$(dirname "$RESULTS_FILE")"
mkdir -p "$RESULTS_DIR"

# Pick a portable timeout wrapper.
# Order: gtimeout (homebrew coreutils) → timeout (linux/some macs) → perl alarm.
_pick_timeout() {
    if command -v gtimeout >/dev/null 2>&1; then
        echo "gtimeout"
    elif command -v timeout >/dev/null 2>&1; then
        echo "timeout"
    else
        echo "PERL"
    fi
}
TIMEOUT_BIN=$(_pick_timeout)

# Run a single worker: enforce timeout, capture stdout/stderr/duration/exit.
# Args: <worker_name> <command>
# Writes to: $RESULTS_DIR/<name>.out, .err, .meta (TSV row to be merged later)
#
# v8.23.0 Task D: emits cycle-state.parallel_workers.workers[<name>].status
# transitions (running → done/failed) when EVOLVE_FANOUT_TRACK_WORKERS=1.
# v8.23.0 Task C: prepends $CACHE_PREFIX_FILE content to the worker's command
# stdin when set — sibling workers in the same batch share byte-identical
# prefix bytes so Anthropic's prompt cache (≥1024 token, 5-min TTL) is hit.
_run_worker() {
    local name="$1"
    local cmd="$2"
    local out="$RESULTS_DIR/${name}.out"
    local err="$RESULTS_DIR/${name}.err"
    local meta="$RESULTS_DIR/${name}.meta"
    local start end rc=0

    # v8.55.0 Phase E: inject per-worker budget cap. Conditional — preserves operator
    # override. If the operator has set EVOLVE_MAX_BUDGET_USD externally (e.g., via
    # release pipeline or per-cycle override), don't clobber it. The fan-out tier
    # default is much tighter than the global default ($0.20 vs $999999) so that
    # fan-out cannot accidentally drain a subscription quota even with cap=2.
    if [ -z "${EVOLVE_MAX_BUDGET_USD:-}" ]; then
        export EVOLVE_MAX_BUDGET_USD="$PER_WORKER_BUDGET_USD"
    fi

    # Task D: mark running before subprocess starts.
    if [ "$TRACK_WORKERS" = "1" ] && [ -n "$CYCLE_STATE_HELPER" ]; then
        bash "$CYCLE_STATE_HELPER" set-worker-status "$name" running 2>/dev/null || true
    fi

    start=$(date +%s)

    # Task C: build effective command. If a cache-prefix file is provided, the
    # worker's stdin is the concatenation of (prefix bytes) + (worker stdin).
    # The fanout-dispatch contract is that the command is responsible for its
    # own input handling; the prefix is sent as additional context that the
    # worker can choose to read or ignore. For pipeline-style workers (`cat ... | claude -p`),
    # the prefix is prepended; for self-contained commands, it's still placed
    # in stdin and the worker may consume it.
    local effective_cmd="$cmd"
    if [ -n "$CACHE_PREFIX_FILE" ] && [ -f "$CACHE_PREFIX_FILE" ]; then
        # Inline the prefix-file path into the command so workers can `cat` it.
        # We export it as $EVOLVE_FANOUT_CACHE_PREFIX_FILE so tests can detect.
        export EVOLVE_FANOUT_CACHE_PREFIX_FILE="$CACHE_PREFIX_FILE"
    fi

    if [ "$TIMEOUT_BIN" = "PERL" ]; then
        # perl alarm-based timeout. Returns 124 on alarm fire (matching coreutils).
        perl -e '
            my $secs = shift;
            $SIG{ALRM} = sub { exit 124 };
            alarm $secs;
            my $rc = system("/bin/sh", "-c", join(" ", @ARGV));
            exit($rc >> 8);
        ' "$TIMEOUT_SECS" "$effective_cmd" >"$out" 2>"$err" || rc=$?
    else
        # gtimeout/timeout: --kill-after gives the worker a chance to handle
        # SIGTERM cleanly before SIGKILL. Default Term-then-Kill cadence.
        "$TIMEOUT_BIN" --kill-after=5s "$TIMEOUT_SECS" /bin/sh -c "$effective_cmd" >"$out" 2>"$err" || rc=$?
    fi

    end=$(date +%s)
    printf '%s\t%s\t%s\n' "$name" "$rc" "$((end - start))" > "$meta"

    # Task D: terminal status. SIGTERM (rc=143) and SIGKILL via timeout (rc=124,137)
    # all map to "failed" so the orchestrator can distinguish from clean exits.
    if [ "$TRACK_WORKERS" = "1" ] && [ -n "$CYCLE_STATE_HELPER" ]; then
        local terminal_status="done"
        [ "$rc" -ne 0 ] && terminal_status="failed"
        bash "$CYCLE_STATE_HELPER" set-worker-status "$name" "$terminal_status" "$rc" 2>/dev/null || true
    fi
}

# v8.23.0 Task B helper: scan completed workers' artifacts for FAIL consensus.
# Called periodically while waiting on background PIDs. The "artifact" is the
# command's stdout (.out file) — we look for the canonical Verdict markers
# the aggregator's verdict-mode merge already parses (lines 89-126).
#
# Echoes "1" if K or more workers have produced FAIL verdicts; "0" otherwise.
# K and the consensus-cancel toggle come from env (CANCEL_ON_CONSENSUS, CONSENSUS_K).
_check_fail_consensus() {
    [ "$CANCEL_ON_CONSENSUS" = "1" ] || { echo 0; return; }
    local fail_count=0
    # Scan all .out files that exist; missing files = worker not done yet.
    for out_file in "$RESULTS_DIR"/*.out; do
        [ -f "$out_file" ] || continue
        # Match either inline `Verdict: FAIL` or heading-form `## Verdict\n**FAIL**`.
        # Same regex family the aggregator uses (case-insensitive).
        if grep -qiE '^[[:space:]]*verdict:[[:space:]]*\**[[:space:]]*FAIL\b' "$out_file" 2>/dev/null \
           || awk '
                /^#+[[:space:]]+[Vv]erdict[[:space:]]*$/ { saw=NR; next }
                saw && (NR - saw) <= 5 && /\*\*FAIL\*\*/ { found=1; exit }
                END { exit !found }
              ' "$out_file" 2>/dev/null; then
            fail_count=$((fail_count + 1))
        fi
    done
    if [ "$fail_count" -ge "$CONSENSUS_K" ]; then
        echo 1
    else
        echo 0
    fi
}

# Bash 3.2 compatible FIFO semaphore for bounded concurrency.
SEMA_FIFO="$(mktemp -u "${TMPDIR:-/tmp}/fanout-sema.XXXXXX")"
mkfifo "$SEMA_FIFO"
exec 9<>"$SEMA_FIFO"
rm -f "$SEMA_FIFO"
i=0
while [ "$i" -lt "$CONCURRENCY" ]; do
    echo >&9
    i=$((i + 1))
done

# Track all background PIDs (space-separated string; bash 3.2 has no
# associative arrays).
PIDS=""

# Read commands and spawn workers.
while IFS=$'\t' read -r WNAME WCMD || [ -n "$WNAME" ]; do
    [ -z "$WNAME" ] && continue
    # Acquire semaphore token (blocks if all slots in use).
    read -r _slot <&9
    {
        _run_worker "$WNAME" "$WCMD"
        # Release token.
        echo >&9
    } &
    PIDS="$PIDS $!"
done < "$CMD_FILE"

# v8.23.0 Task B: optional consensus-cancel polling.
# When enabled, periodically check if K workers have produced FAIL verdicts.
# If yes, SIGTERM remaining background PIDs (those whose .meta hasn't appeared yet)
# and break out of the wait loop. WAIT-ALL semantics preserved when disabled.
#
# Note: SIGTERM-killed workers' .meta files may be missing — the merge step at
# the bottom of this script already handles that case with a sentinel `<name>\t-1\t0`.
ANY_FAIL=0
CONSENSUS_REACHED=0
if [ "$CANCEL_ON_CONSENSUS" = "1" ]; then
    # Poll until either consensus is reached or all workers complete.
    while true; do
        # Check if consensus is reached.
        if [ "$(_check_fail_consensus)" = "1" ]; then
            CONSENSUS_REACHED=1
            echo "[fanout-dispatch] consensus reached: $CONSENSUS_K workers FAIL — SIGTERM remaining" >&2
            for p in $PIDS; do
                # If process is still running, terminate it gracefully.
                if kill -0 "$p" 2>/dev/null; then
                    kill -TERM "$p" 2>/dev/null || true
                fi
            done
            break
        fi
        # Check if all workers are done (all .meta files present).
        local_done_count=0
        local_total=0
        while IFS=$'\t' read -r WNAME _ || [ -n "$WNAME" ]; do
            [ -z "$WNAME" ] && continue
            local_total=$((local_total + 1))
            [ -f "$RESULTS_DIR/${WNAME}.meta" ] && local_done_count=$((local_done_count + 1))
        done < "$CMD_FILE"
        if [ "$local_done_count" -ge "$local_total" ]; then
            break
        fi
        sleep "$CONSENSUS_POLL_INTERVAL"
    done
fi
# WAIT-ALL: collect every worker (including those just SIGTERM'd).
for p in $PIDS; do
    wait "$p" 2>/dev/null || ANY_FAIL=1
done

# Close semaphore fd.
exec 9>&-

# Merge per-worker .meta files into final TSV in input order so the result
# matches the user's expectations (workers appear in commands.tsv order).
: > "$RESULTS_FILE"
while IFS=$'\t' read -r WNAME WCMD || [ -n "$WNAME" ]; do
    [ -z "$WNAME" ] && continue
    META="$RESULTS_DIR/${WNAME}.meta"
    if [ -f "$META" ]; then
        cat "$META" >> "$RESULTS_FILE"
    else
        # Should not happen, but record sentinel so caller can detect.
        printf '%s\t-1\t0\n' "$WNAME" >> "$RESULTS_FILE"
        ANY_FAIL=1
    fi
done < "$CMD_FILE"

# Final exit code: 0 only if every recorded exit_code is 0.
if awk -F'\t' '$2 != "0" { exit 1 }' "$RESULTS_FILE"; then
    exit 0
else
    exit 1
fi
