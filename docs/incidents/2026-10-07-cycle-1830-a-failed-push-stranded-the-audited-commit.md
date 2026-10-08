# Cycle 1830: a GitHub 500 on push became a re-audit FAIL and stranded the audited commit on the shared main (wave 77, 2026-10-07)

**Status:** fixed by this landing: the two-phase landing with a write-ahead intent ([ADR-0039 §8.1](../architecture/adr/0039-failure-floor-and-failure-signal-contract.md), the 2026-10-07 amendment and the 2026-10-08 fix round).

## What happened

Cycle 1830 passed audit and committed its change. Ship then fast-forwarded the shared local `main` and pushed. GitHub returned `Internal Server Error` on the push and on the repair's one identical retry.

Ship did not finish the landing, and it did not undo it. The transient retry ran the whole ship again. The explanation gate measured ship's own inbox consumption as Build content, and the re-audit failed on the same gate.

The audited commit `ba61c7341` stayed on local `main`, with no push and no journal entry. Peer lane 1829 rebased onto it, so the push of 1829 published it. Nothing unaudited was involved: the commit's tree is the audited tree plus the sanctioned consumption.

### Timeline (UTC)

| Step | What happened |
|---|---|
| Audit | 15:05:22. The audit passed: `git_head 63eb3af76`, worktree base `63eb3af76`, audited tree `ad697a29`. The Build handoff had the seal `diff_sha256 7bfb80dc…` over 12 paths. |
| First ship: the commit | 15:08:29. Ship consumed the inbox item and committed `ba61c7341`: parent `63eb3af76`, tree `0bb5f26c`. That tree is `ad697a29` plus the delete of `.evolve/inbox/2026-09-30T10-51-14Z-cli-phase-cycle-request.json` and the add of its `consumed/` copy. **Ship fast-forwarded the shared local `main` to `ba61c7341`** (reflog `merge cycle-cd3ae73e-1830: Fast-forward`). |
| First ship: the push | 15:08:31. The push failed: `! [remote rejected] main -> main (Internal Server Error)`. At 15:08:34 the inline repair fetched origin. Origin `3de30ec3a` was an ancestor of `ba61c7341`, and the identical push got a second 500. The log shows `SHIP_LANDING_PUSH_REPAIR_DECLINED probe=push_retry`, then `SHIP_GIT_PUSH_REJECTED` (transient). |
| Transient retry | The router sent the run to `recover:transient-retry-ship` (1/2). Ship ran again from the start, with the repo-contract pack. At 15:10:38 the explanation gate raised `SHIP_EXPLANATION_DOCUMENTATION`: "build-explanation.json diff SHA256 does not match the base-bound Build content". |
| Re-audit | The router sent the run to `recover:precondition-reaudit` (2/2). The audit ran the same `explanationdocs.Verify` and returned **FAIL** with the same message (15:17:28, a 377 s deep-tier round). |
| Repair round | TDD and Build made no change. The post-TDD refresh sealed the handoff again at `8eb4ced1…`, and that seal took in ship's own consumption. |
| Peer | 15:17:28. The audit of lane 1829 bound `git_head ba61c7341`. The ff-merge of its ship diverged (`SHIP_GIT_FLEET_REBASE_NEEDED`). B1 unwound it, and the fleet rebase put it on `ba61c7341`. The `git push origin main` of 1829 carries `ba61c7341`. |

### The base did not move

The digest's base is the recorded pre-commit base `63eb3af76` on both sides. The base check (`explanationdocs.go:272`) and the material-path check (`:289`) passed. What moved is the root that the gate verifies. The lane worktree now held ship's commit, so `changedSince` found 14 paths, not 12. A copy of `foldPathStates` gives both values:

| Path set | `diff_sha256` |
|---|---|
| The 14 paths that the retry hashed | `8eb4ced1914086c3ae7dd76841377e64d8fcd512690fb2f9900a3f32d2c80308` |
| The same set without the two consumption paths | `7bfb80dc8586cfe65c5870d815605ca9168edb34c8dc1c08439438685348b82d`, the sealed value |

The host's own refresh later sealed exactly `8eb4ced1…`.

## Outcome

- Peer cycle 1829 shipped `45652a87c` on top of the stranded commit `ba61c7341`. Origin `main` now contains `ba61c7341`.
- Cycle 1830 then sealed FAIL (`ORCHESTRATOR_CYCLE_FAILED`, 16 phases). Its sequence:
  1. the mismatch at the explanation gate;
  2. a re-audit FAIL;
  3. a TDD and Build repair round;
  4. `AUDIT_BINDING_HEAD_MOVED`;
  5. `GIT_FLEET_REBASE_NEEDED`.
- The retro of 1830 queued a P0 carryover todo, `cycle-1830-failed-ship`, for work that had already landed. At the boundary, the console dropped it with `evolve carryover apply-decisions`. The reason cites `ba61c7341`.
- The inbox item `cli-phase-cycle-request` is in `.evolve/inbox/consumed/` on origin.

### Consequence: a wrong FAIL verdict

The FAIL verdict of cycle 1830 is wrong. The audited work of 1830 is on origin through the push of 1829. A server error on the push, not a defect in the change, caused the FAIL. The P0 todo of the retro was the next wrong record, and the console removed it.

## Root cause

The landing of ship was not a transaction across the push. Before the only step that can fail remotely, `worktreeShip.integrate` (`go/internal/phases/ship/worktree_ship.go:180-198` at `63eb3af76`) made two local effects that stay:

- the lane commit, which carries the inbox consumption (`:124`, `:169`);
- the fast-forward of the shared `main` (`Landing.Integrate`, `landing/integrate.go:39`).

Only then did it push (`:185`). After a failed push, nothing completed those effects, and nothing undid them. Ship wrote each witness that a resume needs only after a successful push, or kept it only in memory:

- **The post-push idempotency check** (`ship.go:251-261`, `:338-372`) needs `ship-binding.json`. Ship wrote it at `worktree_ship.go:194`, after the push.
- **`--push-only`** (`pushonly.go:86-96`) needs a ship-journal entry. `finalize` wrote it only when `res.CommitSHA` is set (`ship.go:305`), and that occurs only after a successful push (`gitops_landing.go:55-57`).
- **The merged-but-unpushed resume rung** (`repairResumeUnpushed`, `repair.go:310-372`, ADR-0039 §8) needs `HEAD^{tree} == audit-bound tree` (`:318-329`). That is never true for a cycle that consumed an inbox item. The excuse for the consumption (`treeDriftExplainedByConsumption`, `consume.go:397-400`) reads paths that exist only in memory (`consume.go:244`). The rung also runs after the explanation gate (`ship.go:227` before `:265`).

The router keys on the class (`router/recovery.go:102-110`), so `transient` enters `Ship.Run` again from the start. Each gate then measured a world that ship itself had changed:

- the explanation digest now covered the consumption of ship;
- the audit binding saw the plane HEAD that the fast-forward of ship had moved;
- the re-audit read a `git diff HEAD` that was empty, because the change was committed.

A transient server error became a precondition loop that cannot succeed.

The strand then spread:

- `laneStartRef` bases new lanes on the local tip when `main` is ahead of origin (`core/worktree.go:122-124`).
- The fleet rebase targets local `main`, and the carry treats each commit on it as a landed peer.
- A normal ship pushes the whole ancestry of local `main`, with no provenance check.

So a peer published the strand, whatever the verdict of the stranded cycle.

### Why the inline repair did not help

`repairPush` (`landing/push.go:113-146`) did not look at the cause of a failure. A 5xx got the same fetch, the same fast-forward probe, and one immediate identical push, as a push race does. That push met the same outage three seconds later.

Ship did not capture the stderr of the push, so the error read "main is at ba61c734…: <nil>" (`push.go:88-93`). A longer retry helps this one case. But no budget in the step survives a long outage, so the state after a failed push must be resumable.

### Why the tests missed it

`TestRepair_Resume_PostMergePrePush_PushOnlyCloses` (`repair_resume_test.go`) builds the merged-but-unpushed state with a lane that consumes no inbox item and has no active explanation contract. So its bound tree equals its committed tree, and no gate measures the worktree.

Each real loop cycle consumes an item and activates the contract. No test ran `Ship.Run` two times across a failed push. The open item `carried-ship-resume-binding-sites-skip-the-rule` (2026-09-30) had already flagged the raw tree comparison at the resume rung.

## The fix

The landing has two phases and a write-ahead landing intent. The code home is `go/internal/phases/ship/landing` (ADR-0103 unit 07). The design is in ADR-0039 §8.1. A review (J_0, 2026-10-08) found gaps in the first version, and a fix round closed them. This section gives the state after the fix round.

1. **Commit object.** After `verifyStagedTree`, ship makes the lane commit with `git commit-tree`. No ref moves yet.
2. **Catch-up.** Under `ship.lock`, ship fast-forwards `main` to `origin/main` when `main` is behind by journaled commits only.
3. **Check.** Ship checks that `main` is an ancestor of the commit (`CheckFastForward`). On a divergence, ship moves the lane ref and stops.
4. **Record.** Ship journals the commit with its cycle. Then it writes `.evolve/landing/cycle-<N>.json` atomically.
5. **Ref.** Only then does ship move the lane ref with `git update-ref`.
6. **Push first.** Ship checks the commit tree against the audit binding. Then it pushes `commit_sha:refs/heads/main`.
7. **Settle.** Only after the push lands does ship fast-forward the plane `main` and mark the intent `complete`.

The intent holds `cycle`, `run_id`, `audit_artifact_sha256`, `audited_tree`, `lane_tree`, `worktree_base_sha`, `commit_sha`, `commit_tree`, `consumed_paths`, `explanation_view_sha256`, `pre_main`, `branch`, `lane_branch` and `status`. No phase can write it: `.evolve/landing/` is on the protected surface, and the role guard denies a phase write there with an alarm. If ship cannot record the intent or the journal entry, it moves the ref, unwinds the lane and pushes nothing.

**The resume.** `Landing.Resume` is the first stage of `ship.Run`, before the repo-contract pack and every gate.

- If a stop came after the record and before the ref moved, ship moves the lane ref forward first.
- A `prepared` intent resumes only when the host's witnesses agree with it:
  - the ship journal holds `commit_sha`;
  - the cycle and the run are those of `cycle-state.json`, not of the intent;
  - the newest audit of the run is a PASS, its bytes match its SHA256, and it is the audit that the intent names;
  - the lane tip is `commit_sha`, and the sealed Build explanation is unchanged.
- Then `Landing.Admit` reads the ancestry. When origin already holds the commit, ship settles the landing with no push. Otherwise origin and the plane `main` must be ancestors of the commit, and ship resumes at the push.
- The branch of the push comes from the plane, never from the intent.

No gate runs again. The content-addressed commit and the binding before the push already proved these bytes. The legacy rung `repairResumeUnpushed` has no witness of its own, so it uses only `Landing.Admit`, then the same push and settle code.

**Unwind or stale.** When an intent does not resume, ship looks at the lane.

- If the lane tip is the intent's commit of this run, ship resets the lane with B1 `unwindShipCommit` to `lane_tree` on `worktree_base_sha`. Then it marks the intent `unwound`. The unwind goes to `lane_tree`, so a carried lane (ADR-0105 B3) unwinds to its own tree.
- In every other case, ship marks the intent `stale`, changes nothing on the lane and runs the gates.
- A failed write of either status is a `STATE_IO` error.

Ship also unwinds the lane after a push failure that is not transient. So Audit and the explanation gate never measure ship's own commit. If the unwind declines, the cycle stops with `GIT_LANDING_UNWIND_DECLINED` (integrity). The commit stays on the lane and in the journal, and the message gives the operator steps.

**Classified push failures.** Ship keeps a copy of the push's stderr.

- A transport error, a 5xx or a 429 gets the bounded backoff in the step (`transportBackoff` in `landing/push.go`) and the identical push again.
- A policy refusal is final, with no retry: `GIT_PUSH_POLICY_REFUSED`. The router ends the cycle and does not re-audit.
- A race keeps the fetch and fast-forward probe.

The markers come from the real stderr texts of git and GitHub: for example "refusing to allow", HTTP 403 and "Authentication failed" are policy. The message ends with git's own words, so a nil error never shows as `<nil>`.

**No push, no PASS.** A lane with nothing to ship can hold commits that origin does not hold. Ship refuses it with `GIT_LANE_NOT_ON_ORIGIN` (integrity) only for a stranded landing. That is a journaled `cycle` commit whose landing intent is not `complete`. The commits that a boundary adds to local `main` (the sync-main merge, the dossier closeouts and the inbox stamps) keep the old "nothing to ship" result.

### Alternatives not taken

- **Bind the explanation base to the recorded pre-commit base.** It already is, so this fixes nothing.
- **Put the push before the fast-forward, and do nothing else.** The lane commit still carries the consumption, so the digest still does not match. The fix keeps this order inside the transaction, because the order alone stops the strand.
- **Remove `.evolve/inbox/**` from the digest.** The retry then passes the gate and the re-audit. It then reaches a ship that finds the lane level with local `main` and stops cleanly with no push (`worktree_ship.go:143-146`): a false PASS.

## Tests (red first)

Each test runs the real ship entry point against a real repository and a real bare remote. The lane has an inbox item, and the real seal activates the explanation contract. The only fake is the network: a runner wrapper fails the first N pushes with the literal GitHub stderr `! [remote rejected] main -> main (Internal Server Error)`.

| Test | Before the fix | After |
|---|---|---|
| `TestRun_ARetryAfterAFailedPushResumesAtThePushWithoutReverifying` (the pin; the second run uses fresh Options) | **red**, with the live `EXPLANATION_DOCUMENTATION` message: "build-explanation.json diff SHA256 does not match the base-bound Build content" | lands from the intent, `repair_outcome=landing-resumed`, no audit binding again, ship binding written |
| `TestRun_AFailedPushLeavesSharedMainAtItsPreLandingTip` | **red**: `main` moved to the lane commit | `main` stays at the tip before the landing |
| `TestRun_AFailedPushJournalsItsCommitSoPushOnlyCompletesIt` | **red**: "REFUSED — 1 ahead commit(s) lack ship provenance" | push-only lands the journaled commit |
| `TestRun_AServerErrorTwiceIsRetriedWithBackoffInTheStep` | **red**: declined at `push_retry` | lands in the same run after two backoffs |
| `TestRecoverFromShipError_ATransientPushFailureResumesTheShipWithoutAReaudit` (core, the real ship, the real router) | **red**: the router sent ship 2 to `audit` (`recover:precondition-reaudit`) | the ship lands at recovery depth 1 |
| `TestRun_AResumeDeclinesOnADivergedOriginAndUnwindsTheLaneToTheAuditedShape` | **red**: `EXPLANATION_DOCUMENTATION` | the intent declines, the lane unwinds, the intent is `unwound` |
| `TestRun_APolicyRejectionIsNeverRetried` | **red**: two pushes, transient | one push, `GIT_PUSH_POLICY_REFUSED`, the lane unwound |
| `TestRun_APushRaceIsRepairedWithoutBackoff`, `TestRepair_Resume_PostMergePrePush_PushOnlyCloses` and its two twins, the 1825 compose tests, `TestNativeRun_PostPushExplanationRetryUsesLandedTreeAfterWorktreeCleanup` (preservation) | green | green |

Two more tests pin the fail paths of the intent: `TestRun_ALandingIntentThatCannotBeRecordedUnwindsAndNeverPushes` and `TestRun_ALandingThatCannotUnwindStopsTheCycleWithItsCommitJournaled`. `TestPush_APolicyRefusalDuringTheBackoffIsFinalNotTransient` pins that a refusal after a backoff is final.

### The review's fix round (2026-10-08)

The J_0 review gave FIX_THEN_MERGE: 1 CRITICAL, 8 HIGH, 5 MEDIUM and 4 LOW findings. Each review probe became a regression test. Each test was red on its named assertion before the fix.

| Finding | Test | Before the fix |
|---|---|---|
| C1, a test that was not deterministic | `TestRun_ASupersedingAuditOfAnotherTreeMarksTheIntentStaleAndTheGatesBindIt`, `TestRun_ASupersededIntentOnItsOwnCommitUnwindsBeforeTheGates` (each run 50 times) | red: `GIT_LANDING_UNWIND_DECLINED` |
| H1, an unwind of a commit that origin holds (probes P1a, P1b) | `TestRun_AResumeSettlesACommitOriginHoldsUnderAPeerLanding`, `TestRun_AResumeFastForwardsMainToACommitAPeerPushedOnTopOf`, `TestRun_AStopAfterThePushAndBeforeTheFastForwardSettlesWithNoPush` | red: `GIT_LANDING_UNWIND_DECLINED`; `GIT_PUSH_REJECTED`; P1a held before the fix |
| H2, a failed fast-forward that stops later landings (probe P2) | `TestRun_ALaggingMainCatchesUpBeforeTheNextLandingChecksItsFastForward` | red: `GIT_PUSH_REJECTED`, not `GIT_FLEET_REBASE_NEEDED` |
| H3, a forged intent (probes P3, P1d) | `TestRun_AForgedIntentNeverShipsAnAuditFAIL`, `TestRun_AResumeTakesTheBranchFromTheHostNeverFromTheIntent`, `TestRun_AnIntentTheJournalDoesNotHoldUnwindsAndLandsThroughTheGates`, `TestRole_DeniesTheLandingIntentToEveryPhase` | red: exit 0, the forged commit on origin; the branch from the intent; a resume with no journal entry; no protected path |
| H4, an unwind of a lane that does not hold the commit (probe P5) | `TestRun_APreparedIntentOfAnotherRunNeverStopsAPendingLane` (with and without a run id), `TestRun_AResumeNeverUnwindsALaneTipShipDidNotPrepare`, `TestRun_AResumeAfterTheOrchestratorPendedTheLaneMarksTheIntentStale`, `TestRun_AFailedWriteOfTheUnwoundIntentIsAnError` | red: `GIT_LANDING_UNWIND_DECLINED`; no `stale`; a WARN |
| H5, the unwind of a carried lane | `TestRun_APolicyRefusalOfACarriedLaneUnwindsItToItsCarriedTree` | red: `GIT_LANDING_UNWIND_DECLINED` |
| H6, a stop between the lane commit and the intent (probe P1c) | `TestRun_AKillBeforeTheLaneRefMovesRollsTheLaneRefForward` | red: no roll forward |
| H7, five mutants that survived | `TestRun_AResumeBindsTheNewestAuditOfItsOwnRun`, the two cases of the run-id test, the journal assertion of `TestWorktreeShipLand_AMainThatMovedPastTheLaneNeverPushesNorPrepares`, the `unwound` assertions, `TestShip_PostPush_Idempotent_CorrectReportOnly` | mutant tests |
| M1, a policy refusal that re-audits (probe P4) | `TestRecover_Branches` (the row `push policy refusal ends the cycle`), `TestRecoverFromShipError_TheLandingStopCodesEndTheCycleWithoutAReaudit` | red: the router sent it to `audit` |
| M2, wrong classes (probe P6) | `TestClassifyPush_TheRealStderrTextsOfGitAndGitHub` (15 texts) | red: 8 of 15 wrong |
| M3, two fields for one axis | `TestPush_AWorktreeSiteWithNoCommitNeverPushes` | red: a push ran |
| L1, a hidden read error | `TestRun_AnUnreadableCycleStateStopsTheResumeLoudly` | red: `EXPLANATION_DOCUMENTATION`, not `STATE_IO` |
| L3, L4, the steps and the boundary | `TestRun_APeerLaneOnAStrandedLandingStopsWithItsSteps`, `TestRun_ALaneOnAMainAheadByBoundaryCommitsIsStillNothingToShip`, `TestRun_ALaneOnACompletedLandingOriginLostIsStillNothingToShip` | red: no steps; `GIT_LANE_NOT_ON_ORIGIN` for a boundary commit |

### Mutants

The first version killed 14 overlay mutants. The fix round ran the review's 17 mutants again (`mutate.py`, adapted to the new code) and killed all 17. A control mutant (the second backoff 4 s → 5 s) proved that the harness fails. The mutants cover these parts:

- the resume checks (R1–R6);
- the witness and the record (H1–H5);
- the `unwound` mark (H9) and the `prepared` filter (H10);
- the backoff (H12, H13);
- the stranded-lane rule (H15) and the post-push tree check (H16).

## Recurrence data

- This is the first live 5xx on a push in the boundary log. Earlier stranded pushes (cycle 1678, 2026-09-14) came from a diverged origin. An operator recovered them by hand.
- Each cycle that consumes an inbox item, which is each loop cycle, met this path on any failed push. Its resume rung (ADR-0039 §8 mode 2) did not accept such a cycle.
- The cost of this one 500:
  - two runs of the repo-contract pack;
  - a deep-tier re-audit;
  - a TDD and Build repair round.

## What it taught

- **The local effects of a transaction need a durable intent or an undo before its remote step.** Ship wrote each witness for a resume after the push or kept it in memory. So no witness survived the failure that it was for.
- **A retry must not measure its own side effects again.** The gates were correct about the bytes that they measured. They measured ship's commit, not the Build.
- **A check for "landed" must ask origin.** "On local main" let the commit of a failed push become the base and the payload of a peer.
- **The fixture decides what the proof covers.** A resume fixture with no inbox item and no explanation contract proves nothing about the cycles of the loop.

## Operator recovery

- **In a wave.** Do not change git, and do not unwind a strand: peer rebases and carries can bind to it.
- **At the boundary, if a peer published it:** correct the dossier of the stranded cycle ("landed through the push of the peer"). Remove each retro todo for work that already landed. Make sure that the item is in `consumed/`.
- **At the boundary, if it is still stranded:** run `evolve sync-main` if origin moved. Then publish it through the sanctioned ship path: a manual ship with the plane's pending inbox stamps, or `--push-only` for a journaled commit. Do this before the boundary binary refresh, which otherwise builds `go/bin/evolve` from unshipped code.
- **After this fix:** a failed push leaves `main` untouched and the intent `prepared`. A new `evolve ship` run resumes it. If a stranded landing stops a lane with `GIT_LANE_NOT_ON_ORIGIN`, do the steps of its message at the boundary:
  1. In the plane, run `git merge --ff-only <commit>`.
  2. Run `evolve sync-main`.
  3. Run `evolve ship --push-only`.

## Secondary findings (filed, not in this fix)

- A post-TDD refresh can seal the Build binding again over ship's own consumption (inbox `post-tdd-refresh-seals-ship-consumption`).
- Lanes fork from local `main`, and the carry trusts "on local main", not "on origin" (inbox `lanes-fork-from-local-main-not-origin`).
- The boundary binary refresh builds unshipped code from a stranded plane HEAD (inbox `boundary-refresh-builds-a-stranded-head`).
- A stop between ship's inbox consumption and the landing record leaves the consumption with no intent. This window is older than the fix (inbox `ship-consumption-before-the-landing-record-has-no-resume`).

## Related

- [ADR-0039 §8](../architecture/adr/0039-failure-floor-and-failure-signal-contract.md): the ship repair ladder and the two-phase landing.
- [ADR-0105](../architecture/adr/0105-identity-preserving-fleet-rebase.md): B1's unwind, and "a committed change never goes to Audit".
- [build-explanation-contract](../architecture/build-explanation-contract.md): the post-push retry rule that this fix mirrors before the push.
- [The cycle 1825 incident](2026-10-07-cycle-1825-carry-refused-the-reships-consumption.md): the same seam between consumption and binding, on the carry path.
- Inbox `ship-landing-resumes-across-a-failed-push` (this fix), `carried-ship-resume-binding-sites-skip-the-rule` (part of it is still open), `lost-ship-closeout-universal-landing-witness`, `pre-commit-carry-refusal-returns-to-audit`.
