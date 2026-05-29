#!/usr/bin/env bash
# ACS predicate 003 — cycle 52
# With no llm_config.json at all, resolve-llm.sh returns profile-derived values (backward compat)
#
# AC-ID: cycle-52-003
# Description: resolve-llm.sh works without llm_config.json; returns profile cli and source=profile
# Evidence: scripts/dispatch/resolve-llm.sh
# Author: builder (evolve-builder)
# Created: 2026-05-14T15:30:00Z
# Acceptance-of: build-report.md AC-3
#
# metadata:
#   id: 003-llm-config-absent-zero-config-works
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

# Use a path that definitely does not exist
MISSING_CONFIG="/tmp/no-such-llm-config-predicate-003-cycle52-$(date +%s).json"

# Ensure the file really does not exist (safety check)
if [ -f "$MISSING_CONFIG" ]; then
    echo "RED: unexpected: temp config path exists: $MISSING_CONFIG"
    exit 1
fi

rc=0

# Invoke resolver with missing config path — must exit 0 (backward compat)
output=""
if ! output=$(bash "$RESOLVER" scout "$MISSING_CONFIG" 2>/dev/null); then
    echo "RED AC1: resolve-llm.sh exited non-zero when llm_config.json absent (backward compat broken)"
    exit 1
else
    echo "GREEN AC1: exit code 0 when llm_config absent"
fi

# Validate output is valid JSON
if ! echo "$output" | jq empty 2>/dev/null; then
    echo "RED AC2: output is not valid JSON when llm_config absent: $output"
    rc=1
else
    echo "GREEN AC2: output is valid JSON"
fi

# Assert source == profile (config absent → must use profile)
resolved_source=$(echo "$output" | jq -r '.source' 2>/dev/null)
if [ "$resolved_source" != "profile" ]; then
    echo "RED AC3: expected source=profile when config absent, got '$resolved_source'"
    rc=1
else
    echo "GREEN AC3: source=profile (expected for zero-config mode)"
fi

# Assert cli is non-empty (profile must supply a value)
resolved_cli=$(echo "$output" | jq -r '.cli' 2>/dev/null)
if [ -z "$resolved_cli" ] || [ "$resolved_cli" = "null" ]; then
    echo "RED AC4: cli field is empty or null in zero-config mode"
    rc=1
else
    echo "GREEN AC4: cli field non-empty: '$resolved_cli'"
fi

# Assert cli == claude (all profiles currently declare cli=claude per scout-report survey)
if [ "$resolved_cli" != "claude" ]; then
    echo "RED AC5: expected cli=claude (all profiles declare cli=claude), got '$resolved_cli'"
    rc=1
else
    echo "GREEN AC5: cli=claude (matches profile survey: all 13 profiles declare cli=claude)"
fi

# Assert output has both cli and source keys (structural completeness)
key_count=$(echo "$output" | jq 'keys | length' 2>/dev/null)
if [ "${key_count:-0}" -lt 2 ]; then
    echo "RED AC6: output JSON has fewer than 2 keys (expected at minimum cli + source)"
    rc=1
else
    echo "GREEN AC6: output JSON has $key_count keys (structurally complete)"
fi

exit "$rc"
