#!/usr/bin/env bash
# ACS cycle-171 T1 — phase-timing.json is written after RunCycle.
# Behavioral: assert_go_test_pass DRIVES RunCycle end-to-end via the white-box
# test and asserts the workspace file exists with one entry per phase run.
# Adding a magic string cannot satisfy this — the test reads the JSON back.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"
assert_go_test_pass ./internal/core/... 'TestPhaseTimingJSON_WrittenAfterRunCycle' || exit 1
