# internal/lanetime

> Plan: [wave-latency-2026-10](../../plans/wave-latency-2026-10.md) (D7, W0). Research: [wave-latency-2026-10](../../research/wave-latency-2026-10.md) (F2.1, F3.1, F3.2, F4.1). Operator use: [wave-operations.md](../../operations/wave-operations.md) §2. The verb: `evolve wave latency` in [cmd-evolve.md](cmd-evolve.md).

## Purpose

`internal/lanetime` measures where the time of a wave goes. The research of 2026-10-19 did this with a script outside the repository, and no one can run it again. This package holds the same measures as code that a test pins.

The package is a leaf. It imports the standard library only, and it must not import `internal/core`. It reads files and writes none.

## Terms

- **Slot-hour:** one slot for one hour. Capacity is the `target` of the pool in each time slice. Use is its `live`.
- **Lane time:** the wall time of a cycle, from its first `phase.dispatched` to its last `phase.outcome` or `phase.aborted`.
- **Model time:** the part of a phase span that `llm-calls.ndjson` covers with a call of the same phase.
- **Host time:** the phase span minus the model time. The host runs the deterministic gates in it.
- **Gate time:** the time from the end of a call to the next event. The next event is the start of the next call of the same span, or the end of the span.

## Design

| Measure | Function | Source | Rule |
|---|---|---|---|
| Slot use | `SlotUse(samples, from, to)` | `LOOP_ORCH_*` signals with `fields.live` and `fields.target` (`ReadSamples`) | Each sample holds until the next one. The last sample holds until `to`, or it ends the window when `to` is zero. Average live lanes is the used slot-hours over the window hours. |
| Lane time by phase, model and host split, gate time | `Phases(cycles)` | `signals.ndjson` and `llm-calls.ndjson` of each run dir (`ReadCycles`) | A span opens at the first `phase.dispatched` of a phase. The next `phase.outcome` or `phase.aborted` of that phase closes it. The share is the span time over the total lane time. The gaps between spans are one row. |
| Lane time to a PASS per CLI family | `LaneTimeToPass(cycles, last)` | the same run dirs | See the next section. |

## The input of W4

`LaneTimeToPass(cycles []Cycle, last int) []FamilyLane` is the measure that D4 of the plan imports. `DefaultBuilds` (20) is the window.

W4 must call only this pure function. The caller reads the run dirs, at the loop boot or at a wave boundary, and passes the result to `internal/cliroute`. W4 must not read the whole history on the dispatch path.

- The family of a cycle is the `cli` of its first build call without the `-tmux` and `-p` suffixes: `claude`, `agy` or `agy-claude` (`Cycle.BuildFamily`). The first dispatch owns the cycle, so an escalation target does not get the lane time of the family that struggled first.
- A cycle with a phase still open is a live cycle, and a cycle with no closed span has no lane time. The function leaves both out.
- The function sorts the cycles by id and keeps the last `last` build cycles of each family. W0 passes 20.
- A cycle passed when the last verdict of its audit or ship phase is `PASS` (`Cycle.Passed`). A cycle with no verdict has not passed.
- `PerPass` is the lane time of all kept builds over the count of passes. A failed build thus adds to the cost of a pass. A family with no pass has `PerPass` of 0, which means no measure.
- The result is sorted by family name. A family with no build call is absent.

The consumer (W4, 2026-10-19): at each wave boundary, `runPreWaveProbes` in `cmd/evolve` calls `recordLaneTimes`. It writes the result to `.evolve/lane-time.json`, and the routing host of each cycle reads that file ([internal-cliroute.md](internal-cliroute.md), *The headroom order*).

`ReadCycles(evolveDir, 0, 0)` reads the whole history that gc has kept. The caller can pass a floor and a ceiling to read a range.

## Limits

- An outcome signal with no dispatch signal has no start. Run dirs from before the dispatch signal existed give such outcomes. `ReadCycles` skips them and returns a warning with the count. The span of a phase that is still open (a live cycle) is also absent.
- Model time is an estimate. The bridge ends a call about 0.1 minutes before the artifact is read (F3.4), and a call can overlap a span only in part. The overlap is clipped to the span.
- A malformed line is skipped and counted in a warning. A file that cannot be read is an error. A run dir with no `signals.ndjson` is counted in a warning, because gc prunes run dirs.
- The measure compares different tasks, so the per-family value is an estimate and not a controlled result (F3.3).

- Two phases can run in parallel, and then the shares can sum to more than 100%.
- The model time of a span is clipped to the span time.
- A cycle passes on the audit or ship verdict. The sealed verdict that `wave status` uses can differ, for example when a ship fails after an audit PASS. W4 can refine the rule later.
- `Slots.Utilization` is live over target in the pool. It is not the `utilization` of `wavemeasure`, which is busy lane time over the sized width.
- The family measure covers the history that gc keeps, not the history as of the wave that `--wave` names.
- The leaf rule is tested as stdlib only. The package therefore keeps its own copy of the signal names and the shapes of the two ndjson files.

## Tests

The fixture `testdata/evolve` holds five run dirs and one orchestration file with round times. The expected values are computed by hand:

- average live lanes 2.0 (`TestSlotUse_WeightsLiveAndTargetByTime`);
- build 65% and gap 2.5% of the lane time (`TestPhases_SharesAreOfTheTotalLaneTimeWithTheGapsCounted`);
- claude 62.5 minutes and agy 70 minutes per pass (`TestLaneTimeToPass_DividesTheLaneTimeOfTheLastBuildsByTheirPasses`);
- the last-N cap (`TestLaneTimeToPass_OnlyTheLastNBuildsOfAFamilyCount`).

The package is at 100% statement coverage (`go/.cover-strict`).
