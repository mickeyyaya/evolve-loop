# ADR-0151: the host writes a retrospective document for each recovery and each judgment lesson

- **Status:** Proposed (2026-10-21). The code of stage `shadow` is in the same PR. The merge of that PR accepts this ADR. The move to `enforce` is an operator decision.
- **Plan:** [recovery-retrospective-2026-10.md](../../plans/recovery-retrospective-2026-10.md). **Research:** [recovery-retrospective-2026-10.md](../../research/recovery-retrospective-2026-10.md). **Design:** [recovery-retrospective.md](../recovery-retrospective.md). **Package note:** [internal-recoveryretro.md](../packages/internal-recoveryretro.md).
- **Relates to:**
  - [ADR-0055](0055-cycle-dossier.md): the committed cycle dossier is the only input of the recovery history.
  - [ADR-0142](0142-a-capability-failure-climbs-one-bounded-retry-ladder.md): the rungs of the passed try come from its try records.
  - PR #1081: it bounds the other failure-learning todos to the carryover TTL. This ADR decides the judgment-lesson todo that PR #1081 left open.
- **Evidence:**
  - The detector rule finds 12 recoveries in the dossiers of the plane tree on 2026-10-21. No record of a recovery exists (research F2.4, F2.7).
  - The committed dossiers hold the verdict, the class, the fingerprint, the reasons and the models of each try (F2.5, F2.6).
  - An inbox item keeps only its last failure, and its counter resets (F2.1, F2.2).
  - A judgment-lesson todo never expires today (F2.8).
  - The lesson recall reads only YAML lesson files (F2.9).
  - A PASS with no commit did not ship, and no WARN has a commit (F2.12).
  - 51 FAIL dossiers have the class `infra-error` of the failure classifier (F2.13).

## Context

The loop learns from failures. The retrospective phase runs on FAIL and WARN only. When an item fails again and again and then ships, nobody records what changed. The next similar item thus starts with no record of the fix. Current practice (Safety-II, contrastive pairs in agent memory) says that the passed try teaches as much as the failed ones.

The operator also decided on 2026-10-21 that a judgment lesson does not stay in the carryover log for good. It moves to a document under `docs/`, and the todo then leaves the log.

## Decision

1. A ship is a PASS dossier with a commit. A recovery is a ship after two or more FAILs since the last ship.
2. The host builds each document from committed records. It makes no LLM call.
3. A recovery document has one row for each try and a difference section for the last two tries.
4. A judgment-lesson todo `cycle-<N>-judgment-<phase>` gets a document with the same base name.
5. Each document has a sibling lesson YAML that the existing recall can read.
6. The compiled stage is `shadow`: the closeout writes the documents into the cycle workspace only.
7. At `enforce`, the documents reach `docs/retrospectives/` through the boundary publish of the dossiers.
8. At `enforce`, a judgment todo leaves the log only when its document is in the published tree.
9. A failure to write a document prints a WARN line. It never fails the closeout.
10. Each document write is create-only, so the "Judgment" text of a person is never erased.
11. A PASS with no commit and a WARN are neither a failure nor a reset of the count.

## Consequences

Good:

- Each recovery has an exact, reviewed record in `docs/`, at no token cost.
- The carryover log loses its only todo that never expires, and no lesson goes without its document.
- Scout, tdd, build and audit find the lessons through the recall that they already use.

Bad:

- A fact table does not say why a try passed. The "Judgment" section stays a placeholder until a person fills it.
- The window of 400 cycles can miss an old failure. The document gives the count that it found.
- The `enforce` stage adds one more file pair to each boundary publish.

## Rejected

- **An LLM retrospective phase on recovery:** it costs one agent run for each recovery, and an agent can misread a fingerprint (plan §4, design B).
- **A lesson YAML only, and a TTL for the judgment todo:** it writes no document, so it fails the operator ask. A TTL also drops a lesson with no record (design C).
- **A history field on the inbox item:** the dossiers already hold the history, and the item counter resets on a quarantine release (F2.1).
- **A live default:** the operator must read the shadow output before a document reaches `main` (D8).
