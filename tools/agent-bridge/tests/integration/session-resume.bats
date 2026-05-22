#!/usr/bin/env bats
# T-resume — v0.5 named-session + auto-resume tests.
#
# Coverage:
#   T-resume.1   claude-tmux + --session-name=X creates evolve-bridge-named-X
#   T-resume.2   --session-name validates against [a-zA-Z0-9._-]+ pattern
#   T-resume.3   --session-name length cap (>32 chars rejected)
#   T-resume.4   claude-tmux RESUMES existing named session (skips create + REPL boot)
#   T-resume.5   named session PRESERVED (orphan-sweep exempts it)
#   T-resume.6   codex-tmux + --session-name → bridge fails with clear error
#   T-resume.7   agy-tmux + --session-name → bridge fails with clear error
#   T-resume.8   claude-p + --session-name → driver logs NOTE, proceeds
#   T-resume.9   codex + --session-name → driver logs NOTE, proceeds
#   T-resume.10  agy + --session-name → driver logs NOTE, proceeds
#   T-resume.11  validate-only shows session-name row when set
#   T-resume.12  precedence: CLI flag > BRIDGE_SESSION_NAME env > profile.session_name

setup() {
  if ! command -v tmux >/dev/null 2>&1; then
    skip "tmux not available"
  fi
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  FAKES_DIR="${FIXTURE_DIR}/fakes"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-resume-XXXXXX")"
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
  SESSIONS_CREATED=()
  export BRIDGE_BIN FIXTURE_DIR FAKES_DIR WS STDOUT_LOG STDERR_LOG ARTIFACT TOKEN PROMPT
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
  unset BRIDGE_TESTING BRIDGE_CLAUDE_BINARY BRIDGE_CODEX_BINARY BRIDGE_AGY_BINARY \
        BRIDGE_SESSION_NAME
}

_timeout() {
  local secs="$1"; shift
  perl -e 'alarm shift @ARGV; exec @ARGV' "$secs" "$@"
}

_profile() {
  local path="$1" session_name="${2:-}"
  local base='{"name":"resume-test","model":"haiku","allowed_tools":["Read","Write"],"auto_respond":{"destructive_ops":false,"timeout_s":60},"prompt_overrides":[]}'
  if [[ -n "$session_name" ]]; then
    echo "$base" | jq --arg s "$session_name" '. + {session_name: $s}' > "$path"
  else
    echo "$base" > "$path"
  fi
}

@test "T-resume.1 — claude-tmux + --session-name=X uses session 'evolve-bridge-named-X'" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local prof="$WS/p.json"
  _profile "$prof"
  local name="t1-foo"
  run _timeout 6 "$BRIDGE_BIN" launch \
    --cli=claude-tmux --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name="$name" --allow-bypass
  [[ "$output" == *"evolve-bridge-named-$name"* ]]
  [[ "$output" == *"CREATE-NAMED"* ]] || [[ "$output" == *"RESUME"* ]]
}

@test "T-resume.2 — --session-name validates against [a-zA-Z0-9._-]+ (rejects shell metachars)" {
  local prof="$WS/p.json"
  _profile "$prof"
  run "$BRIDGE_BIN" launch \
    --cli=claude-tmux --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name='bad;name' --allow-bypass \
    --validate-only
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalid"* ]]
}

@test "T-resume.3 — --session-name >32 chars rejected" {
  local prof="$WS/p.json"
  _profile "$prof"
  local long_name
  long_name=$(printf 'a%.0s' {1..40})
  run "$BRIDGE_BIN" launch \
    --cli=claude-tmux --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name="$long_name" --allow-bypass \
    --validate-only
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalid"* ]] || [[ "$output" == *"max 32"* ]]
}

@test "T-resume.4 — claude-tmux + existing session = RESUME path (logs 'RESUME: reattaching')" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local name="t4-existing"
  local session_id="evolve-bridge-named-$name"
  tmux new-session -d -s "$session_id" "sleep 60" 2>/dev/null
  SESSIONS_CREATED+=("$session_id")
  tmux has-session -t "$session_id" 2>/dev/null

  local prof="$WS/p.json"
  _profile "$prof"
  run _timeout 6 "$BRIDGE_BIN" launch \
    --cli=claude-tmux --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name="$name" --allow-bypass
  [[ "$output" == *"RESUME: reattaching"* ]]
  [[ "$output" == *"$session_id"* ]]
}

@test "T-resume.5 — orphan-sweep EXEMPTS evolve-bridge-named-* (preserved on next launch)" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local named_session="evolve-bridge-named-t5-preserve"
  tmux new-session -d -s "$named_session" "sleep 60" 2>/dev/null
  SESSIONS_CREATED+=("$named_session")

  local prof="$WS/p.json"
  _profile "$prof"
  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT"
  [ "$status" -eq 0 ]
  tmux has-session -t "$named_session" 2>/dev/null
}

@test "T-resume.6 — codex-tmux + --session-name → rejected with clear error" {
  export BRIDGE_CODEX_BINARY="$FAKES_DIR/fake-codex.sh"
  local prof="$WS/p.json"
  _profile "$prof"
  run _timeout 6 "$BRIDGE_BIN" launch \
    --cli=codex-tmux --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name="t6-foo" --allow-bypass
  [ "$status" -ne 0 ]
  [[ "$output" == *"session-name"* ]]
  [[ "$output" == *"not supported"* ]]
}

@test "T-resume.7 — agy-tmux + --session-name → rejected with clear error" {
  export BRIDGE_AGY_BINARY="$FAKES_DIR/fake-agy.sh"
  local prof="$WS/p.json"
  _profile "$prof"
  run _timeout 6 "$BRIDGE_BIN" launch \
    --cli=agy-tmux --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name="t7-foo" --allow-bypass
  [ "$status" -ne 0 ]
  [[ "$output" == *"session-name"* ]]
  [[ "$output" == *"not supported"* ]]
}

@test "T-resume.8 — claude-p + --session-name → driver logs NOTE, proceeds successfully" {
  export BRIDGE_CLAUDE_BINARY="$FAKES_DIR/fake-claude.sh"
  local prof="$WS/p.json"
  _profile "$prof"
  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name="t8-foo"
  [ "$status" -eq 0 ]
  [[ "$output" == *"NOTE"* ]]
  [[ "$output" == *"session-name"* ]] || [[ "$output" == *"session_name"* ]]
}

@test "T-resume.9 — codex + --session-name → driver logs NOTE, proceeds" {
  export BRIDGE_CODEX_BINARY="$FAKES_DIR/fake-codex.sh"
  local prof="$WS/p.json"
  _profile "$prof"
  run "$BRIDGE_BIN" launch \
    --cli=codex --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name="t9-foo"
  [ "$status" -eq 0 ]
  [[ "$output" == *"NOTE"* ]]
  [[ "$output" == *"session-name"* ]] || [[ "$output" == *"session_name"* ]]
}

@test "T-resume.10 — agy + --session-name → driver logs NOTE, proceeds" {
  export BRIDGE_AGY_BINARY="$FAKES_DIR/fake-agy.sh"
  local prof="$WS/p.json"
  _profile "$prof"
  run "$BRIDGE_BIN" launch \
    --cli=agy --profile="$prof" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name="t10-foo"
  [ "$status" -eq 0 ]
  [[ "$output" == *"NOTE"* ]]
  [[ "$output" == *"session-name"* ]] || [[ "$output" == *"session_name"* ]]
}

@test "T-resume.11 — validate-only output shows session-name row when set" {
  local prof="$WS/p.json"
  _profile "$prof"
  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name="t11-display" \
    --validate-only
  [ "$status" -eq 0 ]
  [[ "$output" == *"session-name"* ]]
  [[ "$output" == *"t11-display"* ]]
}

@test "T-resume.12 — precedence: CLI flag wins over env wins over profile.session_name" {
  local prof="$WS/p.json"
  _profile "$prof" "from-profile"
  export BRIDGE_SESSION_NAME="from-env"
  run "$BRIDGE_BIN" launch \
    --cli=claude-p --profile="$prof" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$STDOUT_LOG" --stderr-log="$STDERR_LOG" \
    --artifact="$ARTIFACT" \
    --session-name="from-flag" \
    --validate-only
  [ "$status" -eq 0 ]
  [[ "$output" == *"from-flag"* ]]
  [[ "$output" != *"from-env"* ]]
  [[ "$output" != *"from-profile"* ]]
}
