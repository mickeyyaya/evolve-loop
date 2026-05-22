#!/usr/bin/env bats
# T-lifecycle — bridge session lifecycle tests (v0.4).
#
# Verifies two safety properties operators rely on:
#
#   Property 1 (FRESH PER LAUNCH): every bridge launch creates a new context-
#   window session. claude-p/codex/agy: new single-shot process per launch.
#   claude-tmux/codex-tmux/agy-tmux: new tmux session with unique name
#   (cycle + agent + pid + timestamp).
#
#   Property 2 (CLEANUP ON EXIT/CRASH): on normal exit, INT, or TERM the
#   driver's trap kills the tmux session. SIGKILL/OOM cannot be trapped
#   and DOES leak sessions — the orphan-sweep on next launch cleans them.
#
# Test coverage focuses on the v0.4 orphan-sweep (the new behavior).
# Trap-based cleanup is tested implicitly by existing mock-cli-drivers.bats.
#
# Coverage:
#   T-lifecycle.1   orphan-sweep removes dead-pid evolve-bridge-* session
#   T-lifecycle.2   orphan-sweep PRESERVES live-pid session
#   T-lifecycle.3   BRIDGE_NO_ORPHAN_CLEANUP=1 disables sweep
#   T-lifecycle.4   orphan-sweep IGNORES non-bridge tmux sessions
#   T-lifecycle.5   --validate-only does NOT trigger sweep
#   T-lifecycle.6   sweep logs each kill clearly

setup() {
  if ! command -v tmux >/dev/null 2>&1; then
    skip "tmux not available"
  fi
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  FAKES_DIR="${FIXTURE_DIR}/fakes"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-lifecycle-XXXXXX")"
  STDOUT_LOG="${WS}/stdout.log"
  STDERR_LOG="${WS}/stderr.log"
  ARTIFACT="${WS}/artifact.md"
  TOKEN="$(openssl rand -hex 8 2>/dev/null || date +%s | tr -d '\n')"
  PROMPT="${WS}/prompt.txt"
  PROF="${WS}/profile.json"
  cat > "$PROMPT" <<EOF
Use your Write tool to create $ARTIFACT containing:
<!-- challenge-token: $TOKEN -->
PROTOTYPE OK
EOF
  cat > "$PROF" <<JSON
{
  "name": "lifecycle-test",
  "model": "haiku",
  "allowed_tools": ["Read", "Write"],
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON
  SESSIONS_CREATED=()
  export BRIDGE_BIN FIXTURE_DIR FAKES_DIR WS STDOUT_LOG STDERR_LOG ARTIFACT TOKEN PROMPT PROF
  export BRIDGE_TESTING=1
}

teardown() {
  local ses
  for ses in "${SESSIONS_CREATED[@]:-}"; do
    [[ -n "$ses" ]] && tmux kill-session -t "$ses" 2>/dev/null || true
  done
  while IFS= read -r ses; do
    [[ -n "$ses" ]] && tmux kill-session -t "$ses" 2>/dev/null || true
  done < <(tmux ls 2>/dev/null | awk -F: '/^evolve-bridge-/ { print $1 }')
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
  unset BRIDGE_TESTING BRIDGE_CLAUDE_BINARY BRIDGE_NO_ORPHAN_CLEANUP
}

# Helper: spawn-then-reap subprocess to get a guaranteed-dead pid.
# `bash -c ':' >/dev/null 2>&1` produces zero stdout so only the function's
# own `echo $pid` reaches the caller (no double-line corruption).
_dead_pid() {
  bash -c ':' >/dev/null 2>&1 &
  local pid=$!
  wait "$pid" 2>/dev/null
  sleep 0.2
  echo "$pid"
}

# Helper: create a fake evolve-bridge-* tmux session with a controlled pid in the name
_make_fake_session() {
  local pid="$1" prefix="${2:-evolve-bridge-c99-test}"
  local ts
  ts=$(date +%s)
  local name="${prefix}-pid${pid}-${ts}"
  tmux new-session -d -s "$name" "sleep 60" 2>/dev/null
  SESSIONS_CREATED+=("$name")
  echo "$name"
}

# Helper: launch bridge against fake-claude (claude-p driver = no session created)
_run_bridge() {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$PROF" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" "$@"
}

# === T-lifecycle.1 — orphan-sweep removes dead-pid session ===
@test "T-lifecycle.1 — orphan-sweep kills evolve-bridge-* session with dead pid" {
  local dead_pid
  dead_pid=$(_dead_pid)
  ! kill -0 "$dead_pid" 2>/dev/null

  local orphan
  orphan=$(_make_fake_session "$dead_pid")
  tmux has-session -t "$orphan" 2>/dev/null

  run _run_bridge
  [ "$status" -eq 0 ]
  ! tmux has-session -t "$orphan" 2>/dev/null
  [[ "$output" == *"orphan-sweep: killing tmux session"* ]]
  [[ "$output" == *"$orphan"* ]]
}

# === T-lifecycle.2 — orphan-sweep PRESERVES live-pid session ===
@test "T-lifecycle.2 — orphan-sweep PRESERVES session whose pid is alive" {
  sleep 30 &
  local live_pid=$!
  kill -0 "$live_pid" 2>/dev/null

  local live_session
  live_session=$(_make_fake_session "$live_pid")
  tmux has-session -t "$live_session" 2>/dev/null

  run _run_bridge
  [ "$status" -eq 0 ]
  tmux has-session -t "$live_session" 2>/dev/null

  kill "$live_pid" 2>/dev/null || true
  wait "$live_pid" 2>/dev/null || true
}

# === T-lifecycle.3 — BRIDGE_NO_ORPHAN_CLEANUP=1 disables sweep ===
@test "T-lifecycle.3 — BRIDGE_NO_ORPHAN_CLEANUP=1 disables sweep" {
  local dead_pid
  dead_pid=$(_dead_pid)
  local orphan
  orphan=$(_make_fake_session "$dead_pid")
  tmux has-session -t "$orphan" 2>/dev/null

  export BRIDGE_NO_ORPHAN_CLEANUP=1
  run _run_bridge
  [ "$status" -eq 0 ]
  tmux has-session -t "$orphan" 2>/dev/null
  [[ "$output" != *"orphan-sweep"* ]]
}

# === T-lifecycle.4 — sweep IGNORES non-bridge tmux sessions ===
@test "T-lifecycle.4 — sweep ignores tmux sessions NOT prefixed evolve-bridge-* (no friendly-fire)" {
  local dead_pid
  dead_pid=$(_dead_pid)
  local ts
  ts=$(date +%s)
  local user_ses="user-session-pid${dead_pid}-${ts}"
  tmux new-session -d -s "$user_ses" "sleep 60" 2>/dev/null
  SESSIONS_CREATED+=("$user_ses")
  tmux has-session -t "$user_ses" 2>/dev/null

  run _run_bridge
  [ "$status" -eq 0 ]
  tmux has-session -t "$user_ses" 2>/dev/null
}

# === T-lifecycle.5 — --validate-only does NOT trigger sweep ===
@test "T-lifecycle.5 — --validate-only is read-only: does NOT trigger orphan sweep" {
  local dead_pid
  dead_pid=$(_dead_pid)
  local orphan
  orphan=$(_make_fake_session "$dead_pid")

  run _run_bridge --validate-only
  [ "$status" -eq 0 ]
  tmux has-session -t "$orphan" 2>/dev/null
  [[ "$output" != *"orphan-sweep"* ]]
}

# === T-lifecycle.6 — sweep logs each kill with clear identification ===
@test "T-lifecycle.6 — sweep stderr clearly identifies what was killed and why" {
  local dead_pid_a dead_pid_b
  dead_pid_a=$(_dead_pid)
  dead_pid_b=$(_dead_pid)
  local orphan_a orphan_b
  orphan_a=$(_make_fake_session "$dead_pid_a" "evolve-bridge-codex-c1-test")
  orphan_b=$(_make_fake_session "$dead_pid_b" "evolve-bridge-agy-c2-test")

  run _run_bridge
  [ "$status" -eq 0 ]
  ! tmux has-session -t "$orphan_a" 2>/dev/null
  ! tmux has-session -t "$orphan_b" 2>/dev/null
  [[ "$output" == *"$orphan_a"* ]]
  [[ "$output" == *"$orphan_b"* ]]
  [[ "$output" == *"killed 2 leaked session"* ]]
}
