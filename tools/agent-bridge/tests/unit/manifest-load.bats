#!/usr/bin/env bats
# T4 — per-CLI manifest load + validation

setup() {
  BRIDGE_LIB_DIR="${BATS_TEST_DIRNAME}/../../lib"
  export BRIDGE_LIB_DIR
  source "${BRIDGE_LIB_DIR}/manifest-loader.sh"
}

@test "T4.1 — manifest claude-tmux loads with cli=claude-tmux, binary=claude" {
  manifest_load claude-tmux
  [ "$bridge_manifest_cli" = "claude-tmux" ]
  [ "$bridge_manifest_binary" = "claude" ]
}

@test "T4.2 — manifest claude-p loads with empty prompt_marker (no TUI)" {
  manifest_load claude-p
  [ "$bridge_manifest_cli" = "claude-p" ]
  [ "$bridge_manifest_prompt_marker" = "" ]
}

@test "T4.3 — manifest claude-tmux exposes prompt_marker=❯" {
  manifest_load claude-tmux
  [ "$bridge_manifest_prompt_marker" = "❯" ]
}

@test "T4.4 — manifest claude-tmux exposes interactive_prompts (>0)" {
  manifest_load claude-tmux
  [ "$bridge_manifest_interactive_prompts_count" -gt 0 ]
}

@test "T4.5 — manifest codex marked stub=true (v2 deferred)" {
  manifest_load codex
  [ "$bridge_manifest_stub" = "true" ]
}

@test "T4.6 — manifest agy marked stub=true (v2 deferred)" {
  manifest_load agy
  [ "$bridge_manifest_stub" = "true" ]
}

@test "T4.7 — unknown cli → manifest_load returns 1" {
  run manifest_load not-a-cli
  [ "$status" -eq 1 ]
}

@test "T4.8 — empty cli arg → manifest_load returns 1" {
  run manifest_load ""
  [ "$status" -eq 1 ]
}

@test "T4.9 — claude-tmux default_args includes --dangerously-skip-permissions" {
  manifest_load claude-tmux
  args_csv=$(jq -r '.default_args | join(",")' "$bridge_manifest_path")
  [[ "$args_csv" == *"--dangerously-skip-permissions"* ]]
}

@test "T4.10 — claude-tmux interactive_prompts has named entries" {
  manifest_load claude-tmux
  names=$(jq -r '.interactive_prompts[].name' "$bridge_manifest_path" | tr '\n' ' ')
  [[ "$names" == *"auth_recheck"* ]]
  [[ "$names" == *"rate_limit"* ]]
}
