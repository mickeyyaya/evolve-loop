---
score_cap:
  - criterion: "At the iteration top, a canonical cycle-state claiming a live phase for a dead cycle the batch already passed (SIGKILLed lane: fresh heartbeat + dead owner pid; leaseless record; phase=retro residue) is WARNed on the loop console and reconciled to Phase=aborted with ActiveAgent cleared and identity preserved; a second iteration top is quiet"
    max_if_missing: 9
    evidence: "cd go && go test -race -count=1 -run '^TestPrepareIteration_StaleCanonicalState$' ./cmd/evolve"
  - criterion: "A genuinely resumable unfinished cycle (unfinishedCycle shape: CycleID > lastCycleNumber) is left byte-identical, unwritten and un-WARNed by the iteration-top check"
    max_if_missing: 8
    evidence: "cd go && go test -race -count=1 -run '^TestPrepareIteration_ResumableCycleUntouched$' ./cmd/evolve"
  - criterion: "A record whose run lease is held by a live owner (the loop's own or a concurrent in-flight cycle) is never reconciled, even when its cycle id is at or below lastCycleNumber"
    max_if_missing: 8
    evidence: "cd go && go test -race -count=1 -run '^TestPrepareIteration_OwnInFlightCycleUntouched$' ./cmd/evolve"
  - criterion: "A cleanly completed lane's closed-out record and terminal/fresh records (aborted, end, CycleID 0) are never rewritten or WARNed about"
    max_if_missing: 7
    evidence: "cd go && go test -race -count=1 -run '^(TestPrepareIteration_CompletedCycleRecordUntouched|TestPrepareIteration_TerminalOrFreshRecordUntouched)$' ./cmd/evolve"
  - criterion: "The check is wired at loopBatchCoordinator.prepareIteration: through the runLoop entrypoint the reconcile lands before the batch's own cycle writes state, and it fires at a non-first iteration under both wave and pool fleet configs, race-clean"
    max_if_missing: 9
    evidence: "cd go && go test -race -count=1 -run '^TestPrepareIteration_WiredBeforeEveryIteration$' ./cmd/evolve"
  - criterion: "An unreadable canonical record is surfaced on the loop console and never overwritten; a failed reconcile write is surfaced, not swallowed"
    max_if_missing: 6
    evidence: "cd go && go test -race -count=1 -run '^(TestPrepareIteration_CoherenceReadErrorSurfacesWithoutWrite|TestPrepareIteration_ReconcileWriteErrorSurfaces)$' ./cmd/evolve"
  - criterion: "Production changes are confined to go/cmd/evolve/cmd_loop_{window,blockerbreaker,control}.go; go/internal/loopwave/, cmd_loop_wave.go, cmd_loop_chain.go, core/cyclerun.go and core/cyclerun_dispatch.go are untouched"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1693_004_ScopeConfinedToIterationTopFiles$' ./acs/cycle1693"
---

# Eval: iteration-state-coherence-sentinel — per-iteration canonical-state coherence check

> `unfinishedCycle` (go/cmd/evolve/cmd_loop_control.go) is the only guard that
> reads canonical `cycle-state.json` as possibly stale. It runs once per batch,
> in `prepareFreshBatch`, and trips only on `CycleID > lastCycleNumber`.
> `loopBatchCoordinator.prepareIteration` (go/cmd/evolve/cmd_loop_window.go)
> runs before every dispatch, sequential or fleet (cmd_loop_batch.go:137). It
> never reads canonical state.
>
> `cmd_fleet.go`'s WaitDelay escalation can SIGKILL a fleet lane. The lane then
> dies before its `abnormalEpilogue` state floor
> (go/internal/core/cyclerun_epilogue.go) can write `Phase="aborted"`. Its
> canonical record keeps claiming a live phase for a dead cycle, and once the
> batch has passed that cycle, nothing mid-batch can see the record.
>
> Source: the console investigation of 2026-07-22 and the premise audit of
> 2026-09-26, which found the gap still present. Cycle 1693 is the TDD cycle
> that pinned this contract. The frozen RED suite is
> `go/cmd/evolve/cmd_loop_iteration_state_coherence_test.go`; the cycle
> predicates are `go/acs/cycle1693/predicates_test.go`.
>
> The negative caps matter as much as the positive ones. A sentinel that
> relabels a live lane, a resumable cycle or a shipped cycle as "aborted" is
> strictly worse than having no sentinel.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| stale-reconciled | stale record → WARN + aborted, idempotent | 9/10 | `go test -race -run '^TestPrepareIteration_StaleCanonicalState$' ./cmd/evolve` |
| resumable-untouched | unfinishedCycle shape byte-identical | 8/10 | `go test -race -run '^TestPrepareIteration_ResumableCycleUntouched$' ./cmd/evolve` |
| live-owner-untouched | live lease ⇒ never reconciled | 8/10 | `go test -race -run '^TestPrepareIteration_OwnInFlightCycleUntouched$' ./cmd/evolve` |
| no-over-correction | completed / terminal / fresh records untouched | 7/10 | `go test -race -run '^(…CompletedCycleRecordUntouched\|…TerminalOrFreshRecordUntouched)$' ./cmd/evolve` |
| wiring | runLoop + wave + pool at prepareIteration, -race | 9/10 | `go test -race -run '^TestPrepareIteration_WiredBeforeEveryIteration$' ./cmd/evolve` |
| fail-loudly | read/write errors surfaced, unread record never clobbered | 6/10 | `go test -race -run '^(…ReadErrorSurfacesWithoutWrite\|…WriteErrorSurfaces)$' ./cmd/evolve` |
| scope | only the three iteration-top files; protected surfaces untouched | 7/10 | `go test -tags acs -run '^TestC1693_004_ScopeConfinedToIterationTopFiles$' ./acs/cycle1693` |
