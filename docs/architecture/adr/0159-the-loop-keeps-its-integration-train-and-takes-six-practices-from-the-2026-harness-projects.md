# ADR-0159: The loop keeps its integration train and takes six practices from the 2026 harness projects

- **Status:** Proposed (2026-11-04).
- **Plan:** [harness-practices-2026-11](../../plans/harness-practices-2026-11.md) and [§10 of the integration plan](../../plans/wave-integration-2026-10.md#10-amendment-of-2026-11-04-console-prs-enter-the-train). **Research:** [harness-practices-2026-11](../../research/harness-practices-2026-11.md). **Design:** [regression-proof-and-supervision](../regression-proof-and-supervision.md).
- **Relates to:**
  - [ADR-0147](0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md): this ADR keeps its design and changes its landed identity.
  - [ADR-0154](0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md): this ADR rejects each outside tool that puts an LLM in control.
  - [ADR-0155](0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md): the terminal refusal limits a self-heal pass.
  - [ADR-0133](0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md): the removal test waits for its M2 baseline.
- **Evidence:**
  - Gas Town reached the same batch-then-bisect design, and Uber's open source queue has no release (research F5.1, F5.2).
  - Ten CRITICAL findings sat on five green, fully covered trees (research F3.6).
  - SWE-bench defines a fix by two mechanical test sets, and agents edit failing tests (research F8.1, F8.3).
  - Two refused agents held one boundary for 35 minutes (research F3.4, F3.5).
  - One nameless red ended 2.5 hours of work, and 6 reruns passed (research F3.2, F3.3).
  - Of 40 open PRs, 23 compose into one batch today, and `main` has no rule that forces a queue (research F5.10, F5.11).

## Context

The operator asked if a better solution exists than the one that we built, and asked what the high-star projects do. The research found no outside system that replaces the integration train. It found that our largest gap is proof. A brief asks for a test of each fix, and no gate checks that the test fails without the fix. It also found small faults in the supervision path. Each one cost hours in waves 152 and 153.

The six practices are in decisions 2 and 3 (proof), 5 (liveness), 6 (refusal), 7 (floor), 8 (identity) and 9 (`enforce` evidence). Decisions 11 to 14 apply the train to console PRs.

## Decision

1. Keep the integration train of ADR-0147. Take no outside merge queue or workflow engine.
2. A `fix:` commit names a registered journey test, or states why behavior does not change (plan D2, R5).
3. A gate runs that test without the fix and with it. It must fail, then pass (plan D3, R6).
4. A registered journey test is on the protected surface, so a cycle cannot weaken it (plan D4, R5).
5. A cycle is active when its phase is live, or its owner lease is live or unreadable (plan D5, R1).
6. A stall after a guard denial is terminal for the chain and the pass (plan D6, R2).
7. The build floor uses the audit scope and retake, and keeps each failed test name (plan D7, R3).
8. The landed identity is the cycle trailer plus a zero-context patch-id (plan D8, R4; integration plan D30).
9. A move to `enforce` needs two consecutive clean shadow waves. The operator decides (plan D10, R8).
10. Each new check starts at `shadow`.
11. A reviewed console PR enters the train as a deliverable (integration plan D24, D25, I12).
12. The console train goes live before lane `enforce`, because it replaces a manual procedure (integration plan D27).
13. A member PR does not update its branch. The batch CI decides (integration plan D26).
14. A sorted line list uses `union`, and a guard test checks order and uniqueness (integration plan D28, I13).
15. `main` gets branch protection in two steps, as the train takes over each writer (integration plan D31).

## Consequences

**Good**
- The rule of the operator on fixes becomes a check, not a request. A green unit test with a made-up input no longer proves a fix.
- A refused agent costs one stall limit, not a walk through each family.
- A floor red names its test, and a one-off red does not end a cycle.
- A nearby peer edit no longer makes a composed cycle read as not landed.
- A batch of N reviewed PRs costs one CI run, and `main` moves once.

**Bad**
- Each `fix:` commit pays for one more package test run.
- A fix whose test needs a new API cannot compile on the old tree. The gate records this as inconclusive, so it proves less in that case.
- A retake can hide a real intermittent fault. Each retake leaves a signal, so the count stays visible.
- The team writes a journey test for each fix, and this takes longer than a unit test alone.
- A batch CI run tests a tree that no member tested alone. A red batch needs the split of integration plan D10.
- A member CI status goes stale in a batch. The review of each member stays bound to its own tree.

**What changes this decision**
- Uber's queue publishes a release and a single-host mode. Then we compare it again with the train.
- The gate at `shadow` reports inconclusive for more than half of the fixes. Then the design of the red run changes.

**Amends:** decision 3 of ADR-0147, the landed identity (integration plan D2). Each other decision of ADR-0147 stays.

## Rejected

| Option | The fact that rules it out |
|---|---|
| Adopt `uber/submitqueue` | It needs MySQL and three services, and it has no release (F5.2). |
| Adopt Temporal or DBOS for resume | Each adds a service or a database, and lane resume works (F7.1, F7.3). |
| A serial local queue | A 3–4 minute suite caps it under 20 landings an hour, and our floor is longer (F5.6). |
| A planner agent over the fleet, as in Agent Orchestrator | ADR-0154 forbids an LLM that controls an LLM (F5.9). |
| A tool-output proxy such as RTK | One benchmark shows 19% more cost for each solved task (F10.1). |
| A mutation gate on the whole module | Gremlins reports hours on a big module (F8.8). |
| `update-branch` for each PR, as today | It moves each other PR behind `main`, and each update reruns CI (F5.12). |
| Union on each hot file | Union on prose can join two edits into wrong text. The measured gain came from `go/.cover-strict` alone (F5.13). |
| Only a stronger rule in each brief | The rule exists now, and F3.6 and F3.7 happened under it. |
