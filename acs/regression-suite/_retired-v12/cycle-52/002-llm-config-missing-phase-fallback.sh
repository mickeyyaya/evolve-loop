#!/usr/bin/env bash
# ACS predicate 002 — cycle 52
# Given llm_config.json with NO phases.builder entry, resolve-llm.sh falls back to profile.cli=claude
#
# AC-ID: cycle-52-002
# Description: resolve-llm.sh falls back to profile.cli when phase not declared in llm_config
# Evidence: scripts/dispatch/resolve-llm.sh
# Author: builder (evolve-builder)
# Created: 2026-05-14T15:30:00Z
# Acceptance-of: build-report.md AC-2
#
# metadata:
#   id: 002-llm-config-missing-phase-falls-back-to-profile
#   cycle: 52
#   task: llm-config-resolver
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RESOLVER="$REPO_ROOT/scripts/dispatch/resolve-llm.sh"

if [ ! -f "$RESOLVER" ]; then
    echo "RED: resolve-llm.sh not found at $RESOLVER"
    exit 1
fi

# Create a temporary llm_config.json with ONLY phases.scout declared (NO builder entry)
TMP_DIR=$(mktemp -d)
TMP_CONFIG="$TMP_DIR/llm_config.json"

cat > "$TMP_CONFIG" <<'EOJSON'
{
  "schema_version": 1,
  "phases": {
    "scout": { "provider": "google", "cli": "gemini", "model": "gemini-3-pro-preview" }
  }
}
EOJSON

rc=0

# Invoke resolver for builder (NOT in phases — should fall through to profile)
output=$(bash "$RESOLVER" builder "$TMP_CONFIG" 2>/dev/null) || {
    echo "RED AC1: resolve-llm.sh exited non-zero for builder with partial llm_config"
    rm -rf "$TMP_DIR"
    exit 1
}

# Validate output is valid JSON
if ! echo "$output" | jq empty 2>/dev/null; then
    echo "RED AC1: output is not valid JSON: $output"
    rc=1
fi

# Assert cli == claude (from profile, NOT from llm_config which has gemini for scout only)
resolved_cli=$(echo "$output" | jq -r '.cli' 2>/dev/null)
if [ "$resolved_cli" != "claude" ]; then
    echo "RED AC2: expected cli=claude (profile fallback), got cli='$resolved_cli'"
    rc=1
else
    echo "GREEN AC2: cli correctly fell back to profile value: claude"
fi

# Assert source == profile (NOT llm_config, because builder has no entry)
resolved_source=$(echo "$output" | jq -r '.source' 2>/dev/null)
if [ "$resolved_source" != "profile" ]; then
    echo "RED AC3: expected source=profile, got source='$resolved_source' (builder was not in llm_config)"
    rc=1
else
    echo "GREEN AC3: source correctly set to profile (fallback path taken)"
fi

# Anti-false-positive: verify scout WOULD resolve to gemini (proves the config was read)
scout_output=$(bash "$RESOLVER" scout "$TMP_CONFIG" 2>/dev/null)
scout_cli=$(echo "$scout_output" | jq -r '.cli' 2>/dev/null)
if [ "$scout_cli" != "gemini" ]; then
    echo "RED AC4: anti-false-positive check failed — scout should have resolved gemini but got '$scout_cli'"
    rc=1
else
    echo "GREEN AC4: anti-false-positive confirmed: scout=gemini, builder=claude (selective fallback)"
fi

rm -rf "$TMP_DIR"
exit "$rc"
