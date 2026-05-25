#!/usr/bin/env bats
# T16 — `bridge add-rule` appends interactive_prompts entry to manifest

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  BRIDGE_LIB_DIR="${BATS_TEST_DIRNAME}/../../lib"
  export BRIDGE_BIN BRIDGE_LIB_DIR

  # Copy manifests to a temp dir so tests don't mutate the real ones
  TEST_LIB_DIR="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-t16-XXXXXX")"
  mkdir -p "$TEST_LIB_DIR/manifests"
  cp "${BRIDGE_LIB_DIR}/manifests/"*.json "$TEST_LIB_DIR/manifests/"
  cp "${BRIDGE_LIB_DIR}/profile.sh" "$TEST_LIB_DIR/"
  cp "${BRIDGE_LIB_DIR}/manifest-loader.sh" "$TEST_LIB_DIR/"
  cp "${BRIDGE_LIB_DIR}/probe.sh" "$TEST_LIB_DIR/"
  cp "${BRIDGE_LIB_DIR}/auto-respond.sh" "$TEST_LIB_DIR/"
  cp "${BRIDGE_LIB_DIR}/manifest-patcher.sh" "$TEST_LIB_DIR/"

  # bridge resolves BRIDGE_LIB_DIR from script location, so we need to invoke
  # bin/bridge with BRIDGE_LIB_DIR override.
  export BRIDGE_LIB_DIR="$TEST_LIB_DIR"
}

teardown() {
  [[ -n "${TEST_LIB_DIR:-}" && -d "${TEST_LIB_DIR}" ]] && rm -rf "${TEST_LIB_DIR}"
}

# Helper: invoke bridge add-rule with explicit BRIDGE_LIB_DIR (test sandbox)
bridge_add_rule() {
  BRIDGE_LIB_DIR="$TEST_LIB_DIR" "$BRIDGE_BIN" add-rule "$@"
}

@test "T16.1 — bridge add-rule with all flags → rc=0, rule appended" {
  run bridge_add_rule \
    --cli=claude-tmux \
    --name=test_rule_1 \
    --regex='Test prompt' \
    --response='Enter' \
    --policy=auto_respond \
    --note='unit test'
  [ "$status" -eq 0 ]
  # Verify the entry is in the manifest. Count = baseline patterns in the
  # claude-tmux manifest + 1 (the rule just added). Use a relative assertion
  # so adding/removing baseline patterns in lib/manifests/claude-tmux.json
  # doesn't break this test.
  count_after=$(jq -r '.interactive_prompts | length' "$TEST_LIB_DIR/manifests/claude-tmux.json")
  baseline=$(jq -r '.interactive_prompts | length' "${BATS_TEST_DIRNAME}/../../lib/manifests/claude-tmux.json")
  [ "$count_after" -eq $((baseline + 1)) ]
  names=$(jq -r '.interactive_prompts[].name' "$TEST_LIB_DIR/manifests/claude-tmux.json" | tr '\n' ' ')
  [[ "$names" == *"test_rule_1"* ]]
}

@test "T16.2 — duplicate name returns non-zero" {
  bridge_add_rule \
    --cli=claude-tmux --name=dup_rule --regex='X' --response='Enter' --policy=auto_respond
  run bridge_add_rule \
    --cli=claude-tmux --name=dup_rule --regex='Y' --response='Enter' --policy=auto_respond
  [ "$status" -ne 0 ]
}

@test "T16.3 — missing --regex → rc=10" {
  run bridge_add_rule --cli=claude-tmux --name=no_regex --response='Enter'
  [ "$status" -eq 10 ]
}

@test "T16.4 — missing --cli (and no --escalation) → rc=10" {
  run bridge_add_rule --name=no_cli --regex='X' --response='Enter'
  [ "$status" -eq 10 ]
}

@test "T16.5 — policy=auto_respond without response_keys → fails" {
  run bridge_add_rule --cli=claude-tmux --name=no_keys --regex='X' --policy=auto_respond
  [ "$status" -ne 0 ]
}

@test "T16.6 — --escalation pulls cli from report" {
  # Create a synthetic escalation report
  ESC="$TEST_LIB_DIR/synth-escalation.json"
  cat > "$ESC" <<EOF
{
  "schema_version": 1,
  "captured_at": "2026-05-21T00:00:00Z",
  "cli": "claude-tmux",
  "pattern_name": "unknown_test_prompt",
  "reason": "escalate",
  "session": "test",
  "pane_tail": "Test: continue? [y/n]"
}
EOF
  run bridge_add_rule \
    --escalation="$ESC" \
    --name=from_escalation \
    --regex='Test: continue\?' \
    --response='y,Enter' \
    --policy=auto_respond
  [ "$status" -eq 0 ]
  names=$(jq -r '.interactive_prompts[].name' "$TEST_LIB_DIR/manifests/claude-tmux.json" | tr '\n' ' ')
  [[ "$names" == *"from_escalation"* ]]
}

@test "T16.7 — auto-generated name when --name omitted" {
  run bridge_add_rule \
    --cli=claude-tmux \
    --regex='AutoGenName: continue' \
    --response='y,Enter' \
    --policy=auto_respond
  [ "$status" -eq 0 ]
  # Auto-generated names start with claude_tmux_rule_
  names=$(jq -r '.interactive_prompts[].name' "$TEST_LIB_DIR/manifests/claude-tmux.json" | tr '\n' ' ')
  [[ "$names" == *"claude_tmux_rule_"* ]]
}
