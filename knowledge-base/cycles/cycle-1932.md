# Cycle 1932 Dossier

**Goal:** Wave 122.

Work the highest-weight lane-eligible inbox items from start to end: claim, tdd, build, audit and ship. Keep full phase integrity.

Every cycle must:
- commit to a claimed inbox item (an empty top_n must not run the spine);
- bind its audit to the changes tree that it ships;
- disposition its own defect ledger and the inherited ids;
- record a terminal outcome on every exit;
- never change a protected control-plane file with a tool;
- leave no stale worktree.

Routing:
- An item whose declared fix surface is a protected file is console work. A pipeline-integrity item that the operator did not route to the lanes is also console work.
- An item that has `route: lane` is lane work. ADR-0099 document deliverables are lane work.
- A card on a protected surface is a route, not a verdict. The cycle ends as planned no-work, and the item goes to the console.

Craft:
- New and changed code has no comments. Names, types and tests give the intent.
- Write each new document and each new log text in ASD-STE100.
- A git-backed test fixture uses internal/gittest. It never uses a raw `git init`.
- A lane never changes go/internal/sizeratchet/offenders.json or the deadline of a gate.

Evidence:
- To know what a cycle did, read the cycle state and the signals (`evolve wave status`). Do not read the text log.

The measure of this wave is the number of shipped cycles. Keep each cycle shipping.

Wave 121 facts (run 20261017T093651Z): 2 of 3 cycles shipped.
- Cycle 1926: final verdict PASS. It shipped.
- Cycle 1927: final verdict PASS. It shipped.
- Cycle 1928: no final verdict. Last phase: build. It did not ship. Quota pauses: 1.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 118 (run 20261017T053340Z): 1 of 1 cycles shipped.
- Wave 119 (run 20261017T061052Z): 1 of 1 cycles shipped.
- Wave 120 (run 20261017T073249Z): 1 of 1 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M54VQV1CJGD92JH2HKD45HZV
**Commit:** 56be5c0aa4fc43dd98c9f658a0eb4c8131fa9a61
**Committed:** `recurrence-reweight-bypasses-the-item-writer`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 10m52s |  |
| triage | plan | PASS | 1m52s |  |
| fault-localization | plan | PASS | 8m3s |  |
| bug-reproduction | evaluate | PASS | 8m52s |  |
| tdd | plan | PASS | 4m34s |  |
| build | build | PASS | 1m46s |  |
| audit | evaluate | FAIL | 2m33s |  |
| tdd | plan | PASS | 4m7s |  |
| build | build | PASS | 17m40s |  |
| audit | evaluate | PASS | 6m4s |  |
| ship | control | FAIL | 3m1s |  |
| build | build | PASS | 2m54s |  |
| audit | evaluate | PASS | 1m45s |  |
| ship | control | PASS | 3m33s |  |

## Timing

**Total:** 1h17m35s across 14 phases (0 retried) · **Longest:** build 17m40s

| Archetype | Wall-clock |
|-----------|------------|
| build | 22m20s |
| control | 6m33s |
| evaluate | 19m14s |
| plan | 29m28s |
