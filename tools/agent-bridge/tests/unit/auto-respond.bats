#!/usr/bin/env bats
# T13 — auto_respond_decide pattern matching (uses claude-tmux manifest)

setup() {
  BRIDGE_LIB_DIR="${BATS_TEST_DIRNAME}/../../lib"
  export BRIDGE_LIB_DIR
  source "${BRIDGE_LIB_DIR}/manifest-loader.sh"
  source "${BRIDGE_LIB_DIR}/auto-respond.sh"
  manifest_load claude-tmux
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-t13-XXXXXX")"
  export WS
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
}

@test "T13.1 — empty pane → noop rc=0" {
  run auto_respond_decide "" "$bridge_manifest_path" "$WS"
  [ "$status" -eq 0 ]
  [ "$output" = "noop" ]
}

@test "T13.2 — normal claude output → noop rc=0" {
  pane="❯ Working on your request...
Reading file...
Writing artifact..."
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  [ "$status" -eq 0 ]
  [ "$output" = "noop" ]
}

@test "T13.3 — 'Please log in' → escalate:auth_recheck rc=85" {
  pane="Authentication required. Please log in again to continue."
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  [ "$status" -eq 85 ]
  [ "$output" = "escalate:auth_recheck" ]
}

@test "T13.4 — 'rate limit exceeded' → escalate:rate_limit rc=85" {
  pane="ERROR: rate limit exceeded, please retry in 60s"
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  [ "$status" -eq 85 ]
  [ "$output" = "escalate:rate_limit" ]
}

@test "T13.5 — 'model is deprecated...Continue?' → send:y,Enter rc=1" {
  pane="Note: this model is deprecated. Continue with the deprecated model? [y/n]"
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  [ "$status" -eq 1 ]
  [ "$output" = "send:y,Enter" ]
}

@test "T13.6 — same pattern 6× → loop_guard rc=86" {
  pane="Please log in again."
  for i in 1 2 3 4 5; do
    run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
    [ "$status" -eq 85 ]
  done
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  [ "$status" -eq 86 ]
  [[ "$output" =~ ^loop_guard: ]]
}

@test "T13.7 — counts file is written and incremented" {
  pane="Please log in"
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  [ -f "$WS/auto-respond-counts.csv" ]
  count=$(awk -F, '$1=="auth_recheck"{print $2}' "$WS/auto-respond-counts.csv")
  [ "$count" -eq 2 ]
}

@test "T13.8 — terminal_resize prompt → send:Enter rc=1" {
  pane="Terminal too small. Please resize and press enter."
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  [ "$status" -eq 1 ]
  [ "$output" = "send:Enter" ]
}

@test "T13.9 — escalation report has the expected schema fields" {
  pane="rate limit exceeded"
  workspace="$WS"
  bridge_manifest_path="$bridge_manifest_path"
  bridge_manifest_cli="$bridge_manifest_cli"
  export workspace bridge_manifest_path bridge_manifest_cli
  auto_respond_write_escalation_report "$WS" "$pane" "rate_limit" "fake-session" "escalate"
  [ -f "$WS/escalation-report.json" ]
  jq -e '.schema_version == 1' "$WS/escalation-report.json"
  jq -e '.pattern_name == "rate_limit"' "$WS/escalation-report.json"
  jq -e '.suggested_rule_template != null' "$WS/escalation-report.json"
}

@test "T13.10 — fact_forcing_gate text in pane → noop rc=0 (rule removed; claude recovers natively)" {
  # v12.1.5: the fact_forcing_gate rule was REMOVED after live-smoke forensics
  # showed that Claude handles the ECC gateguard hook natively (presents facts,
  # retries Write). A bridge auto-respond rule actively interferes by stacking
  # identical messages at the prompt every poll-tick. The bridge stays out of
  # the way. See manifests/claude-tmux.json::_notes_on_removed_patterns.
  pane="◇ Fact-Forcing Gate: please present these facts before writing"
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  [ "$status" -eq 0 ]
  [ "$output" = "noop" ]
}

@test "T13.11 — fact_forcing_gate alt regex (investigate before) → noop rc=0 (rule removed)" {
  pane="The hook requires you to investigate before writing any file."
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  [ "$status" -eq 0 ]
  [ "$output" = "noop" ]
}

@test "T13.12 — extend_timeout with non-integer keys → escalates" {
  # Synthesize a manifest with a malformed extend_timeout (response_keys not integer).
  local bad_manifest="$WS/bad-extend.json"
  cat > "$bad_manifest" <<EOF
{
  "schema_version": 1,
  "cli": "synthetic",
  "interactive_prompts": [
    {
      "name": "bad_extend",
      "regex": "TRIGGER",
      "response_keys": "not-a-number",
      "policy": "extend_timeout"
    }
  ]
}
EOF
  run auto_respond_decide "TRIGGER stuck here" "$bad_manifest" "$WS"
  [ "$status" -eq 85 ]
  [ "$output" = "escalate:bad_extend" ]
}
