#!/usr/bin/env bash
# ACS — cycle-241 step-5 task 3: runPersonaLint is wired into the ship commit
# gate: a persona→profile contradiction (disallowed tool) blocks with
# IntegrityError; undeclared drift logs but passes; missing dirs skip;
# EVOLVE_BYPASS_COMMIT_GATE=1 skips; and a full --class cycle ship runs the
# lint (log-visible) and ships clean fixtures. Behavioral: runs the ship test
# binary (full git ship pipeline inside the tests).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/phases/ship/ \
  'TestPersonaLint|TestCommitGate_CyclePersonaLint' \
  || exit 1
echo "PASS"
exit 0
