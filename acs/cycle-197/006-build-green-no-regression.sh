#!/usr/bin/env bash
# AC-ID: cycle-197-006-build-green-no-regression
# AC-source: intent.md acceptance_criteria[0] ("make cover && go build ./... &&
#            go test ./... all pass, no regression").
# Preservation predicate (start GREEN, must STAY GREEN): `go build ./...` EXITS
#   0 — every non-test package in the module still compiles after the additive
#   test-design changes. This is the cheap, authoritative compile gate for the
#   no-regression AC. Per-package BEHAVIORAL correctness of the touched test
#   packages is pinned separately: 002 (./acs/...), 003 (debugger), 004
#   (./internal/core/), 005 (./internal/router/). Full-suite `go test ./...`
#   no-regression is additionally enforced by the audit-phase EGPS regression
#   suite. Uses assert_go_build (exit code, never output scrape).
#
# Mutation spec:
#   Mutant: a change that breaks compilation of any package -> FAIL (RED).
#
# Exit codes: 0 = GREEN, 1 = RED.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_build ./... || exit 1
echo "PASS"; exit 0
