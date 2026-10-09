# Cycle 1845 Dossier

**Goal:** Wave 85.

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

Wave 84 facts (run 20261009T021848Z): 2 of 2 cycles shipped.
- Cycle 1843: final verdict PASS. It shipped.
- Cycle 1844: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 82 (run 20261008T173042Z): 1 of 3 cycles shipped.
- Wave 83 (run 20261008T221630Z): 1 of 2 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M4FD078WPBCVMWQW5Y945RMT
**Commit:** abbb528923469dcde7452070b8472910b394e50f
**Committed:** `ci-watch-timeout-exits-zero`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 19m19s |  |
| triage | plan | PASS | 6m8s |  |
| fault-localization | plan | PASS | 13m44s |  |
| bug-reproduction | evaluate | PASS | 12m21s |  |
| tdd | plan | PASS | 3m40s |  |
| build | build | PASS | 8m26s |  |
| audit | evaluate | PASS | 1m38s |  |
| ship | control | FAIL | 2m37s |  |
| ship | control | PASS | 3m22s |  |

## Timing

**Total:** 1h11m14s across 9 phases (0 retried) · **Longest:** scout 19m19s

| Archetype | Wall-clock |
|-----------|------------|
| build | 8m26s |
| control | 5m59s |
| evaluate | 13m59s |
| plan | 42m51s |
