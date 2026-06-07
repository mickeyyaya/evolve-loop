#!/usr/bin/env bash
# ACS — cycle-252 task `psmas-phase-skip-wire-go-router`
# Behavioral: Route() consumes Triage.PhaseSkip under the PSMASEnabled
# gate — additive-only (mandatory spine + conditional tdd pin win), gate
# off ⇒ legacy-identical, triage persona vocabulary ("tdd-engineer")
# normalized to canonical "tdd". Invokes the system under test via
# `go test` exit code (assert.sh; cycle-137 lesson — never scrape PASS).
#
# Contract source: go/internal/router/router_psmas_test.go (5 tests).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/router/... 'TestPSMAS' || exit 1

echo "GREEN: PSMAS PhaseSkip wiring behavioral suite passes"
exit 0
