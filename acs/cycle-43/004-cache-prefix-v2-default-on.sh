#!/usr/bin/env bash
# ACS predicate: cycle 43, AC-5+6
# Verify EVOLVE_CACHE_PREFIX_V2 default changed from :-0 to :-1 in subagent-run.sh
# and control-flags.md updated to ACTIVE (default 1)
#
# metadata:
#   cycle: 43
#   id: 004
#   slug: cache-prefix-v2-default-on
#   acceptance_criterion: "EVOLVE_CACHE_PREFIX_V2 default=1 in subagent-run.sh + claude.sh; control-flags.md updated to ACTIVE"

set -uo pipefail

SUBAGENT="scripts/dispatch/subagent-run.sh"
CLAUDE_SH="scripts/cli_adapters/claude.sh"
FLAGS_DOC="docs/architecture/control-flags.md"
OUTPUT_DIR=".evolve/runs/cycle-43/acs-output"
mkdir -p "$OUTPUT_DIR"

PASS=1
MSGS=""

# Check subagent-run.sh uses :-1 (not :-0) for EVOLVE_CACHE_PREFIX_V2
if grep -q 'EVOLVE_CACHE_PREFIX_V2:-1' "$SUBAGENT"; then
    MSGS="$MSGS\nPASS: $SUBAGENT uses EVOLVE_CACHE_PREFIX_V2:-1"
else
    MSGS="$MSGS\nFAIL: $SUBAGENT does not use EVOLVE_CACHE_PREFIX_V2:-1"
    PASS=0
fi

# Check claude.sh uses :-1
if grep -q 'EVOLVE_CACHE_PREFIX_V2:-1' "$CLAUDE_SH"; then
    MSGS="$MSGS\nPASS: $CLAUDE_SH uses EVOLVE_CACHE_PREFIX_V2:-1"
else
    MSGS="$MSGS\nFAIL: $CLAUDE_SH does not use EVOLVE_CACHE_PREFIX_V2:-1"
    PASS=0
fi

# Check control-flags.md updated to ACTIVE
if grep -q 'ACTIVE (default `1`)' "$FLAGS_DOC" && grep -q 'EVOLVE_CACHE_PREFIX_V2' "$FLAGS_DOC"; then
    MSGS="$MSGS\nPASS: $FLAGS_DOC shows EVOLVE_CACHE_PREFIX_V2 as ACTIVE (default 1)"
else
    MSGS="$MSGS\nFAIL: $FLAGS_DOC does not show EVOLVE_CACHE_PREFIX_V2 as ACTIVE (default 1)"
    PASS=0
fi

printf "$MSGS\n" | tee "$OUTPUT_DIR/004-result.txt"

if [ "$PASS" = "1" ]; then
    exit 0
else
    exit 1
fi
