# ADR-0141: every phase handoff is a request, build and check loop with three exits

- **Status:** Accepted (2026-10-18).
- **Plan:** [phase-handoff-contract-2026-10](../../plans/phase-handoff-contract-2026-10.md). **Research:** [phase-handoff-contract-2026-10](../../research/phase-handoff-contract-2026-10.md). **Design:** [phase-handoff-contract](../phase-handoff-contract.md).
- **Relates to:**
  - [ADR-0098](0098-task-contract-block.md): the task contract becomes the source of the request of each pair.
  - [ADR-0088](0088-audit-chain-of-reasoning.md): the audit chain reads the ledgers, and its deterministic links decide.
  - [ADR-0126](0126-every-iterative-loop-converges-or-escalates.md): the convergence policy gives the round vocabulary; this ADR adds the blocked exit.
  - [ADR-0064](0064-pipeline-integrity-boundary.md): a protected path is one of the two proofs of a blocked exit.
- **Evidence:**
  - Cycle 1946 spent 3 rounds on a request that needed a protected file (research F1.1, F1.2).
  - Cycle 1950 had two tdd predicates that contradict each other, and no check found them before the build (F1.3).
  - Three of five phase pairs have no check of the output against the ask (F1.10).
  - A self-check without outside feedback makes results worse; a checker of the same family shares the errors of the author (P2.1, P2.3, P2.4).

## Context

The pipeline gives each phase the asks of the phase before it, but no check comes back. The deterministic checks compare ids and names. The only content check is the audit chain, which is a model, runs at the end and does not decide. When a request cannot be met, the build loop has no exit for it, so the cycle spends its rounds and seals FAIL. Each check that exists is its own design, with its own source for the ask.

## Decision

1. Each phase pair declares one request of numbered asks, each with an id, a source and an evidence kind.
2. The second phase writes a ledger: a status and evidence pointers for each ask, and the request digest.
3. One handoff reviewer checks every pair, through `core.DeliverableReviewer` and the existing correction loop.
4. The deterministic layer runs first. A judge runs only on asks that code cannot decide.
5. A judge is another family, or a stronger tier, than the author. The ledger records it.
6. A pass means that each required ask is met with resolved evidence. A stated confidence never counts.
7. Each loop has three exits: pass, cap and blocked.
8. The cap is the redispatch cap. A round that meets no new ask also ends the loop.
9. Only deterministic proof blocks: an ask needs a protected path, or two asks conflict.
10. A blocked exit sends a revision to the requester or the console. The cycle does not seal FAIL.
11. The audit walks the ledgers and validates the result against the inbox acceptance.
12. Each pair has a stage, `shadow` or `enforce`, in `.evolve/policy.json`, and starts at `shadow`.

## Consequences

Good:
- A request that cannot be met ends in one round, with the conflict named, and the work stays as salvage.
- Each output carries its own proof against its request, so the audit reads a chain, not a story.
- One shared design takes the place of a new check for each gap.
- Most of the code is a new package that lanes can build, so the waves keep 4 lanes.

Bad:
- Each phase writes one more file, and the personas of three phases change.
- A judge at stage enforce adds model cost on each pair that uses it.
- The deterministic layer proves that evidence exists and runs. It does not prove that the evidence means what the ask means.

The condition that changes this decision: two waves at shadow where the check disagrees with the audit verdict on shipped cycles. Then the pair stays at shadow, and its rules get a new review.

## Rejected

- **A new gate for each gap:** it adds a fifth source of the ask and repeats the problem of F1.11 (score 33 of 55).
- **The audit chain at enforce, with no other check:** it checks only after the build spent its rounds. Its check is a model (P2.1; score 26).
- **Signed in-toto attestations:** all phases run in one trust domain, so a signature adds keys and no proof. The ledger keeps the digest, so this move stays open (score 42).
- **A confidence that the model states:** it is too high after RLHF (P2.10).
