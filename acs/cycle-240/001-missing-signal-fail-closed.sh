#!/usr/bin/env bash
# ACS — cycle-240 D2: insert_when on an absent generic signal evaluates FALSE
# for every operator (fail-closed); a PRESENT empty string keeps normal
# string-comparison semantics; the tdd-pin's typed-field semantics are
# untouched. Behavioral: runs the router test binary (exit code is the signal).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/router/ \
  'TestEvalCondition_AbsentFieldIsAlwaysFalse|TestEvalCondition_PresentEmptyString|TestEvalCondition_TypedFieldAbsentKeepsLegacySemantics|TestTriggerFires_AbsentFieldFailsClosed' \
  || exit 1
echo "PASS"
exit 0
