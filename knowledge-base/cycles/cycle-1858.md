# Cycle 1858 Dossier

**Goal:** Wave 91.

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

Wave 90 facts (run 20261011T145758Z): 2 of 2 cycles shipped.
- Cycle 1855: final verdict PASS. It shipped.
- Cycle 1856: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 87 (run 20261009T070638Z): 2 of 2 cycles shipped.
- Wave 88 (run 20261009T075015Z): 2 of 2 cycles shipped.
- Wave 89 (run 20261009T085206Z): 1 of 2 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M4QTSPHZRGZ0N8D57466EZW1
**Commit:** d9491a588fa0c1dcb5bd77390973114e76ea64ff
**Committed:** `remote-open-pr-uses-gh-dash-c`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 11m8s |  |
| triage | plan | PASS | 4m52s |  |
| fault-localization | plan | PASS | 5m30s |  |
| bug-reproduction | evaluate | PASS | 7m56s |  |
| tdd | plan | PASS | 1m11s |  |
| build | build | PASS | 22m16s |  |
| audit | evaluate | PASS | 1m10s |  |
| ship | control | FAIL | 3m3s |  |
| ship | control | PASS | 2m47s |  |

## Timing

**Total:** 59m53s across 9 phases (0 retried) · **Longest:** build 22m16s

| Archetype | Wall-clock |
|-----------|------------|
| build | 22m16s |
| control | 5m50s |
| evaluate | 9m6s |
| plan | 22m41s |
