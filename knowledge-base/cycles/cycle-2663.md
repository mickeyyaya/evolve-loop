# Cycle 2663 Dossier

**Goal:** Wave 192.

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

Wave 191 facts (run 20261126T214239Z): 3 of 3 cycles shipped.
- Cycle 2656: final verdict PASS. It shipped.
- Cycle 2657: final verdict PASS. It shipped.
- Cycle 2658: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 188 (run 20261123T125440Z): 11 of 20 cycles shipped.
- Wave 189 (run 20261125T020616Z): 15 of 20 cycles shipped.
- Wave 190 (run 20261126T115446Z): 2 of 4 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M8DS71P0TG54MRHH8YNJXKQ6
**Commit:** 6dd24b416b9288e5c6fcc7ec70ebfc8509f6a30a
**Committed:** `checkpoint-save-all-skips-a-worktree-mid-merge`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 55s |  |
| triage | plan | PASS | 36s |  |
| fault-localization | plan | PASS | 39s |  |
| bug-reproduction | evaluate | PASS | 39s |  |
| tdd | plan | PASS | 56s |  |
| build | build | PASS | 33m21s |  |
| audit | evaluate | PASS | 5m1s |  |
| ship | control | FAIL | 0s |  |
| ship | control | PASS | 10m13s |  |

## Timing

**Total:** 52m21s across 9 phases (0 retried) · **Longest:** build 33m21s

| Archetype | Wall-clock |
|-----------|------------|
| build | 33m21s |
| control | 10m13s |
| evaluate | 5m41s |
| plan | 3m6s |
