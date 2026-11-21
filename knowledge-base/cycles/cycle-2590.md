# Cycle 2590 Dossier

**Goal:** Wave 185.

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

Wave 184 facts (run 20261120T204107Z): 5 of 8 cycles shipped.
- Cycle 2563: final verdict PASS. It shipped.
- Cycle 2574: final verdict PASS. It shipped.
- Cycle 2575: final verdict PASS. It shipped.
- Cycle 2576: final verdict FAIL. Last phase: memo. It did not ship.
- Cycle 2577: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…directories): .git  fatal: not a git repository (or any of the parent directories): .git fatal: not a git repository (or any of the parent directories): .git…. Last phase: aborted. It did not ship.
- Cycle 2578: final verdict PASS. It shipped.
- Cycle 2579: final verdict FAIL. Last phase: memo. It did not ship.
- Cycle 2580: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 181 (run 20261119T004314Z): 22 of 23 cycles shipped.
- Wave 182 (run 20261119T213950Z): 7 of 11 cycles shipped.
- Wave 183 (run 20261120T094708Z): 9 of 15 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M7Z1377120S3VZJ04W2S814F
**Commit:** 07d29bc2518b7f856d496f4803f01e32bea54573
**Committed:** `pr-refresh-test-measure-checkout`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 36s |  |
| triage | plan | PASS | 42s |  |
| tdd | plan | PASS | 3m50s |  |
| build | build | PASS | 49m53s |  |
| audit | evaluate | FAIL | 1m44s |  |
| build | build | PASS | 39m31s |  |
| audit | evaluate | PASS | 11m47s |  |
| build | build | PASS | 17m8s |  |
| audit | evaluate | PASS | 1m27s |  |
| ship | control | FAIL | 1s |  |
| ship | control | PASS | 5m24s |  |
| memo | control | PASS | 23s |  |

## Timing

**Total:** 2h12m26s across 12 phases (0 retried) · **Longest:** build 49m53s

| Archetype | Wall-clock |
|-----------|------------|
| build | 1h46m33s |
| control | 5m48s |
| evaluate | 14m58s |
| plan | 5m8s |
