#!/usr/bin/env bash
# ACS predicate 004 — cycle 53
# Given llm_config routes scout→gemini AND gemini capabilities.json has
# supports.budget_cap_native=false, --validate-profile emits
# [adapter-cap] WARN and exports CAP_BUDGET_NATIVE=false to adapter env.
#
# AC-ID: cycle-53-004
# Description: subagent-run.sh reads supports.budget_cap_native from
#   capabilities manifest and emits structured WARN + sets env var
# Evidence: scripts/dispatch/subagent-run.sh, scripts/cli_adapters/gemini.capabilities.json
# Author: builder (evolve-builder)
# Created: 2026-05-14T00:00:00Z
# Acceptance-of: build-report.md AC-1
#
# metadata:
#   id: 004-capability-matrix-honored-no-budget-cap
#   cycle: 53
#   task: capability-matrix-files
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SUBAGENT_RUN="$REPO_ROOT/scripts/dispatch/subagent-run.sh"

if [ ! -f "$SUBAGENT_RUN" ]; then
    echo "RED: subagent-run.sh not found at $SUBAGENT_RUN"
    exit 1
fi

# ── Setup ─────────────────────────────────────────────────────────────────────
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# 1. llm_config routing scout→gemini
TMP_CONFIG="$TMP_DIR/llm_config.json"
cat > "$TMP_CONFIG" <<'EOJSON'
{"schema_version":1,"phases":{"scout":{"cli":"gemini","model":"gemini-3-pro-preview"}}}
EOJSON

# 2. Sentinel adapter captures CAP_BUDGET_NATIVE from its env and exits 0
SENTINEL_DIR="$TMP_DIR/adapters"
mkdir -p "$SENTINEL_DIR"
cat > "$SENTINEL_DIR/gemini.sh" <<'EOADA'
#!/usr/bin/env bash
echo "[sentinel-gemini] CAP_BUDGET_NATIVE=${CAP_BUDGET_NATIVE:-unset}" >&2
echo "[sentinel-gemini] RESOLVED_CLI=${RESOLVED_CLI:-unset}" >&2
exit 0
EOADA
chmod +x "$SENTINEL_DIR/gemini.sh"

# 3. Run --validate-profile scout with overrides
stderr_out=$(EVOLVE_TESTING=1 \
  EVOLVE_GEMINI_CLAUDE_PATH="" \
  EVOLVE_LLM_CONFIG_PATH="$TMP_CONFIG" \
  EVOLVE_ADAPTERS_DIR_OVERRIDE="$SENTINEL_DIR" \
  bash "$SUBAGENT_RUN" --validate-profile scout 2>&1 >/dev/null) || true

rc=0

# ── AC1: WARN line present in stderr ─────────────────────────────────────────
if ! printf '%s\n' "$stderr_out" | grep -q "\[adapter-cap\] WARN cli=gemini missing=budget_cap_native"; then
    echo "RED AC1: expected [adapter-cap] WARN cli=gemini missing=budget_cap_native not found in stderr"
    echo "  stderr captured: $stderr_out"
    rc=1
else
    echo "GREEN AC1: [adapter-cap] WARN line present"
fi

# ── AC2: CAP_BUDGET_NATIVE=false reached sentinel ───────────────────────────
if ! printf '%s\n' "$stderr_out" | grep -q "CAP_BUDGET_NATIVE=false"; then
    echo "RED AC2: sentinel did not receive CAP_BUDGET_NATIVE=false"
    echo "  stderr captured: $stderr_out"
    rc=1
else
    echo "GREEN AC2: CAP_BUDGET_NATIVE=false reached sentinel adapter"
fi

# ── AC3 (anti-tautology): no false WARN for claude ───────────────────────────
# When cli=claude (which supports budget_cap_native), WARN must NOT appear.
TMP_CONFIG2="$TMP_DIR/llm_config_claude.json"
cat > "$TMP_CONFIG2" <<'EOJSON'
{"schema_version":1,"phases":{"scout":{"cli":"claude","model":"claude-sonnet-4-6"}}}
EOJSON
cat > "$SENTINEL_DIR/claude.sh" <<'EOADA'
#!/usr/bin/env bash
echo "[sentinel-claude] CAP_BUDGET_NATIVE=${CAP_BUDGET_NATIVE:-unset}" >&2
exit 0
EOADA
chmod +x "$SENTINEL_DIR/claude.sh"

stderr_claude=$(EVOLVE_TESTING=1 \
  EVOLVE_LLM_CONFIG_PATH="$TMP_CONFIG2" \
  EVOLVE_ADAPTERS_DIR_OVERRIDE="$SENTINEL_DIR" \
  bash "$SUBAGENT_RUN" --validate-profile scout 2>&1 >/dev/null) || true

if printf '%s\n' "$stderr_claude" | grep -q "\[adapter-cap\] WARN.*missing=budget_cap_native"; then
    echo "RED AC3 (anti-tautology): WARN appeared for claude which has budget_cap_native support"
    echo "  stderr was: $stderr_claude"
    rc=1
else
    echo "GREEN AC3: no false WARN for claude (anti-tautology passed)"
fi

exit "$rc"
