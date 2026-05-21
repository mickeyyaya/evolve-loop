#!/usr/bin/env bats
# T-permmode-drv — driver-level behavioral tests for the v0.2 --permission-mode feature.
#
# Coverage:
#   T-permmode-drv.1 (claude-p):       --permission-mode plan reaches claude binary
#   T-permmode-drv.2 (claude-tmux):    safety gate relaxed when permission_mode set
#   T-permmode-drv.3 (claude-tmux):    claude_cmd swaps --dangerously-skip-permissions for --permission-mode plan
#   T-permmode-drv.4 (claude-tmux):    back-compat — when unset, uses --dangerously-skip-permissions
#   T-permmode-drv.5 (codex):          permission_mode set → fail with clear error (NOT supported)
#   T-permmode-drv.6 (codex-tmux):     same — explicit rejection
#   T-permmode-drv.7 (agy):            same — explicit rejection
#   T-permmode-drv.8 (agy-tmux):       same — explicit rejection
#   T-permmode-drv.9 (codex):          back-compat — no permission_mode → existing behavior
#   T-permmode-drv.10 (agy):           back-compat — no permission_mode → existing behavior

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  FAKES_DIR="${FIXTURE_DIR}/fakes"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-permmode-drv-XXXXXX")"
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
        BRIDGE_FAKE_ARGS_FILE BRIDGE_PERMISSION_MODE
}

# Forward-declare _kill_leaked_sessions so teardown above can call it
# even though it's defined later in the file (bats sources the whole file
# before running, so this works either way — declaration is for readers).

# Helper: profile JSON with optional permission_mode
_profile() {
  local path="$1" perm="${2:-}"
  if [[ -n "$perm" ]]; then
    cat > "$path" <<JSON
{
  "name": "drv-test-${perm}",
  "model": "haiku",
  "allowed_tools": ["Read", "Write"],
  "permission_mode": "${perm}",
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  else
    cat > "$path" <<JSON
{
  "name": "drv-test-default",
  "model": "haiku",
  "allowed_tools": ["Read", "Write"],
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  fi
}

# Portable timeout wrapper (BSD/macOS lacks GNU `timeout` by default).
# Uses perl's alarm() to kill the command after N seconds.
_timeout() {
  local secs="$1"; shift
  perl -e 'alarm shift @ARGV; exec @ARGV' "$secs" "$@"
}

# Helper: launch bridge with a fake binary substituted for the given cli.
# For tmux drivers (claude-tmux, codex-tmux, agy-tmux), wraps in a 6s
# perl-based timeout because the tmux REPL-boot loop waits for a prompt
# marker that fake binaries don't emit. The driver writes the
# [claude-tmux] launching: <cmd> log BEFORE the REPL wait, so even a
# killed run captures the line we need to assert against.
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

# Helper: kill any leaked tmux sessions matching evolve-bridge-* (cleanup
# defense — _timeout may leave tmux sessions running).
_kill_leaked_sessions() {
  command -v tmux >/dev/null 2>&1 || return 0
  local ses
  while IFS= read -r ses; do
    [ -n "$ses" ] && tmux kill-session -t "$ses" 2>/dev/null || true
  done < <(tmux ls 2>/dev/null | awk -F: '/^evolve-bridge-/ { print $1 }')
}

# ============================================================================
# claude-p — supported path
# ============================================================================

@test "T-permmode-drv.1 — claude-p: --permission-mode=plan reaches claude binary via argv" {
  local prof="$WS/profile.json"
  _profile "$prof"
  run _run_launch claude-p "$prof" --permission-mode=plan
  [ "$status" -eq 0 ]
  [ -f "$ARGS_FILE" ]
  grep -Fxq -- '--permission-mode' "$ARGS_FILE"
  local line_no
  line_no=$(grep -nFx -- '--permission-mode' "$ARGS_FILE" | head -1 | cut -d: -f1)
  [ -n "$line_no" ]
  local next_line=$((line_no + 1))
  local next_arg
  next_arg=$(sed -n "${next_line}p" "$ARGS_FILE")
  [ "$next_arg" = "plan" ]
}

# ============================================================================
# claude-tmux — supported path; safety-gate behavior changes
# ============================================================================

@test "T-permmode-drv.2 — claude-tmux: --allow-bypass NOT required when permission_mode set" {
  local prof="$WS/profile.json"
  _profile "$prof" "plan"
  run _run_launch claude-tmux "$prof"
  [[ "$output" != *"safety gate: --allow-bypass is required"* ]]
}

@test "T-permmode-drv.3 — claude-tmux: claude_cmd has --permission-mode plan (NOT --dangerously-skip-permissions)" {
  local prof="$WS/profile.json"
  _profile "$prof" "plan"
  run _run_launch claude-tmux "$prof"
  [[ "$output" == *"--permission-mode plan"* ]]
  [[ "$output" != *"[claude-tmux] launching: claude"*"--dangerously-skip-permissions"* ]]
}

@test "T-permmode-drv.4 — claude-tmux: back-compat — no permission_mode → uses --dangerously-skip-permissions + requires --allow-bypass" {
  local prof="$WS/profile.json"
  _profile "$prof"
  run _run_launch claude-tmux "$prof"
  [[ "$output" == *"safety gate: --allow-bypass is required"* ]]
}

# ============================================================================
# codex / codex-tmux — UNSUPPORTED path; explicit rejection
# ============================================================================

@test "T-permmode-drv.5 — codex: permission_mode=plan → bridge fails with clear error" {
  local prof="$WS/profile.json"
  _profile "$prof" "plan"
  run _run_launch codex "$prof"
  [ "$status" -ne 0 ]
  [[ "$output" == *"permission_mode"* ]]
  [[ "$output" == *"not supported"* || "$output" == *"unsupported"* ]]
}

@test "T-permmode-drv.6 — codex-tmux: permission_mode=plan → bridge fails with clear error" {
  local prof="$WS/profile.json"
  _profile "$prof" "plan"
  run _run_launch codex-tmux "$prof"
  [ "$status" -ne 0 ]
  [[ "$output" == *"permission_mode"* ]]
  [[ "$output" == *"not supported"* || "$output" == *"unsupported"* ]]
}

# ============================================================================
# agy / agy-tmux — UNSUPPORTED path; explicit rejection
# ============================================================================

@test "T-permmode-drv.7 — agy: permission_mode=plan → bridge fails with clear error" {
  local prof="$WS/profile.json"
  _profile "$prof" "plan"
  run _run_launch agy "$prof"
  [ "$status" -ne 0 ]
  [[ "$output" == *"permission_mode"* ]]
  [[ "$output" == *"not supported"* || "$output" == *"unsupported"* ]]
}

@test "T-permmode-drv.8 — agy-tmux: permission_mode=plan → bridge fails with clear error" {
  local prof="$WS/profile.json"
  _profile "$prof" "plan"
  run _run_launch agy-tmux "$prof"
  [ "$status" -ne 0 ]
  [[ "$output" == *"permission_mode"* ]]
  [[ "$output" == *"not supported"* || "$output" == *"unsupported"* ]]
}

# ============================================================================
# Back-compat across non-claude drivers
# ============================================================================

@test "T-permmode-drv.9 — back-compat: codex without permission_mode works as before" {
  local prof="$WS/profile.json"
  _profile "$prof"
  run _run_launch codex "$prof"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
}

@test "T-permmode-drv.10 — back-compat: agy without permission_mode works as before" {
  local prof="$WS/profile.json"
  _profile "$prof"
  run _run_launch agy "$prof"
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT" ]
}

# ============================================================================
# Coverage extension: untested claude-tmux truth-table combos + tmux-flavored
# back-compat for non-claude drivers
# ============================================================================

@test "T-permmode-drv.11 — claude-tmux: --allow-bypass + NO permission_mode → cmd has --dangerously-skip-permissions" {
  # Branch CT3 + CC2: the pre-v0.2 path. With --allow-bypass and NO
  # permission_mode, the safety gate passes AND the claude command keeps
  # the old --dangerously-skip-permissions flag (NOT --permission-mode).
  local prof="$WS/profile.json"
  _profile "$prof"  # no permission_mode
  run _run_launch claude-tmux "$prof" --allow-bypass
  [[ "$output" != *"safety gate: --allow-bypass is required"* ]]
  [[ "$output" == *"--dangerously-skip-permissions"* ]]
  [[ "$output" != *"--permission-mode"* ]]
}

@test "T-permmode-drv.12 — claude-tmux: --allow-bypass + permission_mode=plan → plan wins, NO bypass" {
  # Branch CT4: BOTH --allow-bypass passed AND permission_mode set.
  # permission_mode takes precedence (driver swaps OUT bypass, not adds).
  local prof="$WS/profile.json"
  _profile "$prof" "plan"
  run _run_launch claude-tmux "$prof" --allow-bypass
  [[ "$output" == *"--permission-mode plan"* ]]
  # The launch log line must NOT contain --dangerously-skip-permissions —
  # plan mode is semantically incompatible with bypass.
  [[ "$output" != *"[claude-tmux] launching: claude"*"--dangerously-skip-permissions"* ]]
}

@test "T-permmode-drv.13 — back-compat: codex-tmux without permission_mode + --allow-bypass works" {
  # Branch D4: codex-tmux back-compat. With NO permission_mode, the
  # rejection guard doesn't fire and the driver proceeds to its existing
  # --allow-bypass safety gate (which passes with --allow-bypass).
  local prof="$WS/profile.json"
  _profile "$prof"
  run _run_launch codex-tmux "$prof" --allow-bypass
  # The permission_mode rejection MUST NOT fire (the bug we just fixed)
  [[ "$output" != *"[codex-tmux] permission_mode"* ]]
  # The existing safety gate ALSO MUST NOT fire
  [[ "$output" != *"safety gate: --allow-bypass is required"* ]]
}

@test "T-permmode-drv.14 — back-compat: agy-tmux without permission_mode + --allow-bypass works" {
  # Branch D8: agy-tmux back-compat (mirror of T-permmode-drv.13)
  local prof="$WS/profile.json"
  _profile "$prof"
  run _run_launch agy-tmux "$prof" --allow-bypass
  [[ "$output" != *"[agy-tmux] permission_mode"* ]]
  [[ "$output" != *"safety gate: --allow-bypass is required"* ]]
}

@test "T-permmode-drv.15 — claude-p: non-plan permission_mode (acceptEdits) reaches binary" {
  # Confirms pass-through works for ALL valid modes, not just plan.
  local prof="$WS/profile.json"
  _profile "$prof"
  run _run_launch claude-p "$prof" --permission-mode=acceptEdits
  [ "$status" -eq 0 ]
  grep -Fxq -- '--permission-mode' "$ARGS_FILE"
  local line_no
  line_no=$(grep -nFx -- '--permission-mode' "$ARGS_FILE" | head -1 | cut -d: -f1)
  local next_arg
  next_arg=$(sed -n "$((line_no + 1))p" "$ARGS_FILE")
  [ "$next_arg" = "acceptEdits" ]
}
