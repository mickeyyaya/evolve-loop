#!/usr/bin/env bash
# AC-ID: cycle-197-003-debugger-coverage-ge-75
# AC-source: scout-report.md Task 2 gate "go test -cover
#            ./internal/phases/debugger/... reports >=75%".
# Behavioral predicate: measured STATEMENT coverage of the debugger package
#   must be >= 75%. RED baseline = 43.3% (nextPhaseFor, every hooks method —
#   PhaseName/AgentPromptName/ArtifactFilename/DefaultModel/ComposePrompt — and
#   the Phase.Run enrichment path are untested). Builder adds test-only coverage.
#   Uses assert_go_coverage_ge, whose field extraction (acs_coverage_pct) is a
#   directly-unit-tested pure function — never inline grep/awk on the coverage
#   line (the cycle-137 008 false-RED footgun).
#
# Mutation spec:
#   Mutant: add no new debugger tests        -> coverage stays 43.3% -> FAIL.
#   Mutant: add a tautological no-assert test -> coverage rises but the
#           behavioral debugger_test cases (003 is paired with the package
#           tests) keep the bar honest.
#
# Exit codes: 0 = GREEN, 1 = RED.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_coverage_ge ./internal/phases/debugger/... 75 || exit 1
echo "PASS"; exit 0
