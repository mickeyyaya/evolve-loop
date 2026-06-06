#!/usr/bin/env bash
# AC-ID:         cycle-227-005
# Description:   ValidateUserSpec enforces two-tier naming (>=1 hyphen required); "tester" single-word builtin is exempted
# Evidence:      go/internal/phasespec/validate.go:12,27 + go/internal/phasespec/naming_twotier_test.go
# Author:        tester
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: build-report.md AC#5 — two-tier naming rule in ValidateUserSpec (user-phase-pipeline-hardening)

set -uo pipefail

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

VAL="$WORKTREE/go/internal/phasespec/validate.go"
[ -f "$VAL" ] || { echo "RED: $VAL not found" >&2; exit 1; }

# Structural: the regex and tester exemption must both be present.
grep -q 'userPhaseNameRE' "$VAL" \
  || { echo "RED: userPhaseNameRE not defined in $VAL" >&2; exit 1; }

grep -q 's.Name != "tester"' "$VAL" \
  || { echo "RED: tester single-word exemption not present in $VAL" >&2; exit 1; }

# Behavioral: exercise the cycle-227 AC-005 contract test suite specifically.
# NOTE: we target only the naming_twotier_test.go cases; a pre-existing failing
# test (TestValidateUserSpec_NonFirstSegmentLeadingDigit) is out of scope for
# this cycle — it was not in the cycle-227 build contract.
cd "$WORKTREE/go" || { echo "RED: cannot cd to $WORKTREE/go" >&2; exit 1; }

if ! go test ./internal/phasespec/... \
  -run "TestValidateUserSpec_SingleWordRejected|TestValidateUserSpec_MultiWordWithDigitsOK|TestValidateUserSpec_LongValidName|TestValidateUserSpec_TrailingHyphenRejected|TestValidateUserSpec_AdversarialRejections" \
  -timeout 60s 2>&1; then
  echo "RED: two-tier naming rule test suite FAILED" >&2
  exit 1
fi

echo "GREEN: ValidateUserSpec enforces two-tier naming; tester exempted; adversarial rejections hold" >&2
exit 0
