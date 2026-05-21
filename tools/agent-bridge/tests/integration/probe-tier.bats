#!/usr/bin/env bats
# T5 — `bridge probe` returns valid JSON listing all CLIs + tiers

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  export BRIDGE_BIN
}

@test "T5.1 — bridge probe exits 0" {
  run "$BRIDGE_BIN" probe
  [ "$status" -eq 0 ]
}

@test "T5.2 — bridge probe output is valid JSON" {
  run "$BRIDGE_BIN" probe
  [ "$status" -eq 0 ]
  echo "$output" | jq -e . >/dev/null
}

@test "T5.3 — bridge probe JSON has 'results' array" {
  run "$BRIDGE_BIN" probe
  [ "$status" -eq 0 ]
  count=$(echo "$output" | jq '.results | length')
  [ "$count" -ge 4 ]
}

@test "T5.4 — bridge probe lists claude-p with tier=full" {
  if ! command -v claude >/dev/null 2>&1; then
    skip "claude binary not on PATH"
  fi
  run "$BRIDGE_BIN" probe
  [ "$status" -eq 0 ]
  tier=$(echo "$output" | jq -r '.results[] | select(.cli=="claude-p") | .tier')
  [ "$tier" = "full" ]
}

@test "T5.5 — bridge probe lists claude-tmux with tier=hybrid (when tmux present)" {
  if ! command -v claude >/dev/null 2>&1 || ! command -v tmux >/dev/null 2>&1; then
    skip "claude or tmux not on PATH"
  fi
  run "$BRIDGE_BIN" probe
  [ "$status" -eq 0 ]
  tier=$(echo "$output" | jq -r '.results[] | select(.cli=="claude-tmux") | .tier')
  [ "$tier" = "hybrid" ]
}

@test "T5.6 — bridge probe lists codex with stub=false (promoted)" {
  run "$BRIDGE_BIN" probe
  [ "$status" -eq 0 ]
  stub=$(echo "$output" | jq -r '.results[] | select(.cli=="codex") | .stub')
  [ "$stub" = "false" ]
}

@test "T5.6b — bridge probe codex tier=full when codex binary present" {
  if ! command -v codex >/dev/null 2>&1; then
    skip "codex binary not on PATH"
  fi
  run "$BRIDGE_BIN" probe
  [ "$status" -eq 0 ]
  tier=$(echo "$output" | jq -r '.results[] | select(.cli=="codex") | .tier')
  [ "$tier" = "full" ]
}

@test "T5.7 — bridge probe lists agy with stub=false (promoted)" {
  run "$BRIDGE_BIN" probe
  [ "$status" -eq 0 ]
  stub=$(echo "$output" | jq -r '.results[] | select(.cli=="agy") | .stub')
  [ "$stub" = "false" ]
}

@test "T5.7b — bridge probe agy tier=full when agy binary present" {
  if ! command -v agy >/dev/null 2>&1; then
    skip "agy binary not on PATH"
  fi
  run "$BRIDGE_BIN" probe
  [ "$status" -eq 0 ]
  tier=$(echo "$output" | jq -r '.results[] | select(.cli=="agy") | .tier')
  [ "$tier" = "full" ]
}

@test "T5.8 — bridge probe --cli=claude-p returns single-CLI result" {
  run "$BRIDGE_BIN" probe --cli=claude-p
  [ "$status" -eq 0 ]
  count=$(echo "$output" | jq '.results | length')
  [ "$count" -eq 1 ]
  cli=$(echo "$output" | jq -r '.results[0].cli')
  [ "$cli" = "claude-p" ]
}

@test "T5.9 — bridge probe output includes 'os' field" {
  run "$BRIDGE_BIN" probe
  [ "$status" -eq 0 ]
  os=$(echo "$output" | jq -r '.os')
  [ -n "$os" ]
  [ "$os" != "null" ]
}
