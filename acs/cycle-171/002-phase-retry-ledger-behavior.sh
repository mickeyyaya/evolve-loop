#!/usr/bin/env bash
# ACS cycle-171 T1 — kind=phase_retry ledger entry emitted on self-heal retry.
# Behavioral: the test drives a cycle with a transient ErrArtifactTimeout and
# asserts a phase_retry ledger entry (role=scout, exit_code=81) was appended.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"
assert_go_test_pass ./internal/core/... 'TestPhaseTimingJSON_RetryEmitsLedgerEntry' || exit 1
