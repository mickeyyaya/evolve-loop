# Cycle 2597 Dossier

**Goal:** Wave 186.

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

Wave 185 facts (run 20261121T081737Z): 4 of 9 cycles shipped.
- Cycle 2582: final verdict PASS. It shipped.
- Cycle 2583: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…cycle-N/ [inbox-mover] claimed: 2026-11-02T00-00-00Z-item-a.json → processing/cycle-N/ FAIL FAIL github.com/mickeyyaya/evolve-loop/go/cmd/evolve 696.475s FAIL. Last phase: aborted. It did not ship.
- Cycle 2584: final verdict PASS. It shipped.
- Cycle 2585: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2586: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2587: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…ate_BaseRedWithRedMainParks2477548364/001/repo/.evolve/runs/cycle-N/ship-repocontract-scan.log [ship] repo-contract gate: GIT_FLEET_REBASE_NEEDED; full outpu…. Last phase: aborted. It did not ship.
- Cycle 2588: final verdict PASS. It shipped.
- Cycle 2589: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2590: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Operator notes for wave 186:
- At this boundary PR #1656 landed: the run that holds the host verification lock gets the host CPU budget (6 CPUs at 5 lanes, not 1). PR #1655 landed: the integration tier passes go test the shared 20 minute budget under a 21 minute tier budget. Wave 185 lost 5 of 9 cycles to host contention.
- The fleet runs 5 lanes (PR #1645) until the lock fix proves out. Each lock acquire now writes the signal HOST_VERIFY_LOCK_WAIT with its wait time.

Earlier waves:
- Wave 182 (run 20261119T213950Z): 7 of 11 cycles shipped.
- Wave 183 (run 20261120T094708Z): 9 of 15 cycles shipped.
- Wave 184 (run 20261120T204107Z): 5 of 8 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M7ZZ4XPF4KKZ1XBZYZVNQ1G7
**Commit:** 92a694b29308122a1bc819b202738180a6b8c69f
**Committed:** `sync-main-carries-console-inbox-edits`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 39s |  |
| triage | plan | PASS | 52s |  |
| fault-localization | plan | PASS | 1m0s |  |
| bug-reproduction | evaluate | PASS | 31s |  |
| tdd | plan | PASS | 3m24s |  |
| build | build | PASS | 29m33s |  |
| audit | evaluate | PASS | 2m53s |  |
| ship | control | FAIL | 0s |  |
| ship | control | FAIL | 12m24s |  |
| audit | evaluate | PASS | 2m10s |  |
| ship | control | FAIL | 0s |  |
| audit | evaluate | PASS | 2m27s |  |
| ship | control | PASS | 21m18s |  |

## Timing

**Total:** 1h17m11s across 13 phases (0 retried) · **Longest:** build 29m33s

| Archetype | Wall-clock |
|-----------|------------|
| build | 29m33s |
| control | 33m43s |
| evaluate | 8m0s |
| plan | 5m55s |
