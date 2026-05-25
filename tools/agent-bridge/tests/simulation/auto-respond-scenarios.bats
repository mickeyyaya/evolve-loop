#!/usr/bin/env bats
# auto-respond-scenarios.bats — per-CLI × per-pattern coverage matrix
#
# Drives auto_respond_decide() against each manifest pattern using
# fake-cli-prompt-emitter.sh to synthesize pane content. No tmux, no live
# LLM; runs in ~1s, deterministic, suitable for release-preflight gate.
#
# Coverage matrix (printed to stderr after run):
#   claude-tmux: auth_recheck, rate_limit, model_deprecation_continue,
#                terminal_resize_redraw (4 active patterns) +
#                fact_forcing_gate (REMOVED v12.1.5; expects noop)
#   codex-tmux:  trust_prompt, auth_recheck, rate_limit (3 patterns)
#   agy:         auth_recheck, rate_limit, quota_exhausted, permission_prompt (4)
#   agy-tmux:    trust_prompt, auth_recheck, rate_limit, quota_exhausted,
#                permission_prompt (5 patterns)

setup() {
  BRIDGE_LIB_DIR="${BATS_TEST_DIRNAME}/../../lib"
  EMITTER="${BATS_TEST_DIRNAME}/fake-cli-prompt-emitter.sh"
  export BRIDGE_LIB_DIR EMITTER
  source "${BRIDGE_LIB_DIR}/manifest-loader.sh"
  source "${BRIDGE_LIB_DIR}/auto-respond.sh"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-sim-XXXXXX")"
  export WS
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
}

# Test helper: load manifest, emit pane for pattern, run auto_respond_decide,
# assert expected status + output. Wraps the common 4-line pattern.
assert_decision() {
  local cli="$1" emit_pattern="$2" expected_status="$3" expected_output="$4"
  manifest_load "$cli"
  local pane
  pane=$(bash "$EMITTER" --emit-pattern="$emit_pattern")
  run auto_respond_decide "$pane" "$bridge_manifest_path" "$WS"
  if [ "$status" -ne "$expected_status" ] || [ "$output" != "$expected_output" ]; then
    echo "cli=$cli pattern=$emit_pattern" >&2
    echo "  expected: status=$expected_status output=$expected_output" >&2
    echo "  actual:   status=$status output=$output" >&2
  fi
  [ "$status" -eq "$expected_status" ]
  [ "$output" = "$expected_output" ]
}

# --- claude-tmux ---------------------------------------------------------

@test "SIM[claude-tmux] auth_recheck → escalate" {
  assert_decision claude-tmux auth_recheck 85 "escalate:auth_recheck"
}

@test "SIM[claude-tmux] rate_limit → escalate" {
  assert_decision claude-tmux rate_limit 85 "escalate:rate_limit"
}

@test "SIM[claude-tmux] model_deprecation_continue → send" {
  assert_decision claude-tmux model_deprecation_continue 1 "send:y,Enter"
}

@test "SIM[claude-tmux] terminal_resize_redraw → send Enter" {
  assert_decision claude-tmux terminal_resize_redraw 1 "send:Enter"
}

@test "SIM[claude-tmux] fact_forcing_gate → noop (rule removed v12.1.5)" {
  # v12.1.5: rule REMOVED. See manifests/claude-tmux.json::_notes_on_removed_patterns
  # and tests/unit/auto-respond.bats::T13.10 for the live-smoke forensics that
  # justified removal. Bridge stays out of Claude's native gateguard recovery.
  assert_decision claude-tmux fact_forcing_gate 0 "noop"
}

# --- codex-tmux ----------------------------------------------------------

@test "SIM[codex-tmux] trust_prompt → send 1,Enter" {
  assert_decision codex-tmux trust_prompt_codex 1 "send:1,Enter"
}

@test "SIM[codex-tmux] auth_recheck → escalate" {
  assert_decision codex-tmux auth_recheck 85 "escalate:auth_recheck"
}

@test "SIM[codex-tmux] rate_limit → escalate" {
  assert_decision codex-tmux rate_limit 85 "escalate:rate_limit"
}

# --- agy (headless) ------------------------------------------------------

@test "SIM[agy] auth_recheck → escalate" {
  assert_decision agy auth_recheck 85 "escalate:auth_recheck"
}

@test "SIM[agy] rate_limit → escalate" {
  assert_decision agy rate_limit 85 "escalate:rate_limit"
}

@test "SIM[agy] quota_exhausted → escalate" {
  assert_decision agy quota_exhausted 85 "escalate:quota_exhausted"
}

@test "SIM[agy] permission_prompt → send y,Enter" {
  assert_decision agy permission_prompt 1 "send:y,Enter"
}

# --- agy-tmux ------------------------------------------------------------

@test "SIM[agy-tmux] trust_prompt → send Enter" {
  assert_decision agy-tmux trust_prompt_agy 1 "send:Enter"
}

@test "SIM[agy-tmux] auth_recheck → escalate" {
  assert_decision agy-tmux auth_recheck 85 "escalate:auth_recheck"
}

@test "SIM[agy-tmux] rate_limit → escalate" {
  assert_decision agy-tmux rate_limit 85 "escalate:rate_limit"
}

@test "SIM[agy-tmux] quota_exhausted → escalate" {
  assert_decision agy-tmux quota_exhausted 85 "escalate:quota_exhausted"
}

@test "SIM[agy-tmux] permission_prompt → send y,Enter" {
  assert_decision agy-tmux permission_prompt 1 "send:y,Enter"
}

# --- Coverage matrix print ----------------------------------------------

@test "SIM[coverage] matrix printed" {
  # Always-pass test that emits the coverage summary on stderr so operators
  # see the matrix at-a-glance after bats output. Counts derived from this
  # file's @test blocks above; update when adding rows.
  echo "[coverage] claude-tmux 5/5 (4 active + 1 noop-for-removed), codex-tmux 3/3, agy 4/4, agy-tmux 5/5 (total 17/17)" >&2
  true
}
