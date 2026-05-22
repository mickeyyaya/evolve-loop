#!/usr/bin/env bats
# T-stream-drv — driver-level behavioral tests for v0.3 stream-output.

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  FAKES_DIR="${FIXTURE_DIR}/fakes"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-stream-drv-XXXXXX")"
  STDOUT_LOG="${WS}/stdout.log"
  STDERR_LOG="${WS}/stderr.log"
  ARTIFACT="${WS}/artifact.md"
  TOKEN="$(openssl rand -hex 8 2>/dev/null || date +%s | tr -d '\n')"
  PROMPT="${WS}/prompt.txt"
  ARGS_FILE="${WS}/fake-args.txt"
  cat > "$PROMPT" <<EOF
Use your Write tool to create $ARTIFACT containing:
<!-- challenge-token: $TOKEN -->
PROTOTYPE OK
EOF
  export BRIDGE_BIN FIXTURE_DIR FAKES_DIR WS STDOUT_LOG STDERR_LOG ARTIFACT TOKEN PROMPT ARGS_FILE
  export BRIDGE_TESTING=1
  export BRIDGE_FAKE_ARGS_FILE="$ARGS_FILE"
}

teardown() {
  _kill_leaked_sessions
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
  unset BRIDGE_TESTING BRIDGE_CLAUDE_BINARY BRIDGE_CODEX_BINARY BRIDGE_AGY_BINARY \
        BRIDGE_FAKE_ARGS_FILE BRIDGE_STREAM_OUTPUT
}

_timeout() {
  local secs="$1"; shift
  perl -e 'alarm shift @ARGV; exec @ARGV' "$secs" "$@"
}

_profile() {
  local path="$1" stream="${2:-}"
  if [[ -n "$stream" ]]; then
    cat > "$path" <<JSON
{
  "name": "stream-drv-${stream}",
  "model": "haiku",
  "allowed_tools": ["Read", "Write"],
  "stream_output": ${stream},
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  else
    cat > "$path" <<JSON
{
  "name": "stream-drv-default",
  "model": "haiku",
  "allowed_tools": ["Read", "Write"],
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  fi
}

_run_launch() {
  local cli="$1"; shift
  local profile="$1"; shift
  case "$cli" in
    claude-p|claude-tmux) export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude-argcapture.sh" ;;
    codex|codex-tmux)     export BRIDGE_CODEX_BINARY="$FAKES_DIR/fake-codex.sh" ;;
    agy|agy-tmux)         export BRIDGE_AGY_BINARY="$FAKES_DIR/fake-agy.sh" ;;
  esac
  case "$cli" in
    *-tmux)
      _timeout 6 "$BRIDGE_BIN" launch \
        --cli="$cli" --profile="$profile" --model=auto \
        --prompt-file="$PROMPT" --workspace="$WS" \
        --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
        --artifact="$ARTIFACT" "$@"
      ;;
    *)
      "$BRIDGE_BIN" launch \
        --cli="$cli" --profile="$profile" --model=auto \
        --prompt-file="$PROMPT" --workspace="$WS" \
        --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
        --artifact="$ARTIFACT" "$@"
      ;;
  esac
}

_kill_leaked_sessions() {
  command -v tmux >/dev/null 2>&1 || return 0
  local ses
  while IFS= read -r ses; do
    [ -n "$ses" ] && tmux kill-session -t "$ses" 2>/dev/null || true
  done < <(tmux ls 2>/dev/null | awk -F: '/^evolve-bridge-/ { print $1 }')
}

@test "T-stream-drv.1 — claude-p + stream_output=true → --output-format=stream-json + --include-partial-messages reach binary" {
  local prof="$WS/profile.json"
  _profile "$prof" "true"
  run _run_launch claude-p "$prof"
  [ "$status" -eq 0 ]
  [ -f "$ARGS_FILE" ]
  grep -Fxq -- '--output-format' "$ARGS_FILE"
  local line_no
  line_no=$(grep -nFx -- '--output-format' "$ARGS_FILE" | head -1 | cut -d: -f1)
  local next_arg
  next_arg=$(sed -n "$((line_no + 1))p" "$ARGS_FILE")
  [ "$next_arg" = "stream-json" ]
  grep -Fxq -- '--include-partial-messages' "$ARGS_FILE"
}

@test "T-stream-drv.2 — claude-p + stream_output unset → NO streaming flags (back-compat)" {
  local prof="$WS/profile.json"
  _profile "$prof"
  run _run_launch claude-p "$prof"
  [ "$status" -eq 0 ]
  ! grep -Fxq -- '--output-format' "$ARGS_FILE"
  ! grep -Fxq -- '--include-partial-messages' "$ARGS_FILE"
}

@test "T-stream-drv.3 — claude-p + CLI flag --stream-output → same effect as profile" {
  local prof="$WS/profile.json"
  _profile "$prof"
  run _run_launch claude-p "$prof" --stream-output
  [ "$status" -eq 0 ]
  grep -Fxq -- '--output-format' "$ARGS_FILE"
  grep -Fxq -- '--include-partial-messages' "$ARGS_FILE"
}

@test "T-stream-drv.4 — claude-tmux + stream_output=true → driver logs note, behavior unchanged" {
  local prof="$WS/profile.json"
  _profile "$prof" "true"
  run _run_launch claude-tmux "$prof" --allow-bypass
  [[ "$output" != *"safety gate:"* ]]
  [[ "$output" == *"stream_output"* ]]
  [[ "$output" == *"no-op"* || "$output" == *"already streams"* || "$output" == *"scrollback"* ]]
}

@test "T-stream-drv.5 — codex + stream_output=true → driver logs note, proceeds" {
  local prof="$WS/profile.json"
  _profile "$prof" "true"
  run _run_launch codex "$prof"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  [[ "$output" == *"stream_output"* ]]
  [[ "$output" == *"not supported"* || "$output" == *"no-op"* ]]
}

@test "T-stream-drv.6 — agy + stream_output=true → driver logs note, proceeds" {
  local prof="$WS/profile.json"
  _profile "$prof" "true"
  run _run_launch agy "$prof"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  [[ "$output" == *"stream_output"* ]]
  [[ "$output" == *"not supported"* || "$output" == *"no-op"* ]]
}

@test "T-stream-drv.7 — back-compat: codex without stream_output → no NOTE, no behavior change" {
  local prof="$WS/profile.json"
  _profile "$prof"
  run _run_launch codex "$prof"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  [[ "$output" != *"stream_output"* ]]
}

# ============================================================================
# Simulate-builder-action — end-to-end verification that v0.3 streaming
# actually solves the phase-observer stall problem.
#
# Scenario: a long-running claude session (mimicking builder/orchestrator)
# emits JSONL events incrementally. We verify the parent's stdout_log grows
# DURING execution (not all at the end), which is what the phase-observer
# needs to see to avoid false-positive stall kills.
#
# T-stream-drv.8: streaming=ON → incremental writes (the fix).
# T-stream-drv.9: streaming=OFF → empty until end (proves the original gap).
# ============================================================================

@test "T-stream-drv.8 — claude-p + stream_output=true + JSONL fake → stdout_log grows incrementally during execution" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude-streaming.sh"
  export BRIDGE_FAKE_STREAM_EVENTS=6
  export BRIDGE_FAKE_STREAM_DELAY_S=0.5

  local prof="$WS/profile.json"
  _profile "$prof" "true"

  "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" &
  local bridge_pid=$!

  sleep 1
  local size_t1
  size_t1=$(stat -f%z "$STDOUT_LOG" 2>/dev/null || stat -c%s "$STDOUT_LOG" 2>/dev/null || echo 0)
  sleep 1
  local size_t2
  size_t2=$(stat -f%z "$STDOUT_LOG" 2>/dev/null || stat -c%s "$STDOUT_LOG" 2>/dev/null || echo 0)

  wait "$bridge_pid"
  local final_rc=$?
  local size_final
  size_final=$(stat -f%z "$STDOUT_LOG" 2>/dev/null || stat -c%s "$STDOUT_LOG" 2>/dev/null || echo 0)

  echo "size_t1=$size_t1 size_t2=$size_t2 size_final=$size_final rc=$final_rc" >&3

  [ "$final_rc" -eq 0 ]
  # By t=1s, stdout_log has SOME content (at least 1-2 events emitted)
  [ "$size_t1" -gt 0 ]
  # By t=2s, stdout_log has MORE content (incremental writes happened)
  [ "$size_t2" -gt "$size_t1" ]
  # Final size is >= mid-execution size
  [ "$size_final" -ge "$size_t2" ]
  # stdout_log contains JSONL events
  grep -q '"type":"message_delta"' "$STDOUT_LOG"
  grep -q '"type":"message_stop"' "$STDOUT_LOG"
}

@test "T-stream-drv.9 — claude-p + stream_output UNSET + same fake → stdout_log stays empty until end (proves the gap)" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude-streaming.sh"
  export BRIDGE_FAKE_STREAM_EVENTS=6
  export BRIDGE_FAKE_STREAM_DELAY_S=0.5

  local prof="$WS/profile.json"
  _profile "$prof"  # no stream_output

  "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" &
  local bridge_pid=$!

  sleep 1
  local size_t1
  size_t1=$(stat -f%z "$STDOUT_LOG" 2>/dev/null || stat -c%s "$STDOUT_LOG" 2>/dev/null || echo 0)
  sleep 1
  local size_t2
  size_t2=$(stat -f%z "$STDOUT_LOG" 2>/dev/null || stat -c%s "$STDOUT_LOG" 2>/dev/null || echo 0)

  wait "$bridge_pid"
  local final_rc=$?
  local size_final
  size_final=$(stat -f%z "$STDOUT_LOG" 2>/dev/null || stat -c%s "$STDOUT_LOG" 2>/dev/null || echo 0)

  echo "[non-streaming] size_t1=$size_t1 size_t2=$size_t2 size_final=$size_final rc=$final_rc" >&3

  [ "$final_rc" -eq 0 ]
  # During execution stdout_log stays EMPTY (the bug the observer trips on)
  [ "$size_t1" -eq 0 ]
  [ "$size_t2" -eq 0 ]
  # Final size > 0 (output written at end)
  [ "$size_final" -gt 0 ]
}
