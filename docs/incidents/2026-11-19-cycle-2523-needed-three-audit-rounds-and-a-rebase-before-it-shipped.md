# 2026-11-19: cycle 2523 needed three audit rounds and a rebase before it shipped

Related documents: the ratchet code is `go/internal/testaudit/tree.go`, and the package note [internal-testaudit.md](../architecture/packages/internal-testaudit.md) describes the package.

## Summary

- **Impact:** cycle 2523 shipped as commit `0718e2d6d` (04:59:10 UTC). From the first audit FAIL (03:21:04) to the ship commit, 98 min 6 s passed. The repair covered two more audit rounds, one TDD run, two build runs and a ship stop on a fleet rebase. The 88 minutes of the request are not reproduced (see Limits).
- **Root cause 1:** the first build made the ratchet refuse a rebaseline on the count of rows, not on the set of rows. An equal-count swap, and central growth from 0 to N, passed. The tests did not pin it.
- **Root cause 2:** the corrected build gave a wrong count in the explanation document: 92 for 93 packages. Audit round 2 asked for a document correction, and round 3 was needed.
- **Root cause 3:** main moved while the cycle repaired. The ship stopped on `GIT_FLEET_REBASE_NEEDED`.
- **Fix:** the repaired code and tests are in `0718e2d6d` (set-based refusal through `addedFindings`). No class fix is filed for root causes 2 and 3. The evidence shows none.
- **Lesson:** a ratchet must compare sets. A number in a document must come from the command that measures it.

## Terms used throughout

- **Ratchet:** the test-audit baseline. A weak test may stay, but a new weak test must fail the tree test.
- **Rebaseline:** the verb `evolve test audit --rebaseline <pkg>`. It writes the shard of baseline rows of a package.
- **Round:** one audit run on one version of the tree. Round 1 is the first.
- **X2 correction:** a document-only correction that follows an audit finding on the explanation.

## Timeline

| Time (UTC) | Event | Evidence |
|---|---|---|
| 11-19 00:50:32 | The first signal of cycle 2523. | warn signals |
| 00:52:33 | The router launch exits 85: `quota exhausted ... 5h window 100% used, resets 4h 57m`. The same signal repeats at 03:00:45, 03:21:17, 03:53:23, 04:05:33, 04:28:30 and 04:40:04 (7 times). | signals `BRIDGE_EXIT_QUOTA_WALL`, `BRIDGE_USAGE_EVIDENCE` |
| 01:02:15 | Build starts. | signal `SIGNAL_EXPECTED_ABSENT` |
| 01:57 and 02:06 | Two `BUILD_FLOOR_RETAKE_RED` on `TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial`. | signals |
| 02:08:16 | `ORCHESTRATOR_BLOCKER_REFUSED`: the blocker on `go/internal/repocontract/repocontract.go` is proven, but the claim also leaves the failures of `./cmd/evolve` and the scanner pack unexplained. Thus the signal refuses the claim as a whole. | signal |
| 03:00:32 | Shadow signal `CYCLE_SPEND_LIMIT`: `max_llm_minutes 91.2 of 90`. | signal |
| 03:21:04 | Audit round 1: verdict FAIL. Defects H1 (HIGH) and M1 (MEDIUM). `ORCHESTRATOR_AUDIT_REPAIR_DECLINED` does not appear for this FAIL. | signal, failure dossier |
| 03:24:56 | TDD runs again (context fill 66.1%). | signal |
| 03:41 to 03:42 | Build runs again. | signals |
| 03:56:33 | Audit round 2. The explanation says 92 packages, and the count is 93. | audit report round 3 |
| 04:17 to 04:18 | Build runs for the X2 document correction. | signals |
| 04:31:15 | Audit round 3: PASS, confidence 0.9, 10/10 ACS. | audit report |
| 04:40:46 | Ship stops: `GIT_FLEET_REBASE_NEEDED`. Main moved since the audit. | signal `SHIP_GIT_FLEET_REBASE_NEEDED` |
| 04:52:43 | The ship aborts and recovers through ship (try 1 of 9). | signal `ORCHESTRATOR_PHASE_ABORTED` |
| 04:59:10 | The ship commit `0718e2d6d` lands. | `git show --stat 0718e2d6d` |

## Key logs

The failure dossier after round 1:

```json
{"cycle_id":2523,"recorded_verdict":"FAIL",
 "audit_declared":{"class":"code-audit-fail","level":"task","defects":[
  "H1 HIGH go/internal/testaudit/tree.go:103 and :125 - Rebaseline refuses on row count, not row set: an equal-count swap (fix TestA, add weak TestB) and central 0->N growth are accepted, so the ratchet admits new weak tests",
  "M1 MEDIUM go/internal/testaudit/tree_test.go - no test pins the equal-count swap or the central 0->N growth; a count-only mutant passes the suite"]}}
```

The ship stop:

```text
SHIP_GIT_FLEET_REBASE_NEEDED main moved since the audit and the lane holds the
audited change; rebase it and re-check the merged tree: ship: fleet ff-merge
cycle-cd3ae73e-2523 into main diverged (a peer cycle moved main mid-pipeline);
rebase + re-verify the merged tree, then re-ship
```

The reflection of audit round 3:

```text
The cost of the extra round came from one miscount in the prose. A count in a
document should come from the command that measures it.
```

## Root cause

**F5.1** Rebaseline refused on the count of added rows, not on the set. An equal-count swap (fix `TestA`, add the weak `TestB`) and central growth from 0 to N were accepted (failure dossier, H1).
*Implication:* the ratchet admitted new weak tests. A ratchet exists to stop this defect.

**F5.2** No test pinned the swap or the 0 to N growth, so a count-only mutant passed the suite (M1).
*Implication:* the test suite did not see F5.1. The audit found it by reading the code and by mutation reasoning.

**F5.3** The repair added `addedFindings` (`go/internal/testaudit/tree.go:142-154`), which both paths call at `tree.go:104` and `tree.go:127`. It added `go/internal/testaudit/rebaseline_set_test.go` and ACS predicates 008 to 010 (audit report round 3).
*Implication:* a set-based refusal is now pinned by tests, and by predicates that name the swap and the growth.

**F5.4** The explanation document said 92 central packages. `jq '.packages|length' go/internal/testaudit/unwritable-baselines.json` gives 93. Round 2 found it, and the X2 correction changed two lines.
*Implication:* a count written by hand cost one more round. The code did not change in that round.

**F5.5** Main moved during the repair, so the audited tree no longer fast-forwarded. The ship needed a rebase and a re-check of the merged tree (signal `SHIP_GIT_FLEET_REBASE_NEEDED`).
*Implication:* each minute of repair raises the chance that a peer moves main. The ship recovery has 9 attempts. The evidence shows the first recovery and the commit.

**F5.6** The router launch hit the quota wall 7 times (5h window 100% used). Cycle 2523 continued each time.
*Implication:* this may have added wait time. The evidence does not show how long each wall delayed a phase, or which CLI took the launch.

## What changed

| Change | Pull request or commit | Regression test |
|---|---|---|
| The rebaseline and the tree test refuse on the set of added rows | commit `0718e2d6d` | `go/internal/testaudit/tree_test.go:70`, `go/internal/testaudit/rebaseline_set_test.go:17` and `:60`, `TestC2523_008` to `TestC2523_010` |
| The count of a document comes from the measuring command | none filed | open, no item in the evidence |
| A repair that outlasts a peer ship rebases earlier | none filed | open, no item in the evidence |

## Limits

- The request names an 88-minute repair. The evidence gives three spans from the first FAIL (03:21:04). To the rebase stop (04:40:46) it is 79 min 42 s. To the ship abort (04:52:43) it is 91 min 39 s. To the ship commit (04:59:10) it is 98 min 6 s. This record cannot say which two times the 88 minutes span.
- The evidence has no retrospective for 2523 and no build report. The audit report is the one for round 3. The text of rounds 1 and 2 comes from the failure dossier and from the round 3 report.
- The evidence does not say why the first build compared counts. That cause is open.
- The evidence holds only WARN, ERROR and INCIDENT signals. It does not show the start and end of each phase. The times of the TDD and build runs above are the times of their signals, not their starts.
- The signal `ORCHESTRATOR_SCOPE_DELTA` appears 804 times in 4 groups of 201 (at 01:43, 02:47, 03:42 and 04:18). This record does not say what the scope delta was.
- The evidence does not show the time of the ship rebase itself. The commit time is from `git show`.

## What we learned

- A ratchet compares sets. A count hides a swap.
- A test that kills a count-only mutant is part of the first build of a ratchet.
- Take each number in a document from a command, and name the command.
- A repair of more than one hour can meet a main that moved. Rebase before the last audit round when the repair is long.
