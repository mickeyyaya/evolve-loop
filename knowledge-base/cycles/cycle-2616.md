# Cycle 2616 Dossier

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
**Run ID:** 01M84GE2SA841YA779YSTP67T3
**Commit:** 0d252b9ae2304d37b4396cf1ec4b4a0afde76b9a
**Committed:** `test-tmux-servers-die-with-their-test-process`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 59s |  |
| triage | plan | PASS | 43s |  |
| fault-localization | plan | PASS | 59s |  |
| bug-reproduction | evaluate | PASS | 52s |  |
| tdd | plan | PASS | 3m55s |  |
| build | build | PASS | 21m38s |  |
| audit | evaluate | PASS | 4m6s |  |
| ship | control | FAIL | 0s |  |
| ship | control | PASS | 9m25s |  |

## Timing

**Total:** 42m38s across 9 phases (0 retried) · **Longest:** build 21m38s

| Archetype | Wall-clock |
|-----------|------------|
| build | 21m38s |
| control | 9m25s |
| evaluate | 4m58s |
| plan | 6m36s |
