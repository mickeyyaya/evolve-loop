#!/usr/bin/env bats
# T-permmode — v0.2 plan-mode and --permission-mode pass-through tests.
#
# Covers:
#   1. --permission-mode=plan flag is accepted and shows in validate-only output
#   2. profile.permission_mode=plan is read and applied without a flag
#   3. CLI flag wins over profile.permission_mode
#   4. invalid permission_mode is rejected with EC_BAD_FLAGS
#   5. claude-tmux --allow-bypass is NOT required when permission-mode is set
#   6. empty permission_mode is back-compat (driver picks default)

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-permmode-XXXXXX")"
  export BRIDGE_BIN FIXTURE_DIR WS
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
  unset BRIDGE_CLI PROFILE_PATH RESOLVED_MODEL PROMPT_FILE \
        WORKSPACE_PATH STDOUT_LOG STDERR_LOG ARTIFACT_PATH \
        BRIDGE_PERMISSION_MODE BRIDGE_ALLOW_BYPASS BRIDGE_DRY_RUN
}

# Helper: write a profile JSON with optional permission_mode field
_make_profile() {
  local path="$1" perm="${2:-}"
  if [[ -n "$perm" ]]; then
    cat > "$path" <<JSON
{
  "name": "t-permmode-${perm}",
  "model": "haiku",
  "allowed_tools": ["Read"],
  "permission_mode": "${perm}",
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  else
    cat > "$path" <<JSON
{
  "name": "t-permmode-default",
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

@test "T-permmode.1 — --permission-mode=plan flag is accepted and appears in validate-only output" {
  local prof="$WS/profile.json"
  _make_profile "$prof"
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  args+=("--permission-mode=plan")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"permission-mode = plan"* ]]
}

@test "T-permmode.2 — profile.permission_mode=plan is applied without a CLI flag" {
  local prof="$WS/profile.json"
  _make_profile "$prof" "plan"
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"permission-mode = plan"* ]]
}

@test "T-permmode.3 — CLI flag wins over profile.permission_mode" {
  local prof="$WS/profile.json"
  _make_profile "$prof" "default"
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  args+=("--permission-mode=plan")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"permission-mode = plan"* ]]
  [[ "$output" != *"permission-mode = default"* ]]
}

@test "T-permmode.4 — invalid permission_mode in profile is rejected with non-zero exit" {
  local prof="$WS/profile.json"
  _make_profile "$prof" "totally-bogus-mode"
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalid permission_mode"* ]] || [[ "$output" == *"invalid --permission-mode"* ]]
}

@test "T-permmode.5 — invalid --permission-mode flag value is rejected" {
  local prof="$WS/profile.json"
  _make_profile "$prof"
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  args+=("--permission-mode=garbage")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalid"* ]]
}

@test "T-permmode.6 — empty permission_mode is back-compat (driver default)" {
  local prof="$WS/profile.json"
  _make_profile "$prof"
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"permission-mode = (driver default)"* ]]
}

@test "T-permmode.7 — env var BRIDGE_PERMISSION_MODE works as fallback" {
  local prof="$WS/profile.json"
  _make_profile "$prof"
  export BRIDGE_PERMISSION_MODE=plan
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"permission-mode = plan"* ]]
}

@test "T-permmode.8 — flag overrides env var BRIDGE_PERMISSION_MODE" {
  local prof="$WS/profile.json"
  _make_profile "$prof"
  export BRIDGE_PERMISSION_MODE=default
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  args+=("--permission-mode=plan")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"permission-mode = plan"* ]]
}
