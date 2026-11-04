# go/internal/integration

The integration engine of the wave orchestrator ([ADR-0147](../adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md), plan [wave-integration-2026-10](../../plans/wave-integration-2026-10.md), design [wave-orchestrator §15](../wave-orchestrator.md)).

## Purpose

The package holds the pure parts of the integration of a wave. A fleet lane delivers its cycle as a record. The orchestrator orders the records, composes them on one integration branch, tests the batch and moves `main` one time for the batch. This package holds the record, the landed identity, the order score, the flush policy and the compose engine. The CI side lives in the ship phase and the orchestrator.

## Design

- **The deliverable record (I0, decision D2).** `DeliverableRecord` holds the cycle, its commits with the patch-id of each, and the `base_sha`. It also holds the audit binding, the scope, the deps, the inbox rank and the risk class. `Validate` refuses a record with a missing field. `WriteRecord` and `ReadRecord` keep one JSON file for each cycle in `WaveDir`, at `RecordPath`. `ReadRecord` also refuses a record that does not validate.
- **The record schema (I0).** The schema is `evolve.deliverable/2`. A record of schema `evolve.deliverable/1` holds a patch-id of the default context width. `ReadRecord` refuses it. `FindRecord` and `FindRecordByCommit` skip a refused record without a signal, so the cycle reads as not delivered. The ancestry check answers first, and the cycle must be delivered again for the record path.
- **The wave directory (I0).** `WaveDir` is the one home of the directory layout `.evolve/waves/integration-<wave>/`. The ship phase writes into it, and `FindRecord` reads from it.
- **The ordering view (I0).** `DeliverableRecord.Deliverable` maps a record to the `Deliverable` of the order score and the flush policy.
- **The landed identity (I0).** A cycle is the cycle trailer (`CycleTrailer`) plus the patch-id of each of its commits, not a SHA. A compose gives each commit a new SHA, so an ancestry check alone misses a composed cycle. `HasCycleTrailer` reads the trailer of one commit message.
- **The repository port (I0).** `Repo` holds the directory and the `GitFunc` of one repository. `Repo.PatchID` gives the stable patch-id of a commit. It reads the diff with one context line (`-U1`). `git patch-id` ignores hunk line numbers, so the identity is the file name, the changed lines and one neighbor line on each side. A peer edit two lines away does not change it, and a peer edit next to a changed line is a merge conflict.
- **The landed check (I0).** `FindLanded` is a pure check: each commit of the record must match its own commit that carries the trailer. A landed commit satisfies one entry only. The trailer decides first, and the patch-id checks the recorded change. `Repo.LandedOnRef` gives the commits from `base_sha` to a ref to `FindLanded`. `FindRecord` finds the record of a cycle, and `FindRecordByCommit` finds the record that holds a commit SHA.
- **The batch (I2, decision D4).** `Batch` names one integration branch, `integration/w<wave>-b<index>`, with its base and its tip. `Repo.StartBatch` cuts the branch at the base with `git update-ref`. It does not check out a branch and it does not touch the work tree. The empty old value makes git refuse a branch that exists.
- **The compose engine (I2, decisions D4, D5 and D14).** `Composer` holds the repository, the stage, the gate and the overlap facts. `Composer.Compose` applies each commit of a record, in order, onto the batch tip. It uses `git merge-tree --write-tree` with the parent of the commit as the merge base, and then `git commit-tree`, so the compose needs no work tree. The compose passes `--attr-source=<tip>` to git, so the attributes of the tip decide a `merge=union` file in a bare clone too. Each step makes a commit with the batch tip as its parent, so the last step gives one commit for the cycle.
- **The composed message (I2).** The subject is `evolve-cycle <N>: integrate`. The trailers are `Evolve-Cycle`, `Evolve-Artifact-SHA256` (the audit artifact) and `Evolve-Audit-Tree` (the tree that the audit saw). `evidence.KeyAuditTree` is the new key, and `evidence.Trailer.AuditTree` holds its value. So `evidence.Trailer.Build` writes the three trailers, and `evidence.Parse` reads them back. The trailer format has one home.
- **The tier (I2).** `Composer` gives the paths of the compose and the paths that the batch holds since its base to `overlap.Prove`. The merge class is `clean`, because a conflict ejects before the proof. Without the overlap facts, or without the catalogs, the tier is `T3` with the rule `unknown`. `overlap.Prove` needs the catalogs, and `T3` runs the full gates of a tier.
- **The gate and the stage (I2).** `Gate` takes the composed tip and its tier. At `enforce`, `Compose` calls the gate after a clean compose. At `shadow`, `Compose` makes the textual compose only and calls no gate (§15 Stage). The compose then moves the branch with `update-ref` from the old tip. So a writer that moved the branch first makes the compose fail.
- **The ejection (I2, decision D6).** A textual conflict or a red gate gives an `*Ejection` error with the cycle, the cause (`conflict` or `gate`) and the conflicted paths. It also holds the diff from the batch base to the tip that the deliverable met. A conflict ejects before the gate, so a conflict runs no gate. A git failure is a plain error and not an ejection.
- **The prefix check (I2, decision D16 L1).** `Composer.PrefixCheck` goes through the records in order. It puts each record on the tip of the records that landed before it. A record that ejects does not move the tip. The check moves the branch one time, to the tip of the landed records.
- **The batch test L2 (I3b, decisions D16 and D17).** `BatchTest.Check` runs after the L1 prefix check. It first checks the composer, so an unknown stage is an error. It acts only at `enforce`. At `shadow` it returns an empty result and calls no seam. At `enforce` it refuses a `BatchTest` that has a nil seam.
- **The stage rule (I3b).** It composes each prefix with the textual compose only. L1 ran the gates on the L1 prefixes. After an L2 ejection, the landed tree is not an L1 prefix, so it relies on L2 and L3 until the repo-contract pack joins L2. It never moves the batch branch.
- **The culprit search (I3b).** It always starts from the batch base, also when the batch holds the moved L1 tip, so each prefix is a real prefix. It uses the same loop as `PrefixCheck`: it tries the prefixes A, A+B and A+B+C, and each prefix is composed one time. A red deliverable is ejected and does not move the tip. Each landed tree was tested by the loop, so no second test of the landed set follows. A failed git call or a failed seam returns an error at once.
- **The layer (I3b).** For one prefix tip, `BatchTest` finds the affected set and runs the tests of each package. It also runs the acceptance predicates of every deliverable in the prefix on the combined tree. So two deliverables that pass alone can fail together. The first red prefix ejects its last deliverable with the cause `batch` (`CauseBatch`).
- **The batch ejection (I3b).** That ejection holds the diff from the batch base to the tip that the deliverable met. For this cause, `Ejection.Paths` holds the red package patterns and the red predicate names, not file paths. I3 decides the final name of the field. A textual conflict keeps the cause `conflict`.
- **The affected set (I3b).** `AffectedSet` gives the edited packages plus every package that imports one of them through `changedpkgs.ImporterClosureChecked`. It returns `Closure.Testable`, so a package that no longer exists is not tested. It fails when the module cannot be listed.
- **The edited packages (I3b).** `editedPackages` takes the packages from the paths that the prefix tip changes since the batch base, read with `Repo.changedPaths`. It does not read `DeliverableRecord.Scope`, because the fleet writes inbox task ids into that field.
- **The path mapping (I3b).** `changedpkgs.PatternOf` is the one home of this rule. A path under a `testdata` directory maps to the parent of that directory, for each file type, a Go file too. `go/go.mod`, `go/go.sum` and each path under `go/vendor/` map to `./...`. Another path under `go/` maps to its directory. A path outside `go/` maps to nothing.
- **The closure limit (I3b).** The closure follows production imports and the direct test imports of a package. A package that reaches an edit only through the imports of a test import is not in the set, and L3 covers it.
- **The flake rule (I3b).** A red package or a red predicate runs one more time.
- **The red with an error (I3b).** A predicate run that returns a verdict with a red and an error is red. This is the shape of `acsrunner.Run` when the predicate package does not compile on the combined tree. A pass on the second run is a flake. The name goes to `LayerVerdict.Flaky` and `BatchResult.Flaky`, and nothing is ejected. A flaky package is not stored as green.
- **The runner error (I3b).** A runner error is fatal. `PackageTest` returns a `TestOutcome` and an error. An error means that the test cannot run, for example a failed checkout or a cancelled context. The check stops and ejects nobody. A red package keeps its test output in `LayerVerdict.Output` and in the error of the ejection.
- **The tested tip (I3b).** `BatchResult.Tip` is the tip of the landed prefix, the commit that L2 tested. It is the batch base when nothing landed. L2 does not move the branch.
- **The predicate flake (I3b).** A predicate that is red on the first run only is a flake too.
- **The verdict record (I3b).** `BatchResult.Verdicts` holds one `LayerVerdict` for each deliverable: the packages that ran, the packages that the store skipped, the flakes and the reds. The `Verdicts` interface (`Green`, `Record`) is a consumer seam. It is not yet the shape of `internal/testverdict.Store`: the store needs a key, a gate and a log. The owner item `wave-integration-wire-batch-test-into-the-train` settles the shape of the port when it writes the adapter.
- **The verdict skip (I3b).** A package with a green verdict on the same key is skipped and is not stored again. `acsrunner.Verdict` is the predicate result, from the same runner that the ship phase uses.
- **The gate verdict (I2).** A gate gives a red verdict as a `*GateRed` error, and only that error ejects the deliverable. Any other gate error means that the gate did not run, for example a binary that did not start. It is a plain error, so the compose does not send an innocent deliverable back to its lane. Pinned by `TestCompose_ARedGateEjectsAndKeepsTheBranch` and `TestCompose_AGateThatCannotRunIsAnErrorNotAnEjection`.
- **Why a loop (I2).** An earlier draft drove `fleet.PrefixQueue` with all of its features off: one group, no window and no trim. That needed a new package, a memo and a side channel for errors, and the loop gives the same result. So the loop replaced it.
- **`GitError` (I2).** `SysGit` returns a `*GitError` with the arguments, the exit code, the output and the error output. `git merge-tree` exits with 1 on a conflict and prints the conflicted paths. The compose needs both facts.
- **The order score (I1, decision D3)** is a pure function in `order.go`.
- **The flush policy with the adaptive cap (I4, decisions D7 to D9)** is a pure function in `flush.go` and `cap.go`.

## Invariants

- A trailer match with a different patch-id is not the cycle. Pinned by the landed tests in this package.
- A commit with the trailer and other changed lines is not landed. The same lines at another place are not landed when the neighbor lines differ. The same change at the same place is landed. Pinned by `TestLandedOnRefRejectsACommitWithOtherContentAfterAPeerEdit`, `TestLandedOnRefRejectsTheSameLinesAtAnotherPosition` and `TestLandedOnRefReadsTheSameChangeAtTheSamePlaceAsLanded`.
- One landed commit satisfies one commit of the record. Pinned by `TestFindLanded_ConsumesEachLandedCommitOnce` and `TestLandedOnRefNeedsACommitForEachCommitOfTheRecord`.
- A cycle with only some of its commits on the ref has not landed. Pinned by `TestFindLandedNeedsEveryCommitOfTheCycle`.
- A record vouches only for a commit that it holds (`HoldsCommit`). The ship, the lost-landing floor and the inbox closeout each ask about one commit. Pinned by `TestLanded_ARecordThatDoesNotHoldTheCommitDoesNotVouchForIt` and `TestDetectLostLanding_ARecordThatDoesNotHoldTheBindingCommitDoesNotVouchForIt`.
- `Repo.LandedOnRef` puts `--end-of-options` before the revision range, so git never reads a `base_sha` of a record as an option. Pinned by `TestLandedOnRefNeverReadsABaseAsAGitOption`.
- `Repo.LandedOnRef` reads each candidate SHA from git, not from a commit message. It runs `git log -z`, and a commit message cannot hold a NUL byte. Pinned by `TestLandedOnRefReadsEachShaFromGitNotFromAMessage`.
- `Repo.PatchID` accepts only a full object id, and it puts `--end-of-options` before the SHA. Pinned by `TestPatchID_RefusesAnythingButAFullObjectID`.
- A record counts only when the patch-id of each of its commits is the real patch-id of that commit. So a planted record cannot pair an unlanded SHA with the patch of a landed commit. When a recorded commit is no longer in the repository, the check fails closed, and the cycle reads as not landed. Pinned by `TestLandedOnRefRefusesARecordWhosePatchIsNotItsOwnCommit`.
- `WriteRecord` stages the record in a new file with a random name (`os.CreateTemp`), so it never writes through a link at a staging name. Pinned by `TestWriteRecordNeverWritesThroughALinkAtAStagingName`.
- A record with a missing D2 field does not validate and is not written.
- A compose refuses a record before it runs git when a commit, a lane base or the audit tree is not a full object id. It also refuses an audit artifact digest that is not 64 lowercase hex characters, and a trailer that is not the trailer of the cycle. So a record cannot put an option into git or a trailer line into the message. Pinned by `TestCompose_RefusesAnUnsafeInputBeforeGit`.
- Each git call of the compose puts `--end-of-options` before a revision, except `git commit-tree`, which does not accept it. The compose gives `commit-tree` only a checked tree id and the checked batch tip. Pinned by `TestCompose_NeverPassesAMergeTreeOutputThatIsNotAnObjectIDToCommitTree`.
- A textual conflict ejects and runs no gate. Pinned by `TestCompose_ATextualConflictEjectsWithNoGateRun` and `TestPrefixCheck_AConflictEjectsWithNoGateRunAndTheRestLand`.
- At `shadow`, the compose and the prefix check call no gate. Pinned by `TestCompose_AtShadowComposesTextuallyAndCallsNoGate` and `TestPrefixCheck_AtShadowComposesTextuallyAndCallsNoGate`.
- The composed commit carries the cycle trailer and the audit binding. When the batch held no change in the context lines of the record, `Repo.LandedOnRef` finds the cycle on the batch branch. Pinned by `TestCompose_OneCommitWithTheCycleAndTheAuditBindingTrailers`.
- With A, B and C, where C breaks the gate, the prefix check passes A and A+B and fails A+B+C. It ejects C with the diff of A+B. Pinned by `TestPrefixCheck_PassesAAndABAndFailsABCThenEjectsCWithTheDiffOfAB`.
- An ejection or a failure does not move the batch branch. Pinned by the ejection tests and `TestPrefixCheck_AGitFailureStopsTheCheck`.
- With two deliverables that pass alone and fail together, L2 runs both predicate sets on the combined tree and ejects the later one. Pinned by `TestBatchTest_TwoDeliverablesThatPassAloneFailTogetherAndTheLaterOneIsEjected`.
- The affected set holds each package that imports an edited package. Pinned by `TestAffectedSet_IncludesEveryPackageThatImportsTheEditedOne`.
- A test that fails and then passes is a flake, and nothing is ejected. Pinned by `TestBatchTest_AFlakeIsRecordedAndEjectsNothing`.
- A package with a green verdict on the same key is skipped and recorded as skipped. Pinned by `TestBatchTest_APackageWithAGreenVerdictOnTheSameKeyIsSkippedAndRecorded`.
- At `shadow`, the batch test runs nothing. Pinned by `TestBatchTest_AtShadowRunsNothing`. An unknown stage or a nil seam is an error. Pinned by `TestBatchTest_AnUnknownStageIsRefusedAndANilSeamIsRefused`.
- A record whose scope is a task id still gets the tests of the packages that its commits edit. Pinned by `TestBatchTest_ATaskIDScopeStillTestsThePackagesTheDiffEdits`.
- A path under a `testdata` directory or under `go/vendor/` still selects a package test. Pinned by `TestBatchTest_ADeliverableThatEditsOnlyAGoFixtureOrAVendoredFileStillTestsAPackage` and `TestPatternOf_NamesTheGoTestPatternOfEachPathUnderGo`.
- After the L1 prefix check, `Check` still ejects the real culprit. Pinned by `TestBatchTest_AfterTheL1PrefixCheckTheSearchStillStartsFromTheBase`.
- A verdict is keyed on the tip that it tested. Pinned by `TestBatchTest_TheSeamsKeyTheirVerdictsOnTheTipUnderTest`.
- A package that no longer exists is not in the affected set. Pinned by `TestAffectedSet_LeavesOutAPackageThatNoLongerExists`.
- A runner error does not eject a lane. Pinned by `TestBatchTest_APackageRunnerFailureStopsTheCheckAndEjectsNobody`.
- A predicate verdict with a red and an error is red. Pinned by `TestBatchTest_APredicateThatFailsWithAVerdictIsRedAndNotFatal`.
- A batch ejection holds the diff. Pinned by `TestBatchTest_ABatchEjectionHoldsTheDiffTheDeliverableMet`.
- L2 never runs the gate and never moves the batch branch. Pinned by `TestBatchTest_L2NeverRunsTheGate` and `TestBatchTest_L2NeverMovesTheBatchBranch`.
- A console record keeps its PR number through L2. `Landed`, `Verdicts` and the ejection use `DeliverableRecord.Ref`, and a console record gets no cycle predicate. Pinned by `TestBatchTest_ConsoleRecordsKeepTheirIdentityThroughL2`.
- Without a record, callers keep the ancestry rule of today.

## Limits

- The compose engine is built. `Train` composes with `Composer.Compose` at `shadow`. `PrefixCheck` has no production caller.
- The batch PR and the PR train are built. The ejection brief to the lane (I3) and the integration record (I10) are not built yet.
- The compose sets no precondition of the proof (`BaseNotAncestor`, `AuditedTreeMissing`, `CompileRed`). So the proof gives `T1` to `T3`, and no tier-T4 ejection occurs yet.
- A record with more than one commit composes as one commit. Its patch-id is the patch-id of the sum, so `Repo.LandedOnRef` finds only a record with one commit. The identity of a record with many commits is an open item for I3.
- The batch test (I3b) is built. `BatchTest.Check` has no production caller. The train lands on CI alone until an owner wires it.
- The inbox item `wave-integration-wire-batch-test-into-the-train` owns the wiring. It owns the three adapters, the branch move to `BatchResult.Tip`, and the output limit. It also owns the split of a host failure from a build failure. It owns the rest of the L2 row too: the repo-contract pack, cover-changed, cover-strict, build, vet and gofmt.
- The adapters bind three seams. `PackageTest` binds to a checkout of the tip. `PredicateRun` binds to `acsrunner.Run`. `Verdicts` binds to `testverdict.Store`. The ship phase is unchanged.
- The L2 repo-contract pack, cover-changed, cover-strict, build, vet and gofmt are not in the batch test yet. `go test` covers the build and part of vet. The rest join as further `PackageTest` seams with the adapters.
- After an L2 ejection, the batch branch still holds the L1 tip. The owner item must move the branch to `BatchResult.Tip`. It must refuse a batch with a tip that was not tested.
- A change outside `go/` selects no package test. A Go test can read such a file, for example a document. The repo-contract pack of the owner item covers this. L3 covers it until then. `LayerVerdict.Ran` is empty for such a change.
- A predicate that fails without a test event, for example when the host kills `go test`, reads as the red `egps/no-predicates`. The error text of the run goes to `LayerVerdict.Output`, so the lane brief holds the cause. Pinned by `TestBatchTest_APredicateRunErrorReachesTheLaneBrief`. `acsrunner` has no way to tell a host failure from a build failure. The owner item must separate the two.
- `LayerVerdict.Output` holds raw test output without a size limit. The owner item and the ejection brief (I3) must set a limit and frame the text as data.
- `Check` and `PrefixCheck` share the culprit-search skeleton in two copies, and `Check` selects "no gate" through the shadow stage of a copy of the composer. A later change removes both. The change is in `prefix.go` and `compose.go`.
- Only a live wave can show that production reaches this path. Today no production code calls `BatchTest.Check`, and the stage is `shadow`.
- The compose makes the patch again on a different tip. A patch-id of one context line does not change when a peer edits a line two lines away. Pinned by `TestCompose_ARecordWhoseContextAPeerChangedReadsAsLanded` and `TestLandedOnRefSurvivesAPeerEditTwoLinesFromTheCycleChange`. A record with more than one commit still misses, because its patch-id is the patch-id of the sum.
- The patch-id cannot tell apart two places that have the same neighbor lines, for example a repeated block. The trailer limits the exposure to commits of the same cycle. I3 closes the limit when it binds the identity to the commit of the compose.
- I3 must bind the identity to the commit that the compose made, before the ledger, the closeout and the consume check use it. The identity of a record with many commits is an open item for I3.
- The gate result is not cached yet. The verdict store of D5 comes with I3b.
- Until they exist, `fleet.integration.stage` stays `shadow`, and a fleet ship writes the record and lands as before.

## The console PR train

A lane record has a cycle number and a trailer. A console PR record has the source console-pr, a PR number and no cycle number. DeliverableRecord.Ref gives the number of either kind, and Label gives its name for a message.

The review binding of a console PR covers the change of the PR. Train.Record gives the binder the evidence: the commits, the changed paths and the digest of the diff. AttestedBinding accepts an attestation only when its tree equals that digest. ClassBinding accepts a class only when the rule of the class covers the paths. The caller owns the attestation schema and the closed list of classes, because this package cannot import the commit gate. That package imports this one through core.

Train plans and lands the ready PRs. Plan does three things and pushes nothing:

- It refuses a PR whose head is not the last recorded commit.
- It sets aside a PR that is already on main.
- It composes the rest in a scratch branch. The branch name holds the wave and "-plan", so a plan never meets the branch of a landing.

Land does these steps, and it never changes a member branch:

- It opens one batch PR and moves main with one fast-forward.
- It closes the inbox items that the landed commits name.
- It closes each landed PR with a link to the batch, and it spends the record.
