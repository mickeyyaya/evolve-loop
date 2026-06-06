#!/usr/bin/env bash
# ACS cycle-242/002 — full phasecoherence suite exits 0 (AC: the direct
# tree_sha fix must not regress the rescue-ref tests, in particular
# TestProvenanceGate_LedgerCrossCheck which requires EXACTLY ONE violation
# for a ledger-backed tree_sha mismatch — the dedup constraint).
#
# Behavioral: exit code of `go test -race` over the whole package.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/phasecoherence/... || exit 1

echo "PASS"; exit 0
