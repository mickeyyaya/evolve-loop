#!/usr/bin/env bash
# ACS — cycle-240 D1: plan run:false vetoes a firing insert_when trigger, at
# BOTH layers — the router kernel (walk/shouldRun) and the orchestrator's
# enforceNext decline-fallback (the actual cycle-238 pile-on mechanism).
# Behavioral: runs router + core test binaries.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/router/ \
  'TestRoute_AdvisoryPlanRunFalse|TestRoute_AdvisoryPlanVetoUserPhaseAbsentSignal' \
  || exit 1
assert_go_test_pass ./internal/core/ \
  'TestOrchestrator_AdvisoryPlanVetoSurvivesSpineDecline' \
  || exit 1
echo "PASS"
exit 0
