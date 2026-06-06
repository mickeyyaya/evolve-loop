#!/usr/bin/env bash
# ACS — cycle-241 step-5 task 1: CheckProvenance parses the
# `<!-- evolve:provenance ... -->` header and returns WARN for an absent
# header, error-severity provenance-mismatch for tampered phase/cycle/tree_sha,
# zero violations for a header mirroring the ledger-derived expectation, and
# skips the tree_sha cross-check when the expected value is empty.
# Behavioral: runs the phasecoherence test binary (exit code is the signal).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/phasecoherence/ 'TestProvenanceGate' || exit 1
echo "PASS"
exit 0
