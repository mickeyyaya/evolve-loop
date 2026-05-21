#!/usr/bin/env bash
# tests/run-tests.sh — run the bridge test suite via bats-core
#
# Usage:
#   bash tests/run-tests.sh                          # all enabled suites
#   bash tests/run-tests.sh --suite=unit
#   bash tests/run-tests.sh --suite=integration
#   bash tests/run-tests.sh --suite=billing          # gated by BRIDGE_BILLING_TESTS=1
#   bash tests/run-tests.sh --suite=unit --filter=exit
#
# Env vars:
#   BRIDGE_BILLING_TESTS=1   enable the opt-in billing suite
#   BATS_FORMATTER=tap       override bats output formatter
#
# Exit codes:
#   0   all selected tests passed
#   1   one or more tests failed
#   10  bad flags
#   127 bats not installed

set -uo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly BRIDGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

selected_suite="all"
filter=""

for arg in "$@"; do
  case "$arg" in
    --suite=*) selected_suite="${arg#--suite=}" ;;
    --filter=*) filter="${arg#--filter=}" ;;
    -h|--help)
      sed -n '2,16p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "[run-tests] unknown arg: $arg" >&2
      exit 10
      ;;
  esac
done

case "$selected_suite" in
  all|all-fast|unit|integration|billing|contract|skill|concurrency|security) ;;
  *)
    echo "[run-tests] bad --suite (want: all|unit|integration|billing): $selected_suite" >&2
    exit 10
    ;;
esac

if ! command -v bats >/dev/null 2>&1; then
  echo "[run-tests] FATAL: bats not on PATH. Install via 'brew install bats-core' (macOS) or 'apt install bats' (Linux)" >&2
  exit 127
fi

FORMATTER="${BATS_FORMATTER:-pretty}"
fail_count=0

run_suite() {
  local suite_name="$1"
  local suite_dir="${SCRIPT_DIR}/${suite_name}"

  if [[ ! -d "$suite_dir" ]]; then
    echo "[run-tests] $suite_name suite dir not found: $suite_dir" >&2
    return 0
  fi

  local test_files=()
  while IFS= read -r f; do
    [[ -n "$filter" ]] && [[ "$f" != *"$filter"* ]] && continue
    test_files+=("$f")
  done < <(find "$suite_dir" -name '*.bats' -type f 2>/dev/null)

  if [[ ${#test_files[@]} -eq 0 ]]; then
    echo "[run-tests] (no .bats files in $suite_name)"
    return 0
  fi

  echo ""
  echo "===== suite: $suite_name (${#test_files[@]} file(s)) ====="
  if ! bats --formatter "$FORMATTER" "${test_files[@]}"; then
    fail_count=$((fail_count + 1))
  fi
}

case "$selected_suite" in
  all)
    run_suite unit
    run_suite integration
    run_suite contract
    run_suite skill
    run_suite concurrency
    run_suite security
    if [[ "${BRIDGE_BILLING_TESTS:-0}" == "1" ]]; then
      run_suite billing
    else
      echo "[run-tests] billing suite skipped (set BRIDGE_BILLING_TESTS=1 to enable)"
    fi
    ;;
  all-fast)
    # everything except billing (LIVE-gated integration tests skip themselves
    # via BRIDGE_RUN_LIVE_LLM gates inside their own setup() blocks).
    run_suite unit
    run_suite integration
    run_suite contract
    run_suite skill
    run_suite concurrency
    run_suite security
    ;;
  unit|integration|contract|skill|concurrency|security)
    run_suite "$selected_suite"
    ;;
  billing)
    if [[ "${BRIDGE_BILLING_TESTS:-0}" != "1" ]]; then
      echo "[run-tests] billing suite requires BRIDGE_BILLING_TESTS=1" >&2
      exit 0
    fi
    run_suite billing
    ;;
esac

if [[ $fail_count -gt 0 ]]; then
  echo ""
  echo "[run-tests] $fail_count suite(s) had failures"
  exit 1
fi

echo ""
echo "[run-tests] all selected suites PASSED"
exit 0
