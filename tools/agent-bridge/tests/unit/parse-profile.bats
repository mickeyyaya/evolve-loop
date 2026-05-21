#!/usr/bin/env bats
# T3 — `--validate-only` reads and surfaces profile JSON fields.
#
# Profile contract: {name, model, allowed_tools[], auto_respond:{destructive_ops, timeout_s}, prompt_overrides[]}

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  PROFILE="${FIXTURE_DIR}/synth-profile.json"
  PROMPT="${FIXTURE_DIR}/minimal-prompt.txt"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-t3-XXXXXX")"
  STDOUT_LOG="${WS}/stdout.log"
  STDERR_LOG="${WS}/stderr.log"
  ARTIFACT="${WS}/artifact.md"
  export BRIDGE_BIN FIXTURE_DIR PROFILE PROMPT WS STDOUT_LOG STDERR_LOG ARTIFACT
}

teardown() {
  [[ -n "${WS:-}" && -d "${WS}" ]] && rm -rf "${WS}"
}

run_validate() {
  "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
}

@test "T3.1 — validate-only surfaces profile.name from JSON" {
  run run_validate
  [ "$status" -eq 0 ]
  [[ "$output" == *"synth-probe-haiku"* ]]
}

@test "T3.2 — validate-only surfaces profile.allowed_tools[]" {
  run run_validate
  [ "$status" -eq 0 ]
  [[ "$output" == *"Read"* ]]
  [[ "$output" == *"Write"* ]]
}

@test "T3.3 — validate-only surfaces profile.auto_respond.timeout_s" {
  run run_validate
  [ "$status" -eq 0 ]
  [[ "$output" == *"300"* ]]
}

@test "T3.4 — invalid profile JSON → rc=10" {
  BAD_PROFILE="$WS/bad-profile.json"
  echo '{not valid json' > "$BAD_PROFILE"
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$BAD_PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 10 ]
}

@test "T3.5 — missing profile file → rc=10" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$WS/does-not-exist.json" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 10 ]
}

@test "T3.6 — profile.model can override --model when --model=auto" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$PROFILE" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 0 ]
  # When --model=auto, profile.model (haiku in fixture) takes effect.
  # Output should mention haiku (resolved from profile), not "auto".
  [[ "$output" == *"haiku"* ]]
}
