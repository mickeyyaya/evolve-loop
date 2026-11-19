# Build Explanation — Cycle 2542

## Build Binding
- Cycle: 2542
- Base SHA: e919bfd1d1cbc78282f3f109704b5094cd7b5951

## Summary
Cycle start now runs the test-cache trim and checks percent-free disk space before dispatch. The guard prints a WARN line below 10% free, pauses new dispatches below 5% free, and resumes at or above 5% free. Failed or zero-total disk reads print a WARN line and do not pause dispatch.

## Rationale
`looppreflight.DiskFreeBytes` returned only free bytes, so percent-free thresholds needed total bytes from `statfs`. Placing `DiskUsageOf`, `DiskGuard`, and `RunCycleStart` in `looppreflight` avoids a new package and keeps disk probes in one home. Both `cycleLaunch.LaunchFn` and `loopBatchCoordinator.dispatchSequentialCycle` call `RunCycleStart`, so fleet, wave, campaign, and sequential cycles share the same guard.

## Changed Areas
- `go/internal/looppreflight/diskfree.go` — adds `DiskUsage` and `DiskUsageOf` so callers can read both free and total filesystem bytes.
- `go/internal/looppreflight/disk_unix.go` — computes `FreeBytes` and `TotalBytes` in `defaultDiskUsageOf` with `syscall.Statfs` and delegates `defaultDiskFreeBytes` to it.
- `go/internal/looppreflight/disk_other.go` — adds the unsupported-platform `defaultDiskUsageOf` stub and delegates `defaultDiskFreeBytes` to it.
- `go/internal/looppreflight/disk_percent.go` — adds `DiskGuardConfig`, `DefaultDiskGuardConfig`, `NewDiskGuard`, and `DiskGuard.Check` with 10% warn and 5% pause thresholds.
- `go/internal/looppreflight/cyclestart.go` — adds `CycleStartOptions` and `RunCycleStart` to run cache trim before the pause loop and stop when the context ends.
- `go/cmd/evolve/cmd_gc_testcache.go` — extracts `trimTestCache` so cycle start and `evolve gc` share the same test-cache trim helper.
- `go/cmd/evolve/cmd_cycle_start.go` — builds `CycleStartOptions` for production cycle paths from policy `gc` settings and test hooks.
- `go/cmd/evolve/cmd_fleet.go` — wires `looppreflight.RunCycleStart` into `cycleLaunch.LaunchFn` for fleet, wave, and campaign dispatches.
- `go/cmd/evolve/cmd_loop_batch.go` — adds the injectable `startHooks` field to `loopBatchCoordinator` for cycle-start tests.
- `go/cmd/evolve/cmd_loop_sequential_dispatch.go` — wires `looppreflight.RunCycleStart` into `dispatchSequentialCycle` for sequential cycles.
- `docs/architecture/packages/internal-looppreflight.md` — documents `DiskUsageOf`, `DiskGuard`, and `RunCycleStart` invariants.

## Design Decisions
- `DiskGuard` lives in `go/internal/looppreflight` instead of a new package because `looppreflight` owns `DiskFreeBytes` and `CheckDiskSpace`.
- `LaunchFn` skips the real disk check and trim when `Simulate` is true and no test hooks are set.
- Cycle start reuses `trimTestCache` with `testverdict.Trim` instead of spawning `go env GOCACHE`, which avoids a subprocess per cycle.

## Verification
Tests in `go/internal/looppreflight`, `go/cmd/evolve`, and `go/acs/cycle2542` verify trim order, threshold warnings, pause-resume hysteresis, context cancel, failed reads, real `statfs` measurement, and production callers.

## Compatibility
`DiskFreeBytes`, `CheckDiskSpace`, `gcRun.testCache`, and simulated fleet launches preserve their existing signatures and behavior.

## Limitations
The percent thresholds default to 10% warn and 5% pause from `DefaultDiskGuardConfig`; per-project policy keys are not added in this cycle.
