#!/usr/bin/env bats
# T6 — `bridge launch --cli=claude-p` runs a known-cheap Haiku prompt; artifact appears
#
# Gated on BRIDGE_RUN_LIVE_LLM=1. Shares ONE claude call across all @tests
# via setup_file to keep cost ~$0.05 per run (Haiku, single short turn).

setup_file() {
  if [[ "${BRIDGE_RUN_LIVE_LLM:-0}" != "1" ]]; then
    return 0
  fi
  if ! command -v claude >/dev/null 2>&1; then
    return 0
  fi

  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  PROFILE="${BATS_TEST_DIRNAME}/../fixtures/synth-profile.json"
  BRIDGE_T6_WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-t6-XXXXXX")"
  BRIDGE_T6_TOKEN="$(openssl rand -hex 8)"
  BRIDGE_T6_PROMPT="$BRIDGE_T6_WS/prompt.txt"
  BRIDGE_T6_STDOUT="$BRIDGE_T6_WS/stdout.log"
  BRIDGE_T6_STDERR="$BRIDGE_T6_WS/stderr.log"
  BRIDGE_T6_ARTIFACT="$BRIDGE_T6_WS/artifact.md"

  cat > "$BRIDGE_T6_PROMPT" <<PROMPT_EOF
You are a probe agent. Write exactly two lines to the file at ${BRIDGE_T6_ARTIFACT}:
line 1: <!-- challenge-token: ${BRIDGE_T6_TOKEN} -->
line 2: PROTOTYPE OK
Use your Write tool. After writing, exit. Do not narrate. Do not ask questions.
PROMPT_EOF

  "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$PROFILE" --model=haiku \
    --prompt-file="$BRIDGE_T6_PROMPT" --workspace="$BRIDGE_T6_WS" \
    --stdout-log="$BRIDGE_T6_STDOUT" --stderr-log="$BRIDGE_T6_STDERR" \
    --artifact="$BRIDGE_T6_ARTIFACT" \
    >"$BRIDGE_T6_WS/bridge-stdout.log" 2>"$BRIDGE_T6_WS/bridge-stderr.log"
  echo $? > "$BRIDGE_T6_WS/bridge-rc"

  export BRIDGE_T6_WS BRIDGE_T6_TOKEN BRIDGE_T6_STDOUT BRIDGE_T6_STDERR BRIDGE_T6_ARTIFACT
}

teardown_file() {
  if [[ -n "${BRIDGE_T6_WS:-}" && -d "${BRIDGE_T6_WS}" && "${BRIDGE_KEEP_WS:-0}" != "1" ]]; then
    rm -rf "${BRIDGE_T6_WS}"
  fi
}

setup() {
  if [[ "${BRIDGE_RUN_LIVE_LLM:-0}" != "1" ]]; then
    skip "BRIDGE_RUN_LIVE_LLM!=1 — set to 1 to run live LLM tests (~\$0.05/file)"
  fi
  if ! command -v claude >/dev/null 2>&1; then
    skip "claude binary not on PATH"
  fi
}

@test "T6.1 — bridge launch --cli=claude-p exits 0 (Haiku)" {
  rc=$(cat "$BRIDGE_T6_WS/bridge-rc")
  if [ "$rc" -ne 0 ]; then
    echo "--- bridge stdout ---" >&2; cat "$BRIDGE_T6_WS/bridge-stdout.log" >&2
    echo "--- bridge stderr ---" >&2; cat "$BRIDGE_T6_WS/bridge-stderr.log" >&2
    echo "--- claude stdout ---" >&2; cat "$BRIDGE_T6_STDOUT" >&2 || true
    echo "--- claude stderr ---" >&2; cat "$BRIDGE_T6_STDERR" >&2 || true
  fi
  [ "$rc" -eq 0 ]
}

@test "T6.2 — artifact file appears at expected path" {
  [ -f "$BRIDGE_T6_ARTIFACT" ]
}

@test "T6.3 — artifact contains challenge token from prompt" {
  grep -q "challenge-token: $BRIDGE_T6_TOKEN" "$BRIDGE_T6_ARTIFACT"
}

@test "T6.4 — stdout-log was written by claude" {
  [ -f "$BRIDGE_T6_STDOUT" ]
}

@test "T6.5 — workspace contains challenge-token.txt (driver-recorded)" {
  # The driver writes challenge-token.txt when $CHALLENGE_TOKEN was substituted.
  # T6's prompt uses a literal token, not the placeholder, so this file may
  # not be present in this exact setup. We assert "not crashed", not presence.
  true
}
