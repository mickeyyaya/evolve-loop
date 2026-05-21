#!/usr/bin/env bats
# T2 — required-flag validation for `bridge launch`
#
# Each required flag's absence must trigger exit 10. Unknown flag → 10.
# All required flags + --validate-only → 0 (parser succeeds, short-circuits).

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  PROFILE="${FIXTURE_DIR}/synth-profile.json"
  PROMPT="${FIXTURE_DIR}/minimal-prompt.txt"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-t2-XXXXXX")"
  STDOUT_LOG="${WS}/stdout.log"
  STDERR_LOG="${WS}/stderr.log"
  ARTIFACT="${WS}/artifact.md"
  export BRIDGE_BIN FIXTURE_DIR PROFILE PROMPT WS STDOUT_LOG STDERR_LOG ARTIFACT
  # Unset bridge env-var fallback names so T2 strictly tests flag-only validation
  unset BRIDGE_CLI PROFILE_PATH RESOLVED_MODEL PROMPT_FILE WORKSPACE_PATH
  unset BRIDGE_DRY_RUN BRIDGE_REQUIRE_FULL BRIDGE_ALLOW_BYPASS VALIDATE_ONLY
  unset CYCLE WORKTREE_PATH AGENT
  unset ARTIFACT_PATH
  # NB: STDOUT_LOG/STDERR_LOG above are local-to-test bats vars, not bridge env vars
  # But bridge cmd_launch DOES read them as fallback. Unset them too:
  local saved_stdout_log="$STDOUT_LOG"
  local saved_stderr_log="$STDERR_LOG"
  unset STDOUT_LOG STDERR_LOG
  STDOUT_LOG="$saved_stdout_log"
  STDERR_LOG="$saved_stderr_log"
  # Now STDOUT_LOG and STDERR_LOG are bats-local vars, not exported
}

teardown() {
  [[ -n "${WS:-}" && -d "${WS}" ]] && rm -rf "${WS}"
}

@test "T2.1 — all required flags + --validate-only → rc=0" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux \
    --profile="$PROFILE" \
    --model=haiku \
    --prompt-file="$PROMPT" \
    --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" \
    --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 0 ]
}

@test "T2.2 — missing --cli → rc=10" {
  run "$BRIDGE_BIN" launch --validate-only \
    --profile="$PROFILE" --model=haiku --prompt-file="$PROMPT" \
    --workspace="$WS" --stdout-log="$STDOUT_LOG" \
    --stderr-log="$STDERR_LOG" --artifact="$ARTIFACT"
  [ "$status" -eq 10 ]
}

@test "T2.3 — missing --profile → rc=10" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --model=haiku --prompt-file="$PROMPT" \
    --workspace="$WS" --stdout-log="$STDOUT_LOG" \
    --stderr-log="$STDERR_LOG" --artifact="$ARTIFACT"
  [ "$status" -eq 10 ]
}

@test "T2.4 — missing --model → rc=10" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$PROFILE" --prompt-file="$PROMPT" \
    --workspace="$WS" --stdout-log="$STDOUT_LOG" \
    --stderr-log="$STDERR_LOG" --artifact="$ARTIFACT"
  [ "$status" -eq 10 ]
}

@test "T2.5 — missing --prompt-file → rc=10" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --workspace="$WS" --stdout-log="$STDOUT_LOG" \
    --stderr-log="$STDERR_LOG" --artifact="$ARTIFACT"
  [ "$status" -eq 10 ]
}

@test "T2.6 — missing --workspace → rc=10" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --stdout-log="$STDOUT_LOG" \
    --stderr-log="$STDERR_LOG" --artifact="$ARTIFACT"
  [ "$status" -eq 10 ]
}

@test "T2.7 — missing --stdout-log → rc=10" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stderr-log="$STDERR_LOG" --artifact="$ARTIFACT"
  [ "$status" -eq 10 ]
}

@test "T2.8 — missing --stderr-log → rc=10" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --artifact="$ARTIFACT"
  [ "$status" -eq 10 ]
}

@test "T2.9 — missing --artifact → rc=10" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG"
  [ "$status" -eq 10 ]
}

@test "T2.10 — unknown flag → rc=10" {
  run "$BRIDGE_BIN" launch --validate-only --not-a-real-flag=x \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 10 ]
}

@test "T2.11 — --validate-only prints resolved cli + model" {
  run "$BRIDGE_BIN" launch --validate-only \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"claude-tmux"* ]]
  [[ "$output" == *"haiku"* ]]
}
