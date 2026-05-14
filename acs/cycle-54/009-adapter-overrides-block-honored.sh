#!/usr/bin/env bash
# ACS predicate 009 — cycle 54
# When profile.adapter_overrides.gemini.tools=[...] exists and CLI resolves to
# gemini, subagent-run.sh exports those tools as ADAPTER_TOOLS_OVERRIDE to the
# adapter. Anti-tautology: without adapter_overrides, ADAPTER_TOOLS_OVERRIDE is
# empty/absent.
#
# AC-ID: cycle-54-009
# Description: profile adapter_overrides per-CLI tool block honored by dispatcher
# Evidence: scripts/dispatch/subagent-run.sh ADR-6 adapter_overrides block
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-3
#
# metadata:
#   id: 009-adapter-overrides-block-honored
#   cycle: 54
#   task: adapter-overrides
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

# llm_config routes scout→gemini
TMP_CONFIG="$TMP_DIR/llm_config.json"
cat > "$TMP_CONFIG" <<'EOJSON'
{"schema_version":1,"phases":{"scout":{"cli":"gemini","model":"gemini-3-pro-preview"}}}
EOJSON

# Profile WITH adapter_overrides.gemini.tools
TMP_PROFILES="$TMP_DIR/profiles"
mkdir -p "$TMP_PROFILES"
cat > "$TMP_PROFILES/scout.json" <<'EOJSON'
{
  "name": "scout",
  "cli": "claude",
  "model_tier_default": "sonnet",
  "output_artifact": ".evolve/runs/cycle-{cycle}/scout-report.md",
  "allowed_tools": ["Read", "Grep", "Glob"],
  "adapter_overrides": {
    "gemini": {
      "tools": ["read_file", "list_dir"]
    }
  }
}
EOJSON

# Sentinel adapter records ADAPTER_TOOLS_OVERRIDE from its env
TMP_ADAPTERS="$TMP_DIR/adapters"
mkdir -p "$TMP_ADAPTERS"
cat > "$TMP_ADAPTERS/gemini.sh" <<'EOADA'
#!/usr/bin/env bash
echo "[sentinel-gemini] ADAPTER_TOOLS_OVERRIDE=${ADAPTER_TOOLS_OVERRIDE:-<unset>}" >&2
echo "[sentinel-gemini] ADAPTER_EXTRA_FLAGS_OVERRIDE=${ADAPTER_EXTRA_FLAGS_OVERRIDE:-<unset>}" >&2
exit 0
EOADA
chmod +x "$TMP_ADAPTERS/gemini.sh"

# Run --validate-profile with the overrides profile
rc=0
stderr_out=$(EVOLVE_TESTING=1 \
  EVOLVE_GEMINI_CLAUDE_PATH="" \
  EVOLVE_LLM_CONFIG_PATH="$TMP_CONFIG" \
  EVOLVE_ADAPTERS_DIR_OVERRIDE="$TMP_ADAPTERS" \
  EVOLVE_PROFILES_DIR_OVERRIDE="$TMP_PROFILES" \
  bash "$SUBAGENT_RUN" --validate-profile scout 2>&1 >/dev/null) || true

# ── AC1: ADAPTER_TOOLS_OVERRIDE contains the gemini tool list ────────────────
ao_line=$(printf '%s\n' "$stderr_out" | grep 'ADAPTER_TOOLS_OVERRIDE=' | head -1 || true)
if printf '%s\n' "${ao_line:-}" | grep -q 'read_file' && \
   printf '%s\n' "${ao_line:-}" | grep -q 'list_dir'; then
    echo "GREEN AC1: ADAPTER_TOOLS_OVERRIDE contains gemini adapter_overrides tools"
else
    echo "RED AC1: ADAPTER_TOOLS_OVERRIDE does not contain expected tools (read_file, list_dir)"
    echo "  sentinel output line: ${ao_line:-<not found>}"
    echo "  full stderr: $stderr_out"
    rc=1
fi

# ── AC2 (anti-tautology): without adapter_overrides, ADAPTER_TOOLS_OVERRIDE empty ──
cat > "$TMP_PROFILES/scout.json" <<'EOJSON'
{
  "name": "scout",
  "cli": "claude",
  "model_tier_default": "sonnet",
  "output_artifact": ".evolve/runs/cycle-{cycle}/scout-report.md",
  "allowed_tools": ["Read", "Grep", "Glob"]
}
EOJSON

stderr_no_ao=$(EVOLVE_TESTING=1 \
  EVOLVE_GEMINI_CLAUDE_PATH="" \
  EVOLVE_LLM_CONFIG_PATH="$TMP_CONFIG" \
  EVOLVE_ADAPTERS_DIR_OVERRIDE="$TMP_ADAPTERS" \
  EVOLVE_PROFILES_DIR_OVERRIDE="$TMP_PROFILES" \
  bash "$SUBAGENT_RUN" --validate-profile scout 2>&1 >/dev/null) || true

ao_line2=$(printf '%s\n' "$stderr_no_ao" | grep 'ADAPTER_TOOLS_OVERRIDE=' | head -1 || true)
if printf '%s\n' "${ao_line2:-}" | grep -q 'read_file\|list_dir'; then
    echo "RED AC2: ADAPTER_TOOLS_OVERRIDE has tool data without adapter_overrides (anti-tautology failed)"
    echo "  sentinel output: ${ao_line2:-}"
    rc=1
else
    echo "GREEN AC2: Without adapter_overrides, ADAPTER_TOOLS_OVERRIDE is empty/absent (anti-tautology passed)"
fi

exit "$rc"
