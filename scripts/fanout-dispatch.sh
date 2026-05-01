#!/usr/bin/env bash
#
# fanout-dispatch.sh — Bounded-concurrency parallel worker dispatcher (Sprint 1).
#
# Reads a TSV commands file (`<worker_name>\t<command>\n`) and runs each
# command concurrently, bounded by EVOLVE_FANOUT_CONCURRENCY (default 4).
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

CMD_FILE="${1:-}"
RESULTS_FILE="${2:-}"

if [ -z "$CMD_FILE" ] || [ -z "$RESULTS_FILE" ]; then
    echo "[fanout-dispatch] usage: $0 <commands.tsv> <results.tsv>" >&2
    exit 2
fi

if [ ! -f "$CMD_FILE" ]; then
    echo "[fanout-dispatch] commands file not found: $CMD_FILE" >&2
    exit 2
fi

CONCURRENCY="${EVOLVE_FANOUT_CONCURRENCY:-4}"
TIMEOUT_SECS="${EVOLVE_FANOUT_TIMEOUT:-600}"

# Validate numeric env values; reject negative or non-numeric.
case "$CONCURRENCY" in ''|*[!0-9]*) CONCURRENCY=4 ;; esac
case "$TIMEOUT_SECS" in ''|*[!0-9]*) TIMEOUT_SECS=600 ;; esac
[ "$CONCURRENCY" -lt 1 ] && CONCURRENCY=1

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
_run_worker() {
    local name="$1"
    local cmd="$2"
    local out="$RESULTS_DIR/${name}.out"
    local err="$RESULTS_DIR/${name}.err"
    local meta="$RESULTS_DIR/${name}.meta"
    local start end rc=0
    start=$(date +%s)

    if [ "$TIMEOUT_BIN" = "PERL" ]; then
        # perl alarm-based timeout. Returns 124 on alarm fire (matching coreutils).
        perl -e '
            my $secs = shift;
            $SIG{ALRM} = sub { exit 124 };
            alarm $secs;
            my $rc = system("/bin/sh", "-c", join(" ", @ARGV));
            exit($rc >> 8);
        ' "$TIMEOUT_SECS" "$cmd" >"$out" 2>"$err" || rc=$?
    else
        # gtimeout/timeout: --kill-after gives the worker a chance to handle
        # SIGTERM cleanly before SIGKILL. Default Term-then-Kill cadence.
        "$TIMEOUT_BIN" --kill-after=5s "$TIMEOUT_SECS" /bin/sh -c "$cmd" >"$out" 2>"$err" || rc=$?
    fi

    end=$(date +%s)
    printf '%s\t%s\t%s\n' "$name" "$rc" "$((end - start))" > "$meta"
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

# WAIT-ALL: collect every worker even if some failed.
ANY_FAIL=0
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
