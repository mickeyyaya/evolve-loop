# Cycle 2357 Dossier

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
**Run ID:** 01M7B79DV7BQ06G5Z2Y4YJYRGQ
**Commit:** ecd6fc44ed2a712664e57bd97f7823add9efc3ff
**Committed:** `builder-fence-erases-new-files-and-the-seat-says-pass`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 47s |  |
| triage | plan | PASS | 39s |  |
| fault-localization | plan | PASS | 58s |  |
| bug-reproduction | evaluate | PASS | 57s |  |
| tdd | plan | PASS | 2m29s |  |
| build | build | PASS | 1h11m37s |  |
| audit | evaluate | FAIL | 5m51s |  |
| tdd | plan | PASS | 2m11s |  |
| build | build | PASS | 27m21s |  |
| audit | evaluate | PASS | 3m18s |  |
| ship | control | FAIL | 7m18s |  |
| audit | evaluate | PASS | 6m12s |  |
| ship | control | PASS | 8m25s |  |

## Timing

**Total:** 2h18m3s across 13 phases (0 retried) · **Longest:** build 1h11m37s

| Archetype | Wall-clock |
|-----------|------------|
| build | 1h38m59s |
| control | 15m42s |
| evaluate | 16m18s |
| plan | 7m4s |
