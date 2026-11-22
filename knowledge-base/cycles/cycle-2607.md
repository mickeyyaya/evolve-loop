# Cycle 2607 Dossier

**Goal:** Wave 187.

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

Wave 186 facts (run 20261121T192937Z): 10 of 13 cycles shipped.
- Cycle 2592: final verdict PASS. It shipped.
- Cycle 2593: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…tomic-ship] ship: fleet ff-merge cycle-cd3ae73e-2593 into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the merged tree, then re-ship. Last phase: aborted. It did not ship.
- Cycle 2594: final verdict PASS. It shipped.
- Cycle 2595: final verdict PASS. It shipped.
- Cycle 2596: final verdict PASS. It shipped.
- Cycle 2597: final verdict PASS. It shipped.
- Cycle 2598: final verdict PASS. It shipped.
- Cycle 2599: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…rsesByKindAndTime error-text-match has no baseline row — remove the weak pattern (for error text: a sentinel error and errors.Is); Rebaseline refuses new rows. Last phase: aborted. It did not ship.
- Cycle 2600: final verdict PASS. It shipped.
- Cycle 2601: final verdict PASS. It shipped.
- Cycle 2602: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…tomic-ship] ship: fleet ff-merge cycle-cd3ae73e-2602 into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the merged tree, then re-ship. Last phase: aborted. It did not ship.
- Cycle 2603: final verdict PASS. It shipped.
- Cycle 2604: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Operator notes for wave 187:
- Operator 2026-11-22: this wave builds the static gc flow (plan PR #1683): recycle X2 (the cache trim counts executable dirs), X1b run dirs, X1d temp, X1e processes and X8 test tmux socket reap. In wave 186 free disk fell to 5 GiB and the disk guard paused cycle starts 20 times.
- New on main: PR #1662 heartbeats the ship-window lease; PR #1672 lists each test pattern the test-audit ratchet refuses in the engineering-craft skill (never match error text, never sync with a sleep); PR #1661 adds evolve wave checkin.

Earlier waves:
- Wave 183 (run 20261120T094708Z): 9 of 15 cycles shipped.
- Wave 184 (run 20261120T204107Z): 5 of 8 cycles shipped.
- Wave 185 (run 20261121T081737Z): 4 of 9 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M81MPGV3DFQK6AJH3MVV5GP4
**Commit:** 5c1412b85dc60b1040da01b833ab1c11a012a3a4
**Committed:** `recycle-x1d-check-registry-temp`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 50s |  |
| triage | plan | PASS | 40s |  |
| tdd | plan | PASS | 3m57s |  |
| build | build | PASS | 24m41s |  |
| audit | evaluate | PASS | 5m21s |  |
| ship | control | PASS | 7m13s |  |

## Timing

**Total:** 42m42s across 6 phases (0 retried) · **Longest:** build 24m41s

| Archetype | Wall-clock |
|-----------|------------|
| build | 24m41s |
| control | 7m13s |
| evaluate | 5m21s |
| plan | 5m28s |
