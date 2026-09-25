# Build Explanation — Cycle 1693

## Build Binding
- Cycle: 1693
- Base SHA: 4b77281a01fb23e90c9a3adc4cde4d57a5da361e

## Summary
The loop now checks canonical `cycle-state.json` at the top of every batch iteration. Before this change it only checked once per batch. If the record says a phase is still running, but the cycle's owner is provably dead and the batch has already passed that cycle, the loop prints a WARN on its console. It then rewrites the record the same way `abnormalEpilogue`'s state floor does: `Phase="aborted"`, `ActiveAgent` cleared, and every identity field kept.

## Rationale
When `cmd_fleet.go`'s WaitDelay escalation SIGKILLs a fleet lane, the lane dies before its `abnormalEpilogue` defer can mark the record aborted. The record then keeps claiming a live phase for a dead cycle. The only existing guard that treats canonical state as possibly stale is `unfinishedCycle`. It runs once per batch in `prepareFreshBatch`, and it only trips when `CycleID > lastCycleNumber`. A lane the batch has already moved past is therefore invisible to it. `loopBatchCoordinator.prepareIteration` is the one call site that runs before every dispatch, whether sequential, wave or pool. That makes it the smallest place that covers all three paths.

## Changed Areas
- `go/cmd/evolve/cmd_loop_control.go` — adds `reconcileStaleCycleState`, `claimsLivePhase` and `cycleOwnerDead` next to `unfinishedCycle`, which is the guard they extend. A record is reconciled only when all four hold: its phase is live, it is not the `unfinishedCycle` shape, its owner is dead, and every read succeeded.
- `go/cmd/evolve/cmd_loop_window.go` — calls the sentinel from `prepareIteration` after the interrupt check and before the blocker-breaker check and pre-wave probes. It therefore runs at every iteration top before any dispatch, and even a batch the breaker halts leaves a coherent record.
- `go/cmd/evolve/cmd_loop_iteration_state_coherence_test.go` — the TDD phase's frozen RED contract (positive, negative, error and wiring cases), committed unchanged so the ship keeps it.
- `go/acs/cycle1693/predicates_test.go` — the TDD phase's cycle predicates, which run the frozen suite and check the lane's scope. Committed unchanged.
- `.evolve/evals/iteration-state-coherence-sentinel.md` — the task's eval and its score caps. Committed unchanged.

## Design Decisions
- **Owner liveness reuses `runlease.OwnerLive`** with `runlease.PIDAlive` and `runlease.DefaultTTL`, the same liveness logic gc uses. A SIGKILLed lane has a fresh heartbeat but a dead pid, and `OwnerLive` already treats that as dead. A missing lease also counts as dead.
- **A live phase excludes completed phases.** Both completion paths, `phaseCompletionRecord.persist` and `completeRetro`, append the phase to `CompletedPhases` and leave `Phase` on it. A cleanly finished cycle's record is therefore history, not residue, and is never relabelled. `aborted`, `end`, an empty phase and `CycleID 0` are never live.
- **The `unfinishedCycle` shape is left alone** because it belongs to `evolve loop --resume` / `evolve cycle reset`. `lastCycleNumber` is re-read from storage at every iteration rather than taken from the batch-start snapshot, so the check matches the cycles completed so far.
- **The sentinel fails loudly and conservatively.** If the cycle-state, state.json or the lease cannot be read, it WARNs and leaves the record untouched. It never reconciles a record it could not fully read. A failed reconcile write is also WARNed. The sentinel never halts the batch.
- Alternative rejected: running the check inside `unfinishedCycle`/`prepareFreshBatch`. That still runs only once per batch, so it cannot see a lane killed mid-batch.

## Verification
- The frozen suite (`go test -race -count=1 -run '^(TestPrepareIteration_…)$' ./cmd/evolve`) passes 19/19 under the race detector. That covers the three stale shapes plus idempotence, the resumable, live-owner, completed and terminal negatives, the read and write error cases, and wiring through `runLoop`, wave and pool.
- The full `go test -count=1 ./cmd/evolve` suite passes, and `go test -tags acs ./acs/cycle1693` passes 5/5.

## Compatibility
There is no schema, flag or policy change. The reconciled record uses the same shape the abnormal epilogue already writes, and fresh trees and healthy records produce no output.

## Limitations
- A dead lane whose id is still above `lastCycleNumber` is left to the resume machinery until a later cycle completes past it.
- Between a new cycle's init write and its first lease heartbeat, the record has no lease. The sentinel only acts on ids at or below `lastCycleNumber`, and minting runs in order, so this window is theoretical.
