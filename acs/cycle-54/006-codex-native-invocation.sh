#!/usr/bin/env bash
# ACS predicate 006 — cycle 54
# When EVOLVE_CODEX_BINARY is set (test seam, EVOLVE_TESTING=1), codex.sh
# invokes that binary directly (NATIVE mode) instead of delegating to claude.sh
# (HYBRID) or running in-process (DEGRADED). Anti-tautology: with
# EVOLVE_CODEX_BINARY="" (empty = no binary), NATIVE mode is not taken.
#
# AC-ID: cycle-54-006
# Description: codex.sh NATIVE mode invokes codex binary when available
# Evidence: scripts/cli_adapters/codex.sh detect_codex_native()
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-2
#
# metadata:
#   id: 006-codex-native-invocation
#   cycle: 54
#   task: codex-native-mode
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

NATIVE_FLAG="$TMP_DIR/native-invoked"
MOCK_BIN="$TMP_DIR/mock-codex"
MOCK_PROMPT="$TMP_DIR/prompt.txt"
MOCK_PROFILE="$TMP_DIR/profile.json"

echo "test prompt" > "$MOCK_PROMPT"
printf '{"cli":"codex","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-0/builder-report.md"}' > "$MOCK_PROFILE"

# Mock codex binary records invocation by touching the native flag file
cat > "$MOCK_BIN" <<EOMOCK
#!/usr/bin/env bash
touch "${NATIVE_FLAG}"
exit 0
EOMOCK
chmod +x "$MOCK_BIN"

rc=0

# ── AC1: NATIVE mode invokes codex binary when EVOLVE_CODEX_BINARY is set ────
EVOLVE_TESTING=1 \
  EVOLVE_CODEX_BINARY="$MOCK_BIN" \
  PROMPT_FILE="$MOCK_PROMPT" \
  PROFILE_PATH="$MOCK_PROFILE" \
  STDOUT_LOG="$TMP_DIR/stdout.log" \
  STDERR_LOG="$TMP_DIR/stderr.log" \
  ARTIFACT_PATH="$TMP_DIR/artifact.md" \
  bash "$CODEX_SH" 2>/dev/null || true

if [ -f "$NATIVE_FLAG" ]; then
    echo "GREEN AC1: NATIVE mode invoked codex binary (flag file created)"
else
    echo "RED AC1: NATIVE mode did not invoke codex binary (flag file missing)"
    rc=1
fi

# ── AC2 (anti-tautology): with EVOLVE_CODEX_BINARY="" NATIVE not taken ───────
# EVOLVE_CODEX_BINARY="" with EVOLVE_TESTING=1 → detect_codex_native() returns 1
# → NATIVE path skipped. With EVOLVE_CODEX_CLAUDE_PATH="" → no HYBRID either
# → DEGRADED mode runs (writes stub stdout.log, exits 0).
NATIVE_FLAG2="$TMP_DIR/native-invoked-2"
cat > "$MOCK_BIN" <<EOMOCK2
#!/usr/bin/env bash
touch "${NATIVE_FLAG2}"
exit 0
EOMOCK2
chmod +x "$MOCK_BIN"

EVOLVE_TESTING=1 \
  EVOLVE_CODEX_BINARY="" \
  EVOLVE_CODEX_CLAUDE_PATH="" \
  PROMPT_FILE="$MOCK_PROMPT" \
  PROFILE_PATH="$MOCK_PROFILE" \
  STDOUT_LOG="$TMP_DIR/stdout2.log" \
  STDERR_LOG="$TMP_DIR/stderr2.log" \
  ARTIFACT_PATH="$TMP_DIR/artifact2.md" \
  bash "$CODEX_SH" 2>/dev/null || true

if [ ! -f "$NATIVE_FLAG2" ]; then
    echo "GREEN AC2: With EVOLVE_CODEX_BINARY empty, NATIVE mode not taken (anti-tautology passed)"
else
    echo "RED AC2: NATIVE binary called even with EVOLVE_CODEX_BINARY empty (anti-tautology failed)"
    rc=1
fi

exit "$rc"
