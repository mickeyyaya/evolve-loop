#!/usr/bin/env bash
# ACS — cycle-240 D4: EVOLVE_REQUIRE_INTENT=1 forces intent into the advisory
# plan — the floor clamp honors RouteInput.IntentRequired (router layer) and
# the orchestrator threads the bit into the plan-clamp input (core layer).
# Behavioral: runs router + core test binaries.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/router/ \
  'TestClampPlanToFloor_Intent' \
  || exit 1
assert_go_test_pass ./internal/core/ \
  'TestOrchestrator_ThreadsIntentRequiredToPlannerInput' \
  || exit 1
echo "PASS"
exit 0
