# internal/qualitypanel

> Decision: [ADR-0153](../adr/0153-quality-is-a-panel-of-separate-indexes-with-intervals-that-the-signal-center-publishes.md). Spec of record: [llm-quality-indexes.md](../llm-quality-indexes.md). Plan: [llm-quality-indexes-2026-10](../../plans/llm-quality-indexes-2026-10.md) component Q1. Caller: `go/cmd/evolve/cmd_quality.go` (`evolve quality report`). Policy: `quality` in `.evolve/policy.json` (`go/internal/policy/quality.go`).

## Purpose

`internal/qualitypanel` turns one quality index reading into Signal Center events. It holds the registry of the 22 indexes and the statistics: Wilson 95%, a cluster bootstrap of the median and the MDE. It also holds the breach test and the codes of module `quality`. It computes no index from a data source. The readers of components Q2 to Q10 do that and hand it a `Reading`. The Q5 mutation reader is the one reader here, and it takes its data from an injected `MutationRunner`.

"Quality index" here is not the review-score grammar of `internal/qualityindex`. That package scores one review. This package measures the loop.

## Design

- **The registry.** `Indexes` returns a copy of the 22 entries of spec §1: id, name, unit, direction, threshold, `NMin` and baseline. An entry with a baseline has the default stage `report`. The others start at `shadow`. PQ3 and JQ4 have no threshold by design, so they are not `Breachable`.
- **The stage.** `StageFor` takes the policy overrides and falls back to the registry default. The overrides come only from `.evolve/policy.json` `quality.indexes.<id>.stage`. The policy decode is strict: an unknown key, an unknown id or a stage outside `shadow`, `report` and `gate` refuses the policy.
- **The breach test.** `Judge` returns `no_data` for n = 0 and `ok` for an index with no threshold. Otherwise it compares the whole interval with the threshold. A breach needs the whole interval on the bad side. An interval across the threshold is `inconclusive`. A breach under `NMin` is also `inconclusive`.
- **The events.** `Events` returns one `quality.reading` (INFO, no code) with 11 field keys, under the `MaxFields` limit of 12, so `Normalize` drops none. At stage `report` or `gate`, a breach adds one `quality.breach` (WARN, `QUALITY_<ID>_BREACH`). A reading with n = 0 adds a `quality.warning` (`QUALITY_VACUOUS_DENOMINATOR`). `SourceWarning` builds the warning for an absent or unreadable source.
- **A reader can refine a reading.** A `Reading` can carry its own `Threshold`, for a sub-measure such as the PQ4 system-fail rate. It can also carry its own `Status`, for a comparison under its MDE. `MethodMDE` names that comparison.
- **The mutation reader (Q5).** `MeasureMutation` scores TQ1 and TQ3 from a `MutationReport`. It sorts each changed function before the run. A kill-guard path is excluded: `go/internal/proctree/`, `go/internal/gc/` or `go/internal/bridge/dispatch_reap.go`. Any file with a `Kill`, `Killpg` or `Signal` call is also excluded.
- **The runner seam.** A source that does not parse, or has no function of that name, is refused. Only the other functions reach a `MutationRunner`, as `MutationTarget` line ranges. With no target, the runner does not run, because gremlins then mutates the whole package. The scratch directory must be absolute and outside the plane tree and the main tree.
- **TQ1 and TQ3.** TQ1 is killed over killed plus lived, with a Wilson 95% interval. A timed-out mutant counts as killed and a not-viable mutant is dropped. The not-covered count is the field `not_covered` of the TQ1 reading.
- **TQ3.** TQ3 is a census: mutants that exactly one test kills, over the summed test seconds. `ZeroUniqueTests` lists the tests with no unique kill. With no measured test time, TQ3 has n = 0 and warns `QUALITY_VACUOUS_DENOMINATOR`.
- **The shadow stage.** Both indexes start at `shadow`, so `Events` emits no breach for them.
- **The codes.** `init` registers 19 breach codes and three warning codes, so `evolve signals codes check` stays in sync with [signal-codes.md](../signal-codes.md).
- **Where the events go.** The command builds its Center with publisher role `quality`. A reading with a cycle goes to that cycle's `signals.ndjson`. A cycle-0 reading goes to the channels `quality.reading`, `quality.breach`, `errors` and `signals`, never to the frozen project file. `evolve signals tail --channel <name>` reads them.

## Invariants

- No function here reads a file, a signal or the clock. The caller passes the reading and the stage.
- The Signal Center only carries the readings. No code reads a quality signal to set a stage or a verdict.
- `ClusterBootstrapMedian` with the same seed gives the same interval, so a reading repeats.
- The boundary step `quality report --scope wave` exits 0 on every collector failure. Each failure is a `quality.warning`.

## Findings

- Since Q2, the wave scope has three readers ([internal-pipelinerates](internal-pipelinerates.md)). The tree, judges and replay scopes still print "has no index readers yet".
- The inbox acceptance asked to add the quality kinds to `legacyProjectKinds`. A pinned test freezes that list at the kinds that existed before the channels, so Q1 routes the readings to the channels instead.
