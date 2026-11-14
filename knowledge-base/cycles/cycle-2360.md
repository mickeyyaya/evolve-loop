# Cycle 2360 Dossier

**Goal:** Wave 171.

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

Wave 170 facts (run 20261113T094348Z): 1 of 4 cycles shipped.
- Cycle 2352: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2353: final verdict PASS. It shipped.
- Cycle 2354: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2355: final verdict FAIL. Termination: triage-empty-commitment-claimable-work. Last phase: triage. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 167 (run 20261112T235603Z): 1 of 4 cycles shipped.
- Wave 168 (run 20261113T054547Z): 3 of 4 cycles shipped.
- Wave 169 (run 20261113T084906Z): 1 of 4 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M7B79DW53TXA72K17N6A07TN
**Commit:** ba124fd277f74d0e4303fab35738c3be21bad361
**Committed:** `ci-wall-variance-deep-dive`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 41s |  |
| triage | plan | PASS | 37s |  |
| tdd | plan | PASS | 7m22s |  |
| build | build | PASS | 14m39s |  |
| audit | evaluate | WARN | 2m46s |  |
| ship | control | FAIL | 48s |  |
| audit | evaluate | WARN | 1m23s |  |
| ship | control | FAIL | 46s |  |
| audit | evaluate | WARN | 1m36s |  |
| ship | control | PASS | 50s |  |

## Timing

**Total:** 31m28s across 10 phases (0 retried) · **Longest:** build 14m39s

| Archetype | Wall-clock |
|-----------|------------|
| build | 14m39s |
| control | 2m24s |
| evaluate | 5m45s |
| plan | 8m40s |
