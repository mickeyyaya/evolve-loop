#!/usr/bin/env bash
# AC-ID: cycle-197-004-floor-enforce-negative-scenario
# AC-source: intent.md acceptance_criteria[2] ("phase-gate test rejects a
#            build+audit-less plan reaching ship — floor violation");
#            scout-report.md Task 3.
# Behavioral + presence predicate (2 checks):
#   (a) An Enforce()-mode scenario now exists in
#       internal/core/floor_activation_scenarios_test.go (>=1 `Enforce()`
#       brick). RED baseline = 0 (all five existing scenarios use
#       Advisory()/Off()). The Enforce() DSL brick exists in
#       routingtest/bricks.go:43 and is exercised here for the first time
#       against the integrity floor.
#   (b) `go test -run TestFloorActivationCycle ./internal/core/` EXITS 0 — the
#       new Enforce scenario passes: a plan that reaches ship WITHOUT build+audit
#       is clamped by ClampPlanToFloor so build+audit RUN before ship under
#       Stage:Enforce (the non-configurable integrity floor holds, not only at
#       Advisory). Authoritative via assert_go_test_pass (exit code).
#
# Mutation spec:
#   Mutant: add no Enforce scenario                 -> (a) FAIL (RED).
#   Mutant: add an Enforce scenario whose ExpectPhases omits build/audit
#           (asserts the floor does NOT fire)        -> (b) FAIL (run fails).
#
# Exit codes: 0 = GREEN, 1 = RED.
set -uo pipefail
top="$(git rev-parse --show-toplevel)"
. "$top/acs/lib/assert.sh"

floor="$top/go/internal/core/floor_activation_scenarios_test.go"
n=$(grep -c "Enforce()" "$floor" 2>/dev/null) || n=0
if [ "${n:-0}" -lt 1 ]; then
  echo "RED: no Enforce() scenario in floor_activation_scenarios_test.go" >&2
  exit 1
fi
echo "GREEN: $n Enforce() brick(s) present in floor scenarios" >&2

assert_go_test_pass ./internal/core/ 'TestFloorActivationCycle' || exit 1
echo "PASS"; exit 0
