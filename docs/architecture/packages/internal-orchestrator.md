# internal/orchestrator

> The spec of record: [wave-orchestrator §4](../wave-orchestrator.md#4-the-supply-census), [§7](../wave-orchestrator.md#7-the-rolling-pool), [§8](../wave-orchestrator.md#8-the-lane-lease) and [§9](../wave-orchestrator.md#9-the-oversight-loop). The decision: [ADR-0139](../adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md). The plan: [wave-orchestrator-2026-10](../../plans/wave-orchestrator-2026-10.md) (O4, O5, D8, D9, Q1). The scheduler under the pool: [internal-fleet.md](internal-fleet.md) (`RunPool`). The admission rule: [internal-admission.md](internal-admission.md). The loop wiring: [cmd-evolve.md](cmd-evolve.md).

## Purpose

`internal/orchestrator` runs the lanes of the loop at the stage `enforce`. It oversees them at both stages. It has five parts:

- **The pool (`Pool`).** It runs `fleet.RunPool` with admission by cell, a freshness check before each launch and one Signal Center event for each transition. A free slot takes the next admissible todo at once.
- **The lease (`Leases`).** Each lane gets a lease at launch. The lease holds the start, the deadline and the last heartbeat. One timer serves all lanes.
- **The intake (`Intake`).** It holds the state that the oversight changes: the lanes whose cells are free, the limit of the window and the stop.
- **The oversight (`Oversight`).** One goroutine reads the channels `loop`, `cycle`, `errors` and `ship`, and it maps each event to one action.
- **The census (`Census`).** It counts the supply before a plan and records the cause of each refusal.

Built on 2026-10-17 as O4 and O5, and on 2026-10-18 as the library of O6. The loop wiring of the oversight and of the census is pending. The package runs no LLM, starts no process and stops no process by itself.
> The spec of record: [wave-orchestrator §7](../wave-orchestrator.md#7-the-rolling-pool), [§8](../wave-orchestrator.md#8-the-lane-lease) and [§10](../wave-orchestrator.md#10-the-adaptive-window). The decision: [ADR-0139](../adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md). The plan: [wave-orchestrator-2026-10](../../plans/wave-orchestrator-2026-10.md) (O4, O5, O8, D8, D9, D13, Q1). The scheduler under the pool: [internal-fleet.md](internal-fleet.md) (`RunPool`). The admission rule: [internal-admission.md](internal-admission.md). The loop wiring: [cmd-evolve.md](cmd-evolve.md).

## Purpose

`internal/orchestrator` runs the lanes of the loop at `fleet.orchestrator.stage` `enforce`. It has three parts:

- **The pool (`Pool`).** It runs `fleet.RunPool` with admission by cell, a freshness check before each launch and one Signal Center event for each transition. A free slot takes the next admissible todo at once.
- **The lease (`Leases`).** Each lane gets a lease at launch. The lease holds the start, the deadline and the last heartbeat. One timer serves all lanes.
- **The window (`Window`).** It holds the number of lanes that the pool admits. It grows by 1 for each clean landing and halves on each landing conflict.

Built on 2026-10-17 as O4 and O5, and on 2026-10-18 as O8, of the wave-orchestrator plan. The package runs no LLM, starts no process and stops no process by itself.
Built on 2026-10-17 as O4 and O5, and on 2026-10-18 as the library of O6. The loop wiring of the census is built (O6, 2026-10-18; see [cmd-evolve.md](cmd-evolve.md)). The loop wiring of the oversight is pending. The package runs no LLM, starts no process and stops no process by itself.

## Design

- **The pool hooks.** `Pool.Run` builds a `fleet.PoolConfig` and calls `fleet.RunPool`:
  - `Next` is `Pool.Admit`. `AdmitByCell` adapts an `admission.Admitter`. The lanes in flight are the holdings, and the pending todos are the candidates in rank order. The first admitted todo takes the slot. A nil `Admit` keeps the file rule of `RunPool`.
  - `Prepare` is `Pool.Freshen`. The loop gives `loopwave.Engine.Freshen`, which runs `fleet.FreshenSpecs` on the one spec just before its launch. A stale todo does not launch, and the slot goes to the next todo.
  - `MaxLaunches` is `fleet.orchestrator.run_max_lanes`. `Halted` is the operator stop.
  - `Resupply` is `Pool.Resupply`. The loop gives the plan source of the run (`orchestratorResupply` in `cmd/evolve`). A nil port keeps the backlog of the run start.
  - No hook re-queues a stopped todo. After a deadline stop, `recover` reaps the lane and releases its claim. The todo does not launch again in the same run, and a later run can take it. The second stop of the todo in the batch parks it ([pool-resupply-2026-10](../../plans/pool-resupply-2026-10.md) D8).
- **The transitions.** `onTransition` publishes one INFO `loop.wave` event for each `fleet.PoolTransition`: `LOOP_ORCH_LANE_LAUNCHED`, `LOOP_ORCH_LANE_EXITED`, `LOOP_ORCH_LANE_BACKFILLED`, `LOOP_ORCH_POOL_RESUPPLIED` and `LOOP_ORCH_POOL_IDLE`. Each event has `live` and `target`, and each lane event has `todo`.
- **The lease table.** `Leases.Watch(launch, settle)` wraps a `fleet.LaunchFn`. It makes a child context for the lane, grants the lease, runs the launch and returns the lease. The key of a lane is its scope joined with `+`. When two lanes have the same key, the second return finds no lease. That lane then returns an error that names the key, and no panic occurs (`TestLease_AGiveWithNoLeaseIsAnErrorNotAPanic`, `TestLease_ADuplicateKeyFailsTheSecondLaneLoudly`).
- **The observed heartbeat (O6).** `Leases.Beat` records the time of an event of the lane. It clamps a time stamp in the future to the clock, so a producer with a fast clock cannot hide a stall (`TestLease_ABeatFromTheFutureCountsAsNowAndNeverPostponesTheStall`). `Leases.Stall` publishes the stall once for a lane, and it does nothing when `stall_minutes` is 0 (`TestLease_AZeroStallTurnsTheReportedStallOffToo`). `Leases.Epoch` counts the grants and the gives (`TestLease_EpochCountsEachGrantAndEachGive`).
- **One timer over a heap.** The heap holds one entry for each lane: the earlier of its deadline and its stall time. A changed lane gets a new entry, and the old entry goes stale by its sequence number. After each change, the table stops its timer and arms one `Clock.AfterFunc` for the top of the heap. `fire` pops each due entry, checks the lane, pushes its next time and arms the timer again. The cost of each lane event is O(log N).
- **The heartbeat port.** `Heartbeat(Lane)` returns the time of the freshest live signal of the lane, or false when the heartbeat is unknown. `RunsHeartbeat` finds the newest `runs/cycle-N` that is not older than the lane and that holds the first todo of the lane in its `lane-scope.json` (`LaneCycle`). The heartbeat is the latest time of two kinds. One is the last time stamp of its `signals.ndjson`. The other is the modification time of each `*-pane-watch.json`, `*-events.ndjson` and `*-stdout.log` in that directory.
  - The lease adds no writer.
  - An unknown heartbeat (no cycle holds the lane) reports no stall. The lease checks it again after `stall_minutes`.
  - A known heartbeat older than the launch counts from the launch.
  - A zero stall time turns the stall check off, so the lease arms only the deadline (`TestLease_AZeroStallChecksNoStallAndArmsOnlyTheDeadline`). The policy never gives zero: it resolves 0 to the default of 30.
- **The deadline.** `DeadlineOf(laneTimes, floor)` is the nearest-rank 95th percentile of the lane times without the longest one, at least the floor. One sample stays. The reason is in [wave-orchestrator §8](../wave-orchestrator.md#8-the-lane-lease). `LaneTimes(runsDir, 50)` reads the lane time of each of the last 50 cycles as the span from the first to the last event of its `signals.ndjson`.
- **The stop.** At `enforce`, a passed deadline cancels the child context of that lane. The launch of `cmd/evolve` (`execCycleLaunch`, through `sysexec.Command`) then sends SIGTERM to that one lane process. The package holds no pid and imports no `os/exec`, `syscall`, `sysexec` or `proctree`.
- **The recovery.** After a stopped lane exits, `Pool` runs three steps. A failed step publishes `LOOP_ORCH_LANE_RECOVERY_FAILED` with the step and the error.
  - The reap. `Pool` calls its `Reap` port with the lane. The loop gives `reapLane` of `cmd/evolve`, which runs the `evolve gc dispatch` sweep for the dispatch ids of that lane only.
  - The seal. When the loop gives the `Locate` port (`orchestrator.LaneCycle` over the runs directory), `Pool` seals the cycle of the lane. It publishes one INFO `cycle.sealed` with `Cycle: N`, `termination_reason: stopped: lane deadline` and no verdict. The loop signal center writes it to the cycle stream (`TestPool_AStoppedLaneSealsItsCycleWithTheLaneDeadlineTermination`, `TestPool_TheDeadlineSealPassesTheSignalCenterAsACycleSeal`).
    - The oversight loop ignores this seal. It records no verdict and keeps the cycle mapping. Thus a later FAIL seal of the lane still frees its cells (`TestOversight_TheLoopSealOfADeadlineStopIsNoVerdictAndKeepsTheMapping`).
  - The release. `Pool` releases the inbox claim through `Claims.Release`. At the second stop of one todo, `Claims.Park` routes it to the console with the reason `deadline twice`.
- **The window.** `NewWindow(WindowConfig{Start, Min, Sized}, publish)` returns a `*Window` (`window.go`). One mutex guards it, because the oversight loop writes it while the pool reads it.
  - `CleanLanding()` adds 1. `Conflict()` halves and rounds down. `Resize(sized)` sets the sized count. Each call then goes through one clamp: not above the sized count, and not under `Min` (`window.go:49-51`). A `Min` above the sized count pins the window to the sized count. Thus a width of 4 or less gets no effect with the default keys.
  - `Size()` returns the size. A call that leaves the size as it was publishes nothing. A call that changes it publishes `LOOP_ORCH_WINDOW` once, after the mutex is free, with `old`, `new`, `sized` and `cause`.
  - The mutex is free during the publish, so a publisher can read the window. The oversight loop is the one writer. A second writer can reorder the published events, because the publish runs after the unlock.
  - `*Window` has the methods `CleanLanding()` and `Conflict()` for the landing events. The O6 loop can thus hold it through a small interface of its own. The package defines no such interface.
  - A zero or negative key in `WindowConfig` clamps to 1. Policy refuses such a key first.
- **The pool gate.** `Pool.Window` and `Pool.Stage` are fields of `Pool`. `admissionWindow` gives `Window.Size` to `fleet.PoolConfig.Window` only when the window is set and the stage is `enforce` (`pool.go:71-76`). `fleet.RunPool` then admits a lane while the lanes in flight number fewer than the smaller of its width and the window ([internal-fleet.md](internal-fleet.md)). The pool never stops a lane when the window shrinks.
- **The loop wiring.** `laneWindow` of `cmd/evolve` makes one window for each run, and prints `[loop] orchestrator: adaptive window (stage S): N of M lane(s), min K` ([cmd-evolve.md](cmd-evolve.md)).
  - It takes `window.start` and `window.min` from the policy. The sized count M is the smaller of `fleet.count` and `fleet.concurrency`, which is the pool width. Thus the window never exceeds the pool width, and each halving binds at once.
  - `laneWindow` runs only inside `orchestratorPool`, and `dispatchPool` builds that pool only at `enforce` (`cmd_loop_window.go:172-174`). At `shadow`, the loop builds no window and publishes no `LOOP_ORCH_WINDOW`.
  - A run is one boundary-to-boundary unit, so each run starts at `window.start`.
- **What feeds the window.** Nothing yet. The O6 loop must call `CleanLanding` and `Conflict` on each `ship.landed` and each landing conflict, and `Resize` on each bench event ([wave-orchestrator §9](../wave-orchestrator.md#9-the-oversight-loop)). Until then, the window stays at `window.start`.
- **Not the landing window.** This window gates lane admission. The composition window of the landing queue (`fleet.landing_queue.window`, [fleet-landing-queue §4](../fleet-landing-queue.md#4-the-composition-order)) is a different window with different keys.

- **The intake (O6).** `Intake.Gate` wraps the admission function of the pool. It refuses a new lane when the lanes in flight reach the limit. It drops the lanes that `Free` marked from the holdings, so their cells admit a new todo. `Pool.Run` makes an intake when the caller gives none.
  - `Free` and `Resize` kick the pool through `Wake`. `fleet.PoolConfig.Kick` makes the pool fill its open slots at once, with no lane exit. Many kicks make one (`TestIntake_StopIsStickyAndEveryChangeThatCanAdmitKicksOnce`).
  - `Hold` runs in the `Prepare` hook of each launch. A todo that launches again holds its cells again (`TestPool_ALaunchClearsTheFreedMarkOfAnEarlierLifeOfTheSameTodo`).
  - `Stop` ends all admission through the `Halted` hook. The lanes in flight finish (`TestPool_AStopOnTheIntakeEndsAdmissionAndLanesInFlightFinish`).
  - A pool that has an intake needs an admit function. Without one, `Pool.Run` fails every lane with `errIntakeNeedsAdmit` and launches nothing, because the limit and the freed cells act through admission (`TestPool_AnIntakeWithNoAdmitFunctionRefusesEveryLaneLoudly`). A caller whose module graph did not load passes no intake, and the pool admits by the file rule.
- **The oversight (O6).** `Oversight.Run` calls `Next` of the event reader with no deadline. The reader arms the kernel wake, reads to the end of each log and waits. A spurious wake reads again and acts on nothing (`TestOrchestrator_WakesOnlyOnEvents`). `Subscribe` builds the reader for the four channels with the compiled default routes of [event-channels §2](../event-channels.md).
  - `Run` refuses to start, with an error, if an events, leases, intake, locate or resize port is missing (`TestOversight_RunRefusesToStartWithAMissingPort`). The publish port can be nil.
  - `handle` is one `switch`. It maps each event to one method (`wave-orchestrator.md` §9 has the table). The order matters: a stall code is tested before the observer module, because a stall report is not a heartbeat.
  - `laneOf` finds the lane of a cycle and says if that life of the lane is still live. The mapping stays after the lane exits, until the `cycle.sealed` event of the cycle comes. Thus the last events of a lane are counted and recorded when they arrive after the exit (`TestOversight_ASealAndALandingAfterTheirLaneLeftAreCountedAndRecorded`). Such an event frees no cells, because the exit freed them. A lane that exits before the oversight reads any event of its cycle is not found.
  - An event of an earlier life of the same todo never touches the new life. It frees no cells, moves no heartbeat and reports no stall (`TestOversight_ALateEventOfAnEarlierLifeOfTheSameTodoIsCountedAndFreesNothingOfTheNewLife`, `TestOversight_AStallReportAndAHeartbeatOfAnEarlierLifeNeverTouchTheNewLife`).
  - A cycle that no lane holds is located once for each change of the lease table (`Leases.Epoch`). The scan does not cache a miss while a lane has no cycle yet, because that lane can own the cycle (`TestOversight_AForeignCycleIsLocatedOncePerChangeOfTheLiveSet`, `TestOversight_AnUnlocatedLaneIsLookedForAgainSoItsFirstEventIsNeverLostToTheNegativeCache`).
  - `RunsLocator` is the production port. It uses `LaneCycle` and keeps the cycle of each life of a lane. A new life of the same todo must get a cycle above that of the earlier life (`TestRunsLocator_ANewLifeNeverTakesTheCycleOfAnEarlierLifeOfTheSameTodo`). It keeps its cache without a lock, because the oversight is its only caller.
  - `do` takes one `action` value: the names, the lane, the extra fields and the function to apply. It applies the function only at `enforce`, and it always publishes one `LOOP_ORCH_OVERSIGHT_ACTION` event with `actions`, `lane`, `cycle`, `event`, `stage` and `applied`. `stamped` clones the fields it gets (`TestOversight_StampedNeverMutatesItsInput`).
  - The verdict field and the value `FAIL` come from `cyclestate`. The three codes of the bridge (`LIVENESS_PHASE_STALLED`, `LIVENESS_PANE_EXHAUSTED` and `BRIDGE_USAGE_EVIDENCE`) are copies, because the package imports no bridge package. `TestCopiedCodes_EqualTheConstantsOfTheirOwnersAndTheOwnersRegisterThem` imports the owners and fails if a copy differs or if the owner no longer registers it. The `usageevidence` package imports `bridge` and `bridgechain`, so it cannot be imported here.
  - The events of one channel set reach the oversight once. One event in two channels gives one action and one count (`TestOversight_OneLandingEventInTwoChannelsIsOneActionAndOneCount`).
  - A rebase code marks the lane. The next `ship.landed` of the lane counts as a conflict. A landing that no rebase code preceded counts as clean. The count goes to `LandingObserver` (`CleanLanding` and `Conflict`). The adaptive window of plan O8 (`*Window`) has both methods, so it needs no adapter.
  - A quota signal runs the `Resize` port. The port returns the window and the families that have no fallback. The oversight publishes `LOOP_ORCH_FAMILY_PAUSED` once for each new family, and it sets the limit of the intake.
  - A `loop.halt` event stops all admission.
- **The census (O6).** `Census.Count` reads the inbox through `inboxbatch.ScanDir` and `inboxmover.RankLaneMenu`, and it writes nothing (`TestSupply_RecordsTheCauseOfEachRefusal` compares every byte of the inbox). `Take` also appends the record and publishes `LOOP_ORCH_SUPPLY`, and `LOOP_ORCH_SUPPLY_SHORT` when the admissible count is below the window.
  - The record file is `.evolve/waves/supply.json`. It holds the last 12 censuses, and each write is atomic. `ReadSupply` and `SupplyIn` read it. A broken file is an error, so the operator sees it.
  - `AppendSupply` holds an exclusive file lock (`supply.json.lock`) across its read, its change and its write, so two writers lose no record (`TestSupplyRecord_ConcurrentAppendsLoseNoRecord`). It waits at most 10 s for the lock, and a longer wait is an error (`TestSupplyRecord_AnAppendHoldsTheFileLockAcrossItsReadModifyWrite`). The lock and the write are ports of a `supplyStore`, so a test fails each one (`TestSupplyRecord_AFailedLockAndAFailedWriteAreErrorsThatNameTheRecord`).
  - `Census.Inbox.Companions` is the stage of the console companion check ([ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) decision 6, `workflow.repair.fixability.stage`). At `enforce`, a lane item with an open console companion counts as `waiting_deps`. At `shadow`, it counts as `admissible`, and the record gets the warning `would defer <id>: console_companion_pending: <companions>` (`TestSupply_ALaneItemWithAPendingConsoleCompanionWaitsAtEnforceAndWarnsAtShadow`). `cmd/evolve` reads the stage from the policy. A refused policy holds `shadow` with a WARN.
  - `Count` refuses a census with no clock (`TestSupply_CountRefusesACensusWithNoClock`).
  - `Supply.Total` is the count of every item that the census saw (`TestSupply_TotalIsTheCountOfEveryItemTheCensusSaw`).
  - The proposal step is the `Proposer` function value. It runs only when the admissible count is below twice the window (`TestSupply_TheSplitProposalStepRunsBelowTwiceTheWindowOnlyAndNilMeansNone`). It gets the `console_protected` items. A nil value proposes nothing. The loop and `evolve wave supply` pass `splitProposer` of `cmd/evolve`, which calls `inboxsplit.Proposals` (2026-10-20).
  - `consoleClass` reads the class from the reason prefix. `TestConsoleClass_FollowsTheReasonsThatTheRoutingAuthorityGives` pins each prefix against `inboxbatch.ConsoleRouted`, so a change of the reason text fails a test.

## Stage behaviour

| Event | `shadow` (the barrier wave) | `enforce` (the pool) |
|---|---|---|
| The window changes size | none: the loop builds no window at `shadow` | `LOOP_ORCH_WINDOW` once for each change. The pool admits a lane only while the lanes in flight number fewer than the size. |
| The window is narrower than the pool width | the barrier wave starts every lane | a lane exit frees a slot, and the pool fills it only up to the size |
| No heartbeat for `stall_minutes` | `LOOP_ORCH_LANE_STALLED` once for the lane | the same |
| The deadline passes | `LOOP_ORCH_LANE_DEADLINE` with `action` `recorded`. Nothing stops. | `LOOP_ORCH_LANE_DEADLINE` with `action` `stopped`, the stop and the claim release. A later run can take the todo. The second stop parks it. |
| A lane exits after its deadline | `LOOP_ORCH_LANE_LATE_EXIT` with `status` and `rc` | the same, with `stopped` true |

At `shadow`, the stage exit counts each `LOOP_ORCH_LANE_LATE_EXIT` with `status` `ok`. Such a lane passed, so a deadline stop at `enforce` was a loss.

| Event | `shadow` | `enforce` |
|---|---|---|
| `cycle.sealed` | `LOOP_ORCH_OVERSIGHT_ACTION`, `applied` false | the same with `applied` true. On `FAIL`, the cells of the lane are free |
| `ship.landed` | the same | the cells are free, `LandingObserver` counts the landing, and the pool fills an open slot |
| a quota signal | the record, and `LOOP_ORCH_FAMILY_PAUSED` with `applied` false | the limit of the intake changes |
| `loop.halt` | the record | admission stops |
| a phase signal, a stall report | the heartbeat moves and the stall publishes at both stages | the same |

## Invariants

- A free slot takes the next admissible todo at once, and a lane exit frees its cells (`TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce`).
- At `enforce`, a lane past its deadline stops and its claim is released. The todo does not launch again in the same run, even when the resupply gives it again (`TestLease_ALanePastItsDeadlineIsReleasedAndNotRelaunchedInTheSameRun`). A second stop in a later run parks it (`TestLease_ATodoStoppedInTwoRunsIsParkedForTheConsole`).
- The resupply port feeds a freed slot (`TestPool_ResupplyFeedsTheFreedSlotFromThePort`).
- At `shadow`, a passed deadline stops nothing and releases nothing (`TestLease_AShadowDeadlineRecordsTheEventAndStopsNothing`, `TestPool_ShadowLeasesNeverStopOrRelease`).
- Only the lane past its deadline stops (`TestLease_AnEnforcedDeadlineStopsOnlyThatLaneThroughItsInjectedStop`).
- A stall publishes once for each lane (`TestLease_ASilentLaneReportsItsStallOnce`). A fresh heartbeat moves the stall (`TestLease_AFreshHeartbeatPostponesTheStall`).
- A fresh pane-watch file with no signal for 25 minutes is not a stall (`TestLease_AFreshPaneWatchWithNoSignalFor25MinutesIsNotStalled`, `TestRunsHeartbeat_IsTheFreshestOfTheSignalsAndThePhaseFiles`). A lane whose cycle is not found reports no stall (`TestLease_ALaneWhoseCycleIsNotFoundEmitsNoStall`, `TestLease_AnUnknownHeartbeatReportsNoStall`).
- A stopped lane gets exactly one reap, and a lane that was not stopped gets none (`TestPool_AStoppedLaneIsReapedOnceAndAnUnstoppedLaneNever`, `TestPool_ShadowLeasesNeverReap`, `TestPool_AStoppedLaneWithNoClaimsIsReapedAndNotTriedAgain`).
- With no lane in flight, the pool admits the top todo even when admission refuses it, because nothing can conflict with an empty set (`TestPool_WithNoLaneInFlightTheTopTodoIsAdmitted`).
- Many lanes arm one timer (`TestLease_ManyLanesArmOneTimer`).
- A landing frees the cells of its lane while the lane still runs, and the pool fills the open slot (`TestOrchestrator_ShipLandedFreesTheCells`). A failed seal frees them too (`TestOversight_ASealedFAILFreesTheCellsOfItsLaneAndAPASSOnlyRecordsTheVerdict`). A passed seal only records the verdict.
- At `shadow`, the oversight frees nothing, counts nothing, resizes nothing and stops nothing, and it records each action (`TestOversight_AShadowComputesEveryActionAndChangesNothing`).
- A landing is a conflict when the stream of its lane held a rebase code. A rebase code of another cycle counts nothing (`TestOversight_ALandingIsAConflictWhenItsLaneStreamHeldARebaseCode`).
- A stall report publishes the stall once. The report, a repeat and the lease timer share one mark (`TestOversight_APhaseStalledEventPublishesTheLaneStallOnce`).
- A bench of a family with no fallback pauses the family once and shrinks the limit. A later signal with the same state adds no pause event (`TestOversight_ABenchWithNoFallbackPausesTheFamilyOnceAndShrinksTheLimit`).
- The census counts the six classes, records the cause of each refusal and never changes the inbox. It publishes `LOOP_ORCH_SUPPLY_SHORT` only below the window.
- The package never polls and never stops a lane by itself. `TestOrchestrator_NeverPollsAndNeverStopsALaneByItself` parses each production file and refuses `time.Sleep`, `time.Tick`, `time.NewTicker`, `syscall.Kill`, `os.FindProcess` and the imports `os/exec`, `sysexec` and `proctree`. `TestOrchestratorGuard_FindsAPlantedPollAndKill` proves that the guard finds them. The same guard now refuses `time.After` and `time.NewTimer`, so `time.AfterFunc` of the lease table is the only timer. `TestOrchestrator_ImportsNoBridgeAndNoDriverPackage` refuses the imports of `bridge`, `bridgechain`, `adapters/bridge`, `subagent` and `consensusdispatch`, because the orchestrator runs no LLM (`TestOrchestratorGuard_FindsASecondTimerAndAnLLMDispatchImport`).
- The window grows by 1 for each clean landing and halves, rounded down, on each landing conflict. It stays between `window.min` and the sized count (`TestWindow_GrowsOnACleanLandingAndHalvesOnAConflict`, `TestWindow_AConflictNeverHalvesBelowWindowMin`, `TestWindow_NeverGrowsPastTheSizedCount`).
- At or below `window.min` (default 4), the window is the sized count and never moves (`TestWindow_AtOrBelowWidthFourItIsTheSizedCount`).
- A smaller sized count clamps the window. A larger one does not open it (`TestWindow_ResizeDownBelowTheWindowClampsItAndResizeUpDoesNotJumpIt`, `TestWindow_AResizeBelowWindowMinPinsTheWindowToTheSizedCountAndTheFloorLiftsWithIt`).
- The window publishes `LOOP_ORCH_WINDOW` once for each change, and nothing for a call that changes nothing (`TestWindow_PublishesOnceForEachChangeWithTheOldSizeTheNewSizeAndTheCause`). The event goes out after the unlock (`TestWindow_APublisherCanReadTheWindowBecauseTheEventGoesOutAfterTheUnlock`). Concurrent calls keep the size in bounds (`TestWindow_ConcurrentLandingsResizesAndReadsKeepItInsideItsBounds`, run with `-race`).
- A smaller window never stops a lane in flight. It delays the next admission (`TestPool_AShrinkingWindowNeverStopsALaneInFlightAndOnlyDelaysTheNextAdmission`, `TestRunPool_TheWindowHookBoundsAdmissionAndAShrinkNeverStopsALaneInFlight`). A grown window fills each freed slot at the next exit (`TestPool_AGrowingWindowFillsEveryFreedSlotAtTheNextExit`).
- Only `enforce` lets the window gate admission (`TestPool_OnlyTheEnforceStageLetsTheWindowGateAdmission`). The loop wires the policy window into a real pool run (`TestDispatchPool_AtEnforceTheWindowFromThePolicyGatesTheRun`, `TestOrchestratorPool_WiresTheEnforceKeysAndTheFileRuleWithoutAGraph`). At `shadow`, the loop builds no window (`TestDispatchPool_AtShadowTheLoopBuildsNoWindow`).
- The window never exceeds the pool width, so a window never lifts the `fleet.concurrency` cap and each halving binds at once (`TestRunPool_AWindowNeverLiftsTheConcurrencyCap`, `TestLaneWindow_TheSizedCountIsTheSmallerOfTheFleetCountAndTheConcurrencyCap`).
- The package never polls and never stops a lane by itself. `TestOrchestrator_NeverPollsAndNeverStopsALaneByItself` parses each production file and refuses `time.Sleep`, `time.Tick`, `time.NewTicker`, `syscall.Kill`, `os.FindProcess` and the imports `os/exec`, `sysexec` and `proctree`. `TestOrchestratorGuard_FindsAPlantedPollAndKill` proves that the guard finds them.
- The package has 100% statement coverage in `go/.cover-strict` and is listed in `go/.apicover-enforce`.

## Findings

- **The pool supply was too shallow (cycle 1951, wave 129, 2026-10-19).** The pool ran 1 lane of 4. Its backlog held 5 todos, and the 4 todos after rank 0 conflicted with its cell, so `AdmitByCell` returned -1 for each. The cause was in the supply, not in `Pool`: the plan width was `fleet.count`, not `scan_k`. `loopwave.PoolSupply` now gives the pool up to `max(fleet.count, scan_k)` todos at `enforce`. `Pool` is unchanged.
  - `Target` bounds the live lanes, and `MaxLanes` bounds the run. The spec is [wave-orchestrator.md](../wave-orchestrator.md) §6.

- **The live lane time (2026-10-17, runtime plane, cycles 1873 to 1928).** 44 of the last 50 cycles have a `signals.ndjson` with a span. The median lane time is 39.0 minutes, the p95 is 98.9 minutes and the longest is 118.1 minutes. Without the longest lane, the p95 is 95.1 minutes. The deadline is thus 95.1 minutes, above the floor of 60 (plan Q1, `TestDeadlineOf_TheLiveSamplesOfPlanQ1GiveNinetyFiveMinutes`).
- **The soak at N = 16 (2026-10-17, simulated driver).** `evolve fleet soak --count 16 --pool` in a fresh scratch clone ran 64 lanes at `enforce`. The clone did not ignore `knowledge-base/cycles/`. The peak in-flight was 16 of 16, and 64 lanes passed. The run took 43.6 s, with 88.0 lanes a minute and a utilization of 0.91. No tree-diff guard stopped a phase.
- **A cross-lane finding of the soak, fixed in the guard.** The first soak run failed 54 of 64 pool lanes. The ship of one lane writes its dossier `knowledge-base/cycles/cycle-N.{json,md}` in the main tree. The tree-diff guard of a sibling lane counted those files as its own leak. The build recovery of a sibling also moved them into its own worktree. The barrier wave hides the race, because all lanes start at the same time.
  - The fix is in `internal/core` (`leak_ownership.go`). A dossier file is keyed to its cycle N. The dossier of another cycle with a run directory is not a leak of this lane, and recovery leaves it in the main tree. A file of the cycle of this lane stays a leak ([internal-core.md](internal-core.md)).
  - The soak above proves the fix with no exclusion.
- **The width-4 policy meets the window (2026-10-18).** PR #971 sets `count` 4 and `min_lanes` 4, with `orchestrator.stage` `enforce`. The default `window.start` is 4 and the default `window.min` is 4. Thus the pool runs all 4 lanes, and `.evolve/policy.json` has no `window` key.
  - *Implication:* a policy with a `count` above 4 must set `window.start` to `count` until the O6 loop calls `CleanLanding`. Without that key, the pool runs 4 lanes.

## Admission shadow record (ADR-0143 W1)

`RecordShadow` turns an `admission.ShadowAdmission` into the signal `LOOP_ORCH_ADMISSION_SHADOW` (INFO; `todo`, `held_by`, `rank`, `cells`, `reason`). `AdmitByCell` is unchanged: the admitter it wraps decides.

- **Once for each pair.** The pool asks admission again for each free slot, so one held todo comes back on each pass. The sink of one `RecordShadow` call publishes each pair of `todo` and `held_by` once. The loop builds one sink for each pool run, so the dedup lasts for one run. A mutex guards the seen set. Pinned by `TestRecordShadow_TwoPassesOverOneHeldTodoPublishOnce` and `TestRecordShadow_TheTodoAndItsHolderAreTheKeyNotTheReason`.
- **The census uses the same rule.** `Census.Optimistic` goes to the admitter of the census. At `fleet.orchestrator.admission` `enforce`, a todo that only meets the reach of a holding counts as `admissible`, not `waiting_cell`. The loop sets no `InFlight` today, so the holdings are the ready items of higher rank. Pinned by `TestCensus_AtEnforceACandidateThatOnlyMeetsARunningLaneReachIsAdmissible`.

## Optimistic admission record (ADR-0143 W1b)

At `fleet.orchestrator.admission` `enforce`, two lanes can run together that the reach rule kept apart. W1b records if such a pair fails at the ship, so that D3 of the [wave-latency plan](../../plans/wave-latency-2026-10.md) can decide from data.

- **The admission names the reach holder.** `ReachHolders.Record` is the `OnOptimistic` sink of the pool admitter. `Pool.Run` clears the record at each admission pass, so a record never outlives the pass that made it.
- **The pool names a family hold** (2026-10-21). `FamilyHolds.Record` is the `OnWait` sink of the pool admitter. It keeps each waiting todo whose `Decision.FamilyHold` is true, so a package-cell hold of a todo with a family is not listed. It forgets the todo when the pool launches it, or when a later decision holds it by a package cell. A todo that goes stale inside the pool stays listed until the pool ends, and each pool starts a new list. `Pool.transition` adds `family_held` to `LOOP_ORCH_POOL_UNDERFILLED`, so an operator reads a family hold, not a supply failure ([internal-admission.md](internal-admission.md) §Id family).
- **The launch carries it.** `Pool.prepare` takes the record of the launched todo after `Freshen`. It sets `EVOLVE_FLEET_REACH_HOLDER` (`ipcenv.FleetReachHolderKey`) in the launch environment and publishes `LOOP_ORCH_ADMISSION_OPTIMISTIC` (INFO; `todo`, `held_by`, `reason`). A todo that the reach rule admits too carries no holder. Pinned by `TestPool_ALaneAdmittedOnlyByTheOptimisticRuleCarriesItsWouldBeHolder`, `TestPool_AnEarlierOptimisticRecordDoesNotFollowALaterReachAdmission` and `TestPool_AFreshenDropKeepsNoOptimisticRecord`.
- **The lane reports its failure once.** `RecordOptimisticFailure` is a listener that the lane process subscribes when its environment has both a scope and a holder (`watchOptimisticAdmission`, `go/cmd/evolve/cmd_cycle_optimistic.go`). On the first composed-tree gate failure (`ORCHESTRATOR_COMPOSED_GATE_DECLINED`, gate from `fields.gates`) or fleet rebase conflict (`ORCHESTRATOR_FLEET_REBASE_CONFLICT`, gate `rebase`), it publishes `LOOP_ORCH_OPTIMISTIC_ADMISSION_FAILED` (WARN; `todo`, `held_by`, `gate`). Pinned by `TestRecordOptimisticFailure_PublishesOneSignalWithTheTodoTheHolderAndTheGate`.
- **The rate is a wave measure.** `wave status` counts both codes inside the run window and prints `optimistic F/A failed (rate)` ([internal-wavemeasure.md](internal-wavemeasure.md)).
