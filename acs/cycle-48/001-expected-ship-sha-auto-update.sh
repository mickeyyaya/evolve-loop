#!/usr/bin/env bash
# ACS predicate 001 — cycle 48
# Verifies that ship.sh success path re-pins expected_ship_sha after a
# cycle push that modifies ship.sh itself (T1 fix).
#
# metadata:
#   id: 001-expected-ship-sha-auto-update
#   cycle: 48
#   task: T1
#   severity: HIGH
set -uo pipefail

SHIP_SH="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null)}/scripts/lifecycle/ship.sh"

# Check that the post-cycle self-update block is present in ship.sh
if grep -q "_repin_ship_sha.*post-cycle self-update" "$SHIP_SH" 2>/dev/null; then
    echo "GREEN: ship.sh contains post-cycle self-update _repin_ship_sha call"
    exit 0
else
    echo "RED: ship.sh missing post-cycle self-update _repin_ship_sha call"
    exit 1
fi
