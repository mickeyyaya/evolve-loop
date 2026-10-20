# ADR-0147: the wave orchestrator owns integration and decides when the wave reaches main

- **Status:** Proposed (2026-10-20).
- **Plan:** [wave-integration-2026-10](../../plans/wave-integration-2026-10.md). **Research:** [wave-integration-2026-10](../../research/wave-integration-2026-10.md). **Design:** [wave-orchestrator §15](../wave-orchestrator.md#15-integration).
- **Amends:**
  - [ADR-0139](0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md): the orchestrator now owns landing.
  - [ADR-0128](0128-landing-queue-tiered-reverification.md) and [ADR-0134](0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md): their composition, overlap proof, tiers and conflict-graph order become parts of the integration engine. A lane no longer lands its own candidate.
- **Evidence:**
  - About one lane in four needed a rebase recovery of about 24 minutes in waves 130 to 135 (research F2.4).
  - `main` moved 100 times in two days, and a console merge strands the next lane (F2.5, F2.6).
  - A patch to the per-lane landing cannot remove the race (F2.7).
  - Every surveyed merge system has one owner of the composed result (M1 to M4).

## Context

Each lane of a wave lands its own deliverable on `main`, so the landings race each other and the other writers of `main`. The design gives no part of the system the order of the landings or the decision to push. The landing queue has the parts of an integration engine, but each lane still lands itself.

## Decision

1. A fleet lane delivers its audited change to the orchestrator and does not push `main`.
2. The orchestrator orders the deliverables by a deterministic score. The keys are the deps, the conflict graph, the rank, the risk and the cost.
3. The orchestrator composes each deliverable onto a short-lived branch for each batch as one commit, and checks each prefix. A cycle's identity is its trailer plus its patch-id.
4. It tests the combined tree in four layers: prefix, batch, PR and after merge. Each deliverable's acceptance predicates run on the combined tree.
5. A conflict, a failed gate or a tier-T4 proof ejects the deliverable. Its lane gets the diff of the change it met.
6. A batch flushes as one PR at wave end, at the cap, at `max_age`, or for an unblocking deliverable. A risky deliverable flushes alone.
7. When the batch CI is green and `main` did not move, the orchestrator fast-forwards `main` to the tested tip. If `main` moved, it composes again and tests again.
8. A flake gets one re-run in each layer. A confirmed red batch halves the cap, between 1 and 4, and a split isolates the culprit. A confirmed red after the merge reverts the culprit's commit alone, and is a system failure of ADR-0072.
9. The loop's own commits go to a bookkeeping batch of their own. A console merge into `main` makes the orchestrator compose the open batch again.
10. An integration record with the loop lease epoch owns the branch and the PR. A resume adopts or discards them.
11. The stage `fleet.integration.stage` starts at `shadow`, with a textual compose only.

## Consequences

- `main` moves once for each batch, and no lane rebases at landing.
- A console PR can merge during a wave without a stranded lane.
- At `enforce`, `main` moves during a wave. That amends the standing rule that code merges only at a boundary, so the operator decides it (plan Q4).
- A deliverable waits in its batch until a flush.
- The orchestrator becomes the single writer of `main` for the loop. The stage `shadow` and the per-lane fallback bound that risk.

## Rejected options

The plan scores five designs ([plan §3](../../plans/wave-integration-2026-10.md#3-candidate-designs)). The per-lane queue keeps a lane as the writer of `main`. A native merge queue gives each deliverable its own PR and CI.

Parallel trains cost several CI runs for each wave. A rebase merge of the batch PR rewrites each SHA, and after a move of `main` it lands a tree that no CI tested. Avoidance alone leaves each lane to land itself. A fix to the per-lane ship that follows an outside move was drafted on 2026-10-20 and not merged, because of F2.7.
