---
score_cap:
  - criterion: "At a chain/wave boundary (n>0), when HEAD carries commits beyond the running binary's build SHA, the loop rebuilds, re-pins with verified provenance, and re-execs — never mid-batch/mid-lane"
    max_if_missing: 8
    evidence: "cd go && go test -C . -run '^(TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary|TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers|TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch|TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch|TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger)$' ./cmd/evolve"
  - criterion: "A successful boundary refresh is ledgered under a distinguishable, auditable 'boundary-refresh' authorization class (never silent)"
    max_if_missing: 6
    evidence: "cd go && go test -C . -run '^(TestLastChainBoundaryRefreshLogEntry_ReturnsMostRecentEntry|TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent)$' ./cmd/evolve"
  - criterion: "Any staleness/rebuild/ahead-check failure, and a live sibling fleet lane holding a fresh run lease, both refuse the rebuild and degrade to refreshed=false (never rebuild the shared plane binary mid-batch)"
    max_if_missing: 8
    evidence: "cd go && go test -C . -run '^(TestMaybeRefreshChainBoundary_AheadCheckErrorDegradesToNoRefresh|TestDefaultChainBoundaryAhead_GitFailureDegradesToSkip|TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh|TestMaybeRefreshChainBoundary_FleetLaneActiveRefusesRebuild|TestMaybeRefreshChainBoundary_FleetLaneCheckErrorRefusesRebuild)$' ./cmd/evolve"
  - criterion: "A second boundary hit at the same running commit refuses the second rebuild attempt (re-exec loop breaker)"
    max_if_missing: 6
    evidence: "cd go && go test -C . -run '^TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker$' ./cmd/evolve"
  - criterion: "No second, superseded stop-only staleness code path exists alongside the shipped rebuild+re-pin+re-exec boundary design"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -run TestC1373_005_no_superseded_stop_only_design_reintroduced ./acs/cycle1373"
---

# Eval: Auto-refresh the running binary at batch/chain boundaries

> Pins the `auto-refresh-binary-at-boundary` inbox item's fix contract
> (P1, weight 0.94): a chained/looping `evolve loop` process must not keep
> running a stale binary for hours across many batch boundaries while
> fixes land on `origin/main` mid-chain (the sentinel tail-anchor fix at
> cycle-1301 64f8620e sat inert across cycles 1302-1309 as the concrete
> incident). The mechanism — `maybeRefreshChainBoundary`
> (`go/cmd/evolve/cmd_loop_chain.go`) — is called at both the plain
> wave/fleet boundary inside `runLoop` (`cmd_loop.go:552`) and the chain
> boundary inside `runLoopChain` (`cmd_loop_chain.go:660`): ahead-check
> against HEAD, fleet-lane guard (refuses while a sibling lane holds a
> fresh run lease — never rebuilds the shared plane binary mid-batch),
> rebuild, provenance-gated re-pin, an auditable "boundary-refresh"
> authorization-class ledger entry, then re-exec. A loop-breaker marker
> refuses a second rebuild attempt at the same running commit so a rebuild
> that doesn't move the binary can never livelock the chain at zero
> batches executed.
>
> TDD finding (cycle 1373, and identically by cycles
> 1314/1323/1340/1343/1352/1356/1368/1370 before it): this contract is
> ALREADY fully implemented, wired, and green on this worktree's HEAD —
> no new production code was warranted this cycle. The score caps below
> regression-lock the shipped behavior for this and future cycles' audit
> gates rather than re-deriving it from scratch each time the inbox item
> resurfaces in fleet_scope.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| boundary-fires-not-midbatch | Refresh fires at wave/chain boundaries, never mid-batch/mid-lane | 8/10 | `go test -run '^(TestRunLoop_Calls...\|TestRunLoopChain_Boundary...)$' ./cmd/evolve` |
| auditable-ledger | Successful refresh ledgered under a distinguishable, non-silent class | 6/10 | `go test -run '^(TestLastChainBoundaryRefreshLogEntry...\|TestChainResultAndLoopResult...)$' ./cmd/evolve` |
| failopen-fleetlane-guard | Any check failure / live sibling lane refuses the rebuild | 8/10 | `go test -run '^(TestMaybeRefreshChainBoundary_AheadCheckError...\|...FleetLane...)$' ./cmd/evolve` |
| loop-breaker | Second same-commit attempt is refused | 6/10 | `go test -run '^TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker$' ./cmd/evolve` |
| no-superseded-design | No duplicate stop-only staleness path reintroduced | 4/10 | `go test -tags acs -run TestC1373_005_... ./acs/cycle1373` |
