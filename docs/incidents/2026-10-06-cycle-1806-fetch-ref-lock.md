# A fleet lane lost its cycle to another process's fetch (cycle 1806, 2026-10-06)

## What happened

Wave 65 ran two lanes; the quota bench had shrunk it from three. Cycle 1806 was the lane for inbox item `inboxbatch-utf8-and-resolution`. It never ran a phase of its own work, and it ended as a FAIL:

| Time (UTC) | Event |
|---|---|
| 05:00:57 | Cycle 1806 is allocated. Provisioning its worktree runs `laneStartRef`, which runs `git fetch origin` in the shared store. At the same moment `gh pr update-branch` has just moved `train/wave64-boundary` on origin, and another process is updating that remote-tracking ref. The fetch fails: `error: cannot lock ref 'refs/remotes/origin/train/wave64-boundary': is at e1d9ae9e… but expected 0bc8caed…` and ` ! 0bc8caed0..e1d9ae9e5  train/wave64-boundary -> origin/train/wave64-boundary  (unable to update local ref)`. |
| 05:00:57 | The orchestrator logs `WARN worktree provisioning failed (source phases will be blocked)`, writes a failure digest naming the provisioning cause, and carries on without a worktree. |
| 05:01:34 | The advisor plans the cycle. |
| 05:01:56 | The scout launch is refused: `BRIDGE_EXIT_BAD_FLAGS … fleet mode: explicit worktree required (refusing process-cwd fallback)`, exit 10. `ORCHESTRATOR_PHASE_ABORTED` follows, verdict FAIL. |
| 05:01:56 to 05:04:45 | Failure learning runs: a retrospective is dispatched (about three minutes) and a P0 carryover todo, `cycle-1806-failed-scout`, is queued about the item ("cycle 1806 failed during scout …"). |
| 05:04:45 | `cycle.sealed` WARN `ORCHESTRATOR_CYCLE_FAILED`, `retro_decision=failure-learning: queued cycle-1806-failed-scout`. The inbox closeout claims the item and releases it again, with no failure bump. The wave ends `1/2 lanes ok`. |

The item itself was never at fault, but the record said it was. The cycle counted as a FAIL toward the operator's three-consecutive-FAIL rule, and its failure digest was counted by the pipeline-blocker breaker.

## Root cause

There were two defects, and either one alone would have saved the lane.

1. **The lane's base fetch was broader than the lane needs, and it had no retry.** One bare store, `~/ai/claude/evolve-loop/.repo.git`, backs the runtime plane, every fleet lane and about fifty console dev worktrees, and any of them may run `git fetch` at any moment. `laneStartRef` (`go/internal/core/worktree.go`) ran a bare `git fetch origin`, which updates every remote-tracking ref. A lane needs one of them, `origin/main`, so contention on any other ref failed its base. git reports that contention as `cannot lock ref` (followed by either `Unable to create '….lock': File exists` or `is at X but expected Y`) and `unable to update local ref`. Each is a lost race on a ref lock, which clears in about a second, but no attempt was ever repeated.
2. **A fleet lane without a worktree still dispatched.** `newCycleRun` (`go/internal/core/cyclerun.go`) treats provisioning as best effort. In a sequential cycle that is right: the phases run in the project root and the role gate refuses source writes. In fleet mode, though, the bridge refuses every launch that has no explicit worktree. The cycle therefore went from a known infrastructure fault into a scout FAIL, and the whole failure path ran on it: a failure digest, failure learning, a retrospective and a FAIL seal. A process failure became a task failure. That breaks the policy that only logic blocks, and that a format or process failure gets a code-owned recovery rung ([operating policy](../operations/operating-policy.md)).

## The fix

- **Fetch only the base, and retry contention** (`gitexec.FetchOriginBranch`, `go/internal/gitexec/fetch.go`). The lane base fetches `+refs/heads/<branch>:refs/remotes/origin/<branch>` for the default branch alone, resolved the way `originDefaultBranch` resolves it, so a ref nobody asked for can no longer fail the lane. `RetryableFetchFailure` is an allow-list of two markers, `cannot lock ref` and `unable to update local ref`; a bare `but expected` was dropped in review because `cannot lock ref` already precedes every lock message that contains it, and alone it matched unrelated stderr. A contended fetch is retried up to `DefaultFetchAttempts` (3) times, 2 s and then 4 s apart. Each retry is announced, and a persistent failure still fails loudly with both the first and the last stderr. The retry loop is the one `AddWorktreeWithRetry` already ran, extracted into `captureWithRetry` so the two cannot drift. `evolve sync-main` had the same bare fetch, and now fetches its checked-out branch the same way; it refuses a detached HEAD before fetching, because the narrow refspec for `HEAD` would write `refs/remotes/origin/HEAD`, the symbolic ref the lane base reads to find the default branch.
- **Re-provision once** (`Orchestrator.reprovisionWorktree`). In fleet mode, when `Create` fails after its own retries, it is called once more before the lane defers. `Create` is idempotent, so the second call is clean.
- **Defer the lane instead of failing it** (`Orchestrator.deferLaneWithoutWorktree`, `go/internal/core/lane_deferral.go`). In fleet mode a lane that still has no worktree ends before any phase dispatches:
  - It returns a `*core.LaneDeferral` before the cycle state is persisted.
  - It records one host ending, `lane-worktree-deferred: <cause>`, which `cyclehealth.ClassifyOutcome` reads as `DEFERRED`. That is the existing deferral outcome the quota wall also uses.
  - It seals with `cycle.sealed` WARN `ORCHESTRATOR_LANE_DEFERRED`, whose `fields.cause` is the git error.
  - It writes no failure digest, no `audit-fail-reason.json`, no failure-learning todo and no retrospective.
  - `evolve cycle run` releases any claim without a bump and exits 5 (`fleet.ExitDeferred`, the code the quota pause already used).
  - Every lane report counts it as deferred, through the one `loopwave.Tally`: the wave line (`wave N: X/Y lanes ok, K deferred`), the pool line, the min-width repair line, `evolve fleet` (exit 0 when nothing failed) and `evolve fleet soak`.
  - The item stays in the queue for the next wave.
- **A deferral that does not clear halts the batch.** The first version of this fix had a hole the architecture review caught: a deferral writes no failure digest, and the pipeline-blocker breaker read only digests, so a persistent provisioning fault (a stale `refs/remotes/origin/main.lock`, a network or credential outage, any deterministic `Create` error) would have deferred every lane of every wave until `--max-cycles`, with no halt and no P0. Before the fix, the same fault halted after three FAILs. The breaker now has a `lane-deferrals` rule: `core.CollectBatchLaneDeferrals` reads the batch's run dirs through `cyclehealth.ClassifyOutcome` and keeps the `DEFERRED` cycles whose abort reason starts `lane-worktree-deferred` (never a quota deferral), and `core.EvaluateLaneDeferrals` halts on `failure_policy.thresholds.lane_deferral_halt_ceiling` (default 3) consecutive ones. The halt goes through the standard machinery with category `lane-provisioning` and the last git cause in its evidence. The delta review then found the mirror-image hole: the deferral run broke on a FAIL and the `consecutive-failures` run broke on a deferral, so lanes alternating deferral and FAIL (F D F D F) escaped both rules, where the same sequence had halted at its third cycle before the fix. A deferred cycle is now transparent to the `consecutive-failures` run (it neither counts nor breaks it), and a deferral whose record cannot be written fails the cycle loudly, since the breaker could not see it. `failure_policy` ceilings cannot be set to `0` to disable a rule: a non-positive value keeps the default.
- **Unchanged in sequential mode.** A sequential cycle still calls `Create` once, runs best effort without a worktree and records the provisioning cause in its failure digest, because there the bridge can fall back to the project root.

## Where else the loop fetches

| Site | Before | Decision |
|---|---|---|
| `core.laneStartRef` | `git fetch origin` (every ref), no retry | Narrowed and retried; this incident |
| `cmd/evolve` `runSyncMain` | `git fetch origin` (every ref), no retry, exit 1 | Narrowed to the checked-out branch and retried: the same class, run at every boundary while console worktrees fetch |
| `cmd/evolve` `syncMainFromOriginAtWaveBoundary` | `git fetch -q origin main` | Left alone: already one branch, and a failure only WARNs and plans against the local main, while the lane base fetch that matters is now retried |
| `looppreflight` `defaultBaseDivergenceProbe` | `git fetch origin <branch>` | Left alone: already one branch, and a failure makes the check warn rather than halt |
| `phases/ship/landing` `probeOrigin` | `git fetch origin <branch>` | Left alone: already one branch, and it is itself the push-rejection repair rung; a failure declines the repair and returns the original transient `GIT_PUSH_REJECTED` |
| `phases/ship` `pushonly.go`, `repair.go` | `git fetch origin <branch>`, error ignored | Left alone: already one branch and best effort, falling back to the local ref |
| `cmd/evolve` `cmd_worktree_dev.go` | `git fetch origin +refs/heads/main:<dedicated ref>` | Left alone: already one ref, into a ref nothing else writes |
| `marketplacepoll` | `git fetch origin main` in a marketplace clone | Left alone: a different repository, not the hub store |

## Tests (red first)

The red evidence is in the console session's scratchpad, `fetchrace/red-*.txt` (the review rounds' in `fetchrace/red-fix-*.txt` and `fetchrace/red-fix3-*.txt`). Each red ran against the unfixed code, or, for a test that kills a surviving mutant, against that mutant's overlay.

- `gitexec`:
  - `TestFetchOriginBranch_FetchesOnlyTheBranchRefspec`
  - `TestFetchOriginBranch_RetriesRefLockContentionThenSucceeds`
  - `TestFetchOriginBranch_PersistentContentionFailsAfterTheBound`, which uses the `Attempts` seam
  - `TestFetchOriginBranch_ZeroAttemptsUsesDefaultFetchAttempts`
  - `TestFetchOriginBranch_NonContentionFailureIsNotRetried`
  - `TestRetryableFetchFailure_ClassifiesRefLockContention`, whose `an unrelated 'but expected'` case was red until the third marker was dropped
- `core`, through a fake runner:
  - `TestLaneStartRef_FetchesOnlyTheDefaultBranch`, red with `fetches = [[fetch origin]]`
  - `TestLaneStartRef_RetriesRefLockContention`
  - `TestLaneStartRef_PersistentContentionFailsAfterTheBound`
  - `TestLaneStartRef_FollowsTheRemotesDefaultBranch` (origin/HEAD → `origin/trunk`), red against the review's mutant that hard-coded `origin/main`
- `core`, against real git, with a `.lock` file holding the ref the way a concurrent fetch does:
  - `TestLaneStartRef_RealLockOnAnUnrelatedRefDoesNotFailTheBase`, whose red reproduced the incident's exact stderr
  - `TestLaneStartRef_RealLockOnTheBaseRefIsRetried`
- `core`, the lane deferral:
  - `TestRunCycle_AFleetLaneWithoutAWorktreeDefersBeforeAnyDispatch`, red with `err = <nil>`
  - `TestRunCycle_AReprovisionedFleetLaneRunsNormally`, red with one `Create` call
  - `TestRunCycle_ASequentialCycleWithoutAWorktreeKeepsItsBestEffortRun`, which pins the unchanged path, including its single `Create` call
  - `TestLaneDeferral_NamesTheCycleAndUnwrapsToItsCause`
- `core` and `policy`, the halt for a deferral that does not clear:
  - `TestCollectBatchLaneDeferrals_CountsOnlyWorktreeDeferralsInTheBatch`
  - `TestEvaluateLaneDeferrals_HaltsOnlyOnAnUnbrokenRun`
  - `TestLaneDeferralHaltCeiling_DefaultsToThreeAndMergesFromOperatorPolicy`
  - `cmd/evolve` `TestBlockerBreakerHalt_ConsecutiveLaneDeferralsHaltWithTheGitCause`, red with `halted=false`, and `TestBlockerBreakerHalt_ABrokenRunOrQuotaDeferralsDoNotHalt`
  - `TestConsecutiveFailures_LaneDeferredCyclesAreTransparent` and `cmd/evolve` `TestBlockerBreakerHalt_AlternatingDeferralsAndFailuresHalt`, red while F D F D F did not halt
  - `TestRunCycle_ADeferralThatCannotBeRecordedFailsLoudly`, red while an unrecorded deferral was still returned as one
- The outcome, the exit code and the wave:
  - `TestClassifyOutcome_AFleetLaneDeferredForItsWorktreeIsDeferred`, red as `FAILED_EXPLAINED`
  - `TestCycleRunErrorExit_ALaneDeferralReleasesItsClaimsUnbumpedAndExitsDeferred`, red with exit 1 and the claim held
  - `TestCycleRunErrorExit_AnyOtherErrorStillExitsOne`
  - `TestCycleTerminationLaneWorktreeDeferred`
  - `TestResult_StatusIsTheOneLaneOutcomeClassifier` (`fleet`) and `TestRunWaves_ADeferredCycleIsRetriedNotCompleted` (`campaign`, a characterization: the campaign already retried any non-zero exit)
  - `TestTally_CountsEachLaneOnceAsOkDeferredOrFailed`, `TestTally_CountsErrOrNonZeroExitAsFailed`
  - `TestWaveSummary_NamesDeferredLanesOnlyWhenThereAreSome`
  - `TestRepairMinWidth_ADeferredRepairLaneIsNamedDeferred`, red with `dispatched 1/1` once exit 5 stopped counting as failed
  - `TestCompleteWave_ADeferredLaneIsCountedDeferredNotOk`, red against the review's mutant that dropped the deferred count from the wave line
  - `TestReportFleetResults_ADeferredLaneIsNotAFailure` and `TestSoakLaunchesOK_ADeferredLaunchIsNotAFailure`, red with exit 1 and `2/3 launches failed`
  - `TestSoakLaunchesOK_ASoakWhereEveryLaunchDeferredIsInconclusive`, red while an all-deferred soak passed
  - `TestLaneTally_ProjectsToTheLeaf`
- `evolve sync-main`:
  - `TestSyncMain_ALockOnAnUnrelatedRemoteRefDoesNotFailTheSync`
  - `TestSyncMain_RetriesALockOnTheBranchRef`
  - `TestSyncMain_RefusesADetachedHEADBeforeFetching`, red with `couldn't find remote ref refs/heads/HEAD`

## The noise it left

Failure learning queued `cycle-1806-failed-scout`, a P0 carryover todo, into the plane's `state.json` `carryoverTodos`. It expires at 2026-10-07T05:01:56Z. It is not an inbox file, and `state.json` is not tracked, so there is nothing for a landing to consume. The operator can drop it with `evolve carryover apply-decisions` in the plane, or let it expire. The cycle's `failure-digest.json` stays in `runs/cycle-1806/` as history.

## The class

**Shared-store contention read as task failure.** Whatever one process does to the shared git store, and whatever fault is outside the item, must reach the queue as a deferral or a retry, never as a verdict on the item. The checks that follow from it:

- A git operation in a shared store touches only the refs its caller needs.
- A retry is bounded and allow-listed to the failure it understands.
- A cycle that never dispatched a phase cannot fail its task.
- An outcome that is not a failure still needs its own ceiling: whatever the breaker cannot see, it cannot halt.

## References

- [internal/core](../architecture/packages/internal-core.md): the lane base fetch, the re-provision, the lane deferral and the `lane-deferrals` breaker rule
- [internal/gitexec](../architecture/packages/internal-gitexec.md): `FetchOriginBranch`, `RetryableFetchFailure` and the shared retry loop
- [internal/loopwave](../architecture/packages/internal-loopwave.md): how a deferred lane is counted
- [internal/fleet](../architecture/packages/internal-fleet.md): `ExitDeferred` and how `Result` decodes a lane exit
- [internal/policy](../architecture/packages/internal-policy.md): `lane_deferral_halt_ceiling`
- [cmd/evolve](../architecture/packages/cmd-evolve.md): the deferral closeout, the lane reports, the breaker wiring and `sync-main`
- [runtime reference](../operations/runtime-reference.md): "Lane base fetch and the lane deferral"
