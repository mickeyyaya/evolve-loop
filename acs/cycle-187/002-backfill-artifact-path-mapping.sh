#!/usr/bin/env bash
# ACS cycle-187 — Task 1 AC-3/AC-4: backfillArtifactPath maps retro →
# "retrospective-report.md" and build-planner → "build-plan.md".
#
# Behavioral: runs the table-driven core test that calls backfillArtifactPath
# directly and asserts the path for every phase. The retro/build-planner rows
# fail at baseline (default branch yields *-report.md) and pass only after the
# switch cases are added. Exit code authoritative via assert_go_test_pass.
#
# AC existence guard: the -run regex would exit 0 if the test were deleted, so
# the test's presence is asserted separately (auxiliary to the behavioral run).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/core/... 'TestBackfillArtifactPath_AllPhases' || exit 1

TT="$(git rev-parse --show-toplevel)/go/internal/core/orchestrator_backfill_path_test.go"
grep -q 'func TestBackfillArtifactPath_AllPhases' "$TT" \
  || { echo "RED: TestBackfillArtifactPath_AllPhases missing" >&2; exit 1; }

echo "PASS: backfillArtifactPath maps retro + build-planner correctly (AC-3/AC-4)"
exit 0
