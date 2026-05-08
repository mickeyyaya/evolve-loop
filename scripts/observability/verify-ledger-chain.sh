#!/usr/bin/env bash
#
# verify-ledger-chain.sh — v8.37.0 tamper-evident ledger verifier (read-only).
#
# Walks .evolve/ledger.jsonl from beginning to end, recomputes prev_hash for
# each v8.37+ entry, verifies the chain. Also verifies the rolling tip file
# (.evolve/ledger.tip) matches the SHA256 of the actual last entry.
#
# Detects:
#   1. Entry rewrite: modifying an old entry breaks every entry after it.
#   2. Truncation: lopping the last N entries breaks tip-vs-actual-last match.
#   3. Forgery insertion: splicing a fake entry between two real ones breaks
#      the next entry's prev_hash check.
#   4. Concurrent-fan-out anomalies: two entries with identical prev_hash.
#
# Tolerates:
#   - Pre-v8.37 entries (no prev_hash field) — treats as a soft-start boundary.
#     The first v8.37+ entry chains from the last pre-v8.37 entry's SHA, but
#     pre-v8.37 entries themselves are not retro-validated (they predate the
#     field).
#   - Brand-new ledger (zero entries) — exits 0 with "nothing to verify".
#
# Usage:
#   bash scripts/observability/verify-ledger-chain.sh                # human-readable
#   bash scripts/observability/verify-ledger-chain.sh --json         # machine-readable
#   bash scripts/observability/verify-ledger-chain.sh --quiet        # exit-code only (CI)
#   bash scripts/observability/verify-ledger-chain.sh --ledger PATH  # alternate ledger
#
# Exit codes:
#   0 — chain intact, tip matches (or ledger is empty)
#   1 — chain break detected (entry mutilated, fake entry inserted, etc.)
#   2 — tip file missing/mismatched (truncation or untracked write)
#  10 — bad arguments

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEFAULT_LEDGER="${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}/.evolve/ledger.jsonl"

LEDGER="$DEFAULT_LEDGER"
JSON=0
QUIET=0

while [ $# -gt 0 ]; do
    case "$1" in
        --json)   JSON=1 ;;
        --quiet)  QUIET=1 ;;
        --ledger) shift; [ $# -ge 1 ] || { echo "[verify-ledger-chain] --ledger requires path" >&2; exit 10; }
                  LEDGER="$1" ;;
        --help|-h) sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*)      echo "[verify-ledger-chain] unknown flag: $1" >&2; exit 10 ;;
        *)        echo "[verify-ledger-chain] unknown arg: $1" >&2; exit 10 ;;
    esac
    shift
done

TIP_FILE="$(dirname "$LEDGER")/ledger.tip"

# Helper: SHA256 of stdin.
sha256_stdin() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}';
    else shasum -a 256 | awk '{print $1}'; fi
}

# Empty / missing ledger → trivially clean.
if [ ! -f "$LEDGER" ] || [ ! -s "$LEDGER" ]; then
    if [ "$JSON" = "1" ]; then
        echo '{"total_entries":0,"chain_breaks":0,"tip_match":null,"first_break_seq":null,"status":"empty"}'
    elif [ "$QUIET" = "0" ]; then
        echo "[verify-ledger-chain] ledger empty or missing ($LEDGER) — nothing to verify"
    fi
    exit 0
fi

command -v jq >/dev/null 2>&1 || { echo "[verify-ledger-chain] jq required" >&2; exit 10; }

# Walk ledger entry-by-entry. Track:
#   - Number of total entries
#   - Number of v8.37+ entries (have prev_hash field)
#   - First chain break (if any)
#   - SHA256 of previous line (to compare against current's prev_hash)

total_entries=0
v837_entries=0
chain_breaks=0
first_break_seq=""
duplicate_prev_hash_seen=0
prev_line_sha=""
seen_prev_hashes=""

# Read each line. Bash 3.2 portable: while-IFS-read pattern.
while IFS= read -r line || [ -n "$line" ]; do
    [ -z "$line" ] && continue
    total_entries=$((total_entries + 1))
    # Extract prev_hash + entry_seq if present
    has_prev_hash=$(echo "$line" | jq -r 'has("prev_hash")' 2>/dev/null || echo "false")
    entry_seq=$(echo "$line" | jq -r '.entry_seq // -1' 2>/dev/null || echo "-1")
    role=$(echo "$line" | jq -r '.role // "?"' 2>/dev/null || echo "?")
    cycle=$(echo "$line" | jq -r '.cycle // 0' 2>/dev/null || echo 0)
    if [ "$has_prev_hash" = "true" ]; then
        v837_entries=$((v837_entries + 1))
        recorded_prev=$(echo "$line" | jq -r '.prev_hash' 2>/dev/null || echo "")
        # Special case: first v8.37 entry following pre-v8.37 entries.
        # Its prev_hash should be SHA256 of the last pre-v8.37 line.
        if [ -n "$prev_line_sha" ] && [ "$recorded_prev" != "$prev_line_sha" ]; then
            chain_breaks=$((chain_breaks + 1))
            if [ -z "$first_break_seq" ]; then
                first_break_seq="$entry_seq"
                first_break_role="$role"
                first_break_cycle="$cycle"
                first_break_recorded="$recorded_prev"
                first_break_actual="$prev_line_sha"
            fi
        elif [ -z "$prev_line_sha" ] && [ "$recorded_prev" != "0000000000000000000000000000000000000000000000000000000000000000" ]; then
            # First entry overall but prev_hash isn't zero-init
            chain_breaks=$((chain_breaks + 1))
            if [ -z "$first_break_seq" ]; then
                first_break_seq="$entry_seq"
                first_break_role="$role"
                first_break_cycle="$cycle"
                first_break_recorded="$recorded_prev"
                first_break_actual="0000...0000 (first-entry zero-init expected)"
            fi
        fi
        # Detect duplicate prev_hash (concurrent fan-out anomaly)
        case " $seen_prev_hashes " in
            *" $recorded_prev "*) duplicate_prev_hash_seen=1 ;;
            *) seen_prev_hashes="$seen_prev_hashes $recorded_prev" ;;
        esac
    fi
    # Compute SHA of THIS line for next iteration's check.
    prev_line_sha=$(printf '%s' "$line" | sha256_stdin)
done < "$LEDGER"

# Tip file check.
tip_match="unknown"
tip_reason=""
if [ -f "$TIP_FILE" ]; then
    tip_content=$(cat "$TIP_FILE" 2>/dev/null || echo "")
    tip_seq=$(echo "$tip_content" | cut -d: -f1)
    tip_sha=$(echo "$tip_content" | cut -d: -f2)
    # Validate format.
    if [ -z "$tip_seq" ] || [ -z "$tip_sha" ] || [ ${#tip_sha} != 64 ]; then
        tip_match="malformed"
        tip_reason="tip file format invalid (expected 'seq:sha256')"
    elif [ "$tip_sha" = "$prev_line_sha" ]; then
        tip_match="match"
    else
        tip_match="mismatch"
        tip_reason="tip references SHA $tip_sha but actual last entry SHA is $prev_line_sha (truncation suspected)"
    fi
else
    # Tip file missing. If there are ZERO v8.37 entries, this is fine
    # (tip is only created on first v8.37 write). Otherwise it's a problem.
    if [ "$v837_entries" -gt 0 ]; then
        tip_match="missing"
        tip_reason="tip file should exist (>=1 v8.37 entry written) but is missing"
    else
        tip_match="not-applicable"
    fi
fi

# Determine final exit code.
exit_code=0
status="OK"
if [ "$chain_breaks" -gt 0 ]; then
    exit_code=1
    status="CHAIN-BREAK"
elif [ "$duplicate_prev_hash_seen" = "1" ]; then
    exit_code=1
    status="DUPLICATE-PREV-HASH"
elif [ "$tip_match" = "mismatch" ] || [ "$tip_match" = "malformed" ] || [ "$tip_match" = "missing" ]; then
    exit_code=2
    status="TIP-MISMATCH"
fi

# Output.
if [ "$JSON" = "1" ]; then
    jq -nc \
        --argjson total "$total_entries" \
        --argjson v837 "$v837_entries" \
        --argjson breaks "$chain_breaks" \
        --argjson dup "$duplicate_prev_hash_seen" \
        --arg tip_match "$tip_match" \
        --arg first_break_seq "${first_break_seq:-}" \
        --arg status "$status" \
        '{total_entries: $total, v837_entries: $v837,
          chain_breaks: $breaks, duplicate_prev_hash: ($dup == 1),
          tip_match: $tip_match,
          first_break_seq: (if $first_break_seq == "" then null else ($first_break_seq | tonumber) end),
          status: $status}'
elif [ "$QUIET" = "0" ]; then
    echo "[verify-ledger-chain] ledger=$LEDGER"
    echo "[verify-ledger-chain] total entries: $total_entries (v8.37+ chained: $v837_entries)"
    echo "[verify-ledger-chain] chain breaks: $chain_breaks"
    if [ -n "${first_break_seq:-}" ]; then
        echo "[verify-ledger-chain]   first break at entry_seq=$first_break_seq (role=$first_break_role cycle=$first_break_cycle)"
        echo "[verify-ledger-chain]   recorded prev_hash: $first_break_recorded"
        echo "[verify-ledger-chain]   actual previous SHA: $first_break_actual"
    fi
    if [ "$duplicate_prev_hash_seen" = "1" ]; then
        echo "[verify-ledger-chain] WARN: duplicate prev_hash detected (concurrent fan-out anomaly)"
    fi
    echo "[verify-ledger-chain] tip: $tip_match"
    [ -n "$tip_reason" ] && echo "[verify-ledger-chain]   $tip_reason"
    echo "[verify-ledger-chain] status: $status (exit $exit_code)"
fi

exit $exit_code
