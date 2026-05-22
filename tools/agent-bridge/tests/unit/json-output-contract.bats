#!/usr/bin/env bats
# T-json-contract — every subcommand under `bridge --json` emits valid JSON.

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  PROFILE="${FIXTURE_DIR}/synth-profile.json"
  PROMPT="${FIXTURE_DIR}/minimal-prompt.txt"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-tjc-XXXXXX")"
  export BRIDGE_BIN FIXTURE_DIR PROFILE PROMPT WS
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
}

# Helper: run any bridge subcommand under --json and assert stdout is JSON.
assert_json_stdout() {
  "$@" 2>/dev/null | jq -e . >/dev/null
}

@test "T-json.1 — bridge --json probe emits valid JSON" {
  run "$BRIDGE_BIN" --json probe
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.results | type == "array"' >/dev/null
  echo "$output" | jq -e '.os' >/dev/null
}

@test "T-json.2 — bridge --json doctor emits valid JSON" {
  run "$BRIDGE_BIN" --json doctor --cli=claude-p
  echo "$output" | jq -e '.scanned_at and .results and .summary' >/dev/null
}

@test "T-json.3 — bridge --json version emits {version, schema_version}" {
  run "$BRIDGE_BIN" --json version
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.version and .schema_version == 1' >/dev/null
}

@test "T-json.4 — bridge --json selftest --filter=exit-codes emits totals + tests[]" {
  run "$BRIDGE_BIN" --json selftest --filter=exit-codes
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.totals and .tests' >/dev/null
}

@test "T-json.5 — bridge --json launch --dry-run emits report-shaped JSON summary" {
  run "$BRIDGE_BIN" --json launch --dry-run \
    --cli=claude-tmux --profile="$PROFILE" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
    --artifact="$WS/artifact.md"
  [ "$status" -eq 0 ]
  # The summary line is the LAST line of stdout (preceding bridge messages may be in stderr)
  json_line=$(echo "$output" | grep -E '^\{' | head -1)
  [ -n "$json_line" ]
  echo "$json_line" | jq -e '.verdict and .artifact and .logs' >/dev/null
}

@test "T-json.6 — bridge --json report --workspace=DIR emits report JSON" {
  # First create a workspace via dry-run
  "$BRIDGE_BIN" launch --dry-run \
    --cli=claude-tmux --profile="$PROFILE" --model=auto \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
    --artifact="$WS/artifact.md" >/dev/null 2>&1
  run "$BRIDGE_BIN" --json report --workspace="$WS"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.verdict == "complete"' >/dev/null
  echo "$output" | jq -e '.artifact.exists == true' >/dev/null
}

@test "T-json.7 — bridge --json add-rule emits status JSON (sandboxed)" {
  TEST_LIB=$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-tjc-lib-XXXXXX")
  mkdir -p "$TEST_LIB/manifests"
  cp "${BATS_TEST_DIRNAME}/../../lib/manifests/claude-tmux.json" "$TEST_LIB/manifests/"
  cp "${BATS_TEST_DIRNAME}/../../lib/"*.sh "$TEST_LIB/" 2>/dev/null || true
  BRIDGE_LIB_DIR="$TEST_LIB" run "$BRIDGE_BIN" --json add-rule \
    --cli=claude-tmux --regex='T-json sandbox' \
    --response='y,Enter' --policy=auto_respond --name=t_json_sandbox_rule
  [ "$status" -eq 0 ]
  # Extract the JSON block from mixed stdout/stderr output
  json_obj=$(echo "$output" | awk '/^\{/{flag=1} flag{print} /^\}/{flag=0; exit}')
  echo "$json_obj" | jq -e '.status == "appended" and .name == "t_json_sandbox_rule"' >/dev/null
  rm -rf "$TEST_LIB"
}

@test "T-json.8 — bridge --json on unknown subcommand → rc=10" {
  run "$BRIDGE_BIN" --json not-a-subcommand
  [ "$status" -eq 10 ]
}

@test "T-json.9 — --json after subcommand is treated as unknown flag (top-level only)" {
  # --json AFTER subcommand should NOT be honored by main()'s parser
  # (it's a top-level flag, not per-subcommand)
  run "$BRIDGE_BIN" doctor --json --cli=claude-p
  # doctor's own parser doesn't know --json (only main() does); this should rc=10
  [ "$status" -eq 10 ]
}
