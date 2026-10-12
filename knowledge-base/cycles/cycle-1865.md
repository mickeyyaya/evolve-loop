# Cycle 1865 Dossier

**Goal:** Wave 94.

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

Wave 93 facts (run 20261012T165919Z): 2 of 2 cycles shipped.
- Cycle 1863: final verdict PASS. It shipped.
- Cycle 1864: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 90 (run 20261011T145758Z): 2 of 2 cycles shipped.
- Wave 91 (run 20261012T102612Z): 4 of 4 cycles shipped.
- Wave 92 (run 20261012T154327Z): 2 of 2 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M4RP0YEFM8YJMAYWBNXBEQ1Z
**Commit:** e755b06395ae9dbc1f88564f09106161a8b78fe3
**Committed:** `cli-doctor-versions`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 7m20s |  |
| triage | plan | PASS | 4m41s |  |
| tdd | plan | PASS | 2m13s |  |
| build | build | PASS | 4m18s |  |
| test-amplification | evaluate | PASS | 6m30s |  |
| audit | evaluate | FAIL | 2m9s |  |
| build | build | PASS | 1m0s |  |
| audit | evaluate | WARN | 2m9s |  |
| ship | control | FAIL | 5m54s |  |
| audit | evaluate | WARN | 7m58s |  |
| ship | control | PASS | 2m37s |  |

## Timing

**Total:** 46m49s across 11 phases (0 retried) · **Longest:** audit 7m58s

| Archetype | Wall-clock |
|-----------|------------|
| build | 5m18s |
| control | 8m31s |
| evaluate | 18m46s |
| plan | 14m14s |
