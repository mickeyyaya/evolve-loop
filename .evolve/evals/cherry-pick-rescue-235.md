---
score_cap:
  - criterion: "Ship-closure idempotency tests present and passing in ship package"
    max_if_missing: 7
    evidence: "grep -q 'func TestShip_PostPush_Idempotent_CorrectReportOnly' go/internal/phases/ship/closure_idempotency_test.go && cd go && go test -count=1 -run 'TestShip_PostPush_Idempotent_CorrectReportOnly|TestShip_PinPostCommitSha' ./internal/phases/ship/"
  - criterion: "Cycle-level bridge-failure classification tests present and passing in core"
    max_if_missing: 7
    evidence: "grep -q 'func TestOrchestrator_BridgeExhaustion_CycleLevelFailure' go/internal/core/cyclelevel_failure_test.go && cd go && go test -count=1 -run 'TestErrCycleLevelFailure_WrapsCauseForErrorsIs|TestOrchestrator_BridgeExhaustion_CycleLevelFailure|TestOrchestrator_RecoveryDepthBudget' ./internal/core/"
  - criterion: "Phase-boundary checkpoint tests present and passing in core + checkpoint"
    max_if_missing: 6
    evidence: "grep -q 'func TestOrchestrator_PhaseBoundaryCheckpoint' go/internal/core/orchestrator_phaseboundary_test.go && cd go && go test -count=1 -run 'TestOrchestrator_PhaseBoundaryCheckpoint|TestOrchestrator_FailedPhase_NoSuccessCheckpoint' ./internal/core/ && go test -count=1 -run 'TestReasonPhaseComplete_IsValid|TestComposeChecked_AcceptsPhaseComplete' ./internal/checkpoint/"
  - criterion: "Audit-phase binary-churn leak recovery wired in orchestrator with its test"
    max_if_missing: 7
    evidence: "grep -q 'discarded binary rebuild churn' go/internal/core/orchestrator.go && grep -q 'func TestOrchestrator_AuditLeakRecover' go/internal/core/orchestrator_auditleak_test.go && cd go && go test -count=1 -run 'TestOrchestrator_AuditLeakRecover' ./internal/core/"
  - criterion: "Landed features carry their permanent evals (no orphaned features)"
    max_if_missing: 5
    evidence: "git ls-files --error-unmatch .evolve/evals/ship-closure-idempotency.md .evolve/evals/cycle-level-bridge-failure.md .evolve/evals/phase-boundary-checkpoint.md .evolve/evals/audit-phase-leak-recover.md"
---

# Eval: Cherry-pick rescue/cycle-235-audited onto main (cycle 236 landing)

> Pins the cycle-236 landing operation: the failure-supervision-tree step-3
> work (cycle-level bridge-failure classification, phase-boundary checkpoints),
> ship-closure idempotency, and audit-phase binary-churn leak recovery — built
> and audit-PASSed in cycle 235 on `rescue/cycle-235-audited`, but never
> shipped because the ship gate rejected a stale `expected_ship_sha` pin.
> This eval is deliberately SHA-free (the rescue branch is expected to be
> deleted post-landing); it asserts on the landed test functions and
> orchestrator wiring instead. Source incidents: cycle 235 ship abort
> (stale pin) and the 232/233 landing saga that motivated the
> `ship-closure-idempotency` inbox item.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ship-idempotency | Post-push re-entry ships report-only, pins post-commit SHA | 7/10 | `go test -run 'TestShip_PostPush_Idempotent_CorrectReportOnly\|TestShip_PinPostCommitSha' ./internal/phases/ship/` |
| cycle-level-failure | Bridge exhaustion classifies as cycle-level, not batch-fatal | 7/10 | `go test -run 'TestOrchestrator_BridgeExhaustion_CycleLevelFailure\|...' ./internal/core/` |
| phase-boundary | Successful phases checkpoint at the boundary; failed phases don't | 6/10 | `go test -run 'TestOrchestrator_PhaseBoundaryCheckpoint\|...' ./internal/core/ ./internal/checkpoint/` |
| audit-leak-recover | Audit-phase tree-diff failure recovers from binary rebuild churn | 7/10 | `grep 'discarded binary rebuild churn' orchestrator.go` + `go test -run TestOrchestrator_AuditLeakRecover` |
| eval-integrity | Feature evals tracked (gitignore-shadow guard) | 5/10 | `git ls-files --error-unmatch .evolve/evals/<4 files>` |

The grep guards in the evidence commands are auxiliary anti-no-op checks:
`go test -run <re>` exits 0 with "no tests to run" when the named tests are
absent, so each behavioral run is paired with a func-presence grep that makes
deletion of the test file detectable. The `go test` exit code remains the
load-bearing signal (cycle-137 rule).
