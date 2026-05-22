#!/usr/bin/env bats
# T-phase — phase-pattern simulation tests for the evolve-loop pipeline.
#
# Each test simulates how bridge is invoked for a specific phase of the
# evolve-loop pipeline. These are integration tests that verify bridge
# handles realistic phase-shaped workloads correctly.
#
# Phases simulated:
#   T-phase.1   intent (short LLM, single artifact, fast)
#   T-phase.2   builder-long with streaming (many tool-call events)
#   T-phase.3   tdd-engineer writes acs/cycle-N/ predicate files
#   T-phase.4   audit-style with deterministic checks (no LLM)
#   T-phase.5   memo-style short post-ship phase
#   T-phase.6   short-phase without streaming (intent fallback path)

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  FAKES_DIR="${FIXTURE_DIR}/fakes"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-phase-XXXXXX")"
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
        BRIDGE_FAKE_STREAM_EVENTS BRIDGE_FAKE_STREAM_DELAY_S \
        BRIDGE_STREAM_OUTPUT BRIDGE_PERMISSION_MODE
}

# Helper: minimal profile with optional fields. Uses jq for reliable JSON
# construction (bash heredoc with trailing-comma stripping proved fragile).
_phase_profile() {
  local path="$1" perm="${2:-}" stream="${3:-}"
  local base='{"name":"phase-test","model":"haiku","allowed_tools":["Read","Write","Bash(test:*)"],"auto_respond":{"destructive_ops":false,"timeout_s":60},"prompt_overrides":[]}'
  local out="$base"
  if [[ -n "$perm" ]]; then
    out=$(echo "$out" | jq --arg p "$perm" '. + {permission_mode: $p}')
  fi
  if [[ -n "$stream" ]]; then
    out=$(echo "$out" | jq --argjson s "$stream" '. + {stream_output: $s}')
  fi
  echo "$out" > "$path"
}

# === T-phase.1: intent — short LLM, single artifact, no streaming needed ===
@test "T-phase.1 — intent: short LLM session produces single artifact quickly" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local prof="$WS/p.json"
  _phase_profile "$prof"
  local t0 t1
  t0=$(date +%s)
  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  t1=$(date +%s)
  local elapsed=$((t1 - t0))
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  grep -q "PROTOTYPE OK" "$ARTIFACT"
  # Intent-shaped phases must complete fast
  [ "$elapsed" -le 5 ]
}

# === T-phase.2: builder-long with streaming — many tool-call events ===
@test "T-phase.2 — builder-long: long-running with streaming emits 8+ JSONL events" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude-streaming.sh"
  export BRIDGE_FAKE_STREAM_EVENTS=10
  export BRIDGE_FAKE_STREAM_DELAY_S=0.2

  local prof="$WS/p.json"
  _phase_profile "$prof" "" "true"

  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"

  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  local event_count
  event_count=$(grep -c '"type":"message_delta"' "$STDOUT_LOG")
  [ "$event_count" -ge 8 ]
  grep -q '"type":"message_stop"' "$STDOUT_LOG"
}

# === T-phase.3: tdd-engineer pattern — writes test-report.md into workspace ===
# Faithful to real tdd-engineer behavior: --artifact is the .md report file;
# the individual acs/cycle-N/*.sh predicates are side-effect Writes from the
# Write tool (not the bridge's artifact contract). Bridge is responsible for
# the .md artifact landing; the profile's allowed_tools governs whether the
# Write tool can target the predicate subdir.
@test "T-phase.3 — tdd-engineer: writes test-report.md as artifact within workspace" {
  # Pre-create the acs/cycle-N subdir to simulate where predicates would live
  mkdir -p "$WS/acs/cycle-99"
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local report="$WS/test-report.md"
  cat > "$PROMPT" <<EOF
Use your Write tool to create $report containing:
<!-- challenge-token: $TOKEN -->
TDD report — predicates would be under acs/cycle-99/*.sh
EOF
  local prof="$WS/p.json"
  _phase_profile "$prof"
  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$report"
  [ "$status" -eq 0 ]
  [ -f "$report" ]
  grep -q "$TOKEN" "$report"
  # The cycle-scoped subdir exists (workspace is operator-controlled)
  [ -d "$WS/acs/cycle-99" ]
}

# === T-phase.4: audit-style — deterministic via dry-run (no LLM cost) ===
@test "T-phase.4 — audit-style: dry-run path produces artifact without LLM" {
  local prof="$WS/p.json"
  _phase_profile "$prof"
  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --dry-run
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  grep -q "DRY-RUN-OK" "$ARTIFACT"
}

# === T-phase.5: memo — short post-ship phase ===
@test "T-phase.5 — memo: short LLM produces artifact (post-ship memo pattern)" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  cat > "$PROMPT" <<EOF
Write a 3-line memo about what we shipped, to $ARTIFACT.
Token: $TOKEN
EOF
  local prof="$WS/p.json"
  _phase_profile "$prof"
  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
}

# === T-phase.6: short-phase without streaming (intent fallback) ===
@test "T-phase.6 — short-phase without streaming: completes cleanly (no observer pressure)" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local prof="$WS/p.json"
  _phase_profile "$prof"  # no stream_output
  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
  ! grep -q '"type":"message_delta"' "$STDOUT_LOG"
}
