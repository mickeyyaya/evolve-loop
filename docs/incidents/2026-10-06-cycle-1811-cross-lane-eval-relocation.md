# A lane relocated a sibling's eval and could not give it back (cycle 1811, wave 68, 2026-10-06)

- **Outcome:** cycle 1811 (`router-silent-errors`) sealed FAIL although its code was correct and its audit found nothing else wrong. The audit's only blocking finding was an eval that belonged to the sibling lane. The repair round could not remove that eval, so the build floor failed twice.
- **Class:** cross-lane misattribution in the shared main tree. A process failure (a scout's extra copy) was charged to a different lane as a logic failure.
- **Landed:** 2026-10-07, on `fix/fleet-leak-recovery-ownership`.

## Timeline (UTC)

1. **13:27:24–13:28:16**: lane 1811's scout runs. Lane 1812 (`phasespec-roots-silent-policy-error`) is in its scout at the same time.
2. **13:28:08**: 1812's scout writes its eval to its workspace, `runs/cycle-1812/.evolve/evals/phasespec-roots-silent-policy-error.md`, as the persona says. In the same shell command it also runs `mkdir -p runtime/.evolve/evals 2>/dev/null; cp .evolve/evals/phasespec-roots-silent-policy-error.md runtime/.evolve/evals/ 2>/dev/null`, which copies the eval into the shared main tree. Evidence: the 1812 scout transcript `85b8c455-…jsonl` at 13:28:08.
3. 1811's scout guard sees the new path but does not fire: the carve-out `isScoutEvalMaterialization` exempted every scout eval under `.evolve/evals/`. 1812's scout guard exempts its own copy the same way, and its recovery leaves deliverables to the guard, so the copy stays in the main tree.
4. **13:31:00–13:39:52**: 1811's TDD runs. At its end `recoverBuildLeak` lists the main tree with `git status --porcelain -uall` and compares the result with `mainDirtyBaseline`, which `porcelainDirtySet` captured once at cycle start. The sibling's eval is not in that baseline, so 1811 treats it as its own leak, moves it into its worktree and stages it. Plane log `boundary-loop.log:178`: `[router-silent-errors] [orchestrator] build-leak-recover: relocated leaked .evolve/evals/phasespec-roots-silent-policy-error.md out of main tree`.
5. **14:04–14:20**: 1811's audit FAILs on H1, an out-of-scope eval for another inbox item (`runs/cycle-1811/audit-report.md`). 1812 ships its own 49-line eval from its worktree in the meantime, so 1811's 5-line copy also conflicts with main.
6. **14:21–14:37**: the repair round's TDD writes a predicate that requires the lane to drop the copy. The builder's sandbox denies deleting `.evolve/evals/*` (`runs/cycle-1811/build-report.md:41`). The build floor fails twice and the cycle seals FAIL. The salvage snapshot captured the foreign eval and was refused for continuation (`boundary-loop.log:156832`, "classified verdict=2 — not stamped").

## Root cause

- **"Absent from my baseline" was taken to mean "mine".** N lanes share one main tree, so a path that appears after a lane's baseline can belong to any of them. `recoverBuildLeak` had no notion of an owner. The baseline was also captured once per cycle, so every write a sibling made after the cycle started was claimable by every later phase. Here the write landed after 1811's cycle-start baseline and 2 min 52 s before its TDD began.
- **The rule for where the scout's eval lives had three homes.** `agents/evolve-scout.md` said the workspace. `isScoutEvalMaterialization` in `core/cycle_outcome.go` said the scout writes evals into the main tree, and the tree-diff guard exempted that. Gate A (`evalgate/materialization.go`) read the workspace, then the project root. With all three in force, a scout's belt-and-braces copy into the main tree passed every check, and the copy is exactly what collides in fleet mode.
- **The same move had happened before.** `logs/batch-20260816a.log:170` shows a lane relocating a sibling's `role-prompt-duplication-trim.md` and shipping it in its own commit (lines 2089/2108). `logs/batch-20260815b.log:117` shows a lane relocating a sibling's minted `.evolve/phases/disposition-preflight/phase.json`.
- **A latent defect hid behind the duplicate belief.** When a lane's own scout copy survived to TDD, recovery's `os.Rename` replaced the TDD-authored eval in the worktree with the scout's copy. 1812's committed eval has 49 lines; the scout copy has 5.

## Fix

- **One ownership Specification** (`core/leak_ownership.go`, `mainTreeOwnership`). Owner rules key a main-tree path to its owner:
  - `.evolve/evals/<slug>.md` → the item `<slug>`;
  - `go/acs/cycle<N>/…` (through `acssuite.AncestorCyclePackages`) and `docs/explain/builds/cycle-<N>-<run>.md` (through the newly exported `explanationdocs.IsCycleChangeRecord`) → cycle N;
  - a verified active mint → the mint registry.

  A lane holds the ids `committedset.Committed` binds it to (lane pin, else triage's top_n, minus deferrals), the slugs it materialized in its workspace eval home, and its own cycle. Both homes are needed: across runtime cycles with a pin, 49 of 138 workspace eval slugs differ from the pin's ids. A lane with no recorded identity holds everything, which is the single-writer rule.
- **Recovery never claims what another owner holds.** `recoverBuildLeak` takes a `leakRecovery` parameter object. A path keyed to an owner the lane does not hold stays in the main tree, with one line that names the owner, whether it is a new file or an edit to a committed one. A sibling re-materializing a committed eval is the 1811 theft by another route, the very case the deleted carve-out's comment described. The function shrank from about 110 lines into short functions under 50 lines each. `Orchestrator.recoverPhaseLeak`, with a `phaseLeakScope` parameter object, is the one seam the live loop and crash-resume share; it replaced resume's private copy.
- **A worktree copy is never overwritten, and a differing main-tree copy is never deleted.** When the worktree already holds a file at a leaked path, the worktree's copy wins. A main-tree copy with exactly the same bytes is dropped, because nothing is lost. One that differs is moved to `.evolve/quarantine/cycle-<N>/` under a unique name, so a later copy never overwrites an earlier one, with a WARN naming the path. A compare that cannot read both copies fails recovery and keeps the main copy. This closes the overwrite defect found above.
- **No live holder, no pass.** A keyed path recovery will not claim, and that no live sibling, the mint registry or the console lease holds, fails recovery. This makes crash-resume, which has no tree-diff guard, fail closed as well. Recovery and the guard read the same exemption set, so an operator's leased edit the guard waives is never a recovery failure.
- **The tree-diff guard stays as strict as before.** The scout carve-out is deleted. A keyed path the lane does not hold is exempt only when a live sibling run holds its owner (`runlease.LiveRuns`, minus the lane's own run) or it is a registered mint. That owner rule is now the guard's only mint exemption. A key nobody live holds is still charged to the phase that wrote it.
- **One eval home: the workspace.** The persona says so explicitly ("never also copy it into the project root or main tree"). Gate A reads the project root only for evals earlier cycles committed. `core.EvalFilePath` is the one layout. A scout's copy of its own eval in the main tree is a misplaced write: the scout now has `evalAuthorAuthority`, so its own phase's recovery moves the copy into the lane's worktree before review. That is a recovery rung for a process failure, not a block.
- **The recovery baseline is captured per phase and carried by the dispatch.** One `git status` per dispatch feeds both recovery and the tree-diff guard (`dispatchResult.beforeDirty`). It is taken at every phase dispatch, at the remediation fix dispatch, at the gate's re-run, and before every resumed dispatch. Every main-tree read is retried 3 times: the pre-phase snapshot, recovery's own `git status`, and the guard's post-phase check.
  - If the pre-phase snapshot still fails in a git checkout, the phase is not run: the dispatch aborts, a remediation round is voided, a resume fails.
  - If recovery's read still fails, recovery fails the phase.
  - If the guard's post-phase read still fails, the guard aborts the phase.
  - A degrade that skipped recovery or the guard would let a leak made during the failure ship silently.
  - Whether the root is a git checkout is decided once per cycle, so a phase that deletes `.git` cannot switch the guard off. `defaultGitDirtyPaths` returns git's error instead of an empty set. 1811's TDD started 2 min 52 s after the sibling's write, so a per-phase baseline alone would have excluded it. The trade-off: a later phase no longer sweeps an earlier phase's residue. After this change that residue is only state that must not move (runtime state, mints, console-leased paths) or what a phase leaves when its own guard snapshot degraded.

## The deadlock: no recovery rung, by evidence

The deadlock needs an out-of-scope eval inside a lane's tree. There are three routes:

- **Recovery relocation**, the only one observed (1811, and batch 2026-08-16a). The Specification closes it, and `TestRecoverBeforeReview_SiblingLaneEvalWrittenMidPhaseStaysInTheMainTree` pins that.
- **Continuation re-seed.** It carries only what the first or third route put in a snapshot, and 1811's contaminated snapshot was refused stamping.
- **An agent authoring another item's eval in its own worktree.** It has zero instances: the only out-of-scope eval finding across the audit reports of cycles 15xx–18xx is 1811's, which came from the first route.

So no rung drops out-of-scope evals before audit (YAGNI). If the third route ever appears, the rung belongs in `recoverPhaseLeak`, keyed by the same Specification.

## Regression coverage

- Regression twin, two lanes on one main tree: `core/leak_ownership_recovery_test.go::TestRecoverBeforeReview_SiblingLaneEvalWrittenMidPhaseStaysInTheMainTree`. Red on the pre-fix tree: the sibling's eval left the main tree.
- End to end: `::TestRunCycle_SiblingEvalWrittenDuringTDDIsNeitherChargedNorClaimed`, red pre-fix.
- Mint: `::TestRecoverBeforeReview_RegisteredMintStaysInTheMainTree`, red pre-fix.
- Scout rung: `::TestRecoverBeforeReview_ScoutsOwnMainTreeEvalCopyLeavesTheMainTree`, red pre-fix.
- Theft of a sibling's committed-eval edit: `::TestRecoverBeforeReview_SiblingEditToACommittedEvalIsNotStolen`, red before the ownership check moved ahead of the status switch.
- Overwrite: `::TestRecoverBeforeReview_OwnEvalLeakNeverOverwritesTheWorktreeCopy`, red before the worktree-wins rule.
- A failed snapshot is loud: `::TestDefaultGitDirtyPaths_ReportsAFailedGitStatus`, `::TestRecoverPhaseLeak_SkipsAPhaseWithNoPrePhaseSnapshot`, and `core/leak_ownership_test.go::TestCycleRun_RecoveryBaselineNeedsASnapshotOfACheckout`, all red before the change.
- Second review round:
  - `leak_ownership_resilience_test.go::TestRunCycle_ATransientPrePhaseSnapshotFailureIsRetriedAndTheLeakRecovered`: red, the leak stayed in main and the cycle passed.
  - `::TestRunCycle_APrePhaseSnapshotThatKeepsFailingStopsBeforeThePhaseRuns` and `::TestRunCycleFromPhase_APrePhaseSnapshotThatKeepsFailingStopsBeforeThePhaseRuns`: red, the phase ran.
  - `::TestRecoverBeforeReview_ADifferingMainTreeCopyIsQuarantinedNotDeleted`: red, the copy was deleted.
  - `::TestRecoverBeforeReview_AKeyedPathNoLiveLaneHoldsFailsRecovery` and `::TestRunCycleFromPhase_AnEditToAnUnownedCommittedEvalFailsTheResume`: red, err was nil.
  - Final check:
  - Red before its fix: `leak_snapshot_test.go::TestRecoverBeforeReview_APostPhaseGitFailureFailsRecovery`, `::TestRunCycle_APostPhaseGuardReadFailureAbortsThePhase`, `::TestSnapshotMainTree_ADotGitRemovedMidCycleDoesNotTurnTheGuardOff`, `::TestRunCycleFromPhase_ADotGitRemovedBetweenResumedPhasesDoesNotTurnTheGuardOff`, `::TestRetryingDirtyPaths_StopsWhenTheContextIsCancelled`, `leak_ownership_resilience_test.go::TestRecoverBeforeReview_QuarantiningTheSamePathTwiceKeepsBothCopies` and `::TestRecoverBeforeReview_AConsoleLeasedPathIsLeftJustAsTheGuardWaivesIt`.
  - Mutant pins: `leak_snapshot_test.go::TestMaybeRemediate_AFixSnapshotFailureVoidsTheRound`, `::TestMaybeRemediate_AGateRerunSnapshotFailureVoidsTheRound`, `::TestSnapshotMainTree_TwoTransientFailuresFitInsideTheRetries`, `::TestSnapshotMainTree_AMalformedGitPointerIsACheckoutNotAnExemption` and `::TestRunCycle_AGuardReCheckFailureAfterTheBinaryDiscardAbortsThePhase`, plus `leak_ownership_resilience_test.go::TestRecoverBeforeReview_AnExpiredSiblingLeaseDoesNotHoldItsEval`, `::TestRecoverBeforeReview_AnUnheldPathAfterAHeldOneStillFailsRecovery`, `::TestRecoverBeforeReview_ADifferingCopyOfTheSameLengthIsQuarantined` and `::TestRecoverBeforeReview_AnUnreadableWorktreeCopyKeepsTheMainCopy`.
- Mutant pins `::TestRecoverBeforeReview_SiblingsStagedChangesToItsCommittedEvalAreLeft`, `::TestMaybeRemediate_TheGateReRunsOwnLeakIsRecovered`, `::TestRecoverBeforeReview_AFailedSnapshotNeverRecoversAgainstAnEmptyBaseline` and `cyclerun_remediate_test.go::TestObservedRun_TheObserverWatchesTheWholeRun`.
- Mutant pins:
  - `::TestGuardChargesThisLanesOwnEvalWhileItsOwnLeaseIsLive`;
  - the own-slug and own-cycle rows of `TestMainTreeOwnership_HeldBySiblingNeedsALiveHolder`;
  - `TestLaneOwnership_MaterializedEvalsAloneMakeTheIdentityKnown`;
  - the per-authority subtests of `buildleak_recover_test.go::TestRecoverBuildLeak_DiscardsRebuiltArtifactEvenWhenWorktreeClean`.
- Per-phase baseline: `::TestRunCycle_PhaseBaselineExcludesASiblingWriteBeforeThePhase`, `::TestRunCycleFromPhase_PhaseBaselineExcludesASiblingWriteBeforeThePhase` and `::TestMaybeRemediate_FixDispatchBaselineExcludesASiblingWriteDuringTheGate`, all red pre-fix. `::TestRunCycleFromPhase_ALeakDuringTheResumedPhaseIsStillRelocated` keeps the capture before the resumed dispatch, not after it.
- Protected surface: `guards/integrity_surface_manifest_test.go::TestProtectedSurfaceManifest_CoversTheMainTreeOwnershipSpecification`, red against the old manifest.
- Preservation, green both before and after the fix:
  - `::TestRecoverBeforeReview_OwnLaneEvalLeakStillRelocates`;
  - `::TestRecoverBeforeReview_SourceLeakStillRelocatesBesideASibling`;
  - `::TestRecoverBeforeReview_SiblingEvalDuringScoutStaysForItsOwner`;
  - `core/orchestrator_guard_test.go::TestGuardIgnoresScoutEvalMaterialization`, rewritten onto a real git tree: its old body asserted the deleted main-tree belief on a tree where recovery could not run;
  - `::TestGuardStillChargesAnEvalNoLiveSiblingHolds`.
- Specification: `core/leak_ownership_test.go` (`TestMainTreeOwnership_*`, `TestLaneOwnership_*`, `TestLiveSiblingsOf_ReadsOnlyLiveRunsOtherThanThisLane`).

## Still open

- A lane's agent that writes a path keyed to a live sibling's item into the main tree is now exempt from its own guard. That sibling's recovery then claims the file. The audit of the owning lane reviews it as its own work. No instance is known.
- A scout that writes an eval only into the main tree, under a slug neither its pin nor its workspace names, is now charged by the guard instead of exempted. The persona forbids main-tree writes, and no instance is known.
- **An edit to a committed path keyed to another owner is left, whether or not it was this lane's.**
  - Across 186 cycle commits since 2026-08-01, 6 edited a committed past-cycle predicate package and 5 edited a committed eval, all in their worktrees.
  - If such an edit leaks into the main tree, recovery cannot tell whose it is. It leaves the edit, and the guard charges the phase unless a live sibling holds the owner. The outcome is fail-closed by design.
- **Crash-resume adopts no console lease.**
  - The fresh loop adopts the operator's lease at cycle start and hands it to both recovery and the guard.
  - `RunCycleFromPhase` adopts none, so a resumed phase that meets a leased path keyed to another owner, which no live lane holds, fails recovery.
  - It fails closed; it never moves the operator's edit.
  - Adopting the lease at resume needs a new parameter through `reviewResumedDeliverable`, a nine-parameter protected function, and its three tests. It was left out of this fix as disproportionate.
- **`ensureCleanWorktree` still reads a git failure as a clean worktree.** It goes through `porcelainDirtySet`, which now WARNs on the failure but still returns an empty set, so a reused worktree is not quarantined when `git status` fails. This predates this fix and is not changed here.
- **Charged-and-left orphans stay in the shared main tree.** A keyed path that no live lane holds, left by recovery and charged by the guard, is not moved by anyone. It stays untracked in the plane until an operator removes it. Every later lane's per-phase snapshot then contains it.
  - Proposed home, not built: the boundary sweep that already runs when no lane is live (`evolve gc` at the wave boundary, beside boot recovery's `QuarantineDirtyTree`). It would list the untracked owner-keyed paths no run holds, move them under `.evolve/quarantine/` with a report, and never delete them.
