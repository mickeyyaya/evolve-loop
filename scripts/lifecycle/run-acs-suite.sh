#!/usr/bin/env bash
#
# run-acs-suite.sh — EGPS predicate suite runner (v10.0.0+).
#
# Runs all predicates in:
#   - acs/cycle-N/*.sh           (this cycle's new predicates)
#   - acs/regression-suite/**/*.sh (every prior cycle's accumulated predicates)
#
# Emits acs-verdict.json — the verdict-bearing artifact replacing the
# auditor's prose Verdict scalar. The verdict is binary: PASS iff every
# predicate exits 0; FAIL otherwise.
#
# Per v10 EGPS contract:
#   - No WARN level (binary verdict)
#   - No confidence scalar (exit-code vector)
#   - Regression-suite predicates count as REGRESSIONS when they fail
#
# Usage:
#   bash scripts/lifecycle/run-acs-suite.sh <cycle>                    # default output to workspace
#   bash scripts/lifecycle/run-acs-suite.sh <cycle> --json              # print verdict to stdout
#   bash scripts/lifecycle/run-acs-suite.sh <cycle> --acs-dir <path>    # override acs/ base
#
# Exit codes:
#   0  — all predicates GREEN (verdict PASS)
#   1  — at least one predicate RED (verdict FAIL)
#   2  — no predicates found
#  10  — bad arguments

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
if [ -f "$SCRIPT_DIR/../lib/acs-schema.sh" ]; then
    source "$SCRIPT_DIR/../lib/acs-schema.sh"
else
    echo "[run-acs-suite] cannot locate scripts/lib/acs-schema.sh" >&2
    exit 1
fi

CYCLE=""
JSON=0
ACS_DIR_OVERRIDE=""

while [ $# -gt 0 ]; do
    case "$1" in
        --json) JSON=1 ;;
        --acs-dir) shift; ACS_DIR_OVERRIDE="$1" ;;
        --help|-h) sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[run-acs-suite] unknown flag: $1" >&2; exit 10 ;;
        *)
            [ -z "$CYCLE" ] && CYCLE="$1" || { echo "[run-acs-suite] too many args" >&2; exit 10; }
            ;;
    esac
    shift
done

[ -n "$CYCLE" ] || { echo "[run-acs-suite] usage: $0 <cycle> [--json] [--acs-dir PATH]" >&2; exit 10; }
[[ "$CYCLE" =~ ^[0-9]+$ ]] || { echo "[run-acs-suite] cycle must be integer" >&2; exit 10; }

command -v jq >/dev/null 2>&1 || { echo "[run-acs-suite] jq required" >&2; exit 1; }

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
ACS_BASE="${ACS_DIR_OVERRIDE:-$PROJECT_ROOT/acs}"
WORKSPACE="${EVOLVE_WORKSPACE_OVERRIDE:-$PROJECT_ROOT/.evolve/runs/cycle-$CYCLE}"

CYCLE_DIR="$ACS_BASE/cycle-$CYCLE"
REGRESSION_DIR="$ACS_BASE/regression-suite"

# ── Discover predicates ──────────────────────────────────────────────────
predicates=()
if [ -d "$CYCLE_DIR" ]; then
    while IFS= read -r f; do
        predicates+=("$f")
    done < <(find "$CYCLE_DIR" -maxdepth 1 -name "*.sh" -type f 2>/dev/null | sort)
fi
this_cycle_count=${#predicates[@]}

if [ -d "$REGRESSION_DIR" ]; then
    while IFS= read -r f; do
        predicates+=("$f")
    done < <(find "$REGRESSION_DIR" -maxdepth 2 -name "*.sh" -type f 2>/dev/null | sort)
fi
total_count=${#predicates[@]}
regression_count=$((total_count - this_cycle_count))

if [ "$total_count" -eq 0 ]; then
    # v10.0.0 bootstrap: when there are NO predicates at all (first cycle, no regression-suite),
    # produce an empty-PASS verdict. Subsequent cycles will accumulate.
    verdict_json=$(jq -nc \
        --argjson cycle "$CYCLE" \
        --arg sv "$ACS_VERDICT_SCHEMA_VERSION" \
        '{
            schema_version: $sv,
            cycle: $cycle,
            predicate_suite: {this_cycle_count: 0, regression_suite_count: 0, total: 0},
            results: [],
            green_count: 0,
            red_count: 0,
            red_ids: [],
            verdict: "PASS",
            ship_eligible: true,
            note: "bootstrap — no predicates yet for this cycle"
        }')
    if [ "$JSON" = "1" ]; then
        echo "$verdict_json" | jq '.'
    else
        out="$WORKSPACE/acs-verdict.json"
        mkdir -p "$WORKSPACE"
        echo "$verdict_json" > "$out.tmp.$$" && mv -f "$out.tmp.$$" "$out"
        echo "[run-acs-suite] wrote $out (BOOTSTRAP: no predicates yet)"
    fi
    exit 0
fi

# ── Run each predicate ──────────────────────────────────────────────────
results_tmp=$(mktemp -t acs-results.XXXXXX)
trap 'rm -f "$results_tmp"' EXIT
green_count=0
red_count=0
red_ids=()

for pred in "${predicates[@]}"; do
    fname=$(basename "$pred")
    ac_id=$(acs_predicate_header "AC-ID" "$pred" 2>/dev/null)
    [ -z "$ac_id" ] && ac_id="(unknown)"

    is_regression="false"
    [[ "$pred" == *"/regression-suite/"* ]] && is_regression="true"

    start_ms=$(date +%s)
    pred_output=$(bash "$pred" 2>&1); rc=$?
    end_ms=$(date +%s)
    duration_s=$((end_ms - start_ms))
    duration_ms=$((duration_s * 1000))

    if [ "$rc" -eq 0 ]; then
        result="green"
        green_count=$((green_count + 1))
    else
        result="red"
        red_count=$((red_count + 1))
        red_ids+=("$ac_id")
    fi

    # Truncate evidence excerpt to keep verdict.json manageable.
    evidence_excerpt=$(echo "$pred_output" | head -c 500 | jq -Rs '.')

    jq -nc \
        --arg ac_id "$ac_id" \
        --arg pred "$pred" \
        --argjson rc "$rc" \
        --arg result "$result" \
        --argjson dur "$duration_ms" \
        --argjson reg "$is_regression" \
        --argjson evidence "$evidence_excerpt" \
        '{
            ac_id: $ac_id,
            predicate: $pred,
            exit_code: $rc,
            result: $result,
            duration_ms: $dur,
            is_regression: $reg,
            evidence_excerpt: $evidence
        }' >> "$results_tmp"
done

results_json=$(jq -s '.' "$results_tmp")
red_ids_json=$(printf '%s\n' "${red_ids[@]:-}" | jq -Rcs 'split("\n") | map(select(length > 0))')

if [ "$red_count" -eq 0 ]; then
    verdict="$ACS_VERDICT_PASS"
    ship_eligible=true
else
    verdict="$ACS_VERDICT_FAIL"
    ship_eligible=false
fi

verdict_json=$(jq -nc \
    --arg sv "$ACS_VERDICT_SCHEMA_VERSION" \
    --argjson cycle "$CYCLE" \
    --argjson this_cycle "$this_cycle_count" \
    --argjson reg "$regression_count" \
    --argjson total "$total_count" \
    --argjson green "$green_count" \
    --argjson red "$red_count" \
    --argjson red_ids "$red_ids_json" \
    --argjson results "$results_json" \
    --arg verdict "$verdict" \
    --argjson eligible "$ship_eligible" \
    '{
        schema_version: $sv,
        cycle: $cycle,
        predicate_suite: {
            this_cycle_count: $this_cycle,
            regression_suite_count: $reg,
            total: $total
        },
        results: $results,
        green_count: $green,
        red_count: $red,
        red_ids: $red_ids,
        verdict: $verdict,
        ship_eligible: $eligible
    }')

if [ "$JSON" = "1" ]; then
    echo "$verdict_json" | jq '.'
else
    out="$WORKSPACE/acs-verdict.json"
    mkdir -p "$WORKSPACE"
    echo "$verdict_json" > "$out.tmp.$$" && mv -f "$out.tmp.$$" "$out"
    echo "[run-acs-suite] verdict=$verdict green=$green_count red=$red_count total=$total_count → $out"
fi

[ "$red_count" -eq 0 ] && exit 0 || exit 1
