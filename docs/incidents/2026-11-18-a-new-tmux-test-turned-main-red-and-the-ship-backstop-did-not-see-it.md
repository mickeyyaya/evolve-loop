# 2026-11-18: a new tmux test turned main red and the ship backstop did not see it

Related documents: the ship backstop is `go/internal/phases/ship/repocontract_importers.go`, the tree-scan guard is `go/internal/tmuxtest/tmux_package_test.go`, and the fix of cycle 2513 is in [the faketmux ps record](2026-11-18-faketmux-tests-need-a-ps-binary-that-a-lane-cell-denies.md).

## Summary

- **Impact:** main was red from commit `b2486e62b` (15:57:45 UTC) until commit `c6c8eaedb` (19:06:38 UTC), 3 h 8 min. Cycle 2510 passed its audit (round 3, confidence 0.88) and then failed at ship on a red that it did not cause. Its worktree needed a salvage.
- **Root cause:** cycle 2509 shipped `go/internal/faketmux/contract_test.go`, which starts a real tmux, in a package with no `TestMain` that calls `tmuxtest.Main`. The scan test `TestEveryTmuxTestPackageUsesTheOrphanGuard` reads the source of all packages. It does not import them.
- **Escape:** the ship backstop tests only the importers of the packages that a ship changes. `internal/tmuxtest` does not import `internal/faketmux`. So the backstop did not select the scan test, and 2509 shipped green.
- **Fix:** commit `c6c8eaedb` adds `go/internal/faketmux/main_test.go` on main. The class fix is the inbox item `ship-backstop-misses-tree-scan-guards` (open).
- **Lesson:** a test that scans the whole tree is a gate for every change. The importer graph cannot select it.

## Terms used throughout

- **Importer backstop:** the ship check that runs the tests of the packages that import what the ship changes.
- **Tree-scan guard:** a test that reads the source files of the whole module and fails on a rule break. It imports none of the packages that it checks.
- **Orphan guard:** `tmuxtest.Main`. A package that starts a real tmux server in a test must call it from `TestMain`. Then the server cannot hold the `go test` output pipe.
- **Lane:** one cycle that runs in its own worktree.

## Timeline

| Time (UTC, 2026-11-18) | Event | Evidence |
|---|---|---|
| 15:57:45 | Cycle 2509 ships `b2486e62b`. It adds `go/internal/faketmux/contract_test.go` (260 lines) and no `TestMain`. The audit of 2509 was PASS (confidence 0.86). | `git show --stat b2486e62b`, cycle 2509 audit report |
| 16:04 to 16:23 | Cycle 2510 builds a change of test and eval files for `internal/bridge`. | cycle 2510 retrospective |
| 16:23:18 | Ship of 2510 stops on `SHIP_REPO_CONTRACT_GATE`. The importer backstop ran `internal/tmuxtest`, and the scan test named `internal/faketmux`. The cycle recovers through audit (try 1 of 2). | signal `SHIP_REPO_CONTRACT_GATE` |
| 16:38:35 | The same stop. Try 2 of 2. | signal `SHIP_REPO_CONTRACT_GATE` |
| 16:45:10 | Shadow signal `CYCLE_SPEND_LIMIT`: `max_audit_rounds 3 of 2`. | signal `CYCLE_SPEND_LIMIT` |
| 16:51:55 | The third ship stop. The ship aborts with verdict FAIL. | ship-failure diagnosis |
| 16:53:24 | `ORCHESTRATOR_CYCLE_FAILED`. The failure digest records class recurrence 10 and routes the cycle to the console. | failure digest |
| 19:06:38 | Commit `c6c8eaedb` puts `go/internal/faketmux/main_test.go` (12 lines) on main. | `git show --stat c6c8eaedb` |

## Key logs

The ship stop, trimmed. It is the same text at 16:23:18, 16:38:35 and 16:51:55.

```text
[REPO_CONTRACT_GATE/precondition @atomic-ship] repo-contract importer backstop
(the packages that import what this ship changes) RED in the lane worktree
(.../go/internal/tmuxtest: exit status 1) - failing:
.../go/internal/tmuxtest.TestEveryTmuxTestPackageUsesTheOrphanGuard
- pushing would red main; land the green fix or use an explicit t.Skip for an
intentionally red-first reproducer
```

The failure decision of cycle 2510:

```json
{"category":"infra-systemic","level":"system","action":"halt-and-diagnose",
 "fix_type":"pipeline-repair",
 "justification":"main is red from cycle 2509's faketmux contract test, so the pipeline (not the task code) blocks this and every later lane that selects internal/tmuxtest; re-audit rounds cannot clear it"}
```

The scan test message, from the audit of 2510 (round 3):

```text
... without a TestMain that calls tmuxtest.Main ...: [internal/faketmux]
```

## Root cause

**F1.1** `TestEveryTmuxTestPackageUsesTheOrphanGuard` fails when a package starts a real tmux in a test and has no `TestMain` that calls `tmuxtest.Main` (`go/internal/tmuxtest/tmux_package_test.go:22`). Commit `b2486e62b` added `contract_test.go` to `internal/faketmux`. The package had no such `TestMain`.
*Implication:* the commit broke a repo rule, and a test existed that can say so.

**F1.2** The scan test reads the source of all packages through `scanTestPackages(t, moduleRoot)`. It imports none of them. The backstop selects the tests of the importers of a changed package, and `internal/tmuxtest` does not import `internal/faketmux`.
*Implication:* the backstop did not select the scan test for 2509. The cycle that broke the rule shipped green. This is the escape.

**F1.3** The backstop selected `internal/tmuxtest` for 2510. The audit of 2510 (round 3) ran the test on the main checkout at `b2486e62b`, and it failed there with the same message. The 2510 diff has four test and eval files and touches no faketmux or tmuxtest file.
*Implication:* the red belonged to main. No lane can fix it, because `internal/faketmux` was outside the scope of 2510. Three rounds did not change the result.

**F1.4** The ship recovery re-ran audit twice (attempts 1/2 and 2/2) after a stop that the auditor had proven to be a red on main.
*Implication:* the retry rounds ran against a cause outside the lane. The retrospective of 2510 asks for a rule that stops the rounds and routes to the console. It proposes the inbox item `stop-retry-rounds-when-ship-red-reproduces-on-main`. The state of the item is not in the evidence.

## What changed

| Change | Pull request or commit | Regression test |
|---|---|---|
| `internal/faketmux` gets a `TestMain` that calls `tmuxtest.Main` | commit `c6c8eaedb` | `TestEveryTmuxTestPackageUsesTheOrphanGuard` (`go/internal/tmuxtest`) |
| `TestEveryTmuxTestPackageUsesTheOrphanGuard` becomes a by-name row of `repocontract.TreeReadingTests()`, so every ship runs it. The detector now reads a root-only string constant, which is the shape of the guard's own climb | inbox item `ship-backstop-misses-tree-scan-guards` | `TestRepoContractGate_AnAddedTmuxTestRunsTheOrphanGuardByName`, `TestRepoContractGate_ARedOrphanGuardBlocksShipAndNamesItself` (`go/internal/phases/ship`), `TestReadingTestsOf_CreditsTheTestsThatReachARead` (`go/internal/repocontract`) |
| The retry rounds stop when a ship red reproduces on main at the same base | inbox item `stop-retry-rounds-when-ship-red-reproduces-on-main` (proposed in the retrospective of 2510) | open |

## Limits

- The inbox items are in the runtime inbox, not in git. This record cannot show their state beyond what the evidence gives.
- The console checked the red run with `gh run view 44341646289`: the required CI of main on `b2486e62b` started at 15:57:50 UTC and failed. This record did not read the log of that run.
- The evidence does not say if another path runs the scan test for 2509, for example as an added test. It also does not say through which import the backstop selected `internal/tmuxtest` for 2510. The retrospective states that the importer graph did not select it. This record does not prove the whole selection logic.
- Commit `c6c8eaedb` also holds the work of cycle 2514. This record does not say which cycle or person made the `main_test.go` change inside it. Cycle 2513 built the same 12-line file, and its audit failed on a different cause (see the record linked above).
- The scan test message in the key logs is quoted from the audit report in short form.

## What we learned

- A scanner that reads the whole tree is a gate for every change. Select it by the files in the diff (a new or changed `_test.go` file), not by the importer graph.
- A ship that fails on a red that the auditor reproduces on main is not a lane failure. Route it to the console at once.
- A fix of one 12-line `TestMain` file cost 3 h 8 min of red main and one failed cycle. The check that finds the break was not in the path of the cycle that made it.
