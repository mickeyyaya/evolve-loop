# Cycle 1844 Dossier

**Goal:** Wave 84.

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

Wave 83 facts (run 20261008T221630Z): 1 of 2 cycles shipped.
- Cycle 1841: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 1842: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 82 (run 20261008T173042Z): 1 of 3 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M4F7GZE26J1ZNWFJG1S0SKXC
**Commit:** b7d31e54fb7e448bf35740f7264308bef0261756
**Committed:** `cli-phase-spec-phases`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 11m11s |  |
| triage | plan | PASS | 6m31s |  |
| api-contract-design | plan | PASS | 10m52s |  |
| tdd | plan | PASS | 2m16s |  |
| build | build | PASS | 12m53s |  |
| audit | evaluate | PASS | 4m5s |  |
| ship | control | PASS | 2m18s |  |

## Timing

**Total:** 50m7s across 7 phases (0 retried) · **Longest:** build 12m53s

| Archetype | Wall-clock |
|-----------|------------|
| build | 12m53s |
| control | 2m18s |
| evaluate | 4m5s |
| plan | 30m50s |
