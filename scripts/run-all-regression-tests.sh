#!/usr/bin/env bash
#
# run-all-regression-tests.sh — Single-command runner for all regression suites (v8.13.6).
#
# v8.13.5 audit identified that the auditor profile's allowlist permits each
# individual `Bash(bash scripts/<test>.sh:*)` entry but NOT compositions like
# `bash a.sh & bash b.sh & wait` (parallel) or `for s in ...; do bash $s.sh; done`
# (loop). Auditors trying to run the full regression matrix had to either
# allowlist 12 distinct entries OR find another mechanism. This helper is that
# mechanism: ONE allowlisted command runs them all.
#
# Usage:
#   bash scripts/run-all-regression-tests.sh                # sequential
#   bash scripts/run-all-regression-tests.sh --parallel     # parallel via &+wait
#   bash scripts/run-all-regression-tests.sh --json         # machine-readable summary
#
# Exit codes:
#   0 — every listed suite reports zero failures
#   1 — one or more suites had failures (or didn't exit cleanly)
#  10 — bad arguments

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

PARALLEL=0
JSON=0

while [ $# -gt 0 ]; do
    case "$1" in
        --parallel) PARALLEL=1 ;;
        --json)     JSON=1 ;;
        --help|-h)  sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*)        echo "[run-all] unknown flag: $1" >&2; exit 10 ;;
        *)          echo "[run-all] extra positional arg: $1" >&2; exit 10 ;;
    esac
    shift
done

# The canonical regression suite list. Order matters only for sequential mode
# (more-frequently-broken suites first to fail fast).
# v8.13.6: SUITES_OVERRIDE env var (space-separated paths) takes precedence —
# used by unit tests to inject stub suites without rewriting this file.
if [ -n "${SUITES_OVERRIDE:-}" ]; then
    # bash 3.2 compatible: split on whitespace via read.
    SUITES=()
    for s in $SUITES_OVERRIDE; do SUITES+=("$s"); done
else
    SUITES=(
        "scripts/tests/claude-adapter-test.sh"
        "scripts/tests/codex-adapter-test.sh"
        "scripts/tests/gemini-adapter-test.sh"
        "scripts/tests/evolve-loop-dispatch-test.sh"
        "scripts/tests/probe-tool-test.sh"
        "scripts/tests/postedit-validate-test.sh"
        "scripts/tests/cycle-state-test.sh"
        "scripts/tests/resolve-roots-test.sh"
        "scripts/tests/intent-test.sh"
        "scripts/tests/orchestrator-sandbox-coverage-test.sh"
        "scripts/tests/role-gate-test.sh"
        "scripts/tests/phase-gate-precondition-test.sh"
        "scripts/tests/guards-test.sh"
        "scripts/release/preflight-test.sh"
        "scripts/release/changelog-gen-test.sh"
        "scripts/release/marketplace-poll-test.sh"
        "scripts/release/rollback-test.sh"
        "scripts/tests/release-pipeline-test.sh"
        "scripts/tests/ship-integration-test.sh"
        "scripts/tests/show-cycle-cost-test.sh"
        "scripts/tests/merge-lesson-test.sh"
        "scripts/tests/subagent-run-test.sh"
        "scripts/tests/run-all-regression-tests-test.sh"
        "scripts/tests/fanout-dispatch-test.sh"
        "scripts/tests/aggregator-test.sh"
        "scripts/tests/dispatch-parallel-test.sh"
        "scripts/tests/swarm-architecture-test.sh"
        "scripts/tests/failure-adapter-test.sh"
        "scripts/tests/state-prune-test.sh"
        "scripts/tests/preflight-environment-test.sh"
        "scripts/tests/diff-complexity-test.sh"
        "scripts/tests/run-cycle-worktree-test.sh"
        "scripts/tests/verify-ledger-chain-test.sh"
    )
fi

# Runs one suite; emits a summary line `<suite>:<rc>` to a tmp file.
# Captures the suite's tail (last 5 lines of mixed stdout+stderr) for diagnosis.
run_suite() {
    local suite="$1"
    local results_file="$2"
    local tmp_out
    tmp_out=$(mktemp -t runall.XXXXXX)
    set +e
    bash "$REPO_ROOT/$suite" > "$tmp_out" 2>&1
    local rc=$?
    set -e
    # Append result line atomically (use flock or just rely on individual mv-of-tmp).
    {
        echo "$suite|$rc|$tmp_out"
    } >> "$results_file"
}

results=$(mktemp -t runall-results.XXXXXX)
trap 'rm -f "$results"; for f in $(awk -F"|" "{print \$3}" "$results" 2>/dev/null); do rm -f "$f"; done' EXIT

start_ts=$(date -u +%s)

if [ "$PARALLEL" = "1" ]; then
    for s in "${SUITES[@]}"; do
        run_suite "$s" "$results" &
    done
    wait
else
    for s in "${SUITES[@]}"; do
        run_suite "$s" "$results"
    done
fi

elapsed=$(( $(date -u +%s) - start_ts ))

# Sort by suite name so output is deterministic regardless of parallel mode.
sort "$results" -o "$results"

# Tally results.
TOTAL=0
PASSED=0
FAILED=0

declare -a SUITE_RESULTS
while IFS='|' read -r suite rc tmpfile; do
    TOTAL=$((TOTAL + 1))
    if [ "$rc" = "0" ]; then
        PASSED=$((PASSED + 1))
        SUITE_RESULTS+=("$suite|$rc|PASS")
    else
        FAILED=$((FAILED + 1))
        SUITE_RESULTS+=("$suite|$rc|FAIL")
    fi
done < "$results"

# --- Output ---------------------------------------------------------------

if [ "$JSON" = "1" ]; then
    suites_json="[]"
    for entry in "${SUITE_RESULTS[@]}"; do
        IFS='|' read -r suite rc status <<< "$entry"
        suites_json=$(echo "$suites_json" | jq --arg s "$suite" --arg r "$rc" --arg st "$status" \
            '. + [{suite:$s, rc:($r|tonumber), status:$st}]')
    done
    jq -nc \
        --argjson total "$TOTAL" \
        --argjson passed "$PASSED" \
        --argjson failed "$FAILED" \
        --argjson elapsed "$elapsed" \
        --argjson parallel "$PARALLEL" \
        --argjson suites "$suites_json" \
        '{total: $total, passed: $passed, failed: $failed, elapsed_s: $elapsed, parallel: ($parallel == 1), suites: $suites}'
else
    if [ "$PARALLEL" = "1" ]; then
        printf 'Mode: PARALLEL (12 suites concurrent)\n'
    else
        printf 'Mode: SEQUENTIAL\n'
    fi
    printf 'Elapsed: %ds\n' "$elapsed"
    printf '\n'
    for entry in "${SUITE_RESULTS[@]}"; do
        IFS='|' read -r suite rc status <<< "$entry"
        if [ "$status" = "PASS" ]; then
            printf '  ✓ %-50s rc=%s\n' "$suite" "$rc"
        else
            printf '  ✗ %-50s rc=%s\n' "$suite" "$rc"
        fi
    done
    printf '\n'
    printf 'Total: %d  Passed: %d  Failed: %d\n' "$TOTAL" "$PASSED" "$FAILED"
fi

if [ "$FAILED" = "0" ]; then
    exit 0
else
    # Print last few lines of any failed suite's output to aid diagnosis.
    if [ "$JSON" != "1" ]; then
        echo
        echo '─── Diagnostic: tails of failed suites ───'
        while IFS='|' read -r suite rc tmpfile; do
            if [ "$rc" != "0" ] && [ -f "$tmpfile" ]; then
                echo
                echo "── $suite (rc=$rc) ──"
                tail -10 "$tmpfile"
            fi
        done < "$results"
    fi
    exit 1
fi
