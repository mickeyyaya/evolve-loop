#!/usr/bin/env bash
# ACS cycle-242/001 — CheckProvenance fires a DIRECT tree_sha violation
# (cycle-241 audit HIGH: tree_sha was only ledger-cross-checked, so a bad
# tree_sha with no ledger produced zero violations).
#
# Behavioral: invokes the system under test via `go test` (exit code is the
# authoritative signal — acs/lib/assert.sh, cycle-137 lesson). Covers the
# whole cycle-242 amplification family, including the dedup contract
# (exactly one tree_sha violation when ledger + direct check both apply).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/phasecoherence/ 'TestCheckProvenance_' || exit 1

echo "PASS"; exit 0
