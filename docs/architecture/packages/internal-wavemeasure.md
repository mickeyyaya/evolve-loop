# internal/wavemeasure

> Spec: [wave-orchestrator.md §11](../wave-orchestrator.md#11-measures). Plan: [wave-orchestrator-2026-10](../../plans/wave-orchestrator-2026-10.md) (D14, O7). Operator use: [wave-operations.md](../../operations/wave-operations.md) §2. The reader of the facts: `internal/wave` (`ReadCycles`, `ReadLoopSignals`, `MeasureFacts`, `ApplyLaneDeadlines`).

## Purpose

`internal/wavemeasure` computes the measures of one wave run. They are the sized and the realized width, the utilization, the conflict rate and the count of admission waits. It is pure. It reads no file and starts no timer. The caller gives the facts and the clock.

## Design

- **The facts.** `Facts` holds the run window (`Start`, `End`), the sized width, one `Lane` for each cycle of the run, and the admission-wait count. A `Lane` holds its interval, `Landed` and `Conflict`.
- **The clock is an argument.** `Compute(f, now)` uses `now` for a zero `End`: a live run, or a lane in flight. Thus a test gives a fixed clock, and the command gives `env.now()`.
- **One sweep.** `Compute` clips each lane to the run window and sweeps the start and end edges in time order. The sweep gives the time at each width. The time-weighted median is the smallest width whose cumulative time is at least half the run time. Idle time counts as width 0, so a run with long gaps has a low realized width.
- **No NaN.** Each ratio has a guard for a zero denominator: no landed lane, a sized width of 0, or a run time of 0. Each such ratio is 0, so the JSON always encodes.
- **A lane with no start is not a lane.** A cycle with no signal and no dossier phase has no interval. It is out of the width and the busy time, but it still counts in `landed` when it shipped.
- **The supply is a copy.** `Facts.Supply` holds the six class counts of the last census of the run. `orchestrator.Supply.Measure` builds it. `Compute` copies it into `Measures.Supply`, and the JSON key `supply` is absent when the run has no census (`TestCompute_CarriesACopyOfTheClassCountsOfTheLastCensusAndOmitsAbsentOnes`). The census is in [internal-orchestrator.md](internal-orchestrator.md). `wave status` reads the record `.evolve/waves/supply.json` and takes the last census inside the run (`TestWaveStatus_ReportsTheSupplyOfTheLastCensusOfEachRun`).
- **The JSON names are a contract.** `TestMeasures_TheJSONFieldNamesAreStable` pins the seven keys. The constants `FieldWidthSized` and `CodeAdmitWait` are the names that the writer and the reader share.

## Where the facts come from

| Fact | Writer | Reader |
|---|---|---|
| the lane interval | the pool: `LOOP_ORCH_LANE_LAUNCHED` or `LOOP_ORCH_LANE_BACKFILLED` opens it, and `LOOP_ORCH_LANE_EXITED` of the same `fields.todo` closes it. Only the signals of the wave loop count. They have the `pid` of the record and a time from the run ID (`gcpolicy.ParseLogRunID`) to the run end. | `wave.MeasureFacts` over `wave.ReadLoopSignals` |
| the live lanes now | `fields.live` of the latest pool signal of the wave loop | `wave.PoolLive`, shown as `pool:    live N` |
| the lane interval, in a run with no pool signals | the cycle's signals (`.evolve/runs/cycle-N/signals.ndjson`), widened by the dossier phase times | `wave.ReadCycles` (`Cycle.StartedAt`, `Cycle.EndedAt`) |
| a deadline stop | the lease: `LOOP_ORCH_LANE_DEADLINE` with `action: stopped` | `wave.ApplyLaneDeadlines`. It finds the unsealed cycle of that todo (`lane-scope.json`) that started last before the stop. It sets `Termination: lane deadline` and ends the cycle at the stop. |
| `Conflict` | ship: `recoverFromShipError` calls `recordShipError` before it recovers. Each ship error thus emits a `ship.error` with the code `SHIP_` plus the error code | `wave.ReadCycles` (`Cycle.Conflict`) |
| `Landed` | the landing intent or the dossier commit | `wave.ReadCycles` (`Cycle.Shipped`) |
| the sized width | the loop: one INFO `loop.wave` record at each sizing (`recordWaveSized` in `cmd/evolve`) | `wave.MeasureFacts` over `wave.ReadLoopSignals` |
| `AdmissionWaits` | the orchestrator (O3) | `wave.MeasureFacts`, by the string `LOOP_ORCH_ADMIT_WAIT`. PR #925 (O3) is not on main yet, so no registered code exists. When it lands, the reader takes the registered constant. |

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
- The wave 130 signals give `width 1/4`, and the cycles 1955 and 1957 show as stopped (`cmd/evolve` `TestWaveStatus_Wave130CountsTheLiveLanesFromThePool`).
- The command reports the rate end to end (`cmd/evolve` `TestWaveStatus_ReportsTheConflictRate`), and the loop writes one sizing record for each wave (`TestDispatchFleetIteration_RecordsTheSizedWidth`).

## Limits

- The run window starts when `wave next` records the wave. It holds the boundary steps before the first lane, so a short run has a low utilization.
- A cycle directory that a reset renamed (`cycle-N.reset-*`) loses its signals. The dossier phase times then give the interval, and its conflict signal is lost.
