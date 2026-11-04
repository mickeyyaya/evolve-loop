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
- **The gate verdict (I2).** A gate gives a red verdict as a `*GateRed` error, and only that error ejects the deliverable. Any other gate error means that the gate did not run, for example a binary that did not start. It is a plain error, so the compose does not send an innocent deliverable back to its lane. Pinned by `TestCompose_ARedGateEjectsAndKeepsTheBranch` and `TestCompose_AGateThatCannotRunIsAnErrorNotAnEjection`.
- **The prefix check (I2, decision D16 L1).** `Composer.PrefixCheck` is a loop. It tries each record on the tip of the records that passed before it, so it tries A, A+B and A+B+C. An ejected record does not move the tip. The check moves the branch one time, to the tip of the landed records.
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
- Without a record, callers keep the ancestry rule of today.

## Limits

- The compose engine and the L1 prefix check (I2) are built. Nothing in production calls them yet.
- The ejection brief to the lane (I3), the batch test (I3b), the batch PR (I5) and the integration record (I10) are not built yet.
- The compose sets no precondition of the proof (`BaseNotAncestor`, `AuditedTreeMissing`, `CompileRed`). So the proof gives `T1` to `T3`, and no tier-T4 ejection occurs yet.
- The compose makes the patch again on a different tip. A patch-id of one context line does not change when a peer edits a line two lines away. Pinned by `TestCompose_ARecordWhoseContextAPeerChangedReadsAsLanded` and `TestLandedOnRefSurvivesAPeerEditTwoLinesFromTheCycleChange`. A record with more than one commit still misses, because its patch-id is the patch-id of the sum.
- The patch-id cannot tell apart two places that have the same neighbor lines, for example a repeated block. The trailer limits the exposure to commits of the same cycle. I3 closes the limit when it binds the identity to the commit of the compose.
- I3 must bind the identity to the commit that the compose made, before the ledger, the closeout and the consume check use it. The identity of a record with many commits is an open item for I3.
- The gate result is not cached yet. The verdict store of D5 comes with I3b.
- Until they exist, `fleet.integration.stage` stays `shadow`, and a fleet ship writes the record and lands as before.
