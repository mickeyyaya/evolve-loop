# Cycle 2212 Dossier

**Goal:** Wave 160.

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

Wave 159 facts (run 20261106T154410Z): 0 of 4 cycles shipped.
- Cycle 2195: final verdict FAIL. Termination: cycle aborted in phase ship (abnormal-exit epilogue) cause=…ted the push of 5243eb97ee28be13f9b92d07bd7e07498bf2e248 because origin/main diverged; Audit must run again on the new base (ship does not rebase or force-push). Last phase: aborted. It did not ship. Quota pauses: 1.
- Cycle 2197: no final verdict. Last phase: build. It did not ship. Quota pauses: 1.
- Cycle 2205: no final verdict. Last phase: build. It did not ship.
- Cycle 2206: no final verdict. Last phase: audit. It did not ship.
- Pull requests merged at this boundary: none.

Operator notes for wave 160:
- The operator asked to resume the loop and use the remaining weekly usage. Wave 159 was stopped for the release work. v23.2.1 is out.

Earlier waves:
- Wave 156 (run 20261105T005830Z): 3 of 9 cycles shipped.
- Wave 157 (run 20261106T023714Z): 4 of 16 cycles shipped.
- Wave 158 (run 20261106T093235Z): 2 of 8 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M6TR3PR3V09XBP3ZF03VFS1Q
**Commit:** 1128e9d82c2a92d9c81ab18461be35fe2298ce42
**Committed:** `retry-ladder-failure-record-carries-rung`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 45s |  |
| triage | plan | PASS | 32s |  |
| tdd | plan | PASS | 5m34s |  |
| build | build | PASS | 10m16s |  |
| audit | evaluate | WARN | 7m8s |  |
| build | build | PASS | 5m58s |  |
| audit | evaluate | WARN | 2m37s |  |
| ship | control | FAIL | 20m39s |  |
| ship | control | PASS | 5m31s |  |

## Timing

**Total:** 59m0s across 9 phases (0 retried) · **Longest:** ship 20m39s

| Archetype | Wall-clock |
|-----------|------------|
| build | 16m14s |
| control | 26m11s |
| evaluate | 9m45s |
| plan | 6m51s |
