# ADR-0139: A wave orchestrator supplies, admits and backfills every lane

- **Status:** Proposed (2026-10-17).
- **Plan:** [wave-orchestrator-2026-10](../../plans/wave-orchestrator-2026-10.md). **Research:** [wave-orchestrator-2026-10](../../research/wave-orchestrator-2026-10.md). **Design:** [wave-orchestrator](../wave-orchestrator.md).
- **Relates to:**
  - [ADR-0049](0049-concurrent-multi-cycle-execution.md): the orchestrator replaces the fire-and-wait supervisor on the wave path. The isolation of each lane stays.
  - [ADR-0134](0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md): the orchestrator builds S14, admission by cell, as its admit step. ADR-0134 keeps the landing.
  - [ADR-0128](0128-landing-queue-tiered-reverification.md): the dispatch partition of Q2 becomes `PartitionGraph` under the orchestrator.
  - [ADR-0127](0127-push-only-event-channels.md): the orchestrator wakes on its channels and never polls.
  - [ADR-0074](0074-typed-signal-contracts-routing-authority.md): the console-routing authority stays. The supply step reports its refusals and does not override them.
- **Evidence:**
  - F2.2, F2.3: each run shrinks the wave from 3 to 2 for a family that has a fallback.
  - F2.6, F2.7: 0 of 230 inbox items can go to a lane.
  - F2.13: a barrier wave of 4 lanes keeps about 66% of the lane time busy.
  - P3.1, P3.9: a file-based split gives no gain, and package-disjointness is the production test of independence.
  - F2.12, F2.15: no component sees all lanes, and a wave lane has no deadline.

## Context

The operator wants at least four lanes at once, with few conflicts, on a design that scales to a large fleet. Today a wave realizes one or two lanes.

Four causes add up to that. The quota shrink removes a lane for a family with a fallback. The inbox gives no lane-routable work. The partition is by file, which research shows gives no gain. The barrier waits for the slowest lane.

Each cause has a built part that can fix it: `RunPool`, `PartitionGraph`, the event reader and `clihealth`. No component joins these parts or acts while the lanes run.

## Decision

1. One deterministic Go component, the wave orchestrator, owns the run of lanes. It runs no LLM.
2. A bench costs a lane only for a family with no fallback in the chains of the wave.
3. `fleet.count` is 4, and `fleet.min_lanes` stays 2.
4. Before each plan, a supply census counts the admissible items and records each refusal with its cause.
5. A directory routes to the console only when it lies inside a protected fragment. (Built: O2b, 2026-10-17.)
6. Dispatch admits a todo only when no lane in flight holds an overlapping package cell.
7. The hub cell admits one lane.
8. A todo with no files takes its named package cell, or else the hub cell.
9. The rolling pool replaces the barrier. A free slot takes the next admissible todo at once.
10. Each lane holds a lease with a heartbeat and a deadline. Past the deadline, it stops and releases its item.
11. The orchestrator wakes on the `cycle`, `ship`, `errors` and `loop` channels. It never polls.
12. A landing frees the cells of its lane at once.
13. A quota wall of a family with no fallback stops new admissions to that family only.
14. Above width 4, the window grows by 1 for each clean landing and halves on each conflict.
15. `evolve wave status` reports the realized width, the utilization and the pairwise conflict rate.
16. One stage key, `fleet.orchestrator.stage`, moves the orchestrator from shadow to enforce.

## Consequences

**Good:**

- The lost lane of each run comes back at once (decision 2).
- Lanes in flight touch disjoint package cells, so most conflicts never reach landing.
- A free slot does not wait for the slowest lane.
- One component sees every lane and acts on events, so the console does less by hand.
- The admission cost grows with the cells of the first K todos, not with the square of N.

**Bad:**

- Admission by cell can lower the width when much of the work is in one hub package. The shadow stage measures this cost.
- The pool keeps lanes in flight together for longer, so more lanes land on a moved `main`. Admission and ADR-0134 limit this cost.
- The orchestrator changes how every wave runs. It needs a shadow stage and a soak at N = 16 before it is enforced.
- A deadline can stop a lane that was about to pass. The lane releases its item, so the cost is the lane time only.

**What changes this decision:**

- If admission by cell costs more than 10% of the width at N = 4 in shadow, the non-hub cells fall back to files.
- If the routing remedy frees fewer than 8 items, a goal-split planner phase (plan design D) becomes the next supply source.
- If the pairwise conflict rate stays above 5% with admission on, the cell rule is too coarse, and the import graph needs a finer unit.

## Rejected

| Option | What it offers | Why it is not chosen |
|---|---|---|
| Configuration only (`fleet.count` 4, `scheduling` pool) | Two edits. | The supply is 0, so the slots stay empty. The file split keeps the conflicts (F2.6, P3.1). |
| Admission by cell inside the barrier wave | It reuses built code and stops conflicts at the source. | The barrier keeps 66% utilization, and nothing acts while lanes run (F2.13, F2.15). |
| An LLM planner phase that splits a goal into N tasks | It makes supply from a goal, with the best measured split. | Scheduling is deterministic work. It costs a deep phase for each wave, and it still needs this runtime. It stays a later supply source. |
| A learned conflict predictor that blocks pairs | It can find conflicts that the cells miss. | Its F1 on conflicts is 0.57–0.68 (P3.7). |
| Hedged second copies of slow lanes | A lower tail of lane time. | A copy costs a full lane of tokens and writes `main` (P3.15). |
| `--max-cycles 4` | No code change. | It runs four waves in sequence, not four lanes at once. |
