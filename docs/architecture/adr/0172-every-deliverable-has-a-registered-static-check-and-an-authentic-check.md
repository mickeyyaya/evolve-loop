# ADR-0172: Every deliverable has a registered static check and an authentic check

- **Status:** Proposed (2026-11-27). The operator asked: "I mean all deliverables are required to be properly tested and examed with a static and authentic method". ADR-0171 is taken by pending PR #1860.
- **Plan:** [deliverable-verification-2026-11](../../plans/deliverable-verification-2026-11.md). **Research:** [deliverable-verification-2026-11](../../research/deliverable-verification-2026-11.md). **Design:** [deliverable-verification](../deliverable-verification.md) (the spec of record).
- **Relates to:**
  - [ADR-0167](0167-shadow-stages-prove-themselves-on-chained-evidence.md) governs each new stage key. This record adds no second bar for a move to `enforce`.
  - [ADR-0170](0170-the-loop-keeps-the-llm-at-the-leaves-and-hardens-its-host-resume-fleet-and-self-edit-edges.md) keeps the LLM at the leaves. This record applies that rule to verification: code runs each check.
- **Evidence:**
  - F1.1: a diff of only `.md` files gets no code check of its citations.
  - F1.2, F1.3: TDD passes on a heading, and the base-red floor excuses base reds. No gate demands red on the base.
  - F1.4: the mutation and revert libraries exist and no gate calls them.
  - F1.6, F1.7: 36 package-note lines cite a test that does not exist, and about one `path:line` cite in five does not resolve.
  - P2.1, P2.3: FAIL_TO_PASS and PASS_TO_PASS are the proven split of tests. UTBoost found tests that let wrong patches pass.

## Context

The loop has many static checks. They sit in many places, and no file lists them by deliverable class. A class can have no authentic check, and nobody sees it. The operator asks that each deliverable has both a static check and an authentic check.

A passing test is weak evidence. The test can be green on the base, or it can assert nothing that the change affects. A doc can cite a test that no longer exists. A "N/N PASS" line can be text with no run behind it. This record sets the rule and the order of work. It does not flip any existing gate.

## Decision

1. Each deliverable class has at least one registered static check. The registry names it by the id of an existing guard.
2. Each deliverable class has at least one registered authentic check. The registry names it the same way.
3. One data file, the registry (V0), maps the path globs of each class to its checks.
4. A meta-guard fails when a changed path maps to no class.
5. The meta-guard fails when a class has no static check or no authentic check.
6. The meta-guard fails when a registered check id does not exist. It reuses V1 for test ids.
7. A new check reuses an existing guard. It adds no second guard for a property that a guard already holds.
8. Each test that a diff adds or changes must fail on the merge base and pass on the change (V2).
9. V2 overlays only the test file on the base, and repeats the run to drop flaky results.
10. A refactor or characterization test is exempt only with an explicit marker and a reason. The ledger counts each marker.
11. At least one mutant of each changed function must fail a test (V3).
12. A doc of record cites only things that exist: tests, `path:line`, links, ids and named numbers (V1, V4).
13. A guard with a backlog starts with a shrink-only baseline.
14. A "N/N PASS" line and the TDD red evidence must match a run that the host recorded (V5).
15. A scheduled run executes the historical ACS packages. Each package is promoted to regression or retired with a record (V6).
16. Each new V-component (V0, V2 to V6) starts in shadow with its own stage key and gate registry seed.
17. A new check that can turn existing work red is a stage decision, so it starts at shadow. Only a held proposal or the operator moves a stage to enforce (ADR-0167). A check with a baseline of every miss of today blocks only a new miss. It can block at landing (V1, V4).
18. Each new deliverable of this design has its own static check and authentic check. The design obeys its own rule.
19. A ratchet file (class D14) moves one way against the merge base. The existing `sizeratchet.CheckAgainstBase` and the boundary ratchet tests hold that rule, and the registry names them.
20. A check of a deliverable runs in the host, on files that the builder cannot change.
21. A reviewer LLM's probe is a second line. It is never the registered check.
22. This record flips no gate and none of its own stages. Each flip is a question for the operator.

## Confirmation

The meta-guard (V0) confirms rules 1 to 7, 18 and 19 on each change. It runs in the repo-contract pack. The commit gate, the build floor and ship run it. The test `TestDocsOfRecord_EveryNamedTestExists` confirms rule 12 for test names today (built, in the V1 PR). This record lands after the V1 PR.

The Signal Center readings of the V2 and V3 stages confirm rules 8 to 11 in shadow. The operator moves each stage to enforce, on the ADR-0167 evidence. The spec has the table of guards.

## Consequences

**Good:**

- A class with no authentic check is visible, and it fails a test.
- A false guarantee in a doc of record fails the check, and the baseline can only shrink.
- A test that proves nothing is counted before it is trusted.
- Each check reuses a guard that exists, so the work adds no second source of truth.

**Bad:**

- The registry is one more file to keep true. The meta-guard keeps it true, and the registry and the guard are protected-surface rows.
- V2 needs a base worktree and a repeat run, so a diff takes longer to check.
- A legitimate refactor test needs a marker. A rise in markers is a signal that the ledger counts.
- Shadow stages protect nothing until the evidence moves them. The operator carries that gap.

**Limits:** this record does not check that the content of a cited line says what the text says. It does not prove that a set of mutants is complete. It does not design the research-loop components C2, C3 and C4.

**What changes this decision:** a flake rate of V2 that stays high after the repeat run. A registry that the meta-guard cannot keep true at a low cost.

## Rejected

| Option | The fact that rules it out |
|---|---|
| A. Patch each gate where it is weak, one at a time. | A class with no gate stays invisible (plan section 4, score 16 of 30). |
| B. One new end-to-end pipeline that replaces the existing gates. | It copies guards that exist and adds a second controller (score 13 of 30). |
| Enforce V2 and V3 at once, or block with V0, V4 and V5 at landing. | We have no measure of the flake rate (research gap 4). |
| Ask a reviewer LLM to check authenticity. | Not static. Its mistakes come back in the same confident register (P2.6, P2.7). |
| Run the historical ACS packages on each PR. | The run time is too long for a PR check, and the failures are legacy (F1.8). |
| Flip the stage-gated gates in this work. | No shadow record backs a flip (F1.9). |
