#!/usr/bin/env bats
# T-selftest — `bridge selftest` CLI wrapper around the bats runner

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  export BRIDGE_BIN
}

@test "T-selftest.1 — --filter=exit-codes runs the exit-code suite, rc=0" {
  run "$BRIDGE_BIN" selftest --filter=exit-codes
  [ "$status" -eq 0 ]
}

@test "T-selftest.2 — JSON output has totals + tests array (--json)" {
  run "$BRIDGE_BIN" --json selftest --filter=exit-codes
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.started_at and .suite and .totals and .tests' >/dev/null
  echo "$output" | jq -e '.totals.passed >= 1' >/dev/null
  echo "$output" | jq -e '.totals.failed == 0' >/dev/null
}

@test "T-selftest.3 — --suite=billing returns 0 when BRIDGE_BILLING_TESTS not set" {
  # billing suite quietly skips when env var not set; rc=0
  unset BRIDGE_BILLING_TESTS
  run "$BRIDGE_BIN" selftest --suite=billing
  [ "$status" -eq 0 ]
}

@test "T-selftest.4 — --help exits 0 and mentions JSON schema" {
  run "$BRIDGE_BIN" selftest --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"JSON shape"* ]]
  [[ "$output" == *"--suite"* ]]
}

@test "T-selftest.5 — bad --suite → rc=10" {
  run "$BRIDGE_BIN" selftest --suite=not-a-suite
  [ "$status" -eq 10 ]
}

@test "T-selftest.6 — unknown flag → rc=10" {
  run "$BRIDGE_BIN" selftest --not-a-real-flag
  [ "$status" -eq 10 ]
}

@test "T-selftest.7 — bats-missing simulation → rc=127 + install instructions" {
  # Empty PATH = no bats. Use env -i + just HOME.
  run env -i HOME="$HOME" "$BRIDGE_BIN" selftest --filter=exit-codes
  [ "$status" -eq 127 ]
  [[ "$output" == *"bats-core not on PATH"* ]]
  [[ "$output" == *"brew install bats-core"* ]]
}

@test "T-selftest.8 — JSON tests[] entries have number, name, status" {
  run "$BRIDGE_BIN" --json selftest --filter=exit-codes
  echo "$output" | jq -e '.tests | length >= 1' >/dev/null
  echo "$output" | jq -e '.tests[0] | .number and .name and .status' >/dev/null
}
