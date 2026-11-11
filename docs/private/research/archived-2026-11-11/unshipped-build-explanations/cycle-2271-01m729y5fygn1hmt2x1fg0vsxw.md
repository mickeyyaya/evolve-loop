# Build Explanation — Cycle 2271

## Build Binding
- Cycle: 2271
- Base SHA: 67a8d49f6e51f8d7b1d26e44d71d883df4e848da

## Summary
When a cycle seals, the loop now runs a gc sweep for that cycle only. The sweep stops the orphan processes in the lane of the cycle. It saves the lane and verifies the bundle. Then it releases the lane and the cycle branch. Plan decisions D12 and X10 define this step.

## Rationale
Before the change, a finished lane waited for the age rules of the wave-end sweep. The disk debt of 2026-11-09 grew until a manual run. A seal is a stronger proof of the end than an age. The sweep reuses the existing salvage, bundle and process code, so the safety rules stay in one home.

## Changed Areas
- `go/internal/gc/cycleseal.go` — adds `SweepSealedCycle` and `SealedCycleReport`. The sweep reads three holds (pause, continuation, live lease) and matches only the lanes and branches of the sealed cycle number. The pause hold reads the plane marker and the per-run `cycle-state.json`. The lease hold ignores a lease that the sweeping process owns, because the finished run leaves such a lease in the loop process. A lease of another live process still holds the sweep.
- `go/internal/gc/worktrees.go` — extracts `runLive` from `isLive`, so the sweep and the plan share one lease check.
- `go/internal/gc/branchsweep.go` — extracts `dropRef` from `deleteBundled`, so both deletes share one ref removal.
- `go/internal/gc/processes.go` — extracts `reapProcesses` from `ReapFinishedCycleOrphans`, so both reapers share one signal loop.
- `go/cmd/evolve/cmd_loop_sealed_gc.go` — adds `sweepSealedCycleGC`, the loop caller. It acts only at `gc.mode: enforce`.
- `go/cmd/evolve/cmd_loop_outcome.go` — extracts `loopGCMode` from `runGCHook`, so the hook and the sweep read the mode in one place.
- `go/cmd/evolve/cmd_loop_sequential_dispatch.go` — calls the sweep after a sequential cycle ends without error.
- `go/cmd/evolve/cmd_loop_resume.go` — calls the sweep after a resumed cycle ends without error.
- `go/internal/gc/cycleseal_failure_test.go` — covers each failure branch of the sweep with a git fault or a corrupt input.
- `docs/architecture/packages/internal-gc.md` — adds the invariant of the cycle-end sweep.

## Design Decisions
The sweep does not call `isLive` for the sealed lane, because the plane marker and `run.json` of a sealed run still name the lane. It checks the three holds directly. An unmerged branch gets a verified bundle before the lane goes. A landed branch gets no bundle, because the bundle of a landed branch holds the whole history.

## Verification
The gc tests and the cmd tests of the sweep pass. Each function of `cycleseal.go` and `cmd_loop_sealed_gc.go` is covered 100%. The nine ACS predicates of cycle 2271 pass.

## Compatibility
The sweep runs only at `gc.mode: enforce`. The shadow and off modes signal nothing. The three extractions keep the behavior of the existing sweeps.

## Limitations
A run lease of another live process holds the sweep, and the lane then stays. Step X11 is not built, so no later sweep cleans such a lane. A cycle that ends with an error is not swept.
