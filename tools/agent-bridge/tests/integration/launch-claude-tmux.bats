#!/usr/bin/env bats
# T7 — `bridge launch --cli=claude-tmux` drives interactive claude via tmux
#
# T7.1-T7.5 gated on BRIDGE_RUN_LIVE_LLM=1 (share one claude call via setup_file).
# T7.6 (safety-gate) runs in default integration mode (no live LLM needed).

# Vars common to all @tests (defined here so setup() and individual tests share them).
T7_BRIDGE_BIN_FILE() {
  echo "${BATS_TEST_DIRNAME}/../../bin/bridge"
}
T7_PROFILE_FILE() {
  echo "${BATS_TEST_DIRNAME}/../fixtures/synth-profile.json"
}

setup_file() {
  if [[ "${BRIDGE_RUN_LIVE_LLM:-0}" != "1" ]]; then
    return 0
  fi
  if ! command -v claude >/dev/null 2>&1 || ! command -v tmux >/dev/null 2>&1; then
    return 0
  fi

  BRIDGE_BIN="$(T7_BRIDGE_BIN_FILE)"
  PROFILE="$(T7_PROFILE_FILE)"
  BRIDGE_T7_WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-t7-XXXXXX")"
  BRIDGE_T7_TOKEN="$(openssl rand -hex 8)"
  BRIDGE_T7_PROMPT="$BRIDGE_T7_WS/prompt.txt"
  BRIDGE_T7_STDOUT="$BRIDGE_T7_WS/stdout.log"
  BRIDGE_T7_STDERR="$BRIDGE_T7_WS/stderr.log"
  BRIDGE_T7_ARTIFACT="$BRIDGE_T7_WS/artifact.md"

  cat > "$BRIDGE_T7_PROMPT" <<PROMPT_EOF
You are a probe agent. Use your Write tool to create the file at:
${BRIDGE_T7_ARTIFACT}
The file must contain exactly two lines:
<!-- challenge-token: ${BRIDGE_T7_TOKEN} -->
PROTOTYPE OK
After writing, exit. Do not narrate. Do not ask questions.
PROMPT_EOF

  "$BRIDGE_BIN" launch \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$BRIDGE_T7_PROMPT" --workspace="$BRIDGE_T7_WS" \
    --stdout-log="$BRIDGE_T7_STDOUT" --stderr-log="$BRIDGE_T7_STDERR" \
    --artifact="$BRIDGE_T7_ARTIFACT" \
    --allow-bypass \
    >"$BRIDGE_T7_WS/bridge-stdout.log" 2>"$BRIDGE_T7_WS/bridge-stderr.log"
  echo $? > "$BRIDGE_T7_WS/bridge-rc"

  export BRIDGE_BIN PROFILE BRIDGE_T7_WS BRIDGE_T7_TOKEN BRIDGE_T7_PROMPT BRIDGE_T7_STDOUT BRIDGE_T7_STDERR BRIDGE_T7_ARTIFACT
}

teardown_file() {
  if [[ -n "${BRIDGE_T7_WS:-}" && -d "${BRIDGE_T7_WS}" && "${BRIDGE_KEEP_WS:-0}" != "1" ]]; then
    tmux ls 2>/dev/null | awk -F: '/evolve-bridge-/{print $1}' \
      | xargs -n1 -I{} tmux kill-session -t {} 2>/dev/null || true
    rm -rf "${BRIDGE_T7_WS}"
  fi
}

# Per-test setup: skip the live tests if not enabled
require_live_llm() {
  if [[ "${BRIDGE_RUN_LIVE_LLM:-0}" != "1" ]]; then
    skip "BRIDGE_RUN_LIVE_LLM!=1 — set to 1 to run live LLM tests"
  fi
  if ! command -v claude >/dev/null 2>&1; then
    skip "claude binary not on PATH"
  fi
  if ! command -v tmux >/dev/null 2>&1; then
    skip "tmux not on PATH"
  fi
}

@test "T7.1 — bridge launch --cli=claude-tmux exits 0 (Haiku via tmux)" {
  require_live_llm
  rc=$(cat "$BRIDGE_T7_WS/bridge-rc")
  if [ "$rc" -ne 0 ]; then
    echo "--- bridge stderr ---" >&2; cat "$BRIDGE_T7_WS/bridge-stderr.log" >&2 || true
  fi
  [ "$rc" -eq 0 ]
}

@test "T7.2 — artifact file appears at expected path" {
  require_live_llm
  [ -f "$BRIDGE_T7_ARTIFACT" ]
}

@test "T7.3 — artifact contains challenge token" {
  require_live_llm
  grep -q "challenge-token: $BRIDGE_T7_TOKEN" "$BRIDGE_T7_ARTIFACT"
}

@test "T7.4 — scrollback captured to stdout-log" {
  require_live_llm
  [ -f "$BRIDGE_T7_STDOUT" ]
  [ -s "$BRIDGE_T7_STDOUT" ]
}

@test "T7.5 — no orphan tmux sessions left after launch" {
  require_live_llm
  ! tmux ls 2>/dev/null | grep -q 'evolve-bridge-' || {
    tmux ls 2>&1 >&2
    return 1
  }
}

@test "T7.6 — claude-tmux without --allow-bypass → rc=2 (safety gate, no live LLM)" {
  # Self-contained: doesn't use setup_file artifacts. Always runs.
  bin="$(T7_BRIDGE_BIN_FILE)"
  profile="$(T7_PROFILE_FILE)"
  ws="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-t7-bypass-XXXXXX")"
  prompt="$ws/prompt.txt"
  echo "irrelevant" > "$prompt"
  run "$bin" launch \
    --cli=claude-tmux --profile="$profile" --model=haiku \
    --prompt-file="$prompt" --workspace="$ws" \
    --stdout-log="$ws/stdout.log" --stderr-log="$ws/stderr.log" \
    --artifact="$ws/artifact.md"
  rm -rf "$ws"
  [ "$status" -eq 2 ]
}
