---
score_cap:
  - criterion: "ReasonPhaseComplete is a canonical checkpoint reason (IsValid + ComposeChecked accept it)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestReasonPhaseComplete_IsValid|TestComposeChecked_AcceptsPhaseComplete' ./internal/checkpoint/"
  - criterion: "Orchestrator writes a phase-complete checkpoint block to cycle-state.json at every phase boundary (observable mid-cycle)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestOrchestrator_PhaseBoundaryCheckpoint' ./internal/core/"
  - criterion: "A failed phase never records itself as completed in the checkpoint"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestOrchestrator_FailedPhase_NoSuccessCheckpoint' ./internal/core/"
---

# Eval: Phase-boundary checkpoint (resume after any crash)

> Pins Invariant 3's checkpoint half (campaign retro cycles 215-231, §4):
> before cycle 234, a checkpoint block was written only at the quota wall
> (`ReasonQuotaLikely` / `ReasonBatchCapNear`), so a hard crash at any other
> phase boundary left `evolve loop --resume` with "no live checkpoint" —
> three failed resume attempts in the 215-231 campaign. Cycle 234 adds
> `ReasonPhaseComplete` and an additive checkpoint splice into
> `.evolve/cycle-state.json` after every completed phase. Source incidents:
> campaign retro I-9 resume-impossible class; cycle-232 abort destroyed the
> only copy of finished work.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| reason-constant | ReasonPhaseComplete valid + composable | 6/10 | `go test -run 'TestReasonPhaseComplete_IsValid\|TestComposeChecked_AcceptsPhaseComplete' ./internal/checkpoint/` |
| boundary-write | checkpoint block on disk mid-cycle, completedPhases includes just-completed phase | 4/10 | `go test -run TestOrchestrator_PhaseBoundaryCheckpoint ./internal/core/` |
| failure-path | failed phase not recorded as completed | 5/10 | `go test -run TestOrchestrator_FailedPhase_NoSuccessCheckpoint ./internal/core/` |
