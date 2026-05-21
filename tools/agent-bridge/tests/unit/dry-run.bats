#!/usr/bin/env bats
# T-dryrun — `bridge launch --dry-run` produces mock outputs without LLM call

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  PROFILE="${FIXTURE_DIR}/synth-profile.json"
  PROMPT="${FIXTURE_DIR}/minimal-prompt.txt"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-tdr-XXXXXX")"
  STDOUT_LOG="${WS}/stdout.log"
  STDERR_LOG="${WS}/stderr.log"
  ARTIFACT="${WS}/artifact.md"
  export BRIDGE_BIN FIXTURE_DIR PROFILE PROMPT WS STDOUT_LOG STDERR_LOG ARTIFACT
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
}

run_dryrun() {
  local cli="${1:-claude-tmux}"
  "$BRIDGE_BIN" launch --dry-run \
    --cli="$cli" --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
}

@test "T-dryrun.1 — --dry-run with all flags → rc=0" {
  run run_dryrun
  [ "$status" -eq 0 ]
}

@test "T-dryrun.2 — --dry-run writes artifact" {
  run_dryrun
  [ -f "$ARTIFACT" ]
  [ -s "$ARTIFACT" ]
}

@test "T-dryrun.3 — --dry-run writes stdout-log" {
  run_dryrun
  [ -f "$STDOUT_LOG" ]
}

@test "T-dryrun.4 — --dry-run writes stderr-log" {
  run_dryrun
  [ -f "$STDERR_LOG" ]
}

@test "T-dryrun.5 — --dry-run artifact includes 'BRIDGE DRY-RUN' marker" {
  run_dryrun
  grep -q 'bridge-dry-run:' "$ARTIFACT"
  grep -q 'DRY-RUN-OK' "$ARTIFACT"
}

@test "T-dryrun.6 — --dry-run artifact includes resolved cli + model" {
  run_dryrun
  grep -q 'cli: claude-tmux' "$ARTIFACT"
  grep -q 'model: haiku' "$ARTIFACT"
}

@test "T-dryrun.7 — --dry-run works for each CLI (all 6 backends)" {
  for cli in claude-p claude-tmux codex codex-tmux agy agy-tmux; do
    rm -f "$ARTIFACT" "$STDOUT_LOG" "$STDERR_LOG"
    run run_dryrun "$cli"
    [ "$status" -eq 0 ]
    [ -f "$ARTIFACT" ]
    grep -q "cli: $cli" "$ARTIFACT"
  done
}

@test "T-dryrun.8 — BRIDGE_DRY_RUN=1 env var equivalent to --dry-run" {
  BRIDGE_DRY_RUN=1 run "$BRIDGE_BIN" launch \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  grep -q 'DRY-RUN-OK' "$ARTIFACT"
}

@test "T-dryrun.9 — dry-run artifact contains challenge token when prompt uses \$CHALLENGE_TOKEN" {
  PROMPT_WITH_TOKEN="$WS/prompt-token.txt"
  cat > "$PROMPT_WITH_TOKEN" <<EOF
Token: \$CHALLENGE_TOKEN
Write to \$ARTIFACT_PATH
EOF
  "$BRIDGE_BIN" launch --dry-run \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT_WITH_TOKEN" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ -f "$WS/challenge-token.txt" ]
  # token in artifact matches the one written to challenge-token.txt
  TOKEN="$(cat "$WS/challenge-token.txt")"
  grep -q "challenge-token: $TOKEN" "$ARTIFACT"
}

@test "T-dryrun.10 — dry-run rc=0 even when --allow-bypass not set for claude-tmux" {
  # Dry-run shouldn't trip safety gates since no real driver runs
  run "$BRIDGE_BIN" launch --dry-run \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 0 ]
}
