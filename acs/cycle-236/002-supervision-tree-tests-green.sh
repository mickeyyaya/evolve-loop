#!/usr/bin/env bash
# ACS — cycle 236 / AC3 (part 1): the named supervision-tree, ship-closure-
# idempotency, and audit-leak-recover Go tests carried by the rescue commits
# RUN and PASS in the landed tree.
#
# Classification: BEHAVIORAL — invokes `go test` as a subprocess via
# acs/lib/assert.sh (exit-code authoritative, cycle-137 rule).
#
# Existence guard (load-bearing): `go test -run <re>` on a package where the
# named tests DON'T EXIST exits 0 with "no tests to run" — without the file
# checks below this predicate would be falsely GREEN pre-cherry-pick. The
# dual-check (disk + git tracking) also catches gitignore-shadow drops.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel)
cd "$TOP"
. "$TOP/acs/lib/assert.sh"

fail=0
for f in \
  go/internal/phases/ship/closure_idempotency_test.go \
  go/internal/core/cyclelevel_failure_test.go \
  go/internal/core/orchestrator_phaseboundary_test.go \
  go/internal/core/orchestrator_auditleak_test.go \
  go/internal/checkpoint/phaseboundary_test.go \
  go/cmd/evolve/cmd_loop_cyclelevel_test.go
do
  if [ ! -f "$f" ]; then
    echo "RED: $f missing on disk — rescue test contract not landed" >&2
    fail=1
    continue
  fi
  if ! git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
    echo "RED: $f untracked" >&2
    fail=1
  fi
done
[ "$fail" -eq 0 ] || exit 1

# Behavioral runs — exit code of go test is the authoritative signal.
assert_go_test_pass ./internal/phases/ship/ \
  'TestShip_PostPush_Idempotent_CorrectReportOnly|TestShip_PrePush_CorrectionStillFullShip|TestShip_BinaryChurnDiscarded|TestShip_SourceChangesPreserved|TestShip_PinPostCommitSha' \
  || exit 1
assert_go_test_pass ./internal/core/ \
  'TestErrCycleLevelFailure_WrapsCauseForErrorsIs|TestOrchestrator_BridgeExhaustion_CycleLevelFailure|TestOrchestrator_IntegrityBreach_StillBatchFatal|TestOrchestrator_RecoveryDepthBudget|TestOrchestrator_PhaseBoundaryCheckpoint|TestOrchestrator_FailedPhase_NoSuccessCheckpoint|TestOrchestrator_AuditLeakRecover' \
  || exit 1
assert_go_test_pass ./internal/checkpoint/ \
  'TestReasonPhaseComplete_IsValid|TestReason_NearMissSpellings_StayInvalid|TestComposeChecked_AcceptsPhaseComplete' \
  || exit 1
assert_go_test_pass ./cmd/evolve/ \
  'TestLoop_CycleLevelFailureContinues' \
  || exit 1

echo "GREEN: all named supervision-tree/ship-idempotency/audit-leak tests pass" >&2
exit 0
