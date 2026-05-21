#!/usr/bin/env bash
# lib/selftest.sh — CLI entrypoint for the bridge test suite
#
# Wraps tests/run-tests.sh so a skill can run `bridge selftest` instead of
# knowing about bats internals. Parses bats TAP output to emit a JSON
# summary when $BRIDGE_JSON=1.
#
# Exit codes:
#   0  all selected tests passed
#   1  one or more tests failed
#   10 bad flags
#   127 bats-core not on PATH

bridge_selftest() {
  local suite="all" filter="" live=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --suite=*) suite="${1#--suite=}" ;;
      --suite)   [[ $# -ge 2 ]] || { echo "[selftest] --suite requires a value" >&2; return 10; }; suite="$2"; shift ;;
      --filter=*) filter="${1#--filter=}" ;;
      --filter)  [[ $# -ge 2 ]] || { echo "[selftest] --filter requires a value" >&2; return 10; }; filter="$2"; shift ;;
      --live)    live=1 ;;
      --help|-h) bridge_selftest_help; return 0 ;;
      *)         echo "[selftest] unknown flag: $1" >&2; return 10 ;;
    esac
    shift
  done

  case "$suite" in
    all|unit|integration|billing) ;;
    *) echo "[selftest] bad --suite (want: all|unit|integration|billing): $suite" >&2; return 10 ;;
  esac

  if ! command -v bats >/dev/null 2>&1; then
    cat >&2 <<'BATS_MISSING'
[bridge selftest] bats-core not on PATH.

Install:
  macOS:  brew install bats-core
  Linux:  apt install bats   (or)   git clone https://github.com/bats-core/bats-core /tmp/bats && /tmp/bats/install.sh /usr/local
BATS_MISSING
    return 127
  fi

  local tests_dir="${BRIDGE_ROOT}/tests"
  local run_tests="${tests_dir}/run-tests.sh"
  if [[ ! -x "$run_tests" ]] && [[ ! -r "$run_tests" ]]; then
    echo "[selftest] runner not found: $run_tests" >&2
    return 10
  fi

  # Build wrapped command
  local args=("--suite=$suite")
  [[ -n "$filter" ]] && args+=("--filter=$filter")

  local started_at; started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  local start_ms;   start_ms=$(perl -MTime::HiRes=time -e 'printf "%d", time*1000')

  local tap_log; tap_log=$(mktemp -t bridge-selftest-XXXXXX)
  local rc=0
  if [[ "${BRIDGE_JSON:-0}" == "1" ]]; then
    # JSON mode: capture bats TAP output to parse
    BATS_FORMATTER=tap BRIDGE_RUN_LIVE_LLM="$live" \
      bash "$run_tests" "${args[@]}" > "$tap_log" 2>&1
    rc=$?
  else
    # Human mode: passthrough (pretty formatter)
    BRIDGE_RUN_LIVE_LLM="$live" \
      bash "$run_tests" "${args[@]}"
    rc=$?
  fi

  local end_ms; end_ms=$(perl -MTime::HiRes=time -e 'printf "%d", time*1000')
  local duration_ms=$((end_ms - start_ms))

  if [[ "${BRIDGE_JSON:-0}" == "1" ]]; then
    bridge_selftest_parse_tap "$tap_log" "$suite" "$filter" "$live" "$started_at" "$duration_ms"
  fi

  rm -f "$tap_log"

  # Normalize rc: bats returns non-zero on failures
  [[ $rc -ne 0 ]] && return 1
  return 0
}

# Parse bats TAP output into a JSON summary.
# TAP format (one example):
#   1..5
#   ok 1 T1.1 — bridge with no args exits 10
#   ok 2 T1.2 — bridge launch with no flags exits 10
#   not ok 3 T1.3 — broken case
#   # (in test file …) failed
#   ok 4 T1.4 — version subcommand exits 0
#   ok 5 # skip T1.5 — skipped reason
bridge_selftest_parse_tap() {
  local tap_log="$1"
  local suite="$2"
  local filter="$3"
  local live="$4"
  local started_at="$5"
  local duration_ms="$6"

  local passed=0 failed=0 skipped=0
  local results_json="[]"
  local results_file; results_file=$(mktemp -t bridge-selftest-results-XXXXXX)
  printf '[' > "$results_file"
  local first=1

  while IFS= read -r line; do
    if [[ "$line" =~ ^ok[[:space:]]+([0-9]+)[[:space:]]+(.*)$ ]]; then
      local num="${BASH_REMATCH[1]}"
      local name="${BASH_REMATCH[2]}"
      if [[ "$name" == *"# skip"* || "$name" == *"# SKIP"* ]]; then
        skipped=$((skipped+1))
        [[ $first -eq 0 ]] && printf ',' >> "$results_file"
        jq -n --arg n "$num" --arg name "$name" --arg status "skipped" \
          '{number: ($n | tonumber), name: $name, status: $status}' >> "$results_file"
        first=0
      else
        passed=$((passed+1))
        [[ $first -eq 0 ]] && printf ',' >> "$results_file"
        jq -n --arg n "$num" --arg name "$name" --arg status "passed" \
          '{number: ($n | tonumber), name: $name, status: $status}' >> "$results_file"
        first=0
      fi
    elif [[ "$line" =~ ^not[[:space:]]+ok[[:space:]]+([0-9]+)[[:space:]]+(.*)$ ]]; then
      failed=$((failed+1))
      local num="${BASH_REMATCH[1]}"
      local name="${BASH_REMATCH[2]}"
      [[ $first -eq 0 ]] && printf ',' >> "$results_file"
      jq -n --arg n "$num" --arg name "$name" --arg status "failed" \
        '{number: ($n | tonumber), name: $name, status: $status}' >> "$results_file"
      first=0
    fi
  done < "$tap_log"
  printf ']' >> "$results_file"

  local filter_arg
  if [[ -z "$filter" ]]; then filter_arg=null; else filter_arg="\"$filter\""; fi

  jq -n \
    --arg started_at "$started_at" \
    --arg suite "$suite" \
    --argjson live "$live" \
    --argjson duration "$duration_ms" \
    --argjson passed "$passed" --argjson failed "$failed" --argjson skipped "$skipped" \
    --slurpfile results "$results_file" \
    --argjson filter_v "$filter_arg" \
    '{
      started_at: $started_at,
      suite: $suite,
      filter: $filter_v,
      live: ($live == 1),
      totals: {passed: $passed, failed: $failed, skipped: $skipped, duration_ms: $duration},
      tests: ($results[0])
    }'

  rm -f "$results_file"
}

bridge_selftest_help() {
  cat <<'STH'
bridge selftest — run the bridge test suite (CLI-driven contract verification)

Usage:
  bridge [--json] selftest [--suite=SUITE] [--filter=PATTERN] [--live]

Flags:
  --suite=SUITE   unit | integration | billing | all (default: all)
  --filter=PAT    substring match against test file paths
  --live          set BRIDGE_RUN_LIVE_LLM=1 for the wrapped runner
  --json          (top-level) emit JSON summary parsed from bats TAP

Exit codes:
  0    all selected tests passed
  1    one or more tests failed
  10   bad flags
  127  bats-core not on PATH (see install hints in stderr)

JSON shape (when --json):
  {
    "started_at": "ISO-8601",
    "suite": "all",
    "filter": null,
    "live": false,
    "totals": {"passed": N, "failed": N, "skipped": N, "duration_ms": N},
    "tests": [{"number": N, "name": "T1.1 — …", "status": "passed|failed|skipped"}, …]
  }
STH
}
