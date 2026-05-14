#!/usr/bin/env bash
# ACS predicate 001 — cycle 52
# Given llm_config.json with phases.scout.cli=gemini, resolve-llm.sh returns cli=gemini + source=llm_config
#
# AC-ID: cycle-52-001
# Description: resolve-llm.sh reads llm_config.json and overrides profile cli when phase entry present
# Evidence: scripts/dispatch/resolve-llm.sh
# Author: builder (evolve-builder)
# Created: 2026-05-14T15:30:00Z
# Acceptance-of: build-report.md AC-1
#
# metadata:
#   id: 001-llm-config-load-and-override-cli
#   cycle: 52
#   task: llm-config-resolver
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RESOLVER="$REPO_ROOT/scripts/dispatch/resolve-llm.sh"

# Require the resolver to exist
if [ ! -f "$RESOLVER" ]; then
    echo "RED: resolve-llm.sh not found at $RESOLVER"
    exit 1
fi

# Create a temporary llm_config.json with phases.scout.cli=gemini
TMP_DIR=$(mktemp -d)
TMP_CONFIG="$TMP_DIR/llm_config.json"

cat > "$TMP_CONFIG" <<'EOJSON'
{
  "schema_version": 1,
  "phases": {
    "scout": { "provider": "google", "cli": "gemini", "model": "gemini-3-pro-preview" }
  },
  "_fallback": { "provider": "anthropic", "cli": "claude", "model_tier": "sonnet" }
}
EOJSON

rc=0
output=""
output=$(bash "$RESOLVER" scout "$TMP_CONFIG" 2>/dev/null) || {
    echo "RED AC1: resolve-llm.sh exited non-zero for scout with llm_config present"
    rm -rf "$TMP_DIR"
    exit 1
}

# Validate output is valid JSON
if ! echo "$output" | jq empty 2>/dev/null; then
    echo "RED AC1: resolve-llm.sh output is not valid JSON: $output"
    rc=1
fi

# Assert cli == gemini
resolved_cli=$(echo "$output" | jq -r '.cli' 2>/dev/null)
if [ "$resolved_cli" != "gemini" ]; then
    echo "RED AC2: expected cli=gemini, got cli='$resolved_cli'"
    rc=1
else
    echo "GREEN AC2: cli correctly resolved to gemini"
fi

# Assert source == llm_config
resolved_source=$(echo "$output" | jq -r '.source' 2>/dev/null)
if [ "$resolved_source" != "llm_config" ]; then
    echo "RED AC3: expected source=llm_config, got source='$resolved_source'"
    rc=1
else
    echo "GREEN AC3: source correctly set to llm_config"
fi

# Assert model is present (non-empty)
resolved_model=$(echo "$output" | jq -r '.model // .model_tier // empty' 2>/dev/null)
if [ -z "$resolved_model" ]; then
    echo "RED AC4: output missing model/model_tier field"
    rc=1
else
    echo "GREEN AC4: model field present: $resolved_model"
fi

rm -rf "$TMP_DIR"
exit "$rc"
