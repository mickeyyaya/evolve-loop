#!/usr/bin/env bats
# T-cross-cli — every backend honors the same input/output contract
# via `bridge launch --dry-run`. No LLM cost; runs in default CI suite.

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  PROFILE="${FIXTURE_DIR}/synth-profile.json"
  PROMPT="${FIXTURE_DIR}/minimal-prompt.txt"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-tcc-XXXXXX")"
  export BRIDGE_BIN FIXTURE_DIR PROFILE PROMPT WS
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
}

# Helper: dry-run launch for any CLI; returns rc + populates workspace
dry_launch() {
  local cli="$1"
  "$BRIDGE_BIN" launch --dry-run \
    --cli="$cli" --profile="$PROFILE" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
    --artifact="$WS/artifact.md"
}

dry_launch_json() {
  local cli="$1"
  "$BRIDGE_BIN" --json launch --dry-run \
    --cli="$cli" --profile="$PROFILE" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
    --artifact="$WS/artifact.md"
}

# ---- Contract A: rc=0 + artifact + logs for every backend ----

@test "T-cross.A1 — claude-p dry-run: rc=0 + artifact + logs" {
  run dry_launch claude-p
  [ "$status" -eq 0 ]
  [ -f "$WS/artifact.md" ]
  [ -f "$WS/stdout.log" ]
  [ -f "$WS/stderr.log" ]
}

@test "T-cross.A2 — claude-tmux dry-run: rc=0 + artifact + logs" {
  run dry_launch claude-tmux
  [ "$status" -eq 0 ]
  [ -f "$WS/artifact.md" ]
}

@test "T-cross.A3 — codex dry-run: rc=0 + artifact + logs" {
  run dry_launch codex
  [ "$status" -eq 0 ]
  [ -f "$WS/artifact.md" ]
}

@test "T-cross.A4 — codex-tmux dry-run: rc=0 + artifact + logs" {
  run dry_launch codex-tmux
  [ "$status" -eq 0 ]
  [ -f "$WS/artifact.md" ]
}

@test "T-cross.A5 — agy dry-run: rc=0 + artifact + logs" {
  run dry_launch agy
  [ "$status" -eq 0 ]
  [ -f "$WS/artifact.md" ]
}

@test "T-cross.A6 — agy-tmux dry-run: rc=0 + artifact + logs" {
  run dry_launch agy-tmux
  [ "$status" -eq 0 ]
  [ -f "$WS/artifact.md" ]
}

# ---- Contract B: --json emits uniform JSON shape across all backends ----

@test "T-cross.B — all 6 backends --json launch emit verdict=complete + same JSON shape" {
  for cli in claude-p claude-tmux codex codex-tmux agy agy-tmux; do
    rm -f "$WS/artifact.md" "$WS/stdout.log" "$WS/stderr.log"
    run dry_launch_json "$cli"
    if [ "$status" -ne 0 ]; then
      echo "FAILED on cli=$cli rc=$status" >&2
      echo "$output" >&2
      return 1
    fi
    # Extract the JSON summary line(s)
    json_obj=$(echo "$output" | awk '/^\{/{flag=1} flag{print} /^\}/{flag=0; exit}')
    [ -n "$json_obj" ] || { echo "NO JSON for cli=$cli" >&2; return 1; }
    # Assert uniform shape: verdict, artifact, logs all present
    if ! echo "$json_obj" | jq -e '.verdict == "complete"' >/dev/null; then
      echo "BAD verdict for cli=$cli: $(echo "$json_obj" | jq .verdict)" >&2
      return 1
    fi
    echo "$json_obj" | jq -e '.artifact.exists and .logs.stdout_log.exists' >/dev/null
  done
}

# ---- Contract C: artifact contains the resolved cli + DRY-RUN-OK marker ----

@test "T-cross.C — all 6 backends produce artifact with cli name + DRY-RUN-OK marker" {
  for cli in claude-p claude-tmux codex codex-tmux agy agy-tmux; do
    rm -f "$WS/artifact.md"
    run dry_launch "$cli"
    [ "$status" -eq 0 ]
    grep -q "cli: $cli" "$WS/artifact.md"
    grep -q "DRY-RUN-OK" "$WS/artifact.md"
  done
}

# ---- Contract D: challenge_token written when prompt uses $CHALLENGE_TOKEN ----

@test "T-cross.D — challenge-token.txt written when prompt has \$CHALLENGE_TOKEN (all backends)" {
  PROMPT_TOKEN="$WS/prompt-token.txt"
  cat > "$PROMPT_TOKEN" <<EOF
Use Write tool to create \$ARTIFACT_PATH with:
<!-- challenge-token: \$CHALLENGE_TOKEN -->
HELLO
EOF
  for cli in claude-p claude-tmux codex codex-tmux agy agy-tmux; do
    rm -f "$WS/artifact.md" "$WS/challenge-token.txt"
    "$BRIDGE_BIN" launch --dry-run \
      --cli="$cli" --profile="$PROFILE" --model=auto \
      --prompt-file="$PROMPT_TOKEN" --workspace="$WS" \
      --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
      --artifact="$WS/artifact.md" >/dev/null 2>&1
    [ -f "$WS/challenge-token.txt" ] || { echo "no token file for $cli" >&2; return 1; }
    token=$(cat "$WS/challenge-token.txt")
    grep -q "$token" "$WS/artifact.md" || { echo "token not in artifact for $cli" >&2; return 1; }
  done
}

# ---- Contract E: bridge report verdict=complete for all 6 ----

@test "T-cross.E — bridge --json report finds verdict=complete for all backends" {
  for cli in claude-p claude-tmux codex codex-tmux agy agy-tmux; do
    rm -rf "$WS"
    mkdir -p "$WS"
    "$BRIDGE_BIN" launch --dry-run \
      --cli="$cli" --profile="$PROFILE" --model=auto \
      --prompt-file="$PROMPT" --workspace="$WS" \
      --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
      --artifact="$WS/artifact.md" >/dev/null 2>&1
    run "$BRIDGE_BIN" --json report --workspace="$WS"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.verdict == "complete"' >/dev/null
  done
}
