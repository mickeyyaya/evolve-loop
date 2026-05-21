#!/usr/bin/env bats
# T-stream — v0.3 stream-output pass-through tests (RED-first TDD).
#
# Background: phase-observer watches orchestrator-stdout.log for byte writes.
# claude -p with default text output emits ONLY the final response — no
# progress until the entire session ends. For long orchestrator runs that
# dispatch subagents (scout fan-out, etc.), this silence triggers false-
# positive stall kills at the observer's 600s threshold.
#
# Fix: opt-in --stream-output flag that makes claude-p append
# --output-format=stream-json --include-partial-messages so claude emits
# realtime JSONL events. The observer sees continuous byte writes and
# doesn't trigger.
#
# Coverage (parser-layer; behavioral coverage lives in stream-output-drivers.bats):
#   T-stream.1   --stream-output flag accepted, appears in validate-only
#   T-stream.2   profile.stream_output=true applied without CLI flag
#   T-stream.3   CLI flag wins over profile (flag=true, profile=false → true)
#   T-stream.4   env BRIDGE_STREAM_OUTPUT=1 works as fallback
#   T-stream.5   flag overrides env (flag=true, env=0 → true)
#   T-stream.6   env wins over profile when no flag (env=1, profile=false → true)
#   T-stream.7   invalid profile.stream_output value (non-bool) → rejected
#   T-stream.8   back-compat: unset → false, no streaming hint in validate-only
#   T-stream.9   3-way precedence: flag > env > profile
#   T-stream.10  JSON null stream_output treated as false (back-compat)

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-stream-XXXXXX")"
  export BRIDGE_BIN FIXTURE_DIR WS
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
  unset BRIDGE_CLI PROFILE_PATH RESOLVED_MODEL PROMPT_FILE \
        WORKSPACE_PATH STDOUT_LOG STDERR_LOG ARTIFACT_PATH \
        BRIDGE_STREAM_OUTPUT BRIDGE_DRY_RUN
}

# Helper: profile with optional stream_output field
_make_profile() {
  local path="$1" val="${2:-}"
  if [[ -n "$val" ]]; then
    cat > "$path" <<JSON
{
  "name": "stream-test-${val}",
  "model": "haiku",
  "allowed_tools": ["Read"],
  "stream_output": ${val},
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  else
    cat > "$path" <<JSON
{
  "name": "stream-test-default",
  "model": "haiku",
  "allowed_tools": ["Read"],
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  fi
}

_base_args() {
  echo "--cli=claude-p"
  echo "--profile=$1"
  echo "--model=haiku"
  echo "--prompt-file=${FIXTURE_DIR}/minimal-prompt.txt"
  echo "--workspace=$WS"
  echo "--stdout-log=$WS/stdout.log"
  echo "--stderr-log=$WS/stderr.log"
  echo "--artifact=$WS/artifact.md"
  echo "--validate-only"
}

@test "T-stream.1 — --stream-output flag accepted, appears in validate-only output" {
  local prof="$WS/profile.json"
  _make_profile "$prof"
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  args+=("--stream-output")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"stream-output  = true"* ]] || [[ "$output" == *"stream-output = true"* ]]
}

@test "T-stream.2 — profile.stream_output=true applied without a CLI flag" {
  local prof="$WS/profile.json"
  _make_profile "$prof" "true"
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"stream-output  = true"* ]] || [[ "$output" == *"stream-output = true"* ]]
}

@test "T-stream.3 — CLI flag wins over profile (flag=true, profile=false → true)" {
  local prof="$WS/profile.json"
  _make_profile "$prof" "false"
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  args+=("--stream-output")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"stream-output  = true"* ]] || [[ "$output" == *"stream-output = true"* ]]
}

@test "T-stream.4 — env BRIDGE_STREAM_OUTPUT=1 works as fallback" {
  local prof="$WS/profile.json"
  _make_profile "$prof"
  export BRIDGE_STREAM_OUTPUT=1
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"stream-output  = true"* ]] || [[ "$output" == *"stream-output = true"* ]]
}

@test "T-stream.5 — flag overrides env BRIDGE_STREAM_OUTPUT=0" {
  local prof="$WS/profile.json"
  _make_profile "$prof"
  export BRIDGE_STREAM_OUTPUT=0
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  args+=("--stream-output")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"stream-output  = true"* ]] || [[ "$output" == *"stream-output = true"* ]]
}

@test "T-stream.6 — env wins over profile when no flag (env=1, profile=false → true)" {
  local prof="$WS/profile.json"
  _make_profile "$prof" "false"
  export BRIDGE_STREAM_OUTPUT=1
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"stream-output  = true"* ]] || [[ "$output" == *"stream-output = true"* ]]
}

@test "T-stream.7 — invalid profile.stream_output value rejected" {
  local prof="$WS/profile.json"
  cat > "$prof" <<'JSON'
{
  "name": "invalid-stream",
  "model": "haiku",
  "allowed_tools": ["Read"],
  "stream_output": "not-a-bool",
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -ne 0 ]
  [[ "$output" == *"stream_output"* ]]
  [[ "$output" == *"invalid"* || "$output" == *"boolean"* ]]
}

@test "T-stream.8 — back-compat: unset → false, no streaming hint" {
  local prof="$WS/profile.json"
  _make_profile "$prof"
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"stream-output  = false"* ]] || [[ "$output" == *"stream-output = false"* ]]
}

@test "T-stream.9 — 3-way precedence: flag > env > profile" {
  local prof="$WS/profile.json"
  _make_profile "$prof" "false"
  export BRIDGE_STREAM_OUTPUT=0
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  args+=("--stream-output")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"stream-output  = true"* ]] || [[ "$output" == *"stream-output = true"* ]]
}

@test "T-stream.10 — JSON null stream_output treated as false (back-compat)" {
  local prof="$WS/profile.json"
  cat > "$prof" <<'JSON'
{
  "name": "null-stream",
  "model": "haiku",
  "allowed_tools": ["Read"],
  "stream_output": null,
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"stream-output  = false"* ]] || [[ "$output" == *"stream-output = false"* ]]
}
