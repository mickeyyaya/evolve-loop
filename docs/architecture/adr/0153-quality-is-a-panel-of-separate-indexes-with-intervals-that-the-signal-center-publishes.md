# ADR-0153: Quality is a panel of separate indexes with intervals that the Signal Center publishes

- **Status:** Proposed (2026-10-21).
- **Plan:** [llm-quality-indexes-2026-10](../../plans/llm-quality-indexes-2026-10.md). **Research:** [llm-quality-indexes-2026-10](../../research/llm-quality-indexes-2026-10.md). **Design:** [llm-quality-indexes](../llm-quality-indexes.md).
- **Relates to:**
  - [ADR-0101](0101-signal-center.md): the Signal Center is the home of every reading; its listeners observe and never decide.
  - [ADR-0127](0127-push-only-event-channels.md): two new channels carry the readings and the breaches.
  - [ADR-0126](0126-every-iterative-loop-converges-or-escalates.md): JQ3 counts the HIGH and CRITICAL findings that block after round 2.
  - [ADR-0151](0151-the-host-writes-a-retrospective-document-for-each-recovery-and-each-judgment-lesson.md): OQ4 reads its recovery detector.
  - [ADR-0152](0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md): the pass^k replay and the judge corpus run through `evolve agent run`.
- **Evidence:**
  - One wave has 4 to 16 cycles, so its ship-rate interval is 40 to 70 points wide (F2.3). A 10-point MDE needs about 382 cycles per arm (P4.3).
  - Three PRs had 100% coverage and a test that did not probe the change (F2.7).
  - A zero-miss rule held only because its denominator was empty (F2.7).
  - Hidden tests cut a measured cheat rate from 54% to near zero (P3.3, secondary).
  - The independent review finds a HIGH or CRITICAL defect on 41.8% [32.6, 51.7] of PRs (F2.8).

## Context

The operator asked for software-quality concepts as measurable indexes for an LLM loop, monitored in the Signal Center. In this loop the author is an agent, each try of an item can end differently, and the author sees its gates. A standard metric assumes none of these. A single number invites a game, a rate from one wave is noise, and a coverage gate can pass on tests that check nothing.

## Decision

1. Quality is a panel of 21 indexes in five layers: code, tests, pipeline, judges and outcomes.
2. There is no composite score.
3. Each rate carries a Wilson 95% interval. Each median carries a cluster bootstrap interval by wave.
4. A reading breaches only when its whole interval is on the bad side of its threshold.
5. Each index has a stage: `shadow`, `report` or `gate`. An index starts at `report` only when it has a baseline. Otherwise it starts at `shadow`.
6. A flip to `gate` needs an MDE smaller than the claimed effect and an ADR amendment.
7. Each index reads only data that the measured agent cannot write.
8. No index feeds the prompt of the agent that it measures.
9. Every index publishes through `signalcenter`: module `quality`, kinds `quality.reading`, `quality.breach` and `quality.warning`.
10. Each index with a threshold has one registered breach code, `QUALITY_<ID>_BREACH`.
11. The Signal Center only carries readings. A stage flip is an explicit policy change.
12. Deterministic Go computes each index at the wave boundary or on request. No index calls an LLM.
13. Channel `quality.breach` is lossless. Channel `quality.reading` is best effort.
14. `evolve status` and the dashboard show the panel next to loop and CLI health, with no total.
15. A rule that claims zero misses also emits its denominator.
16. pass^k comes from a replay harness, not from production cycles.
17. Mutation testing never runs on code that signals a process.

## Consequences

Good:

- A reader sees which layer moved, with its interval, in the place where system health already lives.
- A noisy wave does not trigger a decision, because a breach needs the whole interval past the threshold.
- An agent cannot raise an index by a change to its own reports.
- The 12 indexes with data have a baseline on day one.

Bad:

- 9 indexes have no data until their components land.
- The panel asks the reader to read several numbers. A total is simpler to read, and we give it up.
- The boundary takes longer by the time of the collector, mainly mutation runs.
- JQ3 depends on a pattern match over free text until the review files gain a severity field.

What changes our mind: an index can show breaches with no real defect behind them. If two `report` windows show this, the index goes back to `shadow` or loses its threshold.

## Rejected

| Option | The fact that rules it out |
|---|---|
| One composite score | A weighted sum invites a game of its cheapest term and hides which layer moved. Its terms have different units and samples, so it has no honest interval (P4.1). |
| Outcome indexes only | An escape shows up 14 or more days after the ship. That is many waves late for a warning (plan §4). |
| A vendor suite or the DORA four as they are | They assume a human author, give no interval and need a new service against the CLI-only rule (P3.10, plan §4). |
| A separate quality dashboard and store | The operator asked for one monitoring home in the Signal Center. |
| pass^k from production cycles | No item ships twice, because a ship consumes it (F2.4). |
| A breach on the point value | One wave's interval is 40 to 70 points wide (F2.3). |
