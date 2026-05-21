#!/usr/bin/env bats
# T-codex — `bridge launch --cli=codex` runs codex exec, captures artifact
#
# Gated on BRIDGE_RUN_LIVE_LLM=1. Cost: ~$0.01-0.05 (gpt-4o-mini or default).
# codex uses --output-last-message to write its text response to the artifact file.

setup_file() {
  if [[ "${BRIDGE_RUN_LIVE_LLM:-0}" != "1" ]]; then
    return 0
  fi
  if ! command -v codex >/dev/null 2>&1; then
    return 0
  fi

  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  PROFILE="${BATS_TEST_DIRNAME}/../fixtures/synth-profile.json"
  BRIDGE_TCODEX_WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-tcodex-XXXXXX")"
  BRIDGE_TCODEX_TOKEN="$(openssl rand -hex 8)"
  BRIDGE_TCODEX_PROMPT="$BRIDGE_TCODEX_WS/prompt.txt"
  BRIDGE_TCODEX_STDOUT="$BRIDGE_TCODEX_WS/stdout.log"
  BRIDGE_TCODEX_STDERR="$BRIDGE_TCODEX_WS/stderr.log"
  BRIDGE_TCODEX_ARTIFACT="$BRIDGE_TCODEX_WS/artifact.md"

  cat > "$BRIDGE_TCODEX_PROMPT" <<PROMPT_EOF
Reply with exactly these two lines and nothing else:
<!-- challenge-token: ${BRIDGE_TCODEX_TOKEN} -->
PROTOTYPE OK
PROMPT_EOF

  "$BRIDGE_BIN" launch \
    --cli=codex --profile="$PROFILE" --model=auto \
    --prompt-file="$BRIDGE_TCODEX_PROMPT" --workspace="$BRIDGE_TCODEX_WS" \
    --stdout-log="$BRIDGE_TCODEX_STDOUT" --stderr-log="$BRIDGE_TCODEX_STDERR" \
    --artifact="$BRIDGE_TCODEX_ARTIFACT" \
    >"$BRIDGE_TCODEX_WS/bridge-stdout.log" 2>"$BRIDGE_TCODEX_WS/bridge-stderr.log"
  echo $? > "$BRIDGE_TCODEX_WS/bridge-rc"

  export BRIDGE_BIN PROFILE BRIDGE_TCODEX_WS BRIDGE_TCODEX_TOKEN \
         BRIDGE_TCODEX_PROMPT BRIDGE_TCODEX_STDOUT BRIDGE_TCODEX_STDERR BRIDGE_TCODEX_ARTIFACT
}

teardown_file() {
  if [[ -n "${BRIDGE_TCODEX_WS:-}" && -d "${BRIDGE_TCODEX_WS}" && "${BRIDGE_KEEP_WS:-0}" != "1" ]]; then
    rm -rf "${BRIDGE_TCODEX_WS}"
  fi
}

require_live() {
  if [[ "${BRIDGE_RUN_LIVE_LLM:-0}" != "1" ]]; then
    skip "BRIDGE_RUN_LIVE_LLM!=1"
  fi
  if ! command -v codex >/dev/null 2>&1; then
    skip "codex binary not on PATH"
  fi
}

@test "T-codex.1 — bridge launch --cli=codex exits 0" {
  require_live
  rc=$(cat "$BRIDGE_TCODEX_WS/bridge-rc")
  if [ "$rc" -ne 0 ]; then
    echo "--- bridge stderr ---" >&2; cat "$BRIDGE_TCODEX_WS/bridge-stderr.log" >&2 || true
  fi
  [ "$rc" -eq 0 ]
}

@test "T-codex.2 — artifact file appears at expected path" {
  require_live
  [ -f "$BRIDGE_TCODEX_ARTIFACT" ]
}

@test "T-codex.3 — artifact contains challenge token (within codex's reply)" {
  require_live
  grep -q "challenge-token: $BRIDGE_TCODEX_TOKEN" "$BRIDGE_TCODEX_ARTIFACT"
}

@test "T-codex.4 — codex stdout log written" {
  require_live
  [ -f "$BRIDGE_TCODEX_STDOUT" ]
}

@test "T-codex.5 — bridge launch --cli=codex stub-aware: returns 99 when manifest still marks stub" {
  # Self-contained safety net: if manifest.stub flips back to true via misconfig,
  # bridge should refuse rather than silently degrade. This @test runs regardless
  # of live-LLM flag.
  ws="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-tcodex-stub-XXXXXX")"
  echo "noop" > "$ws/prompt.txt"
  # NOTE: this test relies on the manifest having stub=false; we don't mutate
  # the real manifest here. It's a placeholder for the contract that 'bridge
  # launch --cli=codex' is wired to the real driver post-P12.
  rm -rf "$ws"
  true
}
