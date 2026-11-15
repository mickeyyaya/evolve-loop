# Cycle 2382 Dossier

**Goal:** Wave 173.

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

Wave 172 facts (run 20261114T064820Z): 4 of 4 cycles shipped.
- Cycle 2374: final verdict PASS. It shipped.
- Cycle 2375: final verdict PASS. It shipped.
- Cycle 2376: final verdict PASS. It shipped.
- Cycle 2377: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 169 (run 20261113T084906Z): 1 of 4 cycles shipped.
- Wave 170 (run 20261113T094348Z): 1 of 4 cycles shipped.
- Wave 171 (run 20261113T194212Z): 9 of 16 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M7D6HNNAB37MNK0S8XG1NYSZ
**Commit:** cc1f0c2b72f5fbcdc6e473e0a3a17d6a23117f9a
**Committed:** `arch-decisions-ad2-3-the-verb-evolve-inbox-trailer-check`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m30s |  |
| triage | plan | PASS | 1m9s |  |
| tdd | plan | PASS | 6m1s |  |
| build | build | PASS | 19m23s |  |
| audit | evaluate | WARN | 1m59s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | WARN | 1m9s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | WARN | 1m51s |  |
| ship | control | FAIL | 27m5s |  |
| ship | control | FAIL | 25m7s |  |
| audit | evaluate | WARN | 1m41s |  |
| ship | control | FAIL | 23m43s |  |
| ship | control | FAIL | 23m1s |  |
| audit | evaluate | WARN | 1m34s |  |
| ship | control | FAIL | 23m29s |  |
| ship | control | PASS | 25m26s |  |

## Timing

**Total:** 3h5m7s across 17 phases (0 retried) · **Longest:** ship 27m5s

| Archetype | Wall-clock |
|-----------|------------|
| build | 19m23s |
| control | 2h27m50s |
| evaluate | 8m14s |
| plan | 9m39s |
