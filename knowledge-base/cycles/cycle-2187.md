# Cycle 2187 Dossier

**Goal:** Wave 157.

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

Wave 156 facts (run 20261105T005830Z): 3 of 9 cycles shipped.
- Cycle 2172: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=spine gate: next=ship blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-N class). Last phase: aborted. It did not ship.
- Cycle 2173: final verdict PASS. It shipped.
- Cycle 2174: no final verdict. Last phase: audit. It did not ship. Quota pauses: 1.
- Cycle 2175: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=spine gate: next=ship blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-N class). Last phase: aborted. It did not ship.
- Cycle 2176: final verdict PASS. It shipped.
- Cycle 2177: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=spine gate: next=ship blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-N class). Last phase: aborted. It did not ship.
- Cycle 2178: final verdict PASS. It shipped.
- Cycle 2179: no final verdict. Last phase: audit. It did not ship. Quota pauses: 1.
- Cycle 2180: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=spine gate: next=ship blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-N class). Last phase: aborted. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 153 (run 20261104T002909Z): 0 of 1 cycles shipped.
- Wave 154 (run 20261104T032203Z): 0 of 9 cycles shipped.
- Wave 155 (run 20261104T141300Z): 0 of 11 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M6QCJ33A35E1HK71DZ4CCM1K
**Commit:** c200b50d808b4d23f8fb259e12d780cfe539699d
**Committed:** `inbox-consumed-record-lookup-unique`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 58s |  |
| triage | plan | PASS | 38s |  |
| tdd | plan | PASS | 2m12s |  |
| build | build | PASS | 3m54s |  |
| audit | evaluate | WARN | 14m11s |  |
| build | build | PASS | 4m29s |  |
| audit | evaluate | WARN | 6m14s |  |
| ship | control | FAIL | 3m30s |  |
| ship | control | PASS | 4m28s |  |

## Timing

**Total:** 40m34s across 9 phases (0 retried) · **Longest:** audit 14m11s

| Archetype | Wall-clock |
|-----------|------------|
| build | 8m24s |
| control | 7m57s |
| evaluate | 20m24s |
| plan | 3m48s |
