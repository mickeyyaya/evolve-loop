#!/usr/bin/env bats
# T-agy-tmux — interactive agy via tmux (subscription-preserving)
# Gated on BRIDGE_RUN_LIVE_LLM=1.

setup_file() {
  if [[ "${BRIDGE_RUN_LIVE_LLM:-0}" != "1" ]]; then return 0; fi
  if ! command -v agy >/dev/null 2>&1 || ! command -v tmux >/dev/null 2>&1; then return 0; fi

  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  PROFILE="${BATS_TEST_DIRNAME}/../fixtures/synth-profile.json"
  BRIDGE_ATMUX_WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-atmux-XXXXXX")"
  BRIDGE_ATMUX_TOKEN="$(openssl rand -hex 8)"
  BRIDGE_ATMUX_PROMPT="$BRIDGE_ATMUX_WS/prompt.txt"
  BRIDGE_ATMUX_STDOUT="$BRIDGE_ATMUX_WS/stdout.log"
  BRIDGE_ATMUX_STDERR="$BRIDGE_ATMUX_WS/stderr.log"
  BRIDGE_ATMUX_ARTIFACT="$BRIDGE_ATMUX_WS/artifact.md"

  cat > "$BRIDGE_ATMUX_PROMPT" <<PROMPT_EOF
Use your Write tool to create a file at this exact path: ${BRIDGE_ATMUX_ARTIFACT}
The file must contain exactly:
<!-- challenge-token: ${BRIDGE_ATMUX_TOKEN} -->
PROTOTYPE OK
After writing, exit. Do not narrate.
PROMPT_EOF

  "$BRIDGE_BIN" launch \
    --cli=agy-tmux --profile="$PROFILE" --model=auto \
    --prompt-file="$BRIDGE_ATMUX_PROMPT" --workspace="$BRIDGE_ATMUX_WS" \
    --stdout-log="$BRIDGE_ATMUX_STDOUT" --stderr-log="$BRIDGE_ATMUX_STDERR" \
    --artifact="$BRIDGE_ATMUX_ARTIFACT" \
    --allow-bypass \
    >"$BRIDGE_ATMUX_WS/bridge-stdout.log" 2>"$BRIDGE_ATMUX_WS/bridge-stderr.log"
  echo $? > "$BRIDGE_ATMUX_WS/bridge-rc"

  export BRIDGE_BIN PROFILE BRIDGE_ATMUX_WS BRIDGE_ATMUX_TOKEN \
         BRIDGE_ATMUX_PROMPT BRIDGE_ATMUX_STDOUT BRIDGE_ATMUX_STDERR BRIDGE_ATMUX_ARTIFACT
}

teardown_file() {
  if [[ -n "${BRIDGE_ATMUX_WS:-}" && -d "${BRIDGE_ATMUX_WS}" && "${BRIDGE_KEEP_WS:-0}" != "1" ]]; then
    tmux ls 2>/dev/null | awk -F: '/evolve-bridge-agy-/{print $1}' \
      | xargs -n1 -I{} tmux kill-session -t {} 2>/dev/null || true
    rm -rf "${BRIDGE_ATMUX_WS}"
  fi
}

require_live() {
  [[ "${BRIDGE_RUN_LIVE_LLM:-0}" == "1" ]] || skip "BRIDGE_RUN_LIVE_LLM!=1"
  command -v agy >/dev/null 2>&1 || skip "agy not on PATH"
  command -v tmux >/dev/null 2>&1 || skip "tmux not on PATH"
}

@test "T-agy-tmux.1 — bridge launch --cli=agy-tmux exits 0" {
  require_live
  rc=$(cat "$BRIDGE_ATMUX_WS/bridge-rc")
  if [ "$rc" -ne 0 ]; then
    echo "--- bridge-stderr ---" >&2; cat "$BRIDGE_ATMUX_WS/bridge-stderr.log" >&2 || true
    echo "--- scrollback tail ---" >&2
    [ -f "$BRIDGE_ATMUX_STDERR" ] && tail -50 "$BRIDGE_ATMUX_STDERR" >&2 || true
  fi
  [ "$rc" -eq 0 ]
}

@test "T-agy-tmux.2 — artifact file appears" {
  require_live
  [ -f "$BRIDGE_ATMUX_ARTIFACT" ]
}

@test "T-agy-tmux.3 — artifact written via agy's Write tool (non-empty)" {
  # Bridge contract: subprocess boots, REPL detects prompt-marker, prompt
  # delivers, artifact appears via the Write tool, session kills clean.
  # LLM content fidelity (literal-content reproduction) is a separate concern:
  # gemini-3.5-flash routinely ignores "the file must contain exactly..." and
  # rephrases. agy has no -m flag, so we cannot pin a more obedient model.
  # The non-empty assertion preserves test signal for the bridge contract
  # while letting vendor content drift fail loudly elsewhere (e.g., S2.E
  # cross-CLI parity test).
  require_live
  [ -s "$BRIDGE_ATMUX_ARTIFACT" ]
}

@test "T-agy-tmux.4 — scrollback captured" {
  require_live
  [ -f "$BRIDGE_ATMUX_STDOUT" ]
  [ -s "$BRIDGE_ATMUX_STDOUT" ]
}

@test "T-agy-tmux.5 — no orphan tmux sessions" {
  require_live
  ! tmux ls 2>/dev/null | grep -q 'evolve-bridge-agy-' || {
    tmux ls 2>&1 >&2
    return 1
  }
}

@test "T-agy-tmux.6 — --allow-bypass required → rc=2" {
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-atmux-bypass-XXXXXX")"
  echo "noop" > "$WS/prompt.txt"
  run "$(echo "${BATS_TEST_DIRNAME}/../../bin/bridge")" launch \
    --cli=agy-tmux \
    --profile="${BATS_TEST_DIRNAME}/../fixtures/synth-profile.json" \
    --model=auto \
    --prompt-file="$WS/prompt.txt" --workspace="$WS" \
    --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
    --artifact="$WS/artifact.md"
  rm -rf "$WS"
  [ "$status" -eq 2 ]
}
