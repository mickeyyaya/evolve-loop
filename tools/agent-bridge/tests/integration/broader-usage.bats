#!/usr/bin/env bats
# T-broad — broader operator use-case tests beyond per-phase patterns.

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  FAKES_DIR="${FIXTURE_DIR}/fakes"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-broad-XXXXXX")"
  STDOUT_LOG="${WS}/stdout.log"
  STDERR_LOG="${WS}/stderr.log"
  ARTIFACT="${WS}/artifact.md"
  TOKEN="$(openssl rand -hex 8 2>/dev/null || date +%s | tr -d '\n')"
  PROMPT="${WS}/prompt.txt"
  cat > "$PROMPT" <<EOF
Use your Write tool to create $ARTIFACT containing:
<!-- challenge-token: $TOKEN -->
PROTOTYPE OK
EOF
  export BRIDGE_BIN FIXTURE_DIR FAKES_DIR WS STDOUT_LOG STDERR_LOG ARTIFACT TOKEN PROMPT
  export BRIDGE_TESTING=1
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
  unset BRIDGE_TESTING BRIDGE_CLAUDE_BINARY BRIDGE_CODEX_BINARY BRIDGE_AGY_BINARY \
        BRIDGE_FAKE_STREAM_EVENTS BRIDGE_FAKE_STREAM_DELAY_S BRIDGE_FAKE_ARGS_FILE \
        BRIDGE_STREAM_OUTPUT BRIDGE_PERMISSION_MODE
}

_broad_profile() {
  local path="$1" model="${2:-haiku}"
  cat > "$path" <<JSON
{
  "name": "broad-test",
  "model": "$model",
  "allowed_tools": ["Read", "Write"],
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
}

_run_basic() {
  "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$1" --model="${2:-auto}" \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" "${@:3}"
}

# === T-broad.1: retry — second invocation cleanly overwrites artifact ===
@test "T-broad.1 — retry: 2nd invocation overwrites artifact + emits stale-WARN" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local prof="$WS/p.json"
  _broad_profile "$prof"

  run _run_basic "$prof"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  local first_token
  first_token=$(grep -oE 'challenge-token: [a-f0-9]+' "$ARTIFACT" | awk '{print $2}')

  TOKEN="$(openssl rand -hex 8 2>/dev/null || echo "secondtoken")"
  cat > "$PROMPT" <<EOF
Use your Write tool to create $ARTIFACT containing:
<!-- challenge-token: $TOKEN -->
PROTOTYPE OK (second)
EOF
  run _run_basic "$prof"
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN: workspace contains stale files"* ]]
  local second_token
  second_token=$(grep -oE 'challenge-token: [a-f0-9]+' "$ARTIFACT" | awk '{print $2}')
  [ "$second_token" != "$first_token" ]
}

# === T-broad.2: large prompt (50KB) ===
@test "T-broad.2 — large prompt: 50KB+ prompt accepted and processed" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local i=0
  cat > "$PROMPT" <<EOF
Use your Write tool to create $ARTIFACT containing:
<!-- challenge-token: $TOKEN -->
PROTOTYPE OK
---
EOF
  while [ "$i" -lt 1000 ]; do
    echo "filler line $i: lorem ipsum dolor sit amet consectetur adipiscing" >> "$PROMPT"
    i=$((i + 1))
  done
  local prompt_bytes
  prompt_bytes=$(wc -c < "$PROMPT" | tr -d ' ')
  [ "$prompt_bytes" -ge 50000 ]

  local prof="$WS/p.json"
  _broad_profile "$prof"
  run _run_basic "$prof"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
}

# === T-broad.3: large streamed artifact ===
@test "T-broad.3 — large streamed output: 30+ JSONL events captured to stdout_log" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude-streaming.sh"
  export BRIDGE_FAKE_STREAM_EVENTS=30
  export BRIDGE_FAKE_STREAM_DELAY_S=0.05

  local prof="$WS/p.json"
  cat > "$prof" <<JSON
{
  "name": "large-stream",
  "model": "haiku",
  "allowed_tools": ["Read", "Write"],
  "stream_output": true,
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  run _run_basic "$prof"
  [ "$status" -eq 0 ]
  local event_count
  event_count=$(grep -c '"type":"message_delta"' "$STDOUT_LOG")
  [ "$event_count" -ge 30 ]
  local log_size
  log_size=$(wc -c < "$STDOUT_LOG" | tr -d ' ')
  [ "$log_size" -ge 2000 ]
}

# === T-broad.4: unicode prompts ===
@test "T-broad.4 — unicode prompts: emoji + CJK + RTL chars survive prompt path" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  printf 'Use your Write tool to create %s containing:\n<!-- challenge-token: %s -->\nUnicode test: 🚀✅日本語 العربية ñ\n' \
    "$ARTIFACT" "$TOKEN" > "$PROMPT"
  local prof="$WS/p.json"
  _broad_profile "$prof"
  run _run_basic "$prof"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  grep -q "$TOKEN" "$ARTIFACT"
}

# === T-broad.5: concurrent bridge launches to distinct workspaces ===
@test "T-broad.5 — concurrent launches: two parallel bridges succeed independently" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local prof="$WS/p.json"
  _broad_profile "$prof"

  local ws_a="$WS/wa" ws_b="$WS/wb"
  mkdir -p "$ws_a" "$ws_b"
  cat > "$ws_a/prompt.txt" <<EOF
Use your Write tool to create $ws_a/artifact.md containing:
<!-- challenge-token: aaaaaaaaaaaaaaaa -->
A
EOF
  cat > "$ws_b/prompt.txt" <<EOF
Use your Write tool to create $ws_b/artifact.md containing:
<!-- challenge-token: bbbbbbbbbbbbbbbb -->
B
EOF

  "$BRIDGE_BIN" launch --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$ws_a/prompt.txt" --workspace="$ws_a" \
    --stdout-log="$ws_a/out.log" --stderr-log="$ws_a/err.log" \
    --artifact="$ws_a/artifact.md" &
  local pid_a=$!
  "$BRIDGE_BIN" launch --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$ws_b/prompt.txt" --workspace="$ws_b" \
    --stdout-log="$ws_b/out.log" --stderr-log="$ws_b/err.log" \
    --artifact="$ws_b/artifact.md" &
  local pid_b=$!

  wait "$pid_a"
  local rc_a=$?
  wait "$pid_b"
  local rc_b=$?

  [ "$rc_a" -eq 0 ]
  [ "$rc_b" -eq 0 ]
  [ -f "$ws_a/artifact.md" ]
  [ -f "$ws_b/artifact.md" ]
  grep -q "aaaa" "$ws_a/artifact.md"
  grep -q "bbbb" "$ws_b/artifact.md"
}

# === T-broad.6: shell-metachar prompt — payload NOT executed ===
@test "T-broad.6 — shell-metachar in prompt: dollar-paren and backtick payloads NEVER evaluated" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local marker="$WS/pwned-marker"
  cat > "$PROMPT" <<EOF
Use your Write tool to create $ARTIFACT.
Sentinel: \$(touch $marker)
Sentinel2: \`touch $marker\`
Token: $TOKEN
EOF
  local prof="$WS/p.json"
  _broad_profile "$prof"
  run _run_basic "$prof"
  [ "$status" -eq 0 ]
  [ ! -f "$marker" ]
}

# === T-broad.7: --model=auto resolves to profile.model ===
@test "T-broad.7 — --model=auto resolves to profile.model" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude-argcapture.sh"
  export BRIDGE_FAKE_ARGS_FILE="$WS/fake-args.txt"
  local prof="$WS/p.json"
  _broad_profile "$prof" "opus"
  run _run_basic "$prof" "auto"
  [ "$status" -eq 0 ]
  grep -Fxq -- '--model' "$BRIDGE_FAKE_ARGS_FILE"
  local line_no
  line_no=$(grep -nFx -- '--model' "$BRIDGE_FAKE_ARGS_FILE" | head -1 | cut -d: -f1)
  local next_arg
  next_arg=$(sed -n "$((line_no + 1))p" "$BRIDGE_FAKE_ARGS_FILE")
  [ "$next_arg" = "opus" ]
}

# === T-broad.8: validate-only shows ALL v0.2/v0.3 fields ===
@test "T-broad.8 — validate-only output contains permission-mode AND stream-output rows" {
  local prof="$WS/p.json"
  cat > "$prof" <<JSON
{
  "name": "full-features",
  "model": "haiku",
  "allowed_tools": ["Read"],
  "permission_mode": "plan",
  "stream_output": true,
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --validate-only
  [ "$status" -eq 0 ]
  [[ "$output" == *"permission-mode = plan"* ]]
  [[ "$output" == *"stream-output  = true"* ]] || [[ "$output" == *"stream-output = true"* ]]
}

# === T-broad.9: stale-workspace WARN (regression for F6 guard) ===
@test "T-broad.9 — workspace-reuse WARN fires on stale artifact" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  echo "OLD" > "$WS/artifact.md"
  local prof="$WS/p.json"
  _broad_profile "$prof"
  run _run_basic "$prof"
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN: workspace contains stale files"* ]]
}
