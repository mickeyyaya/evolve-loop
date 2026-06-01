#!/usr/bin/env bash
# ACS cycle-187 — Task 1 AC-1/AC-2/AC-7/AC-8: backfill coverage extended to
# retro + build-planner.
#
# Behavioral: runs the backfill package test suite (exit code authoritative via
# assert_go_test_pass). The two new tests construct a realistic <phase>-stdout
# .clean.txt and assert TryExtract reconstructs the artifact — they pass ONLY
# when phaseHeaders["retro"]/["build-planner"] exist with the right header
# strings (AC-1, AC-2). The whole-package run is AC-8.
#
# AC-7 guard: a bare `go test -run <regex>` exits 0 when nothing matches, so the
# named tests' EXISTENCE is asserted separately (the -run false-positive class).
# This grep is AUXILIARY to the behavioral package run above (mixed/acceptable).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

# AC-8 + AC-1 + AC-2: full backfill package GREEN (includes the two new tests).
assert_go_test_pass ./internal/backfill/... || exit 1

# AC-7: the two named tests must actually exist in the source.
TT="$(git rev-parse --show-toplevel)/go/internal/backfill/backfill_test.go"
grep -q 'func TestTryExtract_Retro_PositiveWritesArtifact' "$TT" \
  || { echo "RED: TestTryExtract_Retro_PositiveWritesArtifact missing from backfill_test.go" >&2; exit 1; }
grep -q 'func TestTryExtract_BuildPlanner_PositiveWritesArtifact' "$TT" \
  || { echo "RED: TestTryExtract_BuildPlanner_PositiveWritesArtifact missing from backfill_test.go" >&2; exit 1; }

echo "PASS: backfill covers retro + build-planner (AC-1/AC-2/AC-7/AC-8)"
exit 0
