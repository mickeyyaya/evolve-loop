#!/usr/bin/env bash
# AC-ID:         cycle-234-T1 (task phase-boundary-checkpoint, ACs 1-4)
# Description:   ReasonPhaseComplete valid + orchestrator writes a phase-boundary checkpoint block; failed phase records no success
# Evidence:      go test (exit code) on the cycle-234 RED tests in internal/checkpoint + internal/core
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: intent.md AC2 (durable checkpoint at every phase boundary)
#
# Behavioral: drives the real checkpoint package and the orchestrator's
# RunCycle (probe runner reads the on-disk cycle-state.json MID-cycle), via
# `go test` exit codes per acs/lib/assert.sh (cycle-137 lesson — never
# scrape PASS strings).

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/checkpoint/... 'TestReasonPhaseComplete_IsValid|TestReason_NearMissSpellings_StayInvalid|TestComposeChecked_AcceptsPhaseComplete' || exit 1
assert_go_test_pass ./internal/core/... 'TestOrchestrator_PhaseBoundaryCheckpoint|TestOrchestrator_FailedPhase_NoSuccessCheckpoint' || exit 1

echo "GREEN: phase-boundary checkpoint behavior verified (reason constant + mid-cycle on-disk block + failure path)" >&2
exit 0
