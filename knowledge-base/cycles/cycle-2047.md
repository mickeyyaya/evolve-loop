# Cycle 2047 Dossier

**Goal:** Wave 143.

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

Wave 142 facts (run 20261026T182320Z): 1 of 2 cycles shipped.
- Cycle 2035: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2036: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 139 (run 20261020T162621Z): 2 of 5 cycles shipped.
- Wave 140 (run 20261021T000432Z): 3 of 4 cycles shipped.
- Wave 141 (run 20261026T115530Z): 4 of 6 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M5XTVNW0YTYH273X5A94WH5C
**Commit:** 2c5b7064c85326b161b2d02e50529744dd25461b
**Committed:** `finder-fix-ownership-f0-fix-spec-format`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 40s |  |
| triage | plan | PASS | 46s |  |
| tdd | plan | PASS | 3m22s |  |
| build | build | PASS | 53s |  |
| audit | evaluate | PASS | 1m44s |  |
| ship | control | FAIL | 5m14s |  |
| audit | evaluate | PASS | 1m43s |  |
| ship | control | PASS | 5m29s |  |

## Timing

**Total:** 19m51s across 8 phases (0 retried) · **Longest:** ship 5m29s

| Archetype | Wall-clock |
|-----------|------------|
| build | 53s |
| control | 10m43s |
| evaluate | 3m27s |
| plan | 4m48s |
