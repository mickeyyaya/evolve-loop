#!/usr/bin/env bash
# ACS — cycle-249 task `runner-base-cycle-context`
# Behavioral: runner.BaseCycleContext emits the byte-identical core block
# AND a migrated caller (tdd) composes prompts as base + extras
# (projection equivalence, intent AC "config-derived == prior hardcoded").
# Invokes the system under test via `go test` exit code (assert.sh).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/phases/runner/... 'TestBaseCycleContext' || exit 1
assert_go_test_pass ./internal/phases/tdd/... 'TestComposePromptParity' || exit 1

echo "GREEN: BaseCycleContext byte-parity + tdd caller projection equivalence hold"
exit 0
