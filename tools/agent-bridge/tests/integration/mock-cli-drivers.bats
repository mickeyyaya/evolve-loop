#!/usr/bin/env bats
# Mock integration tests: each driver runs against a fake CLI binary.
# No live LLM, no $ spend, deterministic. Runs by default (no gates).
#
# Verifies: driver plumbing, prompt placeholder substitution, artifact path
# routing, stdout/stderr capture, exit-code propagation — for ALL 4 CLIs.

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  FAKES_DIR="${FIXTURE_DIR}/fakes"
  PROFILE="${FIXTURE_DIR}/synth-profile.json"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-mock-XXXXXX")"
  STDOUT_LOG="${WS}/stdout.log"
  STDERR_LOG="${WS}/stderr.log"
  ARTIFACT="${WS}/artifact.md"
  TOKEN="$(openssl rand -hex 8)"
  PROMPT="${WS}/prompt.txt"

  cat > "$PROMPT" <<EOF
Use your Write tool to create $ARTIFACT containing:
<!-- challenge-token: $TOKEN -->
PROTOTYPE OK
EOF
  export BRIDGE_BIN FIXTURE_DIR FAKES_DIR PROFILE WS STDOUT_LOG STDERR_LOG ARTIFACT TOKEN PROMPT
  export BRIDGE_TESTING=1
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
  unset BRIDGE_TESTING BRIDGE_CLAUDE_BINARY BRIDGE_CODEX_BINARY BRIDGE_AGY_BINARY
}

run_with_fake() {
  local cli="$1"
  local bypass_flag=""
  [[ "$cli" == "claude-tmux" ]] && bypass_flag="--allow-bypass"

  # Fake binary substitution per CLI
  case "$cli" in
    claude-p|claude-tmux) export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh" ;;
    codex) export BRIDGE_CODEX_BINARY="$FAKES_DIR/fake-codex.sh" ;;
    agy)   export BRIDGE_AGY_BINARY="$FAKES_DIR/fake-agy.sh" ;;
  esac

  "$BRIDGE_BIN" launch \
    --cli="$cli" --profile="$PROFILE" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    $bypass_flag
}

# T-mock.1-4: each driver via fake binary

@test "T-mock.1 — claude-p driver via fake-claude" {
  run run_with_fake claude-p
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  grep -q "$TOKEN" "$ARTIFACT"
}

@test "T-mock.2 — claude-tmux driver via fake-claude (NOT YET — tmux required)" {
  skip "claude-tmux requires real tmux; mock would need tmux mock too"
}

@test "T-mock.3 — codex driver via fake-codex" {
  run run_with_fake codex
  if [ "$status" -ne 0 ]; then
    echo "STDERR_LOG:" >&2; cat "$STDERR_LOG" >&2
    echo "BRIDGE_STDERR (none)" >&2
  fi
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  grep -q "challenge-token: $TOKEN" "$ARTIFACT"
  grep -q "FAKE-CODEX" "$ARTIFACT"
}

@test "T-mock.4 — agy driver via fake-agy" {
  run run_with_fake agy
  if [ "$status" -ne 0 ]; then
    echo "STDERR_LOG:" >&2; cat "$STDERR_LOG" >&2
  fi
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  grep -q "challenge-token: $TOKEN" "$ARTIFACT"
  grep -q "FAKE-AGY" "$ARTIFACT"
}

@test "T-mock.5 — codex driver passes haiku→gpt-5.4-mini via fake" {
  BRIDGE_CODEX_BINARY="$FAKES_DIR/fake-codex.sh" "$BRIDGE_BIN" launch \
    --cli=codex --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ -f "$ARTIFACT" ]
  grep -q "model=gpt-5.4-mini" "$ARTIFACT"
}

@test "T-mock.6 — codex driver passes sonnet→gpt-5.4 via fake" {
  BRIDGE_CODEX_BINARY="$FAKES_DIR/fake-codex.sh" "$BRIDGE_BIN" launch \
    --cli=codex --profile="$PROFILE" --model=sonnet \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ -f "$ARTIFACT" ]
  grep -q "model=gpt-5.4" "$ARTIFACT"
}

@test "T-mock.7 — codex driver passes opus→gpt-5.5 via fake" {
  BRIDGE_CODEX_BINARY="$FAKES_DIR/fake-codex.sh" "$BRIDGE_BIN" launch \
    --cli=codex --profile="$PROFILE" --model=opus \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ -f "$ARTIFACT" ]
  grep -q "model=gpt-5.5" "$ARTIFACT"
}

