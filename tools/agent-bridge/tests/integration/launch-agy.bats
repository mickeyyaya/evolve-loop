#!/usr/bin/env bats
# T-agy — `bridge launch --cli=agy` runs agy -p, expects artifact from tool use
#
# Gated on BRIDGE_RUN_LIVE_LLM=1. Cost: ~$0.05 (Haiku-equivalent on subscription).
# agy is Anthropic-authed, so subscription billing applies.

setup_file() {
  if [[ "${BRIDGE_RUN_LIVE_LLM:-0}" != "1" ]]; then
    return 0
  fi
  if ! command -v agy >/dev/null 2>&1; then
    return 0
  fi

  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  PROFILE="${BATS_TEST_DIRNAME}/../fixtures/synth-profile.json"
  BRIDGE_TAGY_WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-tagy-XXXXXX")"
  BRIDGE_TAGY_TOKEN="$(openssl rand -hex 8)"
  BRIDGE_TAGY_PROMPT="$BRIDGE_TAGY_WS/prompt.txt"
  BRIDGE_TAGY_STDOUT="$BRIDGE_TAGY_WS/stdout.log"
  BRIDGE_TAGY_STDERR="$BRIDGE_TAGY_WS/stderr.log"
  BRIDGE_TAGY_ARTIFACT="$BRIDGE_TAGY_WS/artifact.md"

  cat > "$BRIDGE_TAGY_PROMPT" <<PROMPT_EOF
You are a probe agent. Use your Write tool to create a file at this exact path:
${BRIDGE_TAGY_ARTIFACT}
The file must contain exactly two lines:
<!-- challenge-token: ${BRIDGE_TAGY_TOKEN} -->
PROTOTYPE OK
After writing, exit. Do not narrate.
PROMPT_EOF

  "$BRIDGE_BIN" launch \
    --cli=agy --profile="$PROFILE" --model=auto \
    --prompt-file="$BRIDGE_TAGY_PROMPT" --workspace="$BRIDGE_TAGY_WS" \
    --stdout-log="$BRIDGE_TAGY_STDOUT" --stderr-log="$BRIDGE_TAGY_STDERR" \
    --artifact="$BRIDGE_TAGY_ARTIFACT" \
    >"$BRIDGE_TAGY_WS/bridge-stdout.log" 2>"$BRIDGE_TAGY_WS/bridge-stderr.log"
  echo $? > "$BRIDGE_TAGY_WS/bridge-rc"

  export BRIDGE_BIN PROFILE BRIDGE_TAGY_WS BRIDGE_TAGY_TOKEN \
         BRIDGE_TAGY_PROMPT BRIDGE_TAGY_STDOUT BRIDGE_TAGY_STDERR BRIDGE_TAGY_ARTIFACT
}

teardown_file() {
  if [[ -n "${BRIDGE_TAGY_WS:-}" && -d "${BRIDGE_TAGY_WS}" && "${BRIDGE_KEEP_WS:-0}" != "1" ]]; then
    rm -rf "${BRIDGE_TAGY_WS}"
  fi
}

require_live() {
  if [[ "${BRIDGE_RUN_LIVE_LLM:-0}" != "1" ]]; then
    skip "BRIDGE_RUN_LIVE_LLM!=1"
  fi
  if ! command -v agy >/dev/null 2>&1; then
    skip "agy binary not on PATH"
  fi
}

@test "T-agy.1 — bridge launch --cli=agy exits 0" {
  require_live
  rc=$(cat "$BRIDGE_TAGY_WS/bridge-rc")
  if [ "$rc" -ne 0 ]; then
    echo "--- bridge stderr ---" >&2; cat "$BRIDGE_TAGY_WS/bridge-stderr.log" >&2 || true
    echo "--- agy stderr ---" >&2; cat "$BRIDGE_TAGY_STDERR" >&2 || true
  fi
  [ "$rc" -eq 0 ]
}

@test "T-agy.2 — artifact file written by agy's Write tool" {
  require_live
  [ -f "$BRIDGE_TAGY_ARTIFACT" ]
}

@test "T-agy.3 — artifact contains challenge token" {
  require_live
  grep -q "challenge-token: $BRIDGE_TAGY_TOKEN" "$BRIDGE_TAGY_ARTIFACT"
}

@test "T-agy.4 — agy stdout log written" {
  require_live
  [ -f "$BRIDGE_TAGY_STDOUT" ]
}
