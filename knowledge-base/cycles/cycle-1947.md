# Cycle 1947 Dossier

**Goal:** Wave 128.

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

Wave 127 facts (run 20261018T104954Z): 2 of 2 cycles shipped.
- Cycle 1943: final verdict PASS. It shipped.
- Cycle 1944: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 124 (run 20261017T174807Z): 2 of 3 cycles shipped.
- Wave 125 (run 20261017T194548Z): 1 of 1 cycles shipped.
- Wave 126 (run 20261017T214840Z): 1 of 2 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M57EGYR1N245E8XHQTXERMKG
**Commit:** 2f1d2eb5017eeffa571133ff45374a312203f988
**Committed:** `convergence-v1-review-followups`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 43s |  |
| triage | plan | PASS | 29s |  |
| tdd | plan | PASS | 2m40s |  |
| build | build | PASS | 10m32s |  |
| audit | evaluate | PASS | 1m51s |  |
| ship | control | FAIL | 4m24s |  |
| audit | evaluate | PASS | 1m17s |  |
| ship | control | PASS | 3m58s |  |

## Timing

**Total:** 25m54s across 8 phases (0 retried) · **Longest:** build 10m32s

| Archetype | Wall-clock |
|-----------|------------|
| build | 10m32s |
| control | 8m22s |
| evaluate | 3m8s |
| plan | 3m53s |
