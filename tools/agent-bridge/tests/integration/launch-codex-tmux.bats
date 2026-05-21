#!/usr/bin/env bats
# T-codex-tmux — interactive codex via tmux (subscription-preserving)
# Gated on BRIDGE_RUN_LIVE_LLM=1. Cost: ~$0.05-0.10 per file.

setup_file() {
  if [[ "${BRIDGE_RUN_LIVE_LLM:-0}" != "1" ]]; then return 0; fi
  if ! command -v codex >/dev/null 2>&1 || ! command -v tmux >/dev/null 2>&1; then return 0; fi

  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  PROFILE="${BATS_TEST_DIRNAME}/../fixtures/synth-profile.json"
  BRIDGE_CTMUX_WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-ctmux-XXXXXX")"
  BRIDGE_CTMUX_TOKEN="$(openssl rand -hex 8)"
  BRIDGE_CTMUX_PROMPT="$BRIDGE_CTMUX_WS/prompt.txt"
  BRIDGE_CTMUX_STDOUT="$BRIDGE_CTMUX_WS/stdout.log"
  BRIDGE_CTMUX_STDERR="$BRIDGE_CTMUX_WS/stderr.log"
  BRIDGE_CTMUX_ARTIFACT="$BRIDGE_CTMUX_WS/artifact.md"

  cat > "$BRIDGE_CTMUX_PROMPT" <<PROMPT_EOF
Use your shell tool to create a file at this exact path: ${BRIDGE_CTMUX_ARTIFACT}
The file must contain exactly these two lines:
<!-- challenge-token: ${BRIDGE_CTMUX_TOKEN} -->
PROTOTYPE OK
After writing, exit. Do not narrate.
PROMPT_EOF

  "$BRIDGE_BIN" launch \
    --cli=codex-tmux --profile="$PROFILE" --model=auto \
    --prompt-file="$BRIDGE_CTMUX_PROMPT" --workspace="$BRIDGE_CTMUX_WS" \
    --stdout-log="$BRIDGE_CTMUX_STDOUT" --stderr-log="$BRIDGE_CTMUX_STDERR" \
    --artifact="$BRIDGE_CTMUX_ARTIFACT" \
    --allow-bypass \
    >"$BRIDGE_CTMUX_WS/bridge-stdout.log" 2>"$BRIDGE_CTMUX_WS/bridge-stderr.log"
  echo $? > "$BRIDGE_CTMUX_WS/bridge-rc"

  export BRIDGE_BIN PROFILE BRIDGE_CTMUX_WS BRIDGE_CTMUX_TOKEN \
         BRIDGE_CTMUX_PROMPT BRIDGE_CTMUX_STDOUT BRIDGE_CTMUX_STDERR BRIDGE_CTMUX_ARTIFACT
}

teardown_file() {
  if [[ -n "${BRIDGE_CTMUX_WS:-}" && -d "${BRIDGE_CTMUX_WS}" && "${BRIDGE_KEEP_WS:-0}" != "1" ]]; then
    tmux ls 2>/dev/null | awk -F: '/evolve-bridge-codex-/{print $1}' \
      | xargs -n1 -I{} tmux kill-session -t {} 2>/dev/null || true
    rm -rf "${BRIDGE_CTMUX_WS}"
  fi
}

require_live() {
  [[ "${BRIDGE_RUN_LIVE_LLM:-0}" == "1" ]] || skip "BRIDGE_RUN_LIVE_LLM!=1"
  command -v codex >/dev/null 2>&1 || skip "codex not on PATH"
  command -v tmux  >/dev/null 2>&1 || skip "tmux not on PATH"
}

@test "T-codex-tmux.1 — bridge launch --cli=codex-tmux exits 0" {
  require_live
  rc=$(cat "$BRIDGE_CTMUX_WS/bridge-rc")
  if [ "$rc" -ne 0 ]; then
    echo "--- bridge-stderr ---" >&2; cat "$BRIDGE_CTMUX_WS/bridge-stderr.log" >&2 || true
    echo "--- scrollback tail ---" >&2
    [ -f "$BRIDGE_CTMUX_STDERR" ] && tail -50 "$BRIDGE_CTMUX_STDERR" >&2 || true
  fi
  [ "$rc" -eq 0 ]
}

@test "T-codex-tmux.2 — artifact file appears" {
  require_live
  [ -f "$BRIDGE_CTMUX_ARTIFACT" ]
}

@test "T-codex-tmux.3 — artifact contains challenge token" {
  require_live
  grep -q "challenge-token: $BRIDGE_CTMUX_TOKEN" "$BRIDGE_CTMUX_ARTIFACT"
}

@test "T-codex-tmux.4 — scrollback captured" {
  require_live
  [ -f "$BRIDGE_CTMUX_STDOUT" ]
  [ -s "$BRIDGE_CTMUX_STDOUT" ]
}

@test "T-codex-tmux.5 — no orphan tmux sessions" {
  require_live
  ! tmux ls 2>/dev/null | grep -q 'evolve-bridge-codex-' || {
    tmux ls 2>&1 >&2
    return 1
  }
}

@test "T-codex-tmux.6 — --allow-bypass required → rc=2" {
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-ctmux-bypass-XXXXXX")"
  echo "noop" > "$WS/prompt.txt"
  run "$(echo "${BATS_TEST_DIRNAME}/../../bin/bridge")" launch \
    --cli=codex-tmux \
    --profile="${BATS_TEST_DIRNAME}/../fixtures/synth-profile.json" \
    --model=auto \
    --prompt-file="$WS/prompt.txt" --workspace="$WS" \
    --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
    --artifact="$WS/artifact.md"
  rm -rf "$WS"
  [ "$status" -eq 2 ]
}
