# internal/pipelinerates

> Decision: [ADR-0153](../adr/0153-quality-is-a-panel-of-separate-indexes-with-intervals-that-the-signal-center-publishes.md). Spec of record: [llm-quality-indexes.md](../llm-quality-indexes.md) §6 CQ2 and §8. Plan: [llm-quality-indexes-2026-10](../../plans/llm-quality-indexes-2026-10.md) component Q2. PQ5: [ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) and [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md) item 1. Callers: `go/cmd/evolve/cmd_quality.go` (the wave scope of `evolve quality report`) and `go/cmd/evolve/cmd_quality_compare.go` (`evolve quality compare`).

## Purpose

`internal/pipelinerates` reads the wave records and the cycle records and computes five indexes. They are PQ1 (ship rate), PQ3 (the comparison noise floor) and PQ4 (the deadline and system-fail rates). The other two are PQ5 (the cost for each shipped cycle) and CQ2 (Go packages per ship). It hands each reading to [internal/qualitypanel](internal-qualitypanel.md), which turns it into Signal Center events.

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
