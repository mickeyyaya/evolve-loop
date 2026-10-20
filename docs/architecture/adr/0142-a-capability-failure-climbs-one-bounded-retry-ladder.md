# ADR-0142: a capability failure climbs one bounded retry ladder

- **Status:** Accepted (2026-10-19). The operator chose `max_tier: deep`, `max_effort: high` and a shadow-only critic arm.
- **Amended by:** [ADR-0146](0146-a-defect-that-survives-two-builder-rounds-is-fixed-from-the-finders-spec-and-verified-by-another.md) (accepted 2026-10-20): the ladder gains a third repair rung. R1 is now a plain retry with the findings. R2 is the old R1 (effort +1, or a tier step when the family has no higher effort). R3 is a fix from the finder's spec. "At most three tries" becomes four (R0 to R3), which is three repair rounds under ADR-0126. Item 5 is replaced by the stop after R3.
- **Plan:** [retry-ladder-2026-10](../../plans/retry-ladder-2026-10.md). **Research:** [retry-ladder-2026-10](../../research/retry-ladder-2026-10.md). **Design:** [retry-ladder](../retry-ladder.md).
- **Relates to:**
  - [ADR-0096](0096-repair-rounds-escalate-and-carry-findings.md): the repair-round tier raise becomes one rung of this ladder.
  - [ADR-0076](0076-convergence-architecture.md): the deep floor after a failure becomes the start rung of the next item.
  - [ADR-0126](0126-every-iterative-loop-converges-or-escalates.md): the ladder is the escalation of a phase try, with at most three tries.
  - [ADR-0130](0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md): the digest of a raised rung is the try postmortem.
  - [ADR-0141](0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md): a spec failure takes the blocked exit, and a judge seat never climbs.
- **Evidence:**
  - About half of the build correction rounds of cycles 1800 to 1970 fail again (research F2.8).
  - No correction round changes the tier or the effort (F2.4, F2.5).
  - The builder pin of PR #981 turns off the raises of ADR-0096 and ADR-0076 for build and tdd (F2.3).
  - Two of the four recent aborts were spec failures, which a stronger model cannot fix (F2.11).

## Context

A phase agent that does not meet its request gets correction rounds at the same tier and the same effort. Only the CLI family can change. The two tier raises that exist sit in two places, and a policy pin turns them off. Effort follows the tier, so no retry can ask the same model to think harder. The loop also does not tell a capability failure from a spec failure. A raise then spends a stronger model on a request that no model can meet.

## Decision

1. A deterministic classifier gives each failed try one class: `infra`, `quota`, `halt`, `spec`, `capability` or `unknown`. Exit 85 is `quota` only when the bench confirms it.
2. Only `capability` climbs. `unknown` takes the first rung only. `halt` and `spec` never climb.
3. The ladder has three rungs:
   - R0: the base tier and effort.
   - R1: a fresh session, the failure digest and one effort step at the same tier.
   - R2: a fresh session, the failure digest and one tier step.
4. The ladder skips R1 when the fingerprint repeats after R0. It also skips R1 when the family has no higher effort.
5. When R2 fails with an earlier fingerprint, the failure becomes `spec`. The handoff then takes its blocked exit.
6. `agents.<agent>.model` is the base rung. `max_tier` and `max_effort` bound the ladder. The ladder overlay outranks the pin, and the first-try advisor overlay does not.
7. A tier step goes to the first family with headroom at the target tier. With no headroom, a failure at R0 takes R1, and a failure at R1 stops.
8. The rung is sticky for the rest of the item. Judge seats never climb, and a family floor seat never goes below its floor.
9. Each try records its rung, and the stage starts at `shadow`.

## Consequences

- A capability failure gets a stronger try inside its cycle, and a spec failure stops after at most three tries.
- One package decides every raise. The raises of ADR-0096 and ADR-0076 and the CLI escalation of the contract ladder move into it.
- A raised try costs more and uses the deep quota window. The cap of two raised tries and the headroom check bound this.
- `cliroute` precedence changes for a pinned agent. Tests pin the three cases.

## Rejected options

The plan scores five designs ([plan §3](../../plans/retry-ladder-2026-10.md#3-candidate-designs)). Patched paths drift. A critic first has no local measure. A learned first tier has no data yet. An always-deep build costs the most.

## What changes this decision

- After 30 capability failures at `enforce`, R2 fixes fewer than one in four. Then the critic arm replaces the tier step.
- R1 fixes as many failures as R2. Then R2 becomes rare, and the effort step is the main lever.
