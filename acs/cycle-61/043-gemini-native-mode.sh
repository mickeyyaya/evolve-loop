#!/usr/bin/env bash
# ACS predicate 043 — cycle 61
# Verifies Gemini native mode execution support is present.
#
# AC-ID: cycle-61-043
# Description: Verifies non_interactive_prompt: true in gemini.capabilities.json and native invocation string in gemini.sh
# Evidence: File contents check
# Author: builder (evolve-builder)

set -uo pipefail

if [ -n "${EVOLVE_PROJECT_ROOT:-}" ]; then
    REPO_ROOT="$EVOLVE_PROJECT_ROOT"
else
    REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
fi
if [ -n "${EVOLVE_PLUGIN_ROOT:-}" ]; then
    PLUGIN_ROOT="$EVOLVE_PLUGIN_ROOT"
else
    PLUGIN_ROOT="$REPO_ROOT"
fi

CAP_FILE="$PLUGIN_ROOT/scripts/cli_adapters/gemini.capabilities.json"
SH_FILE="$PLUGIN_ROOT/scripts/cli_adapters/gemini.sh"

if [ ! -f "$CAP_FILE" ]; then
    echo "[FAIL] $CAP_FILE not found"
    exit 1
fi

if ! grep -q '"non_interactive_prompt": true' "$CAP_FILE"; then
    echo "[FAIL] gemini.capabilities.json missing 'non_interactive_prompt: true'"
    exit 1
fi

if [ ! -f "$SH_FILE" ]; then
    echo "[FAIL] $SH_FILE not found"
    exit 1
fi

if ! grep -q "invoking gemini binary directly" "$SH_FILE"; then
    echo "[FAIL] gemini.sh missing 'invoking gemini binary directly'"
    exit 1
fi

echo "[PASS] Gemini native mode verification complete."
exit 0
