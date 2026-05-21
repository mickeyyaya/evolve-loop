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

# === Coverage extension (RED-then-GREEN audit): all 6 valid modes ============

@test "T-permmode.9 — all 6 valid modes accepted via --permission-mode flag" {
  local prof="$WS/profile.json"
  _make_profile "$prof"
  local mode
  for mode in plan default acceptEdits bypassPermissions auto dontAsk; do
    local args=()
    while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
    args+=("--permission-mode=$mode")
    run "$BRIDGE_BIN" launch "${args[@]}"
    if [ "$status" -ne 0 ]; then
      echo "FAIL on mode=$mode rc=$status" >&2
      echo "$output" >&2
    fi
    [ "$status" -eq 0 ]
    [[ "$output" == *"permission-mode = $mode"* ]]
  done
}

@test "T-permmode.10 — all 6 valid modes accepted via profile.permission_mode" {
  local mode
  for mode in plan default acceptEdits bypassPermissions auto dontAsk; do
    local prof="$WS/profile-${mode}.json"
    _make_profile "$prof" "$mode"
    local args=()
    while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
    run "$BRIDGE_BIN" launch "${args[@]}"
    if [ "$status" -ne 0 ]; then
      echo "FAIL on profile mode=$mode rc=$status" >&2
      echo "$output" >&2
    fi
    [ "$status" -eq 0 ]
    [[ "$output" == *"permission-mode = $mode"* ]]
  done
}

# === Coverage extension: precedence — env > profile ==========================

@test "T-permmode.11 — precedence: env var wins over profile when no CLI flag" {
  local prof="$WS/profile.json"
  _make_profile "$prof" "default"     # profile says default
  export BRIDGE_PERMISSION_MODE=plan   # env says plan
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  # No --permission-mode flag — env should win over profile
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  # NOTE: current implementation may actually prefer profile over env.
  # If this test fails, the implementation MUST be corrected to match
  # the documented precedence: CLI flag > env > profile.
  [[ "$output" == *"permission-mode = plan"* ]]
}

# === Coverage extension: 3-way precedence ====================================

@test "T-permmode.12 — precedence: CLI flag wins over both env AND profile (3-way)" {
  local prof="$WS/profile.json"
  _make_profile "$prof" "acceptEdits"  # profile = acceptEdits
  export BRIDGE_PERMISSION_MODE=default # env = default
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  args+=("--permission-mode=plan")     # flag = plan
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"permission-mode = plan"* ]]
  [[ "$output" != *"permission-mode = default"* ]]
  [[ "$output" != *"permission-mode = acceptEdits"* ]]
}

# === Coverage extension: input hardening — shell metacharacter rejection =====

@test "T-permmode.13 — profile.permission_mode with shell metachars rejected, payload NOT executed" {
  local prof="$WS/profile.json"
  # The proper security test: assert the metachar was NEVER executed.
  # Use a sentinel-file payload so we can check filesystem state, not
  # text substring (which would false-positive on echoing-the-rejection).
  local marker="$WS/pwned-marker"
  # JSON-encode a value that would create $marker if subshell-evaluated.
  local payload="plan\$(touch $marker)"
  cat > "$prof" <<JSON
{
  "name": "metachar-test",
  "model": "haiku",
  "allowed_tools": ["Read"],
  "permission_mode": "$payload",
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalid permission_mode"* ]]
  # Definitive security assertion: the sentinel file MUST NOT exist.
  # If it does, the case statement's value substitution was eval'd
  # somewhere unsafe.
  [ ! -f "$marker" ]
}

# === Coverage extension: JSON null/number robustness =========================

@test "T-permmode.14 — JSON null permission_mode treated as empty (back-compat)" {
  local prof="$WS/profile.json"
  cat > "$prof" <<'JSON'
{
  "name": "null-test",
  "model": "haiku",
  "allowed_tools": ["Read"],
  "permission_mode": null,
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  local args=()
  while IFS= read -r line; do args+=("$line"); done < <(_base_args "$prof")
  run "$BRIDGE_BIN" launch "${args[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"permission-mode = (driver default)"* ]]
}
