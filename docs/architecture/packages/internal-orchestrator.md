# internal/orchestrator

> The spec of record: [wave-orchestrator §7](../wave-orchestrator.md#7-the-rolling-pool) and [§8](../wave-orchestrator.md#8-the-lane-lease). The decision: [ADR-0139](../adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md). The plan: [wave-orchestrator-2026-10](../../plans/wave-orchestrator-2026-10.md) (O4, O5, D8, D9, Q1). The scheduler under the pool: [internal-fleet.md](internal-fleet.md) (`RunPool`). The admission rule: [internal-admission.md](internal-admission.md). The loop wiring: [cmd-evolve.md](cmd-evolve.md).

## Purpose

`internal/orchestrator` runs the lanes of the loop at `fleet.orchestrator.stage` `enforce`. It has two parts:

- **The pool (`Pool`).** It runs `fleet.RunPool` with admission by cell, a freshness check before each launch and one Signal Center event for each transition. A free slot takes the next admissible todo at once.
- **The lease (`Leases`).** Each lane gets a lease at launch. The lease holds the start, the deadline and the last heartbeat. One timer serves all lanes.

Built on 2026-10-17 as O4 and O5 of the wave-orchestrator plan. The package runs no LLM, starts no process and stops no process by itself.

## Design

- **The pool hooks.** `Pool.Run` builds a `fleet.PoolConfig` and calls `fleet.RunPool`:
  - `Next` is `Pool.Admit`. `AdmitByCell` adapts an `admission.Admitter`. The lanes in flight are the holdings, and the pending todos are the candidates in rank order. The first admitted todo takes the slot. A nil `Admit` keeps the file rule of `RunPool`.
  - `Prepare` is `Pool.Freshen`. The loop gives `loopwave.Engine.Freshen`, which runs `fleet.FreshenSpecs` on the one spec just before its launch. A stale todo does not launch, and the slot goes to the next todo.
  - `MaxLaunches` is `fleet.orchestrator.run_max_lanes`. `Halted` is the operator stop.
  - `Exited` returns true for a todo that the lease stopped once. That todo gets one more try at the head of the queue.
- **The transitions.** `onTransition` publishes one INFO `loop.wave` event for each `fleet.PoolTransition`: `LOOP_ORCH_LANE_LAUNCHED`, `LOOP_ORCH_LANE_EXITED`, `LOOP_ORCH_LANE_BACKFILLED` and `LOOP_ORCH_POOL_IDLE`. Each event has `live` and `target`, and each lane event has `todo`.
- **The lease table.** `Leases.Watch(launch, settle)` wraps a `fleet.LaunchFn`. It makes a child context for the lane, grants the lease, runs the launch and returns the lease. The key of a lane is its scope joined with `+`. When two lanes have the same key, the second return finds no lease. That lane then returns an error that names the key, and no panic occurs (`TestLease_AGiveWithNoLeaseIsAnErrorNotAPanic`, `TestLease_ADuplicateKeyFailsTheSecondLaneLoudly`).
- **One timer over a heap.** The heap holds one entry for each lane: the earlier of its deadline and its stall time. A changed lane gets a new entry, and the old entry goes stale by its sequence number. After each change, the table stops its timer and arms one `Clock.AfterFunc` for the top of the heap. `fire` pops each due entry, checks the lane, pushes its next time and arms the timer again. The cost of each lane event is O(log N).
- **The heartbeat port.** `Heartbeat(Lane)` returns the time of the freshest live signal of the lane, or false when the heartbeat is unknown. `RunsHeartbeat` finds the newest `runs/cycle-N` that is not older than the lane and that holds the first todo of the lane in its `lane-scope.json` (`LaneCycle`). The heartbeat is the latest time of two kinds. One is the last time stamp of its `signals.ndjson`. The other is the modification time of each `*-pane-watch.json`, `*-events.ndjson` and `*-stdout.log` in that directory.
  - The lease adds no writer.
  - An unknown heartbeat (no cycle holds the lane) reports no stall. The lease checks it again after `stall_minutes`.
  - A known heartbeat older than the launch counts from the launch.
  - A zero stall time turns the stall check off, so the lease arms only the deadline (`TestLease_AZeroStallChecksNoStallAndArmsOnlyTheDeadline`). The policy never gives zero: it resolves 0 to the default of 30.
- **The deadline.** `DeadlineOf(laneTimes, floor)` is the nearest-rank 95th percentile of the lane times without the longest one, at least the floor. One sample stays. The reason is in [wave-orchestrator §8](../wave-orchestrator.md#8-the-lane-lease). `LaneTimes(runsDir, 50)` reads the lane time of each of the last 50 cycles as the span from the first to the last event of its `signals.ndjson`.
- **The stop.** At `enforce`, a passed deadline cancels the child context of that lane. The launch of `cmd/evolve` (`execCycleLaunch`, through `sysexec.Command`) then sends SIGTERM to that one lane process. The package holds no pid and imports no `os/exec`, `syscall`, `sysexec` or `proctree`.
- **The recovery.** After a stopped lane exits, `Pool` first calls its `Reap` port with the lane. The loop gives `reapLane` of `cmd/evolve`, which runs the `evolve gc dispatch` sweep for the dispatch ids of that lane only. Then `Pool` releases the inbox claim through `Claims.Release`. At the second stop of one todo, `Claims.Park` routes it to the console with the reason `deadline twice`. A failed step publishes `LOOP_ORCH_LANE_RECOVERY_FAILED` with the step and the error.

## Stage behaviour

| Event | `shadow` (the barrier wave) | `enforce` (the pool) |
|---|---|---|
| No heartbeat for `stall_minutes` | `LOOP_ORCH_LANE_STALLED` once for the lane | the same |
| The deadline passes | `LOOP_ORCH_LANE_DEADLINE` with `action` `recorded`. Nothing stops. | `LOOP_ORCH_LANE_DEADLINE` with `action` `stopped`, the stop, the claim release, then one more try or the park |
| A lane exits after its deadline | `LOOP_ORCH_LANE_LATE_EXIT` with `status` and `rc` | the same, with `stopped` true |

At `shadow`, the stage exit counts each `LOOP_ORCH_LANE_LATE_EXIT` with `status` `ok`. Such a lane passed, so a deadline stop at `enforce` was a loss.

## Invariants

- A free slot takes the next admissible todo at once, and a lane exit frees its cells (`TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce`).
- At `enforce`, a lane past its deadline stops, its claim is released and it gets one more try (`TestLease_ALanePastItsDeadlineStopsAndReleasesItsItem`). A second stop parks it (`TestLease_ATodoStoppedTwiceIsParkedForTheConsole`).
- At `shadow`, a passed deadline stops nothing and releases nothing (`TestLease_AShadowDeadlineRecordsTheEventAndStopsNothing`, `TestPool_ShadowLeasesNeverStopOrRelease`).
- Only the lane past its deadline stops (`TestLease_AnEnforcedDeadlineStopsOnlyThatLaneThroughItsInjectedStop`).
- A stall publishes once for each lane (`TestLease_ASilentLaneReportsItsStallOnce`). A fresh heartbeat moves the stall (`TestLease_AFreshHeartbeatPostponesTheStall`).
- A fresh pane-watch file with no signal for 25 minutes is not a stall (`TestLease_AFreshPaneWatchWithNoSignalFor25MinutesIsNotStalled`, `TestRunsHeartbeat_IsTheFreshestOfTheSignalsAndThePhaseFiles`). A lane whose cycle is not found reports no stall (`TestLease_ALaneWhoseCycleIsNotFoundEmitsNoStall`, `TestLease_AnUnknownHeartbeatReportsNoStall`).
- A stopped lane gets exactly one reap, and a lane that was not stopped gets none (`TestPool_AStoppedLaneIsReapedOnceAndAnUnstoppedLaneNever`, `TestPool_ShadowLeasesNeverReap`, `TestPool_AStoppedLaneWithNoClaimsIsReapedAndNotTriedAgain`).
- With no lane in flight, the pool admits the top todo even when admission refuses it, because nothing can conflict with an empty set (`TestPool_WithNoLaneInFlightTheTopTodoIsAdmitted`).
- Many lanes arm one timer (`TestLease_ManyLanesArmOneTimer`).
- The package never polls and never stops a lane by itself. `TestOrchestrator_NeverPollsAndNeverStopsALaneByItself` parses each production file and refuses `time.Sleep`, `time.Tick`, `time.NewTicker`, `syscall.Kill`, `os.FindProcess` and the imports `os/exec`, `sysexec` and `proctree`. `TestOrchestratorGuard_FindsAPlantedPollAndKill` proves that the guard finds them.
- The package has 100% statement coverage in `go/.cover-strict` and is listed in `go/.apicover-enforce`.

## Findings

- **The live lane time (2026-10-17, runtime plane, cycles 1873 to 1928).** 44 of the last 50 cycles have a `signals.ndjson` with a span. The median lane time is 39.0 minutes, the p95 is 98.9 minutes and the longest is 118.1 minutes. Without the longest lane, the p95 is 95.1 minutes. The deadline is thus 95.1 minutes, above the floor of 60 (plan Q1, `TestDeadlineOf_TheLiveSamplesOfPlanQ1GiveNinetyFiveMinutes`).
- **The soak at N = 16 (2026-10-17, simulated driver).** `evolve fleet soak --count 16 --pool` in a fresh scratch clone ran 64 lanes at `enforce`. The clone did not ignore `knowledge-base/cycles/`. The peak in-flight was 16 of 16, and 64 lanes passed. The run took 43.6 s, with 88.0 lanes a minute and a utilization of 0.91. No tree-diff guard stopped a phase.
- **A cross-lane finding of the soak, fixed in the guard.** The first soak run failed 54 of 64 pool lanes. The ship of one lane writes its dossier `knowledge-base/cycles/cycle-N.{json,md}` in the main tree. The tree-diff guard of a sibling lane counted those files as its own leak. The build recovery of a sibling also moved them into its own worktree. The barrier wave hides the race, because all lanes start at the same time.
  - The fix is in `internal/core` (`leak_ownership.go`). A dossier file is keyed to its cycle N. The dossier of another cycle with a run directory is not a leak of this lane, and recovery leaves it in the main tree. A file of the cycle of this lane stays a leak ([internal-core.md](internal-core.md)).
  - The soak above proves the fix with no exclusion.
