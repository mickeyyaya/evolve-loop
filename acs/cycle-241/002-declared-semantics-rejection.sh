#!/usr/bin/env bash
# ACS — cycle-241 step-5 task 2: a non-empty fail_if_signal without the
# Stage-3 signal bus is rejected at authoring time — evaluateClassify returns
# VerdictFAIL with a Severity:"error" diagnostic naming fail_if_signal (the
# silent-WARN path is the retro-215-231 Practice-4 defect class). Also pins
# the regression surface: require_sections / verdict_on_pass behavior in the
# same table is unchanged. Behavioral: runs the specrunner test binary.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/phases/specrunner/ \
  'TestEvaluateClassify|TestEvaluateClassify_FailIfSignal_RejectsWithErrorSeverity' \
  || exit 1
echo "PASS"
exit 0
