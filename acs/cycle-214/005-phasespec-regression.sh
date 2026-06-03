#!/usr/bin/env bash
# ACS cycle-214 — the phasespec package still passes (AC1.5 regression guard).
#
# Uses the shared assert lib (cycle-137 lesson): assert_go_test_pass keys off
# `go test`'s EXIT CODE, never on scraping PASS/ok strings, and resolves the
# module dir itself. Adding declarative phase.json files must not break the
# loader/validator package.
set -uo pipefail

. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/phasespec/... || exit 1
exit 0
