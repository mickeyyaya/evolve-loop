# Cycle 2225 Dossier

**Goal:** Wave 161.

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

Wave 160 facts (run 20261107T090007Z): 9 of 16 cycles shipped.
- Cycle 2208: final verdict PASS. It shipped.
- Cycle 2209: final verdict PASS. It shipped.
- Cycle 2210: final verdict PASS. It shipped.
- Cycle 2211: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…tomic-ship] ship: fleet ff-merge cycle-cd3ae73e-2211 into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the merged tree, then re-ship. Last phase: aborted. It did not ship.
- Cycle 2212: final verdict PASS. It shipped.
- Cycle 2213: final verdict PASS. It shipped.
- Cycle 2214: final verdict PASS. It shipped.
- Cycle 2215: final verdict PASS. It shipped.
- Cycle 2216: no final verdict. Last phase: audit. It did not ship. Quota pauses: 1.
- Cycle 2217: final verdict PASS. It shipped.
- Cycle 2218: final verdict PASS. It shipped.
- Cycle 2219: no final verdict. Last phase: audit. It did not ship. Quota pauses: 1.
- Cycle 2220: no final verdict. Last phase: tdd. It did not ship. Quota pauses: 1.
- Cycle 2221: final verdict SKIPPED_UNKNOWN. Termination: triage-empty-commitment. Last phase: triage. It did not ship.
- Cycle 2222: final verdict SKIPPED_UNKNOWN. Termination: triage-empty-commitment. Last phase: triage. It did not ship.
- Cycle 2223: no final verdict. Last phase: tdd. It did not ship. Quota pauses: 1.
- Pull requests merged at this boundary: none.

Operator notes for wave 161:
- The weekly usage reset. Wave 160 shipped 10 of 16 and stopped at the usage limit. Main was red on a coverage gap from cycle 2218, fixed before this wave.

Earlier waves:
- Wave 157 (run 20261106T023714Z): 4 of 16 cycles shipped.
- Wave 158 (run 20261106T093235Z): 2 of 8 cycles shipped.
- Wave 159 (run 20261106T154410Z): 0 of 4 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M6Y1S2VVDEKQK41HQF07ZWWV
**Commit:** c63b9bd24bc53e6784f4aa2c37daa80f266793c9
**Committed:** `model-level-benches-from-per-model-usage-windows-console`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 7m46s |  |
| triage | plan | PASS | 6m32s |  |
| tdd | plan | PASS | 4m51s |  |
| build | build | PASS | 25m33s |  |
| audit | evaluate | FAIL | 4m25s |  |
| tdd | plan | PASS | 2m59s |  |
| build | build | PASS | 30m37s |  |
| audit | evaluate | PASS | 3m35s |  |
| ship | control | FAIL | 7m4s |  |
| audit | evaluate | PASS | 2m18s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | PASS | 2m43s |  |
| ship | control | FAIL | 6m55s |  |
| build | build | PASS | 39m37s |  |
| audit | evaluate | PASS | 2m49s |  |
| ship | control | PASS | 6m44s |  |

## Timing

**Total:** 2h34m29s across 16 phases (0 retried) · **Longest:** build 39m37s

| Archetype | Wall-clock |
|-----------|------------|
| build | 1h35m47s |
| control | 20m44s |
| evaluate | 15m50s |
| plan | 22m8s |
