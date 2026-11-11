# Cycle 2278 Dossier

**Goal:** Wave 164.

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

Wave 163 facts (run 20261109T204337Z): 14 of 16 cycles shipped.
- Cycle 2252: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…tomic-ship] ship: fleet ff-merge cycle-cd3ae73e-2252 into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the merged tree, then re-ship. Last phase: aborted. It did not ship.
- Cycle 2253: final verdict PASS. It shipped.
- Cycle 2254: final verdict PASS. It shipped.
- Cycle 2255: final verdict PASS. It shipped.
- Cycle 2256: final verdict PASS. It shipped.
- Cycle 2257: final verdict SKIPPED_UNKNOWN. Termination: triage-empty-commitment. Last phase: triage. It did not ship.
- Cycle 2258: final verdict PASS. It shipped.
- Cycle 2259: final verdict PASS. It shipped.
- Cycle 2260: final verdict PASS. It shipped.
- Cycle 2261: final verdict PASS. It shipped.
- Cycle 2262: final verdict PASS. It shipped.
- Cycle 2263: final verdict PASS. It shipped.
- Cycle 2264: final verdict PASS. It shipped.
- Cycle 2265: final verdict PASS. It shipped.
- Cycle 2266: final verdict PASS. It shipped.
- Cycle 2267: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 160 (run 20261107T090007Z): 9 of 16 cycles shipped.
- Wave 161 (run 20261108T161932Z): 9 of 14 cycles shipped.
- Wave 162 (run 20261109T053101Z): 11 of 16 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M72N3JT60TJR25HMM1KR6WM4
**Commit:** 0dfcb5c9b773355a13bf521750612239399eb1e3
**Committed:** `practice-claude-prompting-guide-bc673fd8-202611100841`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 40s |  |
| triage | plan | PASS | 37s |  |
| tdd | plan | PASS | 51s |  |
| build | build | PASS | 7m13s |  |
| audit | evaluate | WARN | 9m26s |  |
| ship | control | PASS | 44s |  |
| memo | control | PASS | 29s |  |

## Timing

**Total:** 19m59s across 7 phases (0 retried) · **Longest:** audit 9m26s

| Archetype | Wall-clock |
|-----------|------------|
| build | 7m13s |
| control | 1m12s |
| evaluate | 9m26s |
| plan | 2m8s |
