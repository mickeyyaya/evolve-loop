# internal/hostsentinel

## Purpose

`internal/hostsentinel` reads the host during a wave, compares each reading with its threshold, and signals each breach. At stage `enforce` it asks for the gc scopes that are safe during a wave, and below the floor it holds new lanes. The design is in [host-sentinel.md](../host-sentinel.md).

## Design

- **`Policy` and `Config`.** `Policy` is the `host_sentinel` block of `policy.json`. `Policy.Config` applies the defaults and refuses an unknown stage. The cache threshold comes from the caller, because it is the gc cap (`gc.go_cache_max_gb`). The package keeps no second cache setting.
- **`Sample` and `Reading`.** A sample has one reading for each metric. `Measured` keeps the value or the error. A failed reading is not a zero value, so `Evaluate` does not compare it.
- **`Evaluate`.** One table gives each metric its code, its direction (below or above) and its label. Each reading over its threshold is one `Breach`. A threshold of zero turns the check off.
- **`Ports`.** The caller gives the sampler, the release, the signal sink, the status writer and the warning writer. The package does not run a process and does not know the gc scope names. `cmd/evolve` maps each metric to its scope and runs `evolve gc --only <scope>`.
- **`Sentinel`.** `Tick` samples and evaluates. It asks for the release (a preview at `shadow`, an apply at `enforce`), emits the signals, sets the floor hold and saves the status. `Start` ticks once each interval, and its stop function cancels the ticker and waits for it. The first tick comes after one interval.
- **The throttle.** The sentinel emits each code one time in each interval. Only `Tick` writes the throttle map, and `Start` calls `Tick` from one goroutine. `HoldLanes` reads an atomic value, so a lane launch can read it from another goroutine.
- **`Status`.** `WriteStatus` and `ReadStatus` keep the last status in `.evolve/host-sentinel.json`. `Summary` gives the one line that `evolve wave status` shows.

## Invariants

- **A repeat in the interval does not flood.** Pinned by `TestTick_OverEachThresholdEmitsOneHostSignalAndARepeatInTheIntervalDoesNotFlood`.
- **Shadow releases nothing and holds no lane.** Pinned by `TestTick_AtShadowAsksOnlyForThePreviewAndHoldsNoLane`.
- **A failed sample warns and does not stop the loop.** Pinned by `TestTick_AFailedSampleWarnsOnceAndAFailedSaveWarnsWithoutAStop`.
- **The sentinel stops with the loop.** Pinned by `TestStart_TicksEachIntervalAndStopLeavesNoTicker` and `TestStart_TheLoopContextEndStopsTheSentinel`.
