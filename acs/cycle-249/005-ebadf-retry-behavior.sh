#!/usr/bin/env bash
# ACS — cycle-249 task `macos-ebadf-test-hardening`
# Behavioral: the EBADF-injection tests exercise captureWithEBADFRetry
# (transient EBADF absorbed, persistent EBADF surfaces after exactly one
# retry, non-EBADF errors pass through unretried). Then two structural
# negatives enforce the task's "test-infra only" boundary (scout gate:
# no production ship/ change).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"
top=$(git rev-parse --show-toplevel)

assert_go_test_pass ./internal/phases/ship/... 'TestCaptureWithEBADFRetry' || exit 1

# Negative 1: the helper must live ONLY in _test.go files — referencing it
# from production code would smuggle a behavior change into ship/.
prod_refs=$(grep -rl 'captureWithEBADFRetry' "$top/go/internal/phases/ship/" --include='*.go' \
  | grep -v '_test.go' | wc -l | tr -d ' ')
if [ "$prod_refs" -ne 0 ]; then
  echo "RED: captureWithEBADFRetry referenced from $prod_refs non-test ship/ file(s) — task is test-infra only" >&2
  exit 1
fi

# Negative 2: no uncommitted production-file edits under ship/ (scout
# verifiableBy). Vacuously green once the builder commit lands; the
# load-bearing tripwire pre-commit.
cd "$top" || exit 1
if git diff HEAD -- go/internal/phases/ship/ | grep '^+++' | grep -v '_test.go' | grep -v '/dev/null' >/dev/null; then
  echo "RED: production (non-_test.go) ship/ files modified — task must not change production code" >&2
  git diff HEAD --stat -- go/internal/phases/ship/ >&2
  exit 1
fi

echo "GREEN: EBADF retry behavior verified; change is test-infra only"
exit 0
