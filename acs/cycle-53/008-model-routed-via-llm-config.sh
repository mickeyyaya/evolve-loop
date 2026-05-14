#!/usr/bin/env bash
# ACS predicate 008 — cycle 53
# When llm_config.json sets phases.auditor.model=claude-opus-4-7 AND
# profile.model_tier_default=opus (not exact string), the resolved model
# passed to the adapter is exactly "claude-opus-4-7" (overriding profile default).
# Anti-tautology: without llm_config, model is NOT claude-opus-4-7.
#
# AC-ID: cycle-53-008
# Description: llm_config model field overrides profile model_tier_default
# Evidence: scripts/dispatch/subagent-run.sh, scripts/dispatch/resolve-llm.sh
# Author: builder (evolve-builder)
# Created: 2026-05-14T00:00:00Z
# Acceptance-of: build-report.md AC-3
#
# metadata:
#   id: 008-model-routed-via-llm-config
#   cycle: 53
#   task: wire-resolve-llm-into-subagent-run
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

# llm_config with exact model for auditor
TMP_CONFIG="$TMP_DIR/llm_config.json"
cat > "$TMP_CONFIG" <<'EOJSON'
{"schema_version":1,"phases":{"auditor":{"cli":"claude","model":"claude-opus-4-7"}}}
EOJSON

# Sentinel adapter: echoes RESOLVED_MODEL and exits 0 under VALIDATE_ONLY
SENTINEL_DIR="$TMP_DIR/adapters"
mkdir -p "$SENTINEL_DIR"
cat > "$SENTINEL_DIR/claude.sh" <<'EOADA'
#!/usr/bin/env bash
if [ "${VALIDATE_ONLY:-0}" = "1" ]; then
    echo "[sentinel-claude] model_routed=${RESOLVED_MODEL:-unset}" >&2
    exit 0
fi
exit 0
EOADA
chmod +x "$SENTINEL_DIR/claude.sh"

rc=0

# ── AC1: model from llm_config reaches adapter as RESOLVED_MODEL ─────────────
stderr_with=$(EVOLVE_TESTING=1 \
  EVOLVE_LLM_CONFIG_PATH="$TMP_CONFIG" \
  EVOLVE_ADAPTERS_DIR_OVERRIDE="$SENTINEL_DIR" \
  bash "$SUBAGENT_RUN" --validate-profile auditor 2>&1 >/dev/null) || true

if ! printf '%s\n' "$stderr_with" | grep -q "model_routed=claude-opus-4-7"; then
    echo "RED AC1: expected model_routed=claude-opus-4-7 not found in sentinel output"
    echo "  stderr was: $stderr_with"
    rc=1
else
    echo "GREEN AC1: model_routed=claude-opus-4-7 confirmed in sentinel"
fi

# ── AC2 (anti-tautology): without llm_config, model is NOT claude-opus-4-7 ──
# Auditor profile's model_tier_default = "opus" (not exact string "claude-opus-4-7")
stderr_without=$(EVOLVE_TESTING=1 \
  EVOLVE_ADAPTERS_DIR_OVERRIDE="$SENTINEL_DIR" \
  bash "$SUBAGENT_RUN" --validate-profile auditor 2>&1 >/dev/null) || true

if printf '%s\n' "$stderr_without" | grep -q "model_routed=claude-opus-4-7"; then
    echo "RED AC2 (anti-tautology): model was claude-opus-4-7 even WITHOUT llm_config — predicate is tautological"
    echo "  stderr was: $stderr_without"
    rc=1
else
    echo "GREEN AC2: without llm_config, model is NOT claude-opus-4-7 (anti-tautology passed)"
fi

# ── AC3: cli_resolution_source logged when llm_config used ───────────────────
if ! printf '%s\n' "$stderr_with" | grep -q "cli_resolution:.*source=llm_config"; then
    echo "RED AC3: cli_resolution log line with source=llm_config not found"
    echo "  stderr was: $stderr_with"
    rc=1
else
    echo "GREEN AC3: cli_resolution log line confirms source=llm_config"
fi

exit "$rc"
