#!/usr/bin/env bash
# AC-ID:         cycle-234-T2 (task cycle-level-bridge-failure, ACs 1-7)
# Description:   ErrCycleLevelFailure sentinel + bridge exhaustion = cycle-level; integrity stays batch-fatal; loop continues; recovery budget bounded
# Evidence:      go test (exit code) on the cycle-234 RED tests in internal/core + cmd/evolve
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: intent.md AC1 (bridge/phase errors escalate cycle-level, never batch-fatal rc=2 unless kernel invariant broke)
#
# Behavioral: RunCycle is invoked with dead-bridge / integrity-breach fake
# runners, and runLoop is driven end-to-end through the wireOrchestratorDepsFn
# seam (3-cycle batch, cycle 2 dies). Assertions ride go test's exit code.

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/core/... 'TestErrCycleLevelFailure_WrapsCauseForErrorsIs|TestOrchestrator_BridgeExhaustion_CycleLevelFailure|TestOrchestrator_IntegrityBreach_StillBatchFatal|TestOrchestrator_RecoveryDepthBudget' || exit 1
assert_go_test_pass ./cmd/evolve/... 'TestLoop_CycleLevelFailureContinues' || exit 1

echo "GREEN: cycle-level failure classification verified (sentinel + orchestrator wrap + loop continuation + recovery budget)" >&2
exit 0
