# internal/pipelinerates

> Decision: [ADR-0153](../adr/0153-quality-is-a-panel-of-separate-indexes-with-intervals-that-the-signal-center-publishes.md). Spec of record: [llm-quality-indexes.md](../llm-quality-indexes.md) §6 CQ2 and §8. Plan: [llm-quality-indexes-2026-10](../../plans/llm-quality-indexes-2026-10.md) component Q2. PQ5: [ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) and [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md) item 1. Callers: `go/cmd/evolve/cmd_quality.go` (the wave scope of `evolve quality report`) and `go/cmd/evolve/cmd_quality_compare.go` (`evolve quality compare`).

## Purpose

`internal/pipelinerates` reads the wave records and the cycle records and computes nine indexes. They are PQ1 (ship rate), PQ3 (the comparison noise floor) and PQ4 (the deadline and system-fail rates). Four more are PQ5 (the cost for each shipped cycle), PQ6 (calendar throughput), PQ7 (slot utilization) and PQ8 (idle-cause share). The last two are PQ9 (gate share) and CQ2 (Go packages per ship). It hands each reading to [internal/qualitypanel](internal-qualitypanel.md), which turns it into Signal Center events.

## Design

- **The outcome of a cycle.** `LoadCycles` reads `.evolve/waves/wave-N.json` (`number`, `cycle_floor`) and each `.evolve/runs/cycle-N/`. A cycle belongs to the last wave whose floor is at or below its id. A cycle ships when `run.json` `shipped` is true or its stream has `ship.landed`. A cycle is sealed when its stream has `cycle.sealed`. The loader never reads `run.json` `final_verdict`: in cycle 2024 it says PASS and the seal says FAIL.
- **A cycle that still runs.** A cycle whose run lease is live (`runlease.LiveOwner`) has no outcome yet, so `LoadCycles` leaves it out. Without this rule the latest wave reads too high on PQ4.
- **Deadline and system fail.** A cycle with no seal, or with a seal whose `termination_reason` names a lane deadline, is a deadline end. A cycle whose stream has `ORCHESTRATOR_SYSTEM_FAILURE`, `LOOP_SYSTEM_FAILURE_HALT` or a `BRIDGE_EXIT_*_TIMEOUT` code is a system fail. The system-fail reading is a PQ4 reading with the window suffix `/system-fail` and its own threshold of 0.05.
- **The cost (PQ5, ADR-0158).** `LoadCycles` also reads the spend of each cycle through `cyclespend.Read`. A spend that cannot be read adds one unmeasured call, so the other indexes still load (`TestLoadCycles_AnUnreadableLLMCallLedgerCountsAsAnUnmeasuredCallAndKeepsTheOtherIndexes`). `CostPerShip` gives ten census readings for each window:

  | Window suffix | Value |
  |---|---|
  | none | the LLM minutes for each shipped cycle |
  | `/fresh-tokens`, `/cache-read-tokens` | the tokens for each shipped cycle |
  | `/audit-rounds` | the audit rounds for each started cycle |
  | `/unmeasured-calls` | the calls with no measured usage |
  | `/handoff-would-stops`, `/handoff-stops` | the counts of the `REPAIR_HANDOFF` decisions `would_stop` and `stop` |
  | `/spend-limit-would-stops`, `/spend-limit-stops` | the cycles with a `CYCLE_SPEND_LIMIT` decision `would_stop` or `stop` |
  | `/false-stops` | the cycles with a `would_stop` of either check that shipped |

  A window with no ship has `n` 0, so the panel publishes `no_data`, not a cost of 0 (`TestCostPerShip_AWaveWithNoShipHasNoDataNotAZeroCost`). PQ1 is the convergence rate of ADR-0158: shipped cycles divided by started cycles.
- **The flow readings (PQ6 to PQ9).** Plan: [pipeline-health-indexes-2026-11](../../plans/pipeline-health-indexes-2026-11.md), component H2.
  - `LoadCycles` also reads the ship time of each cycle (`ShippedAt`). It is the time of `ship.landed`. If the stream has no such signal, it is the file time of `run.json`.
  - `CalendarThroughput` gives one census reading for each of the last 7 days. A day with no delivery gives 0.
  - `LoadWaveFlows` reads the last 10 wave records. For each wave it reads the slot use (`SlotUse`), the idle slot-hours of each cause (`IdleUse`) and the phase report of `internal/lanetime`.
  - It reads the `LOOP_ORCH_*` samples of `.evolve/signals.ndjson` one time with `lanetime.ReadSamples`. For each wave it selects the samples one time with `lanetime.WaveSamples`, the same function that `evolve wave latency` calls. It uses the pid of the wave, or the wave time when the record has no pid. `SlotUse` and `IdleUse` get the same set.
  - A cycle belongs to the wave that `wave.Bounds.Holds` names. Its id is above the floor of the wave. It is at or below the floor of the next wave. A cycle equal to the floor of wave N+1 belongs to wave N. `evolve wave latency` uses the same rule, so PQ7 and PQ9 equal it.
  - Limit: PQ1, PQ4 and PQ5 still use `waveOf`, which puts a cycle on the floor of wave N+1 into wave N+1. So the rule splits between PQ1, PQ4 and PQ5 (`waveOf`) and PQ7, PQ8 and PQ9 (`wave.Bounds`). Inbox item `pipelinerates-pq1-pq4-pq5-use-wave-bounds` moves PQ1, PQ4 and PQ5 to `wave.Bounds`.
  - A missing `.evolve/signals.ndjson` is kept as `FlowSet.SamplesErr`. PQ7 and PQ8 give a source-absent warning, and PQ9 still reads.
  - The report loads the flows one time and gives them to PQ7, PQ8 and PQ9.

  | Function | Index | Value | Status |
  |---|---|---|---|
  | `CalendarThroughput` | PQ6 | deliveries for each day | none |
  | `SlotUtilization` | PQ7 | live slot-hours ÷ capacity slot-hours | none; `n` 0 when the wave has no sample |
  | `IdleCauseShare` | PQ8 | idle slot-hours of a cause ÷ all idle slot-hours | none; the registry threshold 0.50 and `qualitypanel.Judge` decide the breach |
  | `GateShare` | PQ9 | gate time ÷ lane time | none; `n` 0 when the wave has no lane time |

  An idle event with no `cause` field counts as the cause `unknown`. A wave whose idle slot-hours total 0 or less gives no PQ8 reading.
- **Windows.** `Windows` gives the latest wave, the last 10 waves, and all waves when they reach past the last 10.
- **Packages per ship.** `PackagesPerShip` runs `git show --numstat` on the `ship-binding.json` commit of each ship. The command uses `-z`, `--no-renames` and `--diff-merges=first-parent`, so a merge ship and a path with a space count right. A commit that git cannot resolve is left out and named in the error; the collector publishes the other readings and one `quality.warning`. It counts the distinct directories of the `.go` paths outside `.evolve/`.
- **The CQ2 interval.** The median carries a cluster bootstrap by wave (2,000 resamples, fixed seed). A wave with no ship gives a `no_data` reading.
- **Comparison.** `Compare` gives the difference of two rates and the MDE at the pooled rate and the smaller arm. A difference under the MDE is `inconclusive`. `Discordant` pairs the inbox items tried in both windows, and `McNemarExact` gives the exact two-sided p-value.

## Invariants

- The readers only read. `evolve quality report` writes events through the Signal Center and nothing else.
- A source that is absent returns an error that wraps `fs.ErrNotExist`, so the collector emits `QUALITY_SOURCE_ABSENT`.

## Findings

- On 2026-10-21 the plane gave PQ1 64.0% [56.9, 70.5] over waves 82 to 140. It gave PQ4 16.7% [12.0, 22.7] and 4.8% [2.6, 8.9], and a CQ2 median of 2 packages over 119 ships. These match the research baselines.
- The package count includes `.go` files outside `go/`, for example archived predicate files under `docs/private/`. The research baseline used the same rule, so the two agree.
