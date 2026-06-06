---
score_cap:
  - criterion: "ErrCycleLevelFailure sentinel exists with errors.As/errors.Is roundtrip to the cause"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestErrCycleLevelFailure_WrapsCauseForErrorsIs' ./internal/core/"
  - criterion: "Bridge exhaustion classifies cycle-level; integrity breaches stay batch-fatal"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run 'TestOrchestrator_BridgeExhaustion_CycleLevelFailure|TestOrchestrator_IntegrityBreach_StillBatchFatal' ./internal/core/"
  - criterion: "A 3-cycle batch survives a cycle-2 failure (loop continues, no rc=2)"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run 'TestLoop_CycleLevelFailureContinues' ./cmd/evolve/"
  - criterion: "audit-and-ship recovery loop bounded at maxRecoveryDepth; exhaustion is cycle-level"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestOrchestrator_RecoveryDepthBudget' ./internal/core/"
---

# Eval: Cycle-level bridge failure classification (batch survival)

> Pins Invariant 3's severity-inversion fix (campaign retro I-9): a bridge or
> phase error used to propagate from `orchestrator.go` to `cmd_loop.go`'s
> bare `break` as a batch-fatal rc=2 — three batch deaths this campaign
> (c225, c230, c231), including c230's signature of 3 PASSed audits / 0 ships
> before the batch died. Cycle 234 wraps the orchestrator's phase-failure
> exit in `ErrCycleLevelFailure` (cause preserved), keeps kernel-integrity
> sentinels (`ErrPhaseGateFailed`, `ErrLedgerChainBroken`, `ErrLockHeld`)
> batch-fatal, and teaches the loop to log + continue. Source incident:
> campaign retro cycles 215-231 §4 Invariant 3.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| sentinel-shape | ErrCycleLevelFailure wraps Phase + Cause | 5/10 | `go test -run TestErrCycleLevelFailure_WrapsCauseForErrorsIs ./internal/core/` |
| severity-classes | bridge=cycle-level, integrity=batch-fatal | 3/10 | `go test -run 'TestOrchestrator_BridgeExhaustion_CycleLevelFailure\|TestOrchestrator_IntegrityBreach_StillBatchFatal' ./internal/core/` |
| batch-survival | loop continues past a failed cycle | 3/10 | `go test -run TestLoop_CycleLevelFailureContinues ./cmd/evolve/` |
| recovery-budget | audit↔ship traversal capped | 6/10 | `go test -run TestOrchestrator_RecoveryDepthBudget ./internal/core/` |
