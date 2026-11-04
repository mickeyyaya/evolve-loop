# go/internal/integration

The integration engine of the wave orchestrator ([ADR-0147](../adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md), plan [wave-integration-2026-10](../../plans/wave-integration-2026-10.md), design [wave-orchestrator §15](../wave-orchestrator.md)).

## Purpose

The package holds the pure parts of the integration of a wave. A fleet lane delivers its cycle as a record. The orchestrator orders the records, composes them on one integration branch, tests the batch and moves `main` one time for the batch. This package holds the record, the landed identity, the order score and the flush policy. The git and CI side lives in the ship phase and the orchestrator.

## Design

- **The deliverable record (I0, decision D2).** `DeliverableRecord` holds the cycle, its commits with the patch-id of each, and the `base_sha`. It also holds the audit binding, the declared scope, the deps, the inbox rank and the risk class. `Validate` refuses a record with a missing field. `WriteRecord` and `ReadRecord` keep one JSON file for each cycle in `WaveDir`, at `RecordPath`. `ReadRecord` also refuses a record that does not validate.
- **The wave directory (I0).** `WaveDir` is the one home of the directory layout `.evolve/waves/integration-<wave>/`. The ship phase writes into it, and `FindRecord` reads from it. `WaveDir` refuses a wave id outside `^[A-Za-z0-9._-]+$` with `ErrBadWaveID`. No code removes a wave directory yet. The integration record (I10) adds the prune at the wave boundary, because I10 defines the wave ids and the boundary.
- **The commit index (I0 hardening).** `WriteRecord` writes one empty file `commits/<sha>-<cycle>` for each commit of the record before it publishes the record. `FindRecordByCommit` finds the index entries with one glob and reads only the record that each entry names. A stale entry does not vouch, because the lookup also checks `HoldsCommit`.
- **The ordering view (I0).** `DeliverableRecord.Deliverable` maps a record to the `Deliverable` of the order score and the flush policy.
- **The landed identity (I0).** A cycle is the cycle trailer (`CycleTrailer`) plus the patch-id of each of its commits, not a SHA. A compose gives each commit a new SHA, so an ancestry check alone misses a composed cycle. `HasCycleTrailer` reads the trailer of one commit message.
- **The repository port (I0).** `Repo` holds the directory and the `GitFunc` of one repository. `Repo.PatchID` gives the stable patch-id of a commit. A merge commit or an empty commit has no patch, so `Repo.PatchID` returns `ErrNoPatch`. The ship phase maps `ErrNoPatch` and `ErrBadWaveID` to a precondition refusal, not to a transient fault, because a retry gives the same result.
- **The patch-id helper (I0 hardening).** `Repo.DiffPatchID` is the one call site of `git patch-id --stable` in `go/internal`. `Repo.PatchID`, `ledger.PatchID` and `core.compositionPatchID` call it.
- **The landed check (I0).** `FindLanded` is a pure check: each patch-id of the record must be on a commit that carries the trailer. `Repo.LandedOnRef` reads the commits from `base_sha` to a ref with `git log` and gives them to `FindLanded`. `Repo.LandedSince` also excludes each commit that is an ancestor of a bound. The lost-landing floor uses `PreCycleHEAD` as the bound, as its ancestry rule does. `FindRecord` finds the record of a cycle, and `FindRecordByCommit` finds the record that holds a commit SHA.
- **The order score (I1, decision D3)** is a pure function in `order.go`.
- **The flush policy with the adaptive cap (I4, decisions D7 to D9)** is a pure function in `flush.go` and `cap.go`.

## Invariants

- A trailer match with a different patch-id is not the cycle. Pinned by the landed tests in this package.
- A cycle with only some of its commits on the ref has not landed. Pinned by `TestFindLandedNeedsEveryCommitOfTheCycle`.
- A record vouches only for a commit that it holds (`HoldsCommit`). The ship, the lost-landing floor and the inbox closeout each ask about one commit. Pinned by `TestLanded_ARecordThatDoesNotHoldTheCommitDoesNotVouchForIt` and `TestDetectLostLanding_ARecordThatDoesNotHoldTheBindingCommitDoesNotVouchForIt`.
- `Repo.LandedOnRef` puts `--end-of-options` before the revision range, so git never reads a `base_sha` of a record as an option. Pinned by `TestLandedOnRefNeverReadsABaseAsAGitOption`.
- `Repo.LandedOnRef` reads each candidate SHA from git, not from a commit message. It runs `git log -z`, and a commit message cannot hold a NUL byte. Pinned by `TestLandedOnRefReadsEachShaFromGitNotFromAMessage`.
- `Repo.PatchID` accepts only a full object id, and it puts `--end-of-options` before the SHA. Pinned by `TestPatchID_RefusesAnythingButAFullObjectID`.
- A record counts only when the patch-id of each of its commits is the real patch-id of that commit. So a planted record cannot pair an unlanded SHA with the patch of a landed commit. When a recorded commit is no longer in the repository, the check fails closed, and the cycle reads as not landed. Pinned by `TestLandedOnRefRefusesARecordWhosePatchIsNotItsOwnCommit`.
- `WriteRecord` stages the record in a new file with a random name (`os.CreateTemp`), so it never writes through a link at a staging name. Pinned by `TestWriteRecordNeverWritesThroughALinkAtAStagingName`.
- A record with a missing D2 field does not validate and is not written.
- `WaveDir` refuses a wave id outside `^[A-Za-z0-9._-]+$`, so a wave id cannot leave `.evolve/waves`. Pinned by `TestWaveDir_RefusesAnIDOutsideTheSafeAlphabet`.
- A commit with no patch is a precondition refusal at enforce, not a transient fault. Pinned by `TestPatchID_AMergeOrAnEmptyCommitIsTheNoPatchRefusal` and `TestDeliver_ANoPatchCommitIsARefusalNotATransientFault`.
- A lookup by commit reads one record, and a miss reads no record. Pinned by `TestFindRecordByCommit_ReadsOneRecordPerLookup`.
- `WriteRecord` refuses a commit that is not a full object id, so an index entry cannot leave its directory. Pinned by `TestWriteRecord_RefusesACommitThatIsNotAnObjectID`.
- The lost-landing floor does not count a compose that was on `HEAD` before the cycle. A bound that is not a full object id fails closed. Pinned by `TestDetectLostLanding_AComposeThatLandedBeforeTheCycleIsNotThisCyclesLanding` and `TestLandedSince_BoundsTheSearchByTheCommitBeforeTheCycle`.
- Without a record, callers keep the ancestry rule of today.

## Limits

- The compose engine (I2) and the prefix check (I3) are not built yet.
- The batch test (I3b), the batch PR (I5) and the integration record (I10) are not built yet.
- Until they exist, `fleet.integration.stage` stays `shadow`, and a fleet ship writes the record and lands as before.
