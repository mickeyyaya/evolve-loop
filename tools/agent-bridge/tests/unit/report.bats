#!/usr/bin/env bats
# T-report — `bridge report --workspace=DIR` emits structured JSON

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  PROFILE="${FIXTURE_DIR}/synth-profile.json"
  PROMPT="${FIXTURE_DIR}/minimal-prompt.txt"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-tr-XXXXXX")"
  export BRIDGE_BIN FIXTURE_DIR PROFILE PROMPT WS

  # Populate the workspace via a dry-run so we have real bridge-written files
  "$BRIDGE_BIN" launch --dry-run \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
    --artifact="$WS/artifact.md" >/dev/null 2>&1
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
}

@test "T-report.1 — bridge report on a complete workspace → rc=0, verdict=complete" {
  run "$BRIDGE_BIN" report --workspace="$WS"
  [ "$status" -eq 0 ]
  verdict=$(echo "$output" | jq -r '.verdict')
  [ "$verdict" = "complete" ]
}

@test "T-report.2 — bridge report output is valid JSON" {
  run "$BRIDGE_BIN" report --workspace="$WS"
  echo "$output" | jq -e . >/dev/null
}

@test "T-report.3 — report identifies artifact size_bytes > 0" {
  run "$BRIDGE_BIN" report --workspace="$WS"
  size=$(echo "$output" | jq -r '.artifact.size_bytes')
  [ "$size" -gt 0 ]
}

@test "T-report.4 — report has_challenge_token=true when token used" {
  PROMPT_WITH_TOKEN="$WS/prompt-token.txt"
  cat > "$PROMPT_WITH_TOKEN" <<EOF
Token: \$CHALLENGE_TOKEN
Write to \$ARTIFACT_PATH
EOF
  WS2=$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-tr2-XXXXXX")
  "$BRIDGE_BIN" launch --dry-run \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT_WITH_TOKEN" --workspace="$WS2" \
    --stdout-log="$WS2/stdout.log" --stderr-log="$WS2/stderr.log" \
    --artifact="$WS2/artifact.md" >/dev/null 2>&1
  run "$BRIDGE_BIN" report --workspace="$WS2"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.artifact.has_challenge_token == true' >/dev/null
  token=$(echo "$output" | jq -r '.challenge_token')
  [ -n "$token" ]
  [ "$token" != "null" ]
  rm -rf "$WS2"
}

@test "T-report.5 — bridge report missing --workspace → rc=10" {
  run "$BRIDGE_BIN" report
  [ "$status" -eq 10 ]
}

@test "T-report.6 — bridge report on nonexistent workspace → rc=10" {
  run "$BRIDGE_BIN" report --workspace=/tmp/does-not-exist-XXXX-$$
  [ "$status" -eq 10 ]
}

@test "T-report.7 — bridge report verdict=escalated when escalation-report.json present" {
  echo '{"schema_version":1,"pattern_name":"test"}' > "$WS/escalation-report.json"
  run "$BRIDGE_BIN" report --workspace="$WS"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.verdict == "escalated"' >/dev/null
  echo "$output" | jq -e '.escalation_report.exists == true' >/dev/null
}

@test "T-report.8 — bridge report verdict=incomplete when artifact missing" {
  rm -f "$WS/artifact.md"
  run "$BRIDGE_BIN" report --workspace="$WS"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.verdict == "incomplete"' >/dev/null
  echo "$output" | jq -e '.artifact.exists == false' >/dev/null
}

@test "T-report.9 — bridge report --help exits 0" {
  run "$BRIDGE_BIN" report --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"workspace"* ]]
}

@test "T-report.10 — bridge report --artifact-name=custom.md respects custom name" {
  mv "$WS/artifact.md" "$WS/custom.md"
  run "$BRIDGE_BIN" report --workspace="$WS" --artifact-name=custom.md
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.artifact.exists == true' >/dev/null
  echo "$output" | jq -e '.artifact.path | test("custom.md$")' >/dev/null
}
