# Cycle 2624 Dossier

**Goal:** Wave 188.

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

Wave 187 facts (run 20261122T121257Z): 1 of 2 cycles shipped.
- Cycle 2606: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…ALiveProcessHolds skip-hides-error has no baseline row — remove the weak pattern (for error text: a sentinel error and errors.Is); Rebaseline refuses new rows. Last phase: aborted. It did not ship.
- Cycle 2607: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 184 (run 20261120T204107Z): 5 of 8 cycles shipped.
- Wave 185 (run 20261121T081737Z): 4 of 9 cycles shipped.
- Wave 186 (run 20261121T192937Z): 10 of 13 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M84YCSYFP1VJE7ZP83NH89V4
**Commit:** 79045510550445ee24a9bbffacea51f4455f4d7b
**Committed:** `ship-paths-accepts-an-already-staged-deletion`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 1m7s |  |
| triage | plan | PASS | 40s |  |
| fault-localization | plan | PASS | 39s |  |
| tdd | plan | PASS | 3m50s |  |
| build | build | PASS | 26m17s |  |
| audit | evaluate | PASS | 7m51s |  |
| build | build | PASS | 21m7s |  |
| audit | evaluate | PASS | 5m19s |  |
| ship | control | FAIL | 2s |  |
| ship | control | FAIL | 13m24s |  |
| audit | evaluate | PASS | 5m44s |  |
| ship | control | FAIL | 1s |  |
| ship | control | PASS | 10m47s |  |

## Timing

**Total:** 1h36m49s across 13 phases (0 retried) · **Longest:** build 26m17s

| Archetype | Wall-clock |
|-----------|------------|
| build | 47m24s |
| control | 24m14s |
| evaluate | 18m55s |
| plan | 6m16s |
