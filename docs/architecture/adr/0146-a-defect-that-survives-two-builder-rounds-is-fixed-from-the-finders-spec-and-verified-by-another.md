# ADR-0146: a defect that survives two builder rounds is fixed from the finder's spec and verified by another

- **Status:** Accepted (2026-10-20). The operator chose design C.
- **Plan:** [finder-fix-ownership-2026-10](../../plans/finder-fix-ownership-2026-10.md). **Research:** [finder-fix-ownership-2026-10](../../research/finder-fix-ownership-2026-10.md).
- **Amends:** [ADR-0142](0142-a-capability-failure-climbs-one-bounded-retry-ladder.md): its third rung becomes a fix from the finder's spec.
- **Relates to:** [ADR-0141](0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md), whose judge stays independent of what it grades.
- **Evidence:**
  - A repair round fixes about 71% of the audit failures that reach a verdict (15 of 21). A second round of the same builder adds little: two cycles show it (F2.2), and R10 agrees but is a preprint.
  - A model fixes an error well once it gets the location (R9).
  - No surveyed practice lets the finder both write and approve a fix (R1 to R5), and a model favors its own output (R8).

## Context

The operator asked that the phase that finds a defect fix it after the owner fails twice. The finder holds the location of the defect, which is the strongest lever for a fix. But the auditor is read-only, and a fix that its finder grades loses its independent check.

## Decision

1. R1: the owner retries in a fresh session with the findings, at the same tier and effort.
2. R2: the owner retries in a fresh session at one effort step higher. Without a higher effort, R2 takes one tier step, as the old R2 of ADR-0142 did.
3. R3: the finder writes a fix spec with the location and the replacement lines into its own workspace. A fresh builder session applies it, on another family when that family has headroom.
4. Three checks verify R3: the floor, the unchanged tdd tests, and an auditor instance that did not write the spec. That auditor is on another family than the R3 builder or at a stronger tier (ADR-0141). When neither holds, R3 runs its builder one tier lower, or the ladder stops at the dossier.
5. The ledger records both authors. A PASS from the spec author on that diff does not count.
6. An R3 fix may touch only the lines that the spec names.
7. When a deterministic check found the defect, the harness applies the spec's patch and runs the same check again.
8. R2 is skipped when R1 repeats its fingerprint, or when no effort step or tier headroom exists. R3 replaces ADR-0142 item 5.
9. After R3 the ladder stops, and the operator gets the dossier.
10. Round count: R0 is the first build, and R1 to R3 are rounds 1 to 3 of ADR-0126. That makes four dispatches and three repair rounds, so "no round 4" holds. The cap of two repair rounds becomes three.

## Consequences

- A defect that survives two builder rounds gets a fix from the phase that knows where it is.
- The auditor stays read-only, and no fix is graded by its author.
- R3 costs one more builder session and one more audit. Each rung records what it fixed, so a rung with no effect over 30 cases is removed. R2 has no outside evidence (R12), so this rule applies to it first.

## Rejected options

The plan scores five designs ([plan §2](../../plans/finder-fix-ownership-2026-10.md#2-candidate-designs)). The finder that edits the code directly loses the independent check and needs a wider sandbox. The owner ladder alone keeps retrying the owner that failed. A builder of another family loses the finder's location. Parallel candidates cost several builds for each defect.
