#!/usr/bin/env bats
# T-doctor — bridge doctor pre-flight auth/binary verification

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  export BRIDGE_BIN
  # Strip env-leak vars so the default suite reflects ready state on this host
  unset BRIDGE_DRY_RUN BRIDGE_REQUIRE_FULL VALIDATE_ONLY BRIDGE_ALLOW_BYPASS
}

@test "T-doctor.1 — bridge doctor on healthy host returns 0/1/2 (not crash)" {
  run "$BRIDGE_BIN" doctor
  # On this host all should be ready (0); but accept warning(1)/blocked(2) too
  [[ "$status" -eq 0 || "$status" -eq 1 || "$status" -eq 2 ]]
}

@test "T-doctor.2 — JSON shape has scanned_at + results[] + summary" {
  run "$BRIDGE_BIN" --json doctor
  echo "$output" | jq -e '.scanned_at and .results and .summary' >/dev/null
  echo "$output" | jq -e '.results[0] | .cli and .binary and .auth and .verdict and .env_warnings and .deep_probe' >/dev/null
}

@test "T-doctor.3 — --cli=claude-p filters to a single result" {
  run "$BRIDGE_BIN" --json doctor --cli=claude-p
  [ "$status" -ne 10 ]
  count=$(echo "$output" | jq '.results | length')
  [ "$count" -eq 1 ]
  cli=$(echo "$output" | jq -r '.results[0].cli')
  [ "$cli" = "claude-p" ]
}

@test "T-doctor.4 — unknown CLI filter → rc=10" {
  run "$BRIDGE_BIN" --json doctor --cli=does-not-exist
  [ "$status" -eq 10 ]
}

@test "T-doctor.5 — ANTHROPIC_API_KEY set → claude-p verdict=warning" {
  ANTHROPIC_API_KEY=test-only run "$BRIDGE_BIN" --json doctor --cli=claude-p
  echo "$output" | jq -e '.results[0].env_warnings | length > 0' >/dev/null
  echo "$output" | jq -e '.results[0].verdict == "warning"' >/dev/null
  # Exit code 1 = at least one warning
  [ "$status" -eq 1 ]
}

@test "T-doctor.6 — missing binary via PATH manipulation → verdict=blocked" {
  # Empty PATH = no binaries findable
  run env -i HOME="$HOME" "$BRIDGE_BIN" --json doctor --cli=claude-p
  echo "$output" | jq -e '.results[0].binary.present == false' >/dev/null
  echo "$output" | jq -e '.results[0].verdict == "blocked"' >/dev/null
  [ "$status" -eq 2 ]
}

@test "T-doctor.7 — human-readable mode prints a table" {
  run "$BRIDGE_BIN" doctor
  [[ "$output" == *"CLI"* ]]
  [[ "$output" == *"VERDICT"* ]]
  [[ "$output" == *"summary:"* ]]
}

@test "T-doctor.8 — --help exits 0" {
  run "$BRIDGE_BIN" doctor --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"bridge doctor"* ]]
  [[ "$output" == *"Exit codes"* ]]
}

@test "T-doctor.9 — unknown flag → rc=10" {
  run "$BRIDGE_BIN" doctor --not-a-real-flag
  [ "$status" -eq 10 ]
}

@test "T-doctor.10 — --deep is LIVE-gated; default skips deep probes" {
  run "$BRIDGE_BIN" --json doctor
  # Default: deep=false, all deep_probe.ran=false
  echo "$output" | jq -e '.deep == false' >/dev/null
  echo "$output" | jq -e '[.results[].deep_probe.ran] | all(. == false)' >/dev/null
}
