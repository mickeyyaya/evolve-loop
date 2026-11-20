# 2026-11-18: blocker claims on a protected test file were refused in cycles 2508 and 2520

Related documents: the blocker prover is `go/internal/core/blocker_proof.go`, the refusal signal is declared in `go/internal/core/build_blocker_exit.go`, and the protected surface is `go/internal/guards/integrity_surface.go`.

## Summary

- **Impact:** cycle 2508 ran its build phase for 3 h 9 min (15:02:09 to 18:11:08 UTC) over two refused claims and failed. Cycle 2520 ran its build phase for 1 h 56 min (21:31:25 to 23:27:42 UTC) over two refused claims and failed. Each cycle had a complete diff in its worktree. Each needed a console salvage.
- **Root cause:** a lane planned work that needs an edit of a protected file. The lane then declared a `BLOCKER` line. The prover refused each of the four claims, so the correction rounds went on against a result that no lane can change.
- **Why the claims were refused:** all four claims name a path that ends in `_test.go`. `claimTargetProblem` refuses every path that is not a non-test Go source file (`go/internal/core/blocker_proof.go:249`). The signal text is cut at 300 characters, so the refusal reason itself is not in the evidence. This is inferred from the code (see Limits).
- **Escape:** the scout of 2508 gave its target set as a glob, which hides the protected file from the route check. In 2520 the scan test of TDD needs a line in the protected `repocontract.go`. The TDD floor does not run the check that finds it.
- **Fix:** inbox items from the retrospectives (all open or not shown in git): `route-check-expands-glob-targets`, `protected-path-blocker-stops-build-redispatch`, `tdd-floor-run-repocontract-whole-tree-registration-check`, `stop-build-corrections-on-protected-floor-blocker`.
- **Lesson:** a blocker that a lane cannot prove must stop the rounds as fast as a proven one. The prover must also handle the protected test files that blockers most often name.

## Terms used throughout

- **BLOCKER line:** a defect line of the form `BLOCKER protected-path <path> <why>` or `BLOCKER predicate-conflict <failing test> <guard test> <path> <why>`. A builder writes it to say that the work cannot finish in the lane.
- **Prover:** `deterministicBlockerProver`. It checks each claim and returns a typed proof. A proven blocker stops the correction rounds and routes the item to the console. A refusal sends a correction.
- **Protected surface:** the manifest of files that a lane may not edit.
- **Build floor:** the checks that run at the build handoff.

## Timeline

| Time (UTC, 2026-11-18) | Event | Evidence |
|---|---|---|
| 15:02:09 | Build of cycle 2508 starts. The item moves the serial-test declarations of about 939 map entries into the tests. | signal `SIGNAL_EXPECTED_ABSENT`, retrospective 2508 |
| 15:24 to 15:32 | Three `BUILD_FLOOR_RETAKE_RED` signals on `TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial`. | signals |
| 16:44:08 | `ORCHESTRATOR_BLOCKER_REFUSED` (2508): `BLOCKER protected-path go/cmd/evolve/request_channel_test.go:178 ...`. The build aborts: the TDD predicate failed byte-identically across a correction. | signals |
| 18:11:08 | `ORCHESTRATOR_BLOCKER_REFUSED` (2508): `BLOCKER predicate-conflict TestParallelDefault_... build-floor-protected-control-plane-path-rule go/cmd/evolve/request_channel_test.go:178 ...`. The build aborts at the review gate after 1 correction. | signals |
| 18:12:56 | Cycle 2508 fails. Class recurrence 14, route console. | failure digest |
| 21:31:25 | Build of cycle 2520 starts. | signal `SIGNAL_EXPECTED_ABSENT` |
| 22:03:20 | `ORCHESTRATOR_BLOCKER_REFUSED` (2520): `BLOCKER predicate-conflict ... go/acs/cycle1932/predicates_test.go:305 ...`. | signal |
| 22:32:08 | `ORCHESTRATOR_BLOCKER_REFUSED` (2520): `BLOCKER protected-path go/acs/cycle1932/predicates_test.go:88 ...`. | signal |
| 23:27:42 | The build aborts after 2 corrections. The floor names `repocontract.TestPackages_HoldEveryTestThatReadsTheWholeTree`. | signal `ORCHESTRATOR_PHASE_ABORTED` |
| 23:31:39 | Cycle 2520 fails. Class recurrence 15, route console. | failure digest |

## Key logs

The refusal of 2508 at 16:44:08, as the signal holds it (cut at 300 characters):

```text
ORCHESTRATOR_BLOCKER_REFUSED the declared blocker is not proven: "BLOCKER
protected-path go/cmd/evolve/request_channel_test.go:178 the role guard denies
a build edit of this file; TestWireOrchestratorDeps_RequestChannelWired needs
serial(\"calls process-global setter SetModelCatalogDirFn\") as its first
statement, so TestParall
```

The refusal of 2520 at 22:32:08:

```text
ORCHESTRATOR_BLOCKER_REFUSED the declared blocker is not proven: "BLOCKER
protected-path go/acs/cycle1932/predicates_test.go:88 the shipped cycle-1932
predicates build ApplyOptions{Reweight:...} and run
TestApplyEscalationBoundary_ReweightsAnOpenItemThroughIn
```

The failure decision of 2520:

```json
{"category":"code-build-fail","level":"task","action":"defer-or-quarantine",
 "fix_type":"pipeline-repair",
 "justification":"The task cannot pass in a lane without console edits to protected repocontract.go and non-lane-editable go/acs/cycle1932, so the item must be deferred/quarantined to console rather than retried in a lane."}
```

## Root cause

**F3.1** In 2508, the protected file was `go/cmd/evolve/request_channel_test.go` (`go/internal/guards/integrity_surface.go:142`, class DispatchCredentialGuarantee). One entry of the central map, `TestWireOrchestratorDeps_RequestChannelWired`, lives in it. The role guard denied the edit four times. After the builder restored the file, that test had no `t.Parallel()` and no `serial(...)` line, so the guard test failed (retrospective 2508, D1 and D2).
*Implication:* the task needed one line in a file that no lane may write. The rest of the work was complete.

**F3.2** The scout of 2508 wrote its target set as a glob, `go/cmd/evolve/*_test.go (tests listed in the map)`. A glob does not intersect the protected manifest, so the route check did not see the file.
*Implication:* the plan sent a protected-surface fan-out to a lane. The retrospective names the class as SYSTEMIC (cycles 1063, 1216, 1426, 2405 and 2494 before it).

**F3.3** In 2520, TDD wrote `go/internal/inboxmover/weightwrite_scan_test.go`, which walks the module with `filepath.WalkDir`. The build floor runs `repocontract.TestPackages_HoldEveryTestThatReadsTheWholeTree`. It needs the new test in `TreeReadingTests()` in `go/internal/repocontract/repocontract.go`, a GateGuarantee file (`go/internal/guards/integrity_surface.go:245`). The TDD floor does not run this check (retrospective 2520, D1).
*Implication:* TDD handed off a test that Build cannot change (`doNotModifyTests`) or enroll. Two correction rounds ran against it.

**F3.4** In 2520, removing `recurrence.ApplyOptions.Reweight` broke the compile of `go/acs/cycle1932/predicates_test.go:88` under `-tags acs`. Build may not edit `go/acs/**` (retrospective 2520, D2).
*Implication:* the second blocker was real, and it sat in a path outside the build sandbox.

**F3.5** Each of the four refused claims names a path that ends in `_test.go`. `claimTargetProblem` returns `<path> is not a non-test Go source file in the tree` for such a path, before it checks the manifest (`go/internal/core/blocker_proof.go:249-251`).
*Implication:* by this code, the prover cannot prove a blocker on a protected test file, even a true one. In 2508 the retrospective says the claim was true. By this code, no claim in that form can pass.

**F3.6** In 2520 the claims named `cycle1932`. The file that the floor named was `repocontract.go`, which no claim named.
*Implication:* the builder found the second blocker and missed the first. A proven claim on `repocontract.go` stops the rounds.

## What changed

| Change | Pull request or commit | Regression test |
|---|---|---|
| The route check expands each glob in `targetFiles` and tests each path against the manifest | inbox item `route-check-expands-glob-targets` (retrospective 2508) | open |
| A protected-path floor finding stops the build correction redispatch and routes to the console | inbox items `protected-path-blocker-stops-build-redispatch` (2508) and `stop-build-corrections-on-protected-floor-blocker` (2520) | open |
| The TDD floor runs `TestPackages_HoldEveryTestThatReadsTheWholeTree` | inbox item `tdd-floor-run-repocontract-whole-tree-registration-check` (retrospective 2520) | open |
| The console enrolls the scan test, retires the `cycle1932` predicate and ships the salvaged worktree | inbox item `console-enroll-c2520-weightwrite-scan-and-retire-c1932` (retrospective 2520) | open |
| The prover handles a claim on a protected test file | none filed | open, see Limits |

## Limits

- The signal text is cut at 300 characters. The refusal reason of each claim is not in the evidence. F3.5 is inferred from `claimTargetProblem` on current main. The evidence does not show that the code was the same when the cycles ran. The last change of `blocker_proof.go` on main is commit `b75347edb` of 2026-11-05, which is before both cycles.
- This record cannot say if a different refusal reason also applied to a claim, for example `predicateBody` for the claim of 22:03:20.
- The times of the retrospectives are local time (UTC+8). This record uses the UTC time of the signals.
- The inbox items are in the runtime inbox, not in git. The evidence does not show if the console has shipped the two salvaged worktrees.
- The claim of 2508 that a salvaged line fixes the cycle is from the retrospective. This record did not test it.
- The prover finding has no inbox item in the evidence. The main-tree fact is only the code at `go/internal/core/blocker_proof.go:249`.

## What we learned

- Expand each glob in a target set to concrete paths, and test each path against the protected manifest, before a route decision.
- A new whole-tree scan test needs an entry in the protected pack list. Run that check in TDD, where the test is made.
- A blocker claim that the prover cannot read must not hold a cycle for hours. Count a refused claim on a protected path as a stop, and route the item to the console.
- Judge a blocker claim on the protected-test-file form as well, because a protected file is often a test file.
