#!/usr/bin/env bats
# T-envvars — bridge launch accepts env vars as fallback for required flags
# (drop-in for evolve-loop adapter contract)

setup() {
  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  FIXTURE_DIR="${BATS_TEST_DIRNAME}/../fixtures"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-tev-XXXXXX")"
  export BRIDGE_BIN FIXTURE_DIR WS
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
  # Unset to avoid leaking between tests
  unset BRIDGE_CLI PROFILE_PATH RESOLVED_MODEL PROMPT_FILE \
        WORKSPACE_PATH STDOUT_LOG STDERR_LOG ARTIFACT_PATH \
        CYCLE WORKTREE_PATH AGENT VALIDATE_ONLY BRIDGE_DRY_RUN
}

@test "T-envvars.1 — all 8 required as env vars (no flags) → rc=0 via --dry-run" {
  export BRIDGE_CLI=claude-tmux
  export PROFILE_PATH="${FIXTURE_DIR}/synth-profile.json"
  export RESOLVED_MODEL=haiku
  export PROMPT_FILE="${FIXTURE_DIR}/minimal-prompt.txt"
  export WORKSPACE_PATH="$WS"
  export STDOUT_LOG="$WS/stdout.log"
  export STDERR_LOG="$WS/stderr.log"
  export ARTIFACT_PATH="$WS/artifact.md"
  export BRIDGE_DRY_RUN=1
  run "$BRIDGE_BIN" launch
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT_PATH" ]
}

@test "T-envvars.2 — flag overrides env var (flag wins)" {
  export BRIDGE_CLI=claude-tmux       # env says claude-tmux
  export PROFILE_PATH="${FIXTURE_DIR}/synth-profile.json"
  export RESOLVED_MODEL=haiku
  export PROMPT_FILE="${FIXTURE_DIR}/minimal-prompt.txt"
  export WORKSPACE_PATH="$WS"
  export STDOUT_LOG="$WS/stdout.log"
  export STDERR_LOG="$WS/stderr.log"
  export ARTIFACT_PATH="$WS/artifact.md"
  export BRIDGE_DRY_RUN=1
  # Flag says claude-p (different from env's claude-tmux)
  run "$BRIDGE_BIN" launch --cli=claude-p
  [ "$status" -eq 0 ]
  grep -q 'cli: claude-p' "$ARTIFACT_PATH"
}

@test "T-envvars.3 — missing env vars AND missing flags → rc=10" {
  run "$BRIDGE_BIN" launch
  [ "$status" -eq 10 ]
}

@test "T-envvars.4 — VALIDATE_ONLY=1 env var works" {
  export BRIDGE_CLI=claude-tmux
  export PROFILE_PATH="${FIXTURE_DIR}/synth-profile.json"
  export RESOLVED_MODEL=haiku
  export PROMPT_FILE="${FIXTURE_DIR}/minimal-prompt.txt"
  export WORKSPACE_PATH="$WS"
  export STDOUT_LOG="$WS/stdout.log"
  export STDERR_LOG="$WS/stderr.log"
  export ARTIFACT_PATH="$WS/artifact.md"
  export VALIDATE_ONLY=1
  run "$BRIDGE_BIN" launch
  [ "$status" -eq 0 ]
  [[ "$output" == *"validate-only"* ]]
}

@test "T-envvars.5 — partial env (4 missing) → rc=10 with the right error" {
  export BRIDGE_CLI=claude-tmux
  export PROFILE_PATH="${FIXTURE_DIR}/synth-profile.json"
  export RESOLVED_MODEL=haiku
  export PROMPT_FILE="${FIXTURE_DIR}/minimal-prompt.txt"
  # missing WORKSPACE_PATH, STDOUT_LOG, STDERR_LOG, ARTIFACT_PATH
  run "$BRIDGE_BIN" launch
  [ "$status" -eq 10 ]
  [[ "$output" == *"missing required"* ]]
}

@test "T-envvars.6 — evolve-loop adapter contract: full env contract works" {
  # Matches scripts/cli_adapters/claude.sh:24-30 contract verbatim
  export PROFILE_PATH="${FIXTURE_DIR}/synth-profile.json"
  export RESOLVED_MODEL=haiku
  export PROMPT_FILE="${FIXTURE_DIR}/minimal-prompt.txt"
  export CYCLE=42
  export WORKSPACE_PATH="$WS"
  export STDOUT_LOG="$WS/stdout.log"
  export STDERR_LOG="$WS/stderr.log"
  export ARTIFACT_PATH="$WS/artifact.md"
  # CLI must be specified somewhere — bridge needs BRIDGE_CLI or --cli
  export BRIDGE_CLI=claude-p
  export BRIDGE_DRY_RUN=1
  run "$BRIDGE_BIN" launch
  [ "$status" -eq 0 ]
  [ -f "$ARTIFACT_PATH" ]
  grep -q 'cycle: 42' "$ARTIFACT_PATH"
}

@test "T-envvars.7 — BRIDGE_ALLOW_BYPASS=1 equivalent to --allow-bypass for claude-tmux" {
  # For the safety-gate, env var should work too. We use --dry-run to short-circuit
  # before the safety gate (dry-run bypasses driver dispatch).
  # But the gate is in the driver, not in bridge — so this test uses --validate-only.
  export BRIDGE_CLI=claude-tmux
  export PROFILE_PATH="${FIXTURE_DIR}/synth-profile.json"
  export RESOLVED_MODEL=haiku
  export PROMPT_FILE="${FIXTURE_DIR}/minimal-prompt.txt"
  export WORKSPACE_PATH="$WS"
  export STDOUT_LOG="$WS/stdout.log"
  export STDERR_LOG="$WS/stderr.log"
  export ARTIFACT_PATH="$WS/artifact.md"
  export BRIDGE_ALLOW_BYPASS=1
  export VALIDATE_ONLY=1
  run "$BRIDGE_BIN" launch
  [ "$status" -eq 0 ]
  [[ "$output" =~ allow-bypass[[:space:]]+=[[:space:]]+1 ]]
}
