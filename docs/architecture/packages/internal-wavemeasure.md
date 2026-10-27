# internal/wavemeasure

> Spec: [wave-orchestrator.md §11](../wave-orchestrator.md#11-measures). Plan: [wave-orchestrator-2026-10](../../plans/wave-orchestrator-2026-10.md) (D14, O7). Operator use: [wave-operations.md](../../operations/wave-operations.md) §2. The reader of the facts: `internal/wave` (`ReadCycles`, `ReadLoopSignals`, `MeasureFacts`, `ApplyLaneDeadlines`).

## Purpose

`internal/wavemeasure` computes the measures of one wave run. They are the sized and the realized width, the utilization, the conflict rate and the count of admission waits. It also gives the cost of the wave ([ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md)). It is pure. It reads no file and starts no timer. The caller gives the facts and the clock.

## Design

- **The facts.** `Facts` holds the run window (`Start`, `End`), the sized width, one `Lane` for each cycle of the run, and the admission-wait count. A `Lane` holds its interval, `Landed` and `Conflict`.
- **The clock is an argument.** `Compute(f, now)` uses `now` for a zero `End`: a live run, or a lane in flight. Thus a test gives a fixed clock, and the command gives `env.now()`.
- **One sweep.** `Compute` clips each lane to the run window and sweeps the start and end edges in time order. The sweep gives the time at each width. The time-weighted median is the smallest width whose cumulative time is at least half the run time. Idle time counts as width 0, so a run with long gaps has a low realized width.
- **No NaN.** Each ratio has a guard for a zero denominator: no landed lane, a sized width of 0, or a run time of 0. Each such ratio is 0, so the JSON always encodes.
- **A lane with no start is not a lane.** A cycle with no signal and no dossier phase has no interval. It is out of the width and the busy time, but it still counts in `landed` when it shipped.
- **The supply is a copy.** `Facts.Supply` holds the six class counts of the last census of the run. `orchestrator.Supply.Measure` builds it. `Compute` copies it into `Measures.Supply`, and the JSON key `supply` is absent when the run has no census (`TestCompute_CarriesACopyOfTheClassCountsOfTheLastCensusAndOmitsAbsentOnes`). The census is in [internal-orchestrator.md](internal-orchestrator.md). `wave status` reads the record `.evolve/waves/supply.json` and takes the last census inside the run (`TestWaveStatus_ReportsTheSupplyOfTheLastCensusOfEachRun`).
- **The optimistic failure rate (ADR-0143 W1b).** `Facts.OptimisticAdmissions` and `Facts.OptimisticFailures` give `optimistic_failure_rate`. It is the failures over the admissions, or 0 with no admission. It is the D3 rate of the wave-latency plan. Pinned by `TestCompute_TheOptimisticFailureRateIsFailuresOverAdmissions`.

  `wave.MeasureFacts` counts each todo once, over the same loop run as the pool lanes (`loopRun`: from the run-id time to the run end). An admission is a todo with an optimistic launch by the run loop pid. A failure is an admitted todo that failed in that window. The lane process has its own pid, so a failure matches by time only. Thus the rate stays in [0,1], and a lane launched before `started_at` counts. Pinned by `TestMeasureFacts_AFailureCountsOnlyWhenItsAdmissionIsCounted` and `TestMeasureFacts_TheOptimisticCountsUseThePoolLaneWindowOfTheRunLoop`.
- **The cost (ADR-0158).** `Facts.Cycles` holds one `CycleSpend` for each cycle of the run: its ship outcome and its `cyclespend.Spend`. `Compute` puts the cost into `Measures.Cost`. The arithmetic is `cyclespend.Totals`:

  | Field | Formula |
  |---|---|
  | `started_cycles`, `shipped_cycles` | the cycles of the run, and the ones that shipped |
  | `llm_minutes_per_shipped_cycle` | the LLM minutes of all cycles of the run, divided by the shipped cycles, rounded to 0.1 |
  | `fresh_tokens_per_shipped_cycle`, `cache_read_tokens_per_shipped_cycle` | the tokens of all cycles, divided by the shipped cycles, rounded to 1 |
  | `unmeasured_calls` | the LLM calls whose `usage_status` is not `measured` |
  | `audit_rounds_per_cycle` | the sum of `audit_dispatches`, divided by the started cycles, rounded to 0.01 |
  | `convergence_rate` | the shipped cycles divided by the started cycles |
  | `handoff_stops` | the cycles that the fixability check sealed `handed-off` (`REPAIR_HANDOFF` with `decision: stop`) |
  | `handoff_would_stops` | the repair rounds that started with a hand-off finding that blocks (`decision: would_stop`, ADR-0158 metric 5) |
  | `false_stops` | the cycles with a `would_stop` that then shipped (ADR-0158 decision 26). The target before a flip to `enforce` is 0. |

  A failed cycle adds its spend to the numerator, because its spend bought no ship. With no shipped cycle, each figure for each shipped cycle is `null`, not 0. With no cycle, the two rates are `null` too (`TestCompute_AWaveWithNoShipHasNoDefinedCost`, `TestCompute_AWaveWithNoCycleHasNoDefinedRate`). `wave status` prints one `cost:` line under the measures line (`cmd/evolve` `TestWaveStatus_TextPrintsOneCostLine`).
- **The JSON names are a contract.** `TestMeasures_TheJSONFieldNamesAreStable` pins the eleven top-level keys of `Measures`, and `cost` is one of them. `TestCost_TheJSONFieldNamesAreStable` pins the keys of `cost`. When a hand-off count is not 0, `wave status` adds the three counts to the `cost:` line. It prints `handed-off (<reason>: <blockers>)` on the line of a handed-off cycle (`cmd/evolve` `TestWaveCycleText_AHandedOffCycleShowsItsReason`). The constants `FieldWidthSized` and `CodeAdmitWait` are the names that the writer and the reader share.

## Where the facts come from

| Fact | Writer | Reader |
|---|---|---|
| the lane interval | the pool: `LOOP_ORCH_LANE_LAUNCHED` or `LOOP_ORCH_LANE_BACKFILLED` opens it, and `LOOP_ORCH_LANE_EXITED` of the same `fields.todo` closes it. Only the signals of the wave loop count. They have the `pid` of the record and a time from the run ID (`gcpolicy.ParseLogRunID`) to the run end. | `wave.MeasureFacts` over `wave.ReadLoopSignals` |
| the live lanes now | `fields.live` of the latest pool signal of the wave loop | `wave.PoolLive`, shown as `pool:    live N` |
| the lane interval, in a run with no pool signals | the cycle's signals (`.evolve/runs/cycle-N/signals.ndjson`), widened by the dossier phase times | `wave.ReadCycles` (`Cycle.StartedAt`, `Cycle.EndedAt`) |
| a deadline stop | the pool: one `cycle.sealed` with `termination_reason: stopped: lane deadline` in the cycle stream, after the reap (`Pool.recover`). A run from before 2026-10-21 has only the lease signal `LOOP_ORCH_LANE_DEADLINE` with `action: stopped`. | `wave.ReadRunCycles`, the one reader of `wave status`, `wave watch` and `wave next`. It reads the seal through `wave.ReadCycles`. Then `wave.ApplyLaneDeadlines` is the fallback: it finds the cycle of that todo (`lane-scope.json`) that started last before the stop. When that cycle has no verdict and no termination, it sets `Termination: stopped: lane deadline` and ends the cycle at the stop. |
| `Conflict` | ship: `recoverFromShipError` calls `recordShipError` before it recovers. Each ship error thus emits a `ship.error` with the code `SHIP_` plus the error code | `wave.ReadCycles` (`Cycle.Conflict`) |
| `Landed` | the landing intent or the dossier commit | `wave.ReadCycles` (`Cycle.Shipped`) |
| `Cycles` (the cost) | the bridge writes one row to `.evolve/runs/cycle-N/llm-calls.ndjson` for each LLM call. The orchestrator counts the audits in `audit_dispatches` of `cycle-state.json` | `wave.ReadCycles` (`Cycle.Spend`, through `cyclespend.Read`). An unreadable ledger gives a warning that names the cycle. |
| the sized width | the loop: one INFO `loop.wave` record at each sizing (`recordWaveSized` in `cmd/evolve`) | `wave.MeasureFacts` over `wave.ReadLoopSignals` |
| `AdmissionWaits` | the orchestrator (O3) | `wave.MeasureFacts`, by the string `LOOP_ORCH_ADMIT_WAIT`. PR #925 (O3) is not on main yet, so no registered code exists. When it lands, the reader takes the registered constant. |
| `OptimisticAdmissions`, `OptimisticFailures` | the pool publishes `LOOP_ORCH_ADMISSION_OPTIMISTIC` when it launches an optimistic-only lane. That lane publishes `LOOP_ORCH_OPTIMISTIC_ADMISSION_FAILED` at its first composed-gate failure or rebase conflict | `wave.MeasureFacts` over `wave.ReadLoopSignals` |

`internal/loopwave` does not write the sizing record. Its import guard keeps it a leaf, so `cmd/evolve` writes the record through `loopwave.EmitWave`.

## The conflict codes

The set is `GIT_FLEET_REBASE_NEEDED`, `GIT_FLEET_REBASE_CONFLICT` and `AUDIT_BINDING_HEAD_MOVED`. The recovery in `internal/core/ship_recovery_debugger.go` (`fleetRebaseRecovery`) names the first two, because only they route to the debugger. The measure adds the head-moved code. It is a landing conflict: main moved after the audit bound the tree. The recovery re-audits it, and it needs no debugger.

The ship phase has no code named `REBASE_CONFLICT`. Its code is `GIT_FLEET_REBASE_CONFLICT`.

## Invariants

- 4 lanes with known intervals give the exact utilization and the time-weighted median, not the mean (`TestCompute_FourLanesGiveTheExactUtilizationAndTheTimeWeightedMedian`).
- A landed lane with a fleet rebase is a conflict. A lane that did not land is not in the rate (`TestCompute_ALandedLaneWithAFleetRebaseIsAConflict`).
- An empty run gives zeros, not NaN (`TestCompute_AnEmptyRunGivesZerosNotNaN`).
- A lane that rebased and then landed counts one conflict (`TestReadCycles_ALaneThatRebasedAndThenLandedCountsOneConflict`).
- The pool signals give the lane intervals, and the cycles give only `landed` and the conflicts (`TestMeasureFacts_ThePoolSignalsGiveTheLaneSpans`).
- A lane that the wave loop launched before the record counts. A signal of another `pid` or from before the run ID does not count (`TestMeasureFacts_ThePoolLanesOfTheRunLoopCountFromTheRunIDTime`). The wave 137 signals give `width 2/4` and `pool:    live 1` (`cmd/evolve` `TestWaveStatus_Wave137CountsTheLanesThatLaunchedBeforeTheRecord`).
- A deadline stop stops the unsealed cycle of that todo that started last before the stop. A `recorded` action stops nothing (`TestApplyLaneDeadlines_AStoppedLaneStopsTheCycleThatRanIt`).
- A seal with no verdict does not replace an earlier seal with a verdict. Thus a lane that sealed PASS just before its stop stays PASS (`TestReadCycles_ALoopSealAfterARealSealKeepsTheVerdict`).
- The loop seal of a deadline stop holds, and the deadline signal is only the fallback (`TestReadRunCycles_TheLoopSealHoldsAndTheDeadlineSignalIsTheFallback`). `wave watch` and `wave next` show the stops of wave 130 as `wave status` does (`cmd/evolve` `TestWaveWatchAndNext_Wave130ApplyTheDeadlineStopsLikeWaveStatus`).
- The wave 130 signals give `width 1/4`, and the cycles 1955 and 1957 show as stopped (`cmd/evolve` `TestWaveStatus_Wave130CountsTheLiveLanesFromThePool`).
- The cost of a wave of 2 shipped cycles and 1 failed cycle is the spend of the 3 cycles divided by 2 (`TestCompute_TheCostOfAShippedCycleCountsTheSpendOfEveryCycleOfTheWave`). `wave status --json` carries it (`cmd/evolve` `TestWaveStatus_JSONCarriesTheCostOfTheWave`).
- The command reports the rate end to end (`cmd/evolve` `TestWaveStatus_ReportsTheConflictRate`), and the loop writes one sizing record for each wave (`TestDispatchFleetIteration_RecordsTheSizedWidth`).

## Limits

- The run window starts when `wave next` records the wave. It holds the boundary steps before the first lane, so a short run has a low utilization.
- A cycle directory that a reset renamed (`cycle-N.reset-*`) loses its signals. The dossier phase times then give the interval, and its conflict signal is lost.
