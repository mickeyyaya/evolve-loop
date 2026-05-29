#!/usr/bin/env bash
# ACS predicate 012 — cycle 55
# When non_interactive_prompt=false in capabilities, codex.sh must NOT activate
# NATIVE mode even when codex binary is on PATH. Anti-tautology: with cap=true
# + binary override seam, NATIVE IS taken.
#
# AC-ID: cycle-55-012
# Description: capability-gate blocks NATIVE when non_interactive_prompt=false (codex)
# Evidence: scripts/cli_adapters/codex.sh _CODEX_NATIVE_CAP gate
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-4
#
# metadata:
#   id: 012-capability-gate-blocks-native-codex
#   cycle: 55
#   task: capability-gate-hardening
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
CODEX_SH="$REPO_ROOT/scripts/cli_adapters/codex.sh"

if [ ! -f "$CODEX_SH" ]; then
    echo "RED: codex.sh not found at $CODEX_SH"
    exit 1
fi

# ── Setup ─────────────────────────────────────────────────────────────────────
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

NATIVE_FLAG1="$TMP_DIR/native-invoked-ac1"
NATIVE_FLAG2="$TMP_DIR/native-invoked-ac2"
# Binary must be named 'codex' so command -v codex finds it via PATH
MOCK_BIN="$TMP_DIR/codex"
MOCK_PROMPT="$TMP_DIR/prompt.txt"
MOCK_PROFILE="$TMP_DIR/profile.json"

echo "test prompt" > "$MOCK_PROMPT"
printf '{"cli":"codex","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-0/builder-report.md"}' > "$MOCK_PROFILE"

# Mock codex binary records invocation by touching the native flag file
cat > "$MOCK_BIN" <<EOMOCK
#!/usr/bin/env bash
touch "${NATIVE_FLAG1}"
exit 0
EOMOCK
chmod +x "$MOCK_BIN"

# Fixture: capabilities file with non_interactive_prompt=false
CAP_FALSE="$TMP_DIR/cap-false.json"
printf '{"adapter":"codex","version":2,"supports":{"non_interactive_prompt":false}}' > "$CAP_FALSE"

# Fixture: capabilities file with non_interactive_prompt=true (for anti-tautology)
CAP_TRUE="$TMP_DIR/cap-true.json"
printf '{"adapter":"codex","version":2,"supports":{"non_interactive_prompt":true}}' > "$CAP_TRUE"

rc=0

# ── AC1: cap=false blocks NATIVE even when codex binary is on PATH ─────────────
# Mock is named 'codex' at $TMP_DIR/codex → command -v codex finds it.
# With cap=false, NATIVE block must be skipped entirely → mock never called.
# EVOLVE_CODEX_CLAUDE_PATH="" suppresses HYBRID mode (hermetic test).
PATH="$TMP_DIR:$PATH" \
EVOLVE_TESTING=1 \
  EVOLVE_CODEX_CAP_FILE="$CAP_FALSE" \
  EVOLVE_CODEX_CLAUDE_PATH="" \
  PROMPT_FILE="$MOCK_PROMPT" \
  PROFILE_PATH="$MOCK_PROFILE" \
  STDOUT_LOG="$TMP_DIR/stdout1.log" \
  STDERR_LOG="$TMP_DIR/stderr1.log" \
  ARTIFACT_PATH="$TMP_DIR/artifact1.md" \
  bash "$CODEX_SH" 2>/dev/null || true

if [ ! -f "$NATIVE_FLAG1" ]; then
    echo "GREEN AC1: cap=false blocked NATIVE mode (sentinel not created)"
else
    echo "RED AC1: cap=false did NOT block NATIVE mode (sentinel was created — gate is ineffective)"
    rc=1
fi

# ── AC2 (anti-tautology): cap=true + EVOLVE_CODEX_BINARY → NATIVE IS taken ───
cat > "$MOCK_BIN" <<EOMOCK2
#!/usr/bin/env bash
touch "${NATIVE_FLAG2}"
exit 0
EOMOCK2
chmod +x "$MOCK_BIN"

EVOLVE_TESTING=1 \
  EVOLVE_CODEX_CAP_FILE="$CAP_TRUE" \
  EVOLVE_CODEX_BINARY="$MOCK_BIN" \
  PROMPT_FILE="$MOCK_PROMPT" \
  PROFILE_PATH="$MOCK_PROFILE" \
  STDOUT_LOG="$TMP_DIR/stdout2.log" \
  STDERR_LOG="$TMP_DIR/stderr2.log" \
  ARTIFACT_PATH="$TMP_DIR/artifact2.md" \
  bash "$CODEX_SH" 2>/dev/null || true

if [ -f "$NATIVE_FLAG2" ]; then
    echo "GREEN AC2: cap=true + EVOLVE_CODEX_BINARY → NATIVE mode taken (anti-tautology passed)"
else
    echo "RED AC2: cap=true + EVOLVE_CODEX_BINARY → NATIVE mode NOT taken (anti-tautology failed)"
    rc=1
fi

exit "$rc"
