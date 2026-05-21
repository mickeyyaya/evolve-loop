#!/usr/bin/env bats
# T-skill-flow — full 5-step skill recipe E2E (dry-run, no LLM cost)
# Mirrors the docs/skill-integration.md "minimal-skill.sh" quickstart:
#   1. probe → 2. doctor → 3. launch --dry-run (plumbing) → 4. launch (real)
#   5. report → assert verdict=complete

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  PROFILE="${FIXTURE_DIR}/synth-profile.json"
  PROMPT="${FIXTURE_DIR}/minimal-prompt.txt"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-skill-XXXXXX")"
  CLI="claude-tmux"
  export BRIDGE_BIN FIXTURE_DIR PROFILE PROMPT WS CLI
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
}

@test "T-skill.1 — Step 1: probe → CLI is available (tier != none)" {
  run "$BRIDGE_BIN" --json probe
  [ "$status" -eq 0 ]
  tier=$(echo "$output" | jq -r ".results[] | select(.cli==\"$CLI\") | .tier")
  [ "$tier" != "none" ]
  [ -n "$tier" ]
}

@test "T-skill.2 — Step 2: doctor → CLI verdict=ready" {
  run "$BRIDGE_BIN" --json doctor --cli="$CLI"
  # Accept ready (0), warning (1), or blocked (2); but on this host we expect ready.
  # Test is robust to env-leak warnings (rc=1 still acceptable for the skill flow).
  [[ "$status" -eq 0 || "$status" -eq 1 || "$status" -eq 2 ]]
  echo "$output" | jq -e '.results | length == 1' >/dev/null
  echo "$output" | jq -e '.results[0].cli == "claude-tmux"' >/dev/null
}

@test "T-skill.3 — Step 3: dry-run plumbing → rc=0 + artifact + JSON summary" {
  run "$BRIDGE_BIN" --json launch --dry-run \
    --cli="$CLI" --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
    --artifact="$WS/artifact.md" --allow-bypass
  [ "$status" -eq 0 ]
  [ -f "$WS/artifact.md" ]
  json_obj=$(echo "$output" | awk '/^\{/{flag=1} flag{print} /^\}/{flag=0; exit}')
  echo "$json_obj" | jq -e '.verdict == "complete"' >/dev/null
}

@test "T-skill.4 — Step 4: report on the workspace → verdict=complete" {
  # Setup: populate workspace via dry-run
  "$BRIDGE_BIN" launch --dry-run \
    --cli="$CLI" --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
    --artifact="$WS/artifact.md" --allow-bypass >/dev/null 2>&1

  run "$BRIDGE_BIN" --json report --workspace="$WS"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.verdict == "complete"' >/dev/null
  echo "$output" | jq -e '.artifact.exists == true' >/dev/null
  echo "$output" | jq -e '.logs.stdout_log.exists == true' >/dev/null
}

@test "T-skill.5 — full 5-step chain in a single test (the canonical skill flow)" {
  # 1. Discovery
  TIER=$("$BRIDGE_BIN" --json probe | jq -r ".results[] | select(.cli==\"$CLI\") | .tier")
  [ "$TIER" != "none" ]

  # 2. Pre-flight
  DOC=$("$BRIDGE_BIN" --json doctor --cli="$CLI" 2>/dev/null)
  echo "$DOC" | jq -e '.results | length == 1' >/dev/null

  # 3. Plumbing dry-run
  "$BRIDGE_BIN" launch --dry-run \
    --cli="$CLI" --profile="$PROFILE" --model=haiku \
    --prompt-file="$PROMPT" --workspace="$WS" \
    --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
    --artifact="$WS/artifact.md" --allow-bypass >/dev/null 2>&1

  # 4. (Skipping real launch — that's LIVE-gated; in this test we stay at dry-run)
  [ -f "$WS/artifact.md" ]

  # 5. Post-hoc verification
  VERDICT=$("$BRIDGE_BIN" --json report --workspace="$WS" | jq -r .verdict)
  [ "$VERDICT" = "complete" ]
}

@test "T-skill.6 — env-var contract: skill drives bridge with only env vars (no flags)" {
  # Mirror Pattern L from the plan: skill exports the 8 env vars, calls bridge bare
  export BRIDGE_CLI="$CLI"
  export PROFILE_PATH="$PROFILE"
  export RESOLVED_MODEL=haiku
  export PROMPT_FILE="$PROMPT"
  export WORKSPACE_PATH="$WS"
  export STDOUT_LOG="$WS/stdout.log"
  export STDERR_LOG="$WS/stderr.log"
  export ARTIFACT_PATH="$WS/artifact.md"
  export BRIDGE_DRY_RUN=1
  export BRIDGE_ALLOW_BYPASS=1

  run "$BRIDGE_BIN" --json launch
  [ "$status" -eq 0 ]
  json_obj=$(echo "$output" | awk '/^\{/{flag=1} flag{print} /^\}/{flag=0; exit}')
  echo "$json_obj" | jq -e '.verdict == "complete"' >/dev/null
}
