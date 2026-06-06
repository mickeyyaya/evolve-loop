#!/usr/bin/env bash
# ACS — cycle-240 D3: a trigger-class (EnableContent) phase scheduled by the
# advisory plan respects MaxInsertions (skip + max-insertions-cap clamp at the
# cap); operator-forced (EnableOn) plan phases and ship-floor phases stay
# cap-exempt. Behavioral: runs the router test binary.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/router/ \
  'TestRoute_AdvisoryTriggerCapEnforced|TestRoute_AdvisoryTriggerWithinCap|TestRoute_AdvisoryPlanPhaseExemptsFromCap|TestRoute_AdvisoryFloorPhaseNotCapped' \
  || exit 1
echo "PASS"
exit 0
