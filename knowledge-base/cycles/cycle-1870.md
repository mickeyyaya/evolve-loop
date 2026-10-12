# Cycle 1870 Dossier

**Goal:** Wave 96.

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

Wave 95 facts (run 20261012T195727Z): 0 of 2 cycles shipped.
- Cycle 1867: no final verdict. Last phase: scout. It did not ship.
- Cycle 1868: no final verdict. Last phase: scout. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 92 (run 20261012T154327Z): 2 of 2 cycles shipped.
- Wave 93 (run 20261012T165919Z): 2 of 2 cycles shipped.
- Wave 94 (run 20261012T182416Z): 2 of 2 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M4RVYFKVNC212JGD015NFY9G
**Commit:** be9bfd824821d8ae810f9f8f41e22be7644e3796
**Committed:** `wave-shared-state-reader-and-squash-prs`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 10m38s |  |
| triage | plan | PASS | 7m5s |  |
| tdd | plan | PASS | 44s |  |
| build | build | PASS | 23m9s |  |
| audit | evaluate | PASS | 2m51s |  |
| ship | control | PASS | 9m26s |  |

## Timing

**Total:** 53m54s across 6 phases (0 retried) · **Longest:** build 23m9s

| Archetype | Wall-clock |
|-----------|------------|
| build | 23m9s |
| control | 9m26s |
| evaluate | 2m51s |
| plan | 18m27s |
