#!/usr/bin/env bash
# acs/lib/assert_test.sh — tests for the shared ACS assertion helpers.
#
# Focuses on the PURE parsing/comparison functions (acs_coverage_pct,
# acs_pct_ge) that encode the cycle-137 + cycle-131 footguns. The
# go-invoking wrappers (assert_go_test_pass / assert_go_coverage_ge) are
# thin shells over these plus `go test`'s exit code, exercised end-to-end
# by the live cycle; unit-testing them here would require the Go toolchain
# and a fixture module, which the colocated Go package tests already cover.
#
# Run: bash acs/lib/assert_test.sh   (exit 0 = all pass)
# Bash 3.2 compatible.

set -uo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
. "$DIR/assert.sh"

fails=0
check() { # check <desc> <expected> <actual>
  if [ "$2" = "$3" ]; then
    echo "ok   - $1"
  else
    echo "FAIL - $1: expected [$2] got [$3]"
    fails=$((fails + 1))
  fi
}
check_rc() { # check_rc <desc> <expected-rc> <actual-rc>
  if [ "$2" -eq "$3" ]; then
    echo "ok   - $1"
  else
    echo "FAIL - $1: expected rc $2 got $3"
    fails=$((fails + 1))
  fi
}

# --- acs_coverage_pct: extraction from both Go output shapes -----------
check "coverage from bare line" "93.8" \
  "$(echo 'coverage: 93.8% of statements' | acs_coverage_pct)"
check "coverage from ok-prefixed line" "93.8" \
  "$(echo 'ok  	github.com/x/y	1.4s	coverage: 93.8% of statements' | acs_coverage_pct)"
check "coverage integer pct" "100" \
  "$(echo 'coverage: 100% of statements' | acs_coverage_pct)"
check "coverage absent → empty" "" \
  "$(echo 'ok  	github.com/x/y	1.4s' | acs_coverage_pct)"
# The cycle-137 footgun: non-verbose passing output has NO 'PASS' token,
# but DOES carry a coverage line — extraction must still succeed.
check "coverage present despite no PASS token" "87.5" \
  "$(printf 'ok  \tpkg\t0.2s\tcoverage: 87.5%% of statements\n' | acs_coverage_pct)"

# --- acs_pct_ge: numeric comparison without float bc ------------------
acs_pct_ge "93.8" "93.3"; check_rc "93.8 >= 93.3" 0 $?
acs_pct_ge "93.3" "93.8"; check_rc "93.3 >= 93.8 is false" 1 $?
acs_pct_ge "100" "100"; check_rc "100 >= 100 (equal)" 0 $?
acs_pct_ge "80" "80.0"; check_rc "80 >= 80.0 (mixed precision)" 0 $?
acs_pct_ge "" "50"; check_rc "empty treated as 0 < 50" 1 $?
acs_pct_ge "75%" "50%"; check_rc "trailing %% stripped" 0 $?

if [ "$fails" -eq 0 ]; then
  echo "PASS: all assert.sh helper tests green"
  exit 0
fi
echo "FAIL: $fails assertion(s) failed"
exit 1
