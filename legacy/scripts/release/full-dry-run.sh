#!/usr/bin/env bash
#
# full-dry-run.sh — Pipeline preflight harness (v8.50.0+).
#
# Runs three sub-suites in sequence and reports a single PASS/FAIL summary:
#
#   1. Regression — scripts/utility/run-all-regression-tests.sh (35 suites, ~30s)
#   2. Cycle simulate — scripts/dispatch/run-cycle.sh --simulate (~5s, no LLM)
#   3. Release pipeline dry-run — scripts/release-pipeline.sh <next> --dry-run (~5s)
#
# Used by:
#   - bin/preflight (operator entry)
#   - scripts/release-pipeline.sh --require-preflight (opt-in pre-flight gate)
#
# Usage:
#   bash scripts/release/full-dry-run.sh                     # use auto-bumped version
#   bash scripts/release/full-dry-run.sh --version X.Y.Z     # use explicit version
#   bash scripts/release/full-dry-run.sh --skip <suite>      # repeatable: regression, simulate, release
#   bash scripts/release/full-dry-run.sh --json              # machine-readable summary
#
# Exit codes:
#   0  — all sub-suites pass
#   1  — one or more sub-suites failed
#  10  — bad arguments

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

JSON=0
SKIPS=""
VERSION=""

while [ $# -gt 0 ]; do
    case "$1" in
        --json)    JSON=1 ;;
        --version)
            shift
            [ $# -gt 0 ] || { echo "[preflight] --version requires a value" >&2; exit 10; }
            VERSION="$1"
            ;;
        --skip)
            shift
            [ $# -gt 0 ] || { echo "[preflight] --skip requires a value" >&2; exit 10; }
            SKIPS="${SKIPS}${SKIPS:+ }$1"
            ;;
        --help|-h)
            sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --*) echo "[preflight] unknown flag: $1" >&2; exit 10 ;;
        *)   echo "[preflight] extra positional arg: $1" >&2; exit 10 ;;
    esac
    shift
done

# Resolve VERSION if not supplied: bump patch from current plugin.json.
if [ -z "$VERSION" ]; then
    current=$(jq -r '.version' "$REPO_ROOT/.claude-plugin/plugin.json" 2>/dev/null)
    if [ -z "$current" ] || [ "$current" = "null" ]; then
        echo "[preflight] could not read current version from plugin.json" >&2
        exit 10
    fi
    # Bump patch: A.B.C -> A.B.(C+1)
    major=$(echo "$current" | awk -F. '{print $1}')
    minor=$(echo "$current" | awk -F. '{print $2}')
    patch=$(echo "$current" | awk -F. '{print $3}')
    VERSION="${major}.${minor}.$((patch + 1))"
fi

skipped() { case " $SKIPS " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }

run_suite() {
    local name="$1" cmd="$2"
    local out
    out=$(mktemp -t "preflight-${name}.XXXXXX")
    local start; start=$(date -u +%s)
    set +e
    bash -c "$cmd" > "$out" 2>&1
    local rc=$?
    set -e
    local end; end=$(date -u +%s)
    local elapsed=$((end - start))
    echo "$rc|$elapsed|$out"
}

# --- run sub-suites ---------------------------------------------------------

declare_results() { :; }  # placeholder for bash 3.2 (no associative arrays)

REG_RC=0; REG_ELAPSED=0; REG_OUT=""
SIM_RC=0; SIM_ELAPSED=0; SIM_OUT=""
REL_RC=0; REL_ELAPSED=0; REL_OUT=""

if skipped regression; then
    REG_RC=-1; REG_OUT="<skipped>"
else
    [ "$JSON" = "0" ] && echo "[preflight] (1/3) running regression suite..."
    res=$(run_suite regression "bash $REPO_ROOT/scripts/utility/run-all-regression-tests.sh")
    REG_RC=$(echo "$res" | awk -F'|' '{print $1}')
    REG_ELAPSED=$(echo "$res" | awk -F'|' '{print $2}')
    REG_OUT=$(echo "$res" | awk -F'|' '{print $3}')
fi

if skipped simulate; then
    SIM_RC=-1; SIM_OUT="<skipped>"
else
    [ "$JSON" = "0" ] && echo "[preflight] (2/3) running cycle simulator (in isolated temp project)..."
    # Create an isolated EVOLVE_PROJECT_ROOT so the simulator's state writes
    # don't pollute the live repo's .evolve/cycle-state.json. We copy in only
    # the scripts the simulator actually invokes; everything else is read from
    # the original plugin root.
    SIM_TEMP=$(mktemp -d -t "preflight-sim.XXXXXX")
    mkdir -p "$SIM_TEMP/scripts/dispatch" "$SIM_TEMP/scripts/lifecycle" \
             "$SIM_TEMP/scripts/observability" "$SIM_TEMP/.evolve/runs"
    cp "$REPO_ROOT/scripts/dispatch/cycle-simulator.sh"        "$SIM_TEMP/scripts/dispatch/"
    cp "$REPO_ROOT/scripts/lifecycle/resolve-roots.sh"          "$SIM_TEMP/scripts/lifecycle/"
    cp "$REPO_ROOT/scripts/lifecycle/cycle-state.sh"            "$SIM_TEMP/scripts/lifecycle/"
    cp "$REPO_ROOT/scripts/lifecycle/ship.sh"                   "$SIM_TEMP/scripts/lifecycle/"
    cp "$REPO_ROOT/scripts/lifecycle/phase-gate.sh"             "$SIM_TEMP/scripts/lifecycle/"
    cp "$REPO_ROOT/scripts/observability/verify-ledger-chain.sh" "$SIM_TEMP/scripts/observability/"
    chmod +x "$SIM_TEMP/scripts"/*/*.sh
    : > "$SIM_TEMP/.evolve/ledger.jsonl"
    echo '{}' > "$SIM_TEMP/.evolve/state.json"
    echo "fixture" > "$SIM_TEMP/fixture.txt"
    (
        cd "$SIM_TEMP"
        git init -q
        git config user.email t@t.t
        git config user.name t
        git config core.hooksPath /dev/null
        echo ".evolve/" > .gitignore
        git add -A
        git -c commit.gpgsign=false commit -q -m "preflight initial"
    ) >/dev/null 2>&1
    SIM_CYCLE=$((90000 + RANDOM % 100))
    EVOLVE_PROJECT_ROOT="$SIM_TEMP" bash "$SIM_TEMP/scripts/lifecycle/cycle-state.sh" \
        init "$SIM_CYCLE" ".evolve/runs/cycle-$SIM_CYCLE" >/dev/null 2>&1
    res=$(run_suite simulate "EVOLVE_PROJECT_ROOT=$SIM_TEMP bash $SIM_TEMP/scripts/dispatch/cycle-simulator.sh $SIM_CYCLE .evolve/runs/cycle-$SIM_CYCLE")
    SIM_RC=$(echo "$res" | awk -F'|' '{print $1}')
    SIM_ELAPSED=$(echo "$res" | awk -F'|' '{print $2}')
    SIM_OUT=$(echo "$res" | awk -F'|' '{print $3}')
    rm -rf "$SIM_TEMP"
fi

if skipped release; then
    REL_RC=-1; REL_OUT="<skipped>"
else
    [ "$JSON" = "0" ] && echo "[preflight] (3/3) running release-pipeline dry-run for v$VERSION..."
    res=$(run_suite release "bash $REPO_ROOT/scripts/release-pipeline.sh $VERSION --dry-run")
    REL_RC=$(echo "$res" | awk -F'|' '{print $1}')
    REL_ELAPSED=$(echo "$res" | awk -F'|' '{print $2}')
    REL_OUT=$(echo "$res" | awk -F'|' '{print $3}')
fi

# --- aggregate --------------------------------------------------------------
TOTAL_FAIL=0
[ "$REG_RC" -gt 0 ] 2>/dev/null && TOTAL_FAIL=$((TOTAL_FAIL + 1))
[ "$SIM_RC" -gt 0 ] 2>/dev/null && TOTAL_FAIL=$((TOTAL_FAIL + 1))
[ "$REL_RC" -gt 0 ] 2>/dev/null && TOTAL_FAIL=$((TOTAL_FAIL + 1))

# --- output -----------------------------------------------------------------
if [ "$JSON" = "1" ]; then
    jq -nc \
        --arg version "$VERSION" \
        --argjson reg_rc "$REG_RC" --argjson reg_t "$REG_ELAPSED" \
        --argjson sim_rc "$SIM_RC" --argjson sim_t "$SIM_ELAPSED" \
        --argjson rel_rc "$REL_RC" --argjson rel_t "$REL_ELAPSED" \
        --argjson failed "$TOTAL_FAIL" \
        '{version: $version, regression: {rc: $reg_rc, elapsed_s: $reg_t},
          simulate: {rc: $sim_rc, elapsed_s: $sim_t},
          release_dry_run: {rc: $rel_rc, elapsed_s: $rel_t},
          failed: $failed, status: (if $failed == 0 then "PASS" else "FAIL" end)}'
else
    echo
    echo "=========================================="
    echo "  PREFLIGHT SUMMARY (target v$VERSION)"
    echo "=========================================="
    fmt() {
        local name="$1" rc="$2" elapsed="$3" out="$4"
        if [ "$rc" = "-1" ]; then
            printf "  %-22s %s\n" "$name" "SKIPPED"
        elif [ "$rc" = "0" ]; then
            printf "  ✓ %-20s rc=0    %ds\n" "$name" "$elapsed"
        else
            printf "  ✗ %-20s rc=%-5s %ds  (log: %s)\n" "$name" "$rc" "$elapsed" "$out"
        fi
    }
    fmt regression       "$REG_RC" "$REG_ELAPSED" "$REG_OUT"
    fmt simulate         "$SIM_RC" "$SIM_ELAPSED" "$SIM_OUT"
    fmt release-dry-run  "$REL_RC" "$REL_ELAPSED" "$REL_OUT"
    echo
    if [ "$TOTAL_FAIL" = "0" ]; then
        echo "  PREFLIGHT PASS"
    else
        echo "  PREFLIGHT FAIL ($TOTAL_FAIL of 3 sub-suites failed)"
        echo "  Inspect the per-suite logs above for diagnosis."
    fi
fi

[ "$TOTAL_FAIL" = "0" ] && exit 0 || exit 1
