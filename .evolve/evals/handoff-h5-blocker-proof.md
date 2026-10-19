---
score_cap:
  - criterion: "The cycle 1950 conflict returns blocked with both ask ids through the exit decision"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestDecide_TheCycle1950ConflictBlocksWithBothAskIds' ./internal/handoff/exit"
  - criterion: "An ask that needs go/internal/core/phase_bindings_selfcheck.go returns blocked with the protected path"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestDecide_AnAskThatNeedsAProtectedPathBlocksWithThePath' ./internal/handoff/exit"
  - criterion: "A ledger that claims blocked with no proof returns unmet and retry, never blocked"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestDecide_ABlockedClaimWithoutProofIsUnmet|TestDecide_AClaimDoesNotExcuseAnotherUnmetRequiredAsk' ./internal/handoff/exit"
  - criterion: "The exit package holds no protected-path matcher and does not import core or guards"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1970_004_TheProofReusesTheBlockerExitSeamAndAddsNoMatcher' ./acs/cycle1970"
---

# Eval: Phase handoff H5, the deterministic blocker proof

> Pins the blocked exit of `go/internal/handoff/exit` (ADR-0141, plan component H5). A blocked exit holds only when the injected prover, which the console wires to `core.NewBlockerProver`, proves the claim. The cycle 1946 and cycle 1950 stalls are the fixtures. Source incident: cycles 1946 and 1950 spent three correction rounds on asks no lane could meet.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| conflict-blocks | cycle 1950 asks return blocked with both ids | 4/10 | `go test -run TestDecide_TheCycle1950ConflictBlocksWithBothAskIds ./internal/handoff/exit` |
| protected-blocks | selfcheck ask returns blocked with the path | 4/10 | `go test -run TestDecide_AnAskThatNeedsAProtectedPathBlocksWithThePath ./internal/handoff/exit` |
| claim-needs-proof | an unproven blocked claim is unmet | 5/10 | `go test -run TestDecide_ABlockedClaimWithoutProofIsUnmet ./internal/handoff/exit` |
| no-second-matcher | the seam alone decides; exit imports neither core nor guards | 6/10 | `go test -tags acs -run TestC1970_004 ./acs/cycle1970` |
