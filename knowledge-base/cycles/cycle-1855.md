# Cycle 1855 Dossier

**Goal:** Wave 90.

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

Wave 89 facts (run 20261009T085206Z): 1 of 2 cycles shipped.
- Cycle 1853: final verdict FAIL. Last phase: aborted. It did not ship. Quota pauses: 1.
- Cycle 1854: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Operator notes for wave 90:
- Pull requests merged since wave 89: #836 (a phase agent cannot reach or kill the run tmux server), #837 (Q1 derived catalog), #838 (an agent runs real-tmux tests on its own pty), #839 and #840 through train #841 (attempt postmortem; pane loss and quota evidence), and #842 (a retry and a resume carry the evidence of the dispatch they replace).
- Cycle 1853 resumed with its attempt postmortem and did not repeat the tmux probe. It then sealed FAIL on a task cause: the fix needs the protected bridge (ADR-0064). The item swarm-worker-pgid-never-recorded is now console work.
- This wave is the release gate of v22.28.0. Every cycle must ship clean, with no pipeline-defect failure.

Earlier waves:
- Wave 86 (run 20261009T055036Z): 2 of 2 cycles shipped.
- Wave 87 (run 20261009T070638Z): 2 of 2 cycles shipped.
- Wave 88 (run 20261009T075015Z): 2 of 2 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M4NQVDP0Q2VKFCGHBDJ0W2CM
**Commit:** d0bf86ae0b84dec25c1e98e511f7554bcbee7aa3
**Committed:** `coherence-failreasons-unread`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 5m20s |  |
| triage | plan | PASS | 4m7s |  |
| tdd | plan | PASS | 1m34s |  |
| build | build | PASS | 7m6s |  |
| audit | evaluate | PASS | 14m57s |  |
| ship | control | FAIL | 7m7s |  |
| ship | control | PASS | 6m51s |  |

## Timing

**Total:** 47m3s across 7 phases (0 retried) · **Longest:** audit 14m57s

| Archetype | Wall-clock |
|-----------|------------|
| build | 7m6s |
| control | 13m58s |
| evaluate | 14m57s |
| plan | 11m1s |
