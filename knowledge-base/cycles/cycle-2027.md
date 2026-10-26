# Cycle 2027 Dossier

**Goal:** Wave 140.

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

Wave 139 facts (run 20261020T162621Z): 2 of 5 cycles shipped.
- Cycle 2020: no final verdict. Termination: stopped: lane deadline. Last phase: build. It did not ship.
- Cycle 2021: final verdict PASS. It shipped.
- Cycle 2022: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…mPolicy2677044484/002/ship-repocontract-scan.log FAIL coverage: 98.2% of statements FAIL github.com/mickeyyaya/evolve-loop/go/internal/phases/ship 103.089s FAIL. Last phase: aborted. It did not ship.
- Cycle 2023: final verdict PASS. It shipped.
- Cycle 2024: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…TestWorktreeCarrier_DirtyReuseQuarantineReachesTheOrchestratorWriter did not run and pass FAIL FAIL github.com/mickeyyaya/evolve-loop/go/acs/cycle-N 3.040s FAIL. Last phase: aborted. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 136 (run 20261020T042221Z): 0 of 1 cycles shipped.
- Wave 137 (run 20261020T063647Z): 2 of 5 cycles shipped.
- Wave 138 (run 20261020T123050Z): 1 of 6 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M5DXY4TPJ71VPTD1AXECPKRB
**Commit:** 9f6c5f0bc4abcebe6ab6bdf71c4b7cab1c6883ae
**Committed:** `practice-claude-model-release-notes-bfd090d6-202610210011`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 32s |  |
| triage | plan | PASS | 30s |  |
| tdd | plan | PASS | 1m14s |  |
| build | build | PASS | 1m5s |  |
| audit | evaluate | WARN | 2m23s |  |
| ship | control | PASS | 59s |  |

## Timing

**Total:** 6m42s across 6 phases (0 retried) · **Longest:** audit 2m23s

| Archetype | Wall-clock |
|-----------|------------|
| build | 1m5s |
| control | 59s |
| evaluate | 2m23s |
| plan | 2m15s |
