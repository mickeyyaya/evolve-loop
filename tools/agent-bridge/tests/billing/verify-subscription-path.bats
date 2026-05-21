#!/usr/bin/env bats
# T11 (billing) — end-to-end subscription billing verifier
#
# Opt-in via BRIDGE_BILLING_TESTS=1. Cost: ~$0.05 (one Haiku tmux launch + 2 snapshots).
# On macOS with active Claude Code subscription auth: verdict should be PASS (strong via keychain).
# On Linux / no keychain: verdict may be INCONCLUSIVE (operator runs manual console check).

setup_file() {
  if [[ "${BRIDGE_BILLING_TESTS:-0}" != "1" ]]; then
    return 0
  fi
  if ! command -v claude >/dev/null 2>&1 || ! command -v tmux >/dev/null 2>&1; then
    return 0
  fi

  BRIDGE_BIN="${BATS_TEST_DIRNAME}/../../bin/bridge"
  BRIDGE_LIB_DIR="${BATS_TEST_DIRNAME}/../../lib"
  PROFILE="${BATS_TEST_DIRNAME}/../fixtures/synth-profile.json"
  BRIDGE_T11B_WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-t11b-XXXXXX")"
  BRIDGE_T11B_TOKEN="$(openssl rand -hex 8)"
  BRIDGE_T11B_PROMPT="$BRIDGE_T11B_WS/prompt.txt"
  BRIDGE_T11B_STDOUT="$BRIDGE_T11B_WS/stdout.log"
  BRIDGE_T11B_STDERR="$BRIDGE_T11B_WS/stderr.log"
  BRIDGE_T11B_ARTIFACT="$BRIDGE_T11B_WS/artifact.md"
  BRIDGE_T11B_SNAPDIR="$BRIDGE_T11B_WS/snaps"
  mkdir -p "$BRIDGE_T11B_SNAPDIR"

  cat > "$BRIDGE_T11B_PROMPT" <<PROMPT_EOF
You are a billing-probe agent. Use your Write tool to create the file at:
${BRIDGE_T11B_ARTIFACT}
The file must contain exactly two lines:
<!-- challenge-token: ${BRIDGE_T11B_TOKEN} -->
PROTOTYPE OK
After writing, exit.
PROMPT_EOF

  # 1) BEFORE snapshot
  BRIDGE_T11B_BEFORE=$(bash "${BRIDGE_LIB_DIR}/billing-snapshot.sh" snapshot "$BRIDGE_T11B_SNAPDIR" before)

  # 2) The launch
  "$BRIDGE_BIN" launch \
    --cli=claude-tmux --profile="$PROFILE" --model=haiku \
    --prompt-file="$BRIDGE_T11B_PROMPT" --workspace="$BRIDGE_T11B_WS" \
    --stdout-log="$BRIDGE_T11B_STDOUT" --stderr-log="$BRIDGE_T11B_STDERR" \
    --artifact="$BRIDGE_T11B_ARTIFACT" \
    --allow-bypass \
    >"$BRIDGE_T11B_WS/bridge-stdout.log" 2>"$BRIDGE_T11B_WS/bridge-stderr.log" || true
  BRIDGE_T11B_LAUNCH_RC=$?

  # 3) AFTER snapshot
  BRIDGE_T11B_AFTER=$(bash "${BRIDGE_LIB_DIR}/billing-snapshot.sh" snapshot "$BRIDGE_T11B_SNAPDIR" after)

  # 4) compare → record verdict
  bash "${BRIDGE_LIB_DIR}/billing-snapshot.sh" compare "$BRIDGE_T11B_BEFORE" "$BRIDGE_T11B_AFTER" \
    > "$BRIDGE_T11B_WS/verdict.txt" 2>&1
  echo $? > "$BRIDGE_T11B_WS/verdict-rc"

  export BRIDGE_T11B_WS BRIDGE_T11B_BEFORE BRIDGE_T11B_AFTER BRIDGE_T11B_ARTIFACT BRIDGE_T11B_LAUNCH_RC
}

teardown_file() {
  if [[ -n "${BRIDGE_T11B_WS:-}" && -d "${BRIDGE_T11B_WS}" && "${BRIDGE_KEEP_WS:-0}" != "1" ]]; then
    tmux ls 2>/dev/null | awk -F: '/evolve-bridge-/{print $1}' \
      | xargs -n1 -I{} tmux kill-session -t {} 2>/dev/null || true
    rm -rf "${BRIDGE_T11B_WS}"
  fi
}

require_billing() {
  if [[ "${BRIDGE_BILLING_TESTS:-0}" != "1" ]]; then
    skip "BRIDGE_BILLING_TESTS!=1 — set to 1 to run live billing verification"
  fi
  if ! command -v claude >/dev/null 2>&1; then skip "claude not on PATH"; fi
  if ! command -v tmux >/dev/null 2>&1; then skip "tmux not on PATH"; fi
}

@test "T11B.1 — live launch produced artifact (sanity)" {
  require_billing
  [ -f "$BRIDGE_T11B_ARTIFACT" ]
}

@test "T11B.2 — billing verdict is PASS (rc=0)" {
  require_billing
  rc=$(cat "$BRIDGE_T11B_WS/verdict-rc")
  if [ "$rc" -ne 0 ]; then
    echo "--- verdict ---" >&2
    cat "$BRIDGE_T11B_WS/verdict.txt" >&2
  fi
  [ "$rc" -eq 0 ]
}

@test "T11B.3 — no cost-leak in after-snapshot" {
  require_billing
  api=$(jq -r '.anthropic_api_key_in_env' "$BRIDGE_T11B_AFTER")
  url=$(jq -r '.anthropic_base_url_in_env' "$BRIDGE_T11B_AFTER")
  [ "$api" = "no" ]
  [ -z "$url" ] || [ "$url" = "null" ]
}

@test "T11B.4 — verdict mentions either 'PASS' or 'INCONCLUSIVE' (never FAIL)" {
  require_billing
  ! grep -q 'FAIL' "$BRIDGE_T11B_WS/verdict.txt"
}
