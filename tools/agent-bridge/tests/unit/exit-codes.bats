#!/usr/bin/env bats
# T1 — exit-code surface for bin/bridge

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  export BRIDGE_BIN
}

@test "T1.1 — bridge with no args exits 10" {
  run "$BRIDGE_BIN"
  [ "$status" -eq 10 ]
}

@test "T1.2 — bridge launch with no flags exits 10" {
  run "$BRIDGE_BIN" launch
  [ "$status" -eq 10 ]
}

@test "T1.3 — bridge with unknown subcommand exits 10" {
  run "$BRIDGE_BIN" not-a-real-subcommand
  [ "$status" -eq 10 ]
}

@test "T1.4 — bridge version subcommand exits 0 and prints semver" {
  run "$BRIDGE_BIN" version
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
}

@test "T1.5 — bridge help subcommand exits 0 and lists subcommands" {
  run "$BRIDGE_BIN" help
  [ "$status" -eq 0 ]
  [[ "$output" == *"launch"* ]]
  [[ "$output" == *"probe"* ]]
  [[ "$output" == *"validate"* ]]
  [[ "$output" == *"report"* ]]
  [[ "$output" == *"version"* ]]
  [[ "$output" == *"help"* ]]
}
