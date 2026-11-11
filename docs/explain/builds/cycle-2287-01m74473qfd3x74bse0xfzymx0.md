# Build Explanation — Cycle 2287

## Build Binding
- Cycle: 2287
- Base SHA: 7000301dd23f969e5d966c5fc677c9ad9e60bb93

## Summary
A cycle that seals now has its dirty lane, its cycle branch and its orphan processes cleaned at once. This holds on three paths: the sequential loop, the resume batch and the fleet lane that runs through `evolve cycle run`. The change adds the sweep `gc.SweepSealedCycle` (`go/internal/gc/cycleseal.go:40`) and its three callers. Plan decisions D12 and X10 define this step. The plan is docs/plans/gc-closes-the-legacy-gap-2026-11.md.

## Rationale
Before this change no sweep of a sealed cycle existed. The sweep is new in `go/internal/gc/cycleseal.go`. The sequential loop (`go/cmd/evolve/cmd_loop_sequential_dispatch.go:44`) and the resume batch (`go/cmd/evolve/cmd_loop_resume.go:78`) call it, and so does `runCycleRun` (`go/cmd/evolve/cmd_cycle.go:270`) for a fleet lane. The fleet package never touches lane worktrees. Without the third call, no later step releases such a lane at once.

The sweep also checks the run lease through `runLiveElsewhere` (`go/internal/gc/cycleseal.go:77`). This check ignores the lease of the calling process. The calling process holds the lease of the cycle that it just sealed. Without this fix, the sweep sees its own lease as a live run and keeps the lane.

## Changed Areas
- `go/cmd/evolve/cmd_cycle.go` — `runCycleRun` calls `sweepSealedCycleGC` after the cycle seals without error and after the inbox closeout. The sweep reads the gc mode and the holds, so the caller adds no second hold.
- `docs/architecture/packages/internal-gc.md` — the invariant of the cycle-end sweep lists the third caller and its test names.
- `go/cmd/evolve/cmd_loop_sealed_gc.go` — new `sweepSealedCycleGC` reads the gc mode, acts only at `enforce`, and calls `gc.SweepSealedCycle` with an injected process signal sender. One home serves all three callers.
- `go/cmd/evolve/cmd_loop_outcome.go` — the gc mode check moves into `loopGCMode`, so the end-of-loop gc and the cycle-end sweep read the mode in one place.
- `go/cmd/evolve/cmd_loop_resume.go` — the resume batch calls `sweepSealedCycleGC` after a resumed cycle ends. The stop reason moves into `resumeStopReason`, which keeps the function small.
- `go/cmd/evolve/cmd_loop_sequential_dispatch.go` — the sequential loop calls `sweepSealedCycleGC` after each cycle result.
- `go/internal/gc/cycleseal.go` — new `SweepSealedCycle` releases the lanes, the branch and the orphan processes of a sealed cycle. It keeps the lane of a pause, a continuation or a live run of another process.
- `go/internal/gc/branchsweep.go` — `dropRef` is extracted so the sealed sweep and the branch sweep share the branch removal.
- `go/internal/gc/processes.go` — `reapProcesses` is extracted so the sealed sweep and the finished-cycle reap share the kill loop.
- `go/internal/gc/worktrees.go` — `runLive` is extracted from the lane-live check so the sealed sweep asks the same run-lease question.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the three new `runCycleRun` tests as serial, because they set the process environment.
- `CHANGELOG.md` — records the fix for the operator.

## Design Decisions
The call sits after `closeoutCycleOutcome`, so the inbox claims of the cycle are settled before the lane goes. An error, quota or system-failure exit returns earlier and does not sweep. The sweep keeps all decisions (pause, continuation, live run, other cycles), so one home holds the safety rules.

## Verification
`TestRunCycleRun_ASealedFleetLaneCycleHasItsDirtyLaneBranchAndOrphanCleanedAtOnce` drives `runCycleRun` with `--simulate`. It fails before the change and passes after it. The shadow and off modes keep the lane, and the live lane of another cycle stays. `runCycleRun` is covered 100.0% under `-tags integration`.

## Compatibility
The sweep acts only at `gc.mode: enforce`. At shadow and off modes the sequential loop, the resume batch and the fleet lane keep their lanes as before.

## Limitations
This change does not build step X11. A lane that a live run of another process holds stays until a later sweep.
