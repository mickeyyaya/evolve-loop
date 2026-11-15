# Integration driver: the EnforceActor card

> Parent spec: [integration-driver](integration-driver.md) §9. Package note: [internal-integration](packages/internal-integration.md).
> **Status:** Redesigned (2026-11-15): delivery is write-once through a WritePolicy strategy; review round 3 (final). This card has no code. The code follows in a later cycle, after the card passes review.
> Decision record: [solutions/wave-integration-i14-driver-pr-c](../../solutions/wave-integration-i14-driver-pr-c/recommendation.md).

**Terms used throughout:**
- **EnforceActor:** the `Actor` that lands a pool at stage `enforce`. PR (b) has only `ShadowActor`.
- **Delivered:** a lane shipped its patch to a wave record. The patch is not yet on `main`.
- **Lane identity:** the pool id, the lane cycle number and the scope of one fleet lane.
- **Brief:** the file that `Ejection.WriteBrief` writes. It tells the next lane why a patch conflicted.
- **Fake CI:** a test double of the `CI` port that returns a set verdict.
- **Bare origin:** a bare git repository that serves as `origin` in a test.
- **Lane Heads:** the `Merger.Heads` check of a lane. It ejects each member whose lane patch ids differ from the record (§2, §10).
- **Write-once record:** the delivery record of a member at enforce. The first write is final. No later delivery changes it (§10).
- **Redelivered:** at shadow only. The latest patch ids that a member landed after a second delivery (§10).

## 1. Goal and scope

At `enforce`, a lane ship writes its record and stops before `main`. Nothing lands it ([integration-driver](integration-driver.md) §2). The `EnforceActor` lands it.

The card decides the seven open inputs of §9 of the parent spec. Each decision has a fact, a decision and a consequence. Acceptance of the inbox item `wave-integration-i14-driver-pr-c`:

1. A journey with a fake CI and a bare origin lands one member by fast-forward. It requeues a conflict with its brief.
2. No second lane starts on a delivered item.
3. `evolve loop --resume` of a lane checkpoint cannot land on `main` at enforce.

The card cites symbol names, not line numbers, because lines drift. The `EnforceActor` itself is new code that sits beside `Driver`. The card also needs these changes to existing code:

| Change | Where | Part |
|---|---|---|
| R-a at shadow: a delivery with the patch ids of the member is a no-op. | `deliverable.go`, `driver.go` | 0 |
| R-d: `noteRedelivered` stops writing a failure. It stores the new patch ids as `redelivered` information on the member. The guard of `noteRedelivered` (`m.Status != MemberPending`) scopes this. At shadow, `admits` keeps today's re-admit of a pending member with new patch ids. | `driver.go` | 0 |
| The member gets a field `Redelivered`. It holds the latest patch ids that landed. `evolve integration status` prints it, and the `clean` verdict of the parent spec §7 ignores it. | wave record, `status`, the `clean` verdict | 0 |
| The write-once check lives in the single writer, `integration.WriteRecord`. A `WritePolicy` strategy selects it (§10). Under `WriteOnce`, the writer publishes with an exclusive create (`os.Link` of the temp file in place of `os.Rename`). If the file exists, it compares the patch ids of the record file: equal is a no-op, different is a typed refusal (§10). As defence in depth, `admits` at enforce refuses a changed record of a known member and writes a failure. It never replaces `PatchIDs`. | `deliverable.go`, `driver.go` | 7 |
| The lane `Heads` ejects a member whose lane patch ids differ from the record. | lane `Heads` in `integration` | 2 |
| H6.2: the stage gate refuses `enforce` while N < 2 (§12). | stage gate in the driver | 8 |
| H6.3: the pool hold. The loop (`runFork` in `cmd_loop_driver.go`) holds a second pool start. It reads the state of the first enforce pool from its wave record. | loop in `cmd` | 8 |
| Evidence integrity of the clean-wave count (§12). | `status`, stage gate | 8 |

Part 0 of the split (§13) holds the shadow prerequisite: R-a and R-d. It ranks first.

## 2. EnforceActor

`EnforceActor` implements the `Actor` interface of the parent spec §3. It holds the enforce ports. `Driver` holds none. `Driver` keeps the Template Method skeleton: `drain` and `finishWave` stay in `Driver`. An actor fills in the steps.

```go
type EnforceActor struct {
    Merger  Merger
    Plane   PlaneSync
    Landing LandingFeed
    Promote Promoter
}
```

The `Requeue` and `Briefs` ports of the first draft are removed. `Merger.Eject` does their work (§4).

| Port | Method set |
|---|---|
| `Merger` (exists in `batchpr.go`) | `Land(ctx, recs) (Outcome, error)`. Its fields are `CI`, `Main`, `Compose`, `Heads` and `Eject`. |
| `PlaneSync` | `Sync(ctx) (SyncOutcome, error)` |
| `LandingFeed` | `Intent(ctx, rec DeliverableRecord) (LandingIntent, error)` |
| `Promoter` | `Promote(ctx, cycle int, commitSHA string) error` |

| Method | What it does at enforce |
|---|---|
| `Resume` | Reads the record. Adopts a fence that was pushed with an old record by the discard path. Deletes leaked `integration/p<pool>-*` refs. Never lands. |
| `Flush` | Takes one `PendingBatch`. Calls `Merger.Land` with a lane batcher as `Compose` and the lane `Heads` as `Heads`. Lands by fast-forward through `Main.FastForward`. Writes the outcome into the record. |
| `WaveEnd` | Calls `Plane.Sync` once. Marks each member that is still `pending`. It does not loop `Flush` and does not write `Ended`. |

`Driver.drain` loops `Flush` until `Hold` is empty. It is bounded by `maxFlushPass`. `Driver.finishWave` writes `Ended` and `LastCycle` after `actor.WaveEnd`. Both stay there, so one bound and one writer serve shadow and enforce. A member still `pending` after `MaxMoves` stays pending and raises `_DRIVER_FAILED` (row 5b of §10).

**Lane Heads.** A lane has no PR, so `Train`'s `movedHeads` does not apply. The lane `Heads` computes the patch ids of each member from the lane ref (`base..laneref`, with the same `Repo.PatchID` that `buildDeliverable` uses). It compares them with the patch ids of the record. It does not compare SHAs, because a retry commits again with a new SHA and the same patch ids. `Merger.Heads` means "eject this member", and `Merger.recheck` sends each result to `Merger.Eject`.

`Merger.Heads` keeps its signature and its one outcome: `Ejections`. The delivery record is write-once (§10), so a fault is: the patch ids of the lane differ from the record. The lane `Heads` ejects that member. The brief names the fault. `Merger.Eject` requeues the item, and a new cycle builds it fresh. No second record of a member exists, so no check can miss a newer record.

A lane ref that is gone is not a fault. Compose from the commits of the record and do not eject.

An item is never both requeued and pending, so no second lane starts on it.

The stage table of the parent spec §3 stays the source for the compose, gate and refs steps. This card adds the decisions below.

## 3. F2: the delivered inbox state

**Fact.** Post-ship sees `isLanded` false at a delivered ship (the `isLanded` branch of `promoteLandedCycle` in `postship.go`). It releases the item. A second lane then starts on the same work.

**Decision.** A delivered ship moves the inbox item to a new state `delivered`. `delivered` is a value of the existing lifecycle state type, next to `processed`, `rejected`, `retry` and `quarantine` in `validStates` of `go/internal/inboxmover/lifecycle/lifecycle.go`. The state has these rules:

- The menu builder skips a `delivered` item. So no second lane starts on it.
- The stale-claim sweep does not release a `delivered` item.
- `EnforceActor` ends the state. On `merged`, it promotes the item through `Promoter` (§7). On ejection, `Merger.Eject` requeues the item with the brief (§4).
- A `delivered` item with no member in any wave record is an orphan. The driver reports it once as a failure. It never releases it by itself.

**Readers.** Each reader below branches on the item state. A reader that does not know `delivered` treats it as unknown. The first six are in `go/internal/inboxmover`.

| Reader | What it does with `delivered` |
|---|---|
| menu builder | Skips the item. |
| stale-claim sweep | Does not release the item. |
| `dispatchability.go` | Reports the item as not dispatchable. |
| `claimstate.go` | Reads `delivered` as a held claim. |
| `continuation_release.go` | Does not release a continuation of the item. |
| `retire_unbacked.go` | Does not retire the item as unbacked. |
| `outcome.go` | Maps no outcome to a release of the item. |
| post-ship `ApplyCycleOutcome` with `Reason: "cycle-release-unlanded-ship-retry"` | Does not run for a `delivered` item. Else the F2 bug comes back. |

**Test.** A table test lists each reader with the state `delivered`. It fails when a reader does not handle the state. A new reader must join the table, or the test fails on a scan of the `inboxmover` readers.

**Consequence.** The state is data on the inbox item. No flag is added. Shadow keeps today's release, because shadow lands nothing.

## 4. F3: the brief reaches the next lane

**Fact.** `inboxbatch.Item` has no brief field. A brief-only continuation is unlikely to work, because readers skip an empty `SnapshotSHA`. `Merger` already has `Eject func(ctx, rec, cause) error`, and `Train.eject` fills it with a brief.

**Decision.**

- `inboxbatch.Item` gets a field `BriefPath`. `inboxBatchesSection` renders it into the prompt of the next lane.
- The enforce eject is `Merger.Eject` filled with requeue-with-brief. It calls `Ejection.WriteBrief`, then returns the item to the queue through `inboxmover` with `BriefPath` set. No `Requeue` port and no `Briefs` port exist. `Train.Brief` is not used, because it serves a PR author.
- Each ejection adds one to the failure count of the item. The convergence count of ADR-0126 bounds the retries.
- The requeue keeps the original item text. The brief is added, never substituted.

**Consequence.** A conflicted member is requeued with its brief. The next lane reads what conflicted and where. The train path and the enforce path use the same `Merger.Eject` seam.

## 5. F11: the mid-wave sync

**Fact.** `core.FastForwardPlaneMain` does not exist. The plane fast-forward lives in `package main` of `go/cmd/evolve`, in `cmd_loop_wavesync.go`: `syncMainFromOriginAtWaveBoundary`, `fastForwardWithInboxStamps` and `fastForwardMain`. The stamp step is `prepareInboxStampsForFastForward` and `replayInboxStampsAfterFastForward`. After a merge, the plane must follow `origin/main`.

**Decision.** `EnforceActor` cannot call a function in `package main`. The sync sits behind the `PlaneSync` port of §2. The `cmd` composition root builds the implementation. It wraps `fastForwardMain` and gives the stamp strategy as an injected step:

- Mid-wave, the strategy is a plain `merge --ff-only`. No stamp runs.
- At the boundary, `syncMainFromOriginAtWaveBoundary` keeps the stamp path.
- A blocked mid-wave sync waits for the boundary. `_MERGED` stays.
- `SyncOutcome` is `Current`, `Ahead`, `Diverged`, `Blocked` or `Moved`. Mid-wave, `Moved` and `Current` proceed.

`syncToOrigin(ctx, g, rel, warn) (SyncOutcome, error)` is the function that the composition root extracts from these three. It stays in `cmd`. `integration` sees only `PlaneSync`.

**Consequence.** A merge never waits for a boundary stamp. A blocked sync never retracts `_MERGED`, because `main` did merge. The dependency points from `cmd` to `integration`.

## 6. F12: a local bookkeeping commit after a merge

**Fact.** A plane with a local bookkeeping commit after a merge reads `Diverged`. `bookkeeping.go` holds two pure predicates and no flush.

**Decision.** The merge step builds the flush that plan D12 names.

1. Rebase the local-only commits onto the new `origin/main`.
2. Flush them with `evolve ship`, not a bare push.
3. Run `Plane.Sync`.

Any other `Diverged` raises `_DRIVER_FAILED`. The plane sync refuses `Ahead` only after the flush ran.

**Consequence.** Plane `main` equals `origin/main` after each merge. Zero-ship reads 1 in the journey.

## 7. Finding 3: promotion at merged

**Fact.** `rec.Scope` is not the promotion set. A deferred id must stay open. A marker id must be promoted. Two homes exist today:

- post-ship `promoteLandedCycle`. It reads the triage decision, the lane-scope pin and the build-report markers from the cycle directory.
- `Train.complete`, then `closeInbox`, then `Repo.closingIDs` on the commits of the record, then `inboxmover.ReconcileSuperseded`. It reads the set from commit trailers.

**Decision.** One `Promoter` port, owned by `integration`. It takes the cycle, not commits: `Promote(ctx, cycle int, commitSHA string)`. A lane commit carries no `closes-inbox` trailer, so a trailer set is empty for a lane. The port has two input strategies:

- The lane strategy runs the existing `promoteLandedCycle` rules over the cycle-directory artifacts at merge time. The artifacts are the triage body, `DeferredIDs`, `ClosedDroppedIDs`, the verdict, the lane-scope pin and the build-report `Closes-Inbox` markers. `FilterPromotionScope` re-reads the inbox at call time, so the set cannot go stale. The rules move into `inboxmover`, or stay behind the port with a `cmd` implementation.
- The console strategy supplies the `closes-inbox` trailer ids that `Repo.closingIDs` reads for `Train`.

Post-ship, `Train` and `EnforceActor` all call the port. The cycle directory must outlive the wave until the member is `merged`. The driver does not archive it earlier.

| Id | Decision | Because | Rejected option and why |
|---|---|---|---|
| D7.1 | The lane set is computed at merge time from the cycle artifacts. | The rules need the artifacts, and the inbox re-read at call time keeps the set current. | Commit trailers for lanes: a lane commit has none, so nothing is promoted. Store the set at delivery: it goes stale. |
| D7.2 | One `Promoter` implementation with two input strategies. | Three homes make two paths promote different sets for one lane. | Add `PromoteLanded` as a third home: it duplicates the rules. |

**Test.** A lane with one deferred id and one marker id merges at enforce. The marker id is promoted and the deferred id stays open (row 11 of §11).

**Consequence.** One home for promotion. A deferred id stays open, and a marker id is promoted, on all three paths.

## 8. Finding 4: a rebase on a dirty plane

**Fact.** A rebase on a dirty plane fails. `--autostash` can hide operator work.

**Decision.** A refused rebase waits for the boundary. `_MERGED` stays. No `--autostash`. The driver writes the wait into the record, so `status` shows it.

**Consequence.** Operator edits in the plane are never moved or lost.

## 9. Finding 5: evolve loop --resume of a lane checkpoint

**Fact.** `evolve loop --resume` of a lane checkpoint reads as a non-lane. It then lands on `main` at enforce, which skips the driver.

**Decision.** The checkpoint of a lane records its lane identity. The resume path reads it. At `enforce`:

- If the checkpoint holds a lane identity, `evolve loop --resume` takes the lane identity from the checkpoint. The lane keeps its scope. It gets the current pool id. It writes a record and stops before `main`.
- If the pool of the checkpoint has ended or its epoch is stale, the resume refuses with a typed error that names the checkpoint.
- A checkpoint with no lane identity is a legacy file. At enforce, the resume refuses.

No resume path calls the landing code of a non-lane at `enforce`. The check sits in the ship step, where all paths meet, so a new resume path cannot skip it.

**Consequence.** A resumed lane cannot land on `main` at enforce.

## 10. Delivery is write-once

**Fact.** At shadow, `worktreeShip.deliver` (in `deliverable.go`) writes the record and returns `false`, so ship goes on to land. Each ship retry (HEAD_MOVED or FLEET_REBASE_NEEDED) rebuilds the record with new commit SHAs and new patch ids. The driver does not admit a redelivery of a member that is not pending. It records "delivered again under new patches" as a failure. A failure makes a wave `not clean` (parent spec §7).

**Evidence.** Every pool record so far is not clean only by redelivery:

- Pool 2373, wave 172: "cycle 2374 was delivered again under new patches after it was composed".
- Pool 2378, wave 173: cycles 2379, 2382, 2383 and 2386.

*Implication:* a wave with any ship retry after delivery is `not clean` by construction. Lane quality alone cannot meet the gate of two clean shadow waves.

**Why a redesign.** Four review rounds blocked the first designs of this section. Each design let a lane deliver again. Each accommodation (recompose on `Heads`, a superseded outcome) opened a new hole. Old patches landed, A' was stranded at wave end, and a race ran between `recheck` and `FastForward`.

The rule is: after three blocks, stop fix rounds and redesign at the source. The source is that a record can change after the first write.

**Decision.** At enforce, a lane ship writes the record of a member once and stops before `main`. The record is immutable after that write. Four rows decide a second delivery:

| Row | Case | Rule | Test |
|---|---|---|---|
| R-a | Same patch ids as the record file, in any status. | A no-op. No write. No failure. The no-op still moves the lane ref. This covers a retry after `moveLaneRef` fails and a `--resume` of the same build. | Deliver twice. Assert one record mtime, an empty `Failures` and no `*.tmp` file in the wave directory. |
| R-b | Different patch ids, in any member status (`pending`, `composed`, `merged`, `ejected`), at enforce. | `WriteRecord` refuses with a typed error that names the member and its status. No record write. The lane ends and the item state does not change. `main` never sees A'. | For each status, deliver A, then A'. Assert the refusal, the unchanged record and that `Merger.Land` saw only A. |
| R-c | A member that is `merged`. | It lands at most once. A second delivery of any kind never reaches `Main.FastForward`. | Merge A. Deliver again with any patches. Assert that `Main.FastForward` was called once. |
| R-d | At shadow, a second delivery with new patches, for a member that is not `pending` (the guard of `noteRedelivered`). | The lane lands on its own at shadow. The driver stores the new patch ids as `Redelivered` on the member. This is information, not a failure, and it does not count against `clean`. The shadow comparison uses the patches that reached `main`. | Deliver A, land, deliver A'. Assert no failure, a `Redelivered` entry with the A' ids and a `clean` wave. For a `pending` member, `admits` keeps today's re-admit and replaces its `PatchIDs` (row 10e). |

**The only path to new patches.** If the first patches conflict at compose, or fail the batch CI, the existing eject path applies. `Merger.Eject` requeues the item with its brief. A new cycle builds the item fresh. No other path gives a member new patches at enforce.

**Write-once placement.** The invariant lives in the one writer, `integration.WriteRecord`. The check reads the record file, not the wave member. A member exists only after the driver admits it on a later tick. A check on the member misses a second delivery before the admit. The exclusive create closes that race: a concurrent second writer fails and the first record stays.

**The seam: a write policy.** The publish step varies on one axis: replace the file, or create it once. `integration` exports a typed `WritePolicy` with two values, `Replace` and `WriteOnce`. `recordFile` takes the policy. `publish` applies it.

`Replace` publishes with `os.Rename`. `WriteOnce` publishes with `os.Link` and the patch id compare, and `WriteRecord` takes no bool.

The ship composition chooses the policy from `Integration.Stage`. This is policy data, not a flag:

| Case | Policy |
|---|---|
| A lane record at stage `enforce` | `WriteOnce` |
| Every other case | `Replace` |

`Train.Record` keeps `Replace`. That covers the console PR record and its re-record after a rebase. `Train` tells the author to record the review again. The shadow lane keeps `Replace` too. At shadow, a second delivery with new patches must reach the disk. The driver reads the new record for R-d and for row 10e.

Each caller gets the rule from the policy that its composition passes, so no list of callers is needed.

**Lane Heads link.** The record is write-once, so a fault is: the patch ids of the lane differ from the record. The lane `Heads` ejects that member, and the brief names the fault (§2). No recompose runs, and no second record can arrive between `recheck` and `Main.FastForward`.

**Guard.** A table test keeps the redesign in place. It lists every ship path that can call `deliver`, and it asserts at enforce:

| Path | Same patches | New patches |
|---|---|---|
| first ship | n/a (first write) | n/a (first write) |
| ship retry HEAD_MOVED | no-op | typed refusal, `main` unchanged |
| FLEET_REBASE_NEEDED | no-op | typed refusal, `main` unchanged |
| `moveLaneRef` failure, then retry | no-op, lane ref moves | typed refusal, `main` unchanged |
| `evolve loop --resume` | no-op | typed refusal, `main` unchanged |
| exclusive create under a concurrent second writer | the second fails, the first record stays | the second fails, the first record stays |
| driver-admit race: a second delivery before the driver admits the member | no-op | typed refusal, the record file is unchanged |

Mutation: use `os.Rename` in place of `os.Link` under `WriteOnce`, and the guard goes red. The `Replace` policy keeps `os.Rename`, and this mutation does not touch it.

**Shadow prerequisite.** The shadow fix (R-a and R-d) is part 0 of the split (§13). It ranks first, before part 1. Without it, I9 can never be reached.

**`Ended` is not a prerequisite.** A review claimed that the pool 2378 record had no `Ended` because the `finishWave` path failed. That premise is wrong. Wave 173 was still running, so the record had no `Ended` yet.

Pool 2373 (wave 172) has `ended=true` and `last_cycle` 2377. So the `Driver.finishWave` path is verified on pool 2373. No item for `Ended` is needed.

**Consequence.** A redelivery is safe at both stages. At shadow, it is information. At enforce, it is a no-op or a refusal, and `main` sees one set of patches per member.

## 11. The test journey

A real git repo and a bare origin are the fixture. The CI port is a fake CI that returns green or red as set.

| Step | Setup | Expected |
|---|---|---|
| 1 | Two members, A and B. Both deliver with the item state `delivered`. | A second lane started on A or B is refused by the menu. The stale-claim sweep does not release them. |
| 2 | A touches `a.txt`. B and a console commit both touch `b.txt`. | `Flush` composes A onto the live head. B conflicts. |
| 3 | The fake CI returns green for the batch of A. | `Main.FastForward` lands A by fast-forward on the bare origin. `main` equals the batch tip. |
| 4 | B is ejected. | `WriteBrief` writes the brief. `Merger.Eject` returns B to the queue with `BriefPath`. The failure count of B is 1. |
| 5 | The fake `Main.Head` moves between `test` and `fastForward`. | `Merger.Land` composes again and lands within `MaxMoves`. |
| 5b | `Main.Head` moves `MaxMoves`+1 times. | `Merger.Land` returns the error of the `MaxMoves`+1 loop. The member stays `pending`. `_DRIVER_FAILED` is raised. |
| 6 | The plane has a local bookkeeping commit. | The F12 flush runs. Plane `main` equals `origin/main`. Zero-ship reads 1. |
| 7 | `evolve loop --resume` of a lane checkpoint at `enforce`. | The lane keeps its identity and writes a record. A stale-epoch checkpoint refuses. The tip of `main` is unchanged. |
| 8 | A batch is open at the boundary. | The batch continues in the next wave. No second lane starts on a delivered item. |
| 9 | Drive a wave to its end with each actor. | `Driver` writes `Ended` and `LastCycle` for `ShadowActor` and for `EnforceActor`. The actor writes neither. |
| 10 | Rows R-a to R-d of §10. | Each row meets its expected result. |
| 10b | The guard table of §10: every ship path and both race rows, at enforce. | Same patches give a no-op. New patches give a typed refusal and `main` is unchanged. |
| 10c | The patch ids of the lane of a pending member differ from the record. | The lane `Heads` ejects the member. The brief names the fault. |
| 10d | `moveLaneRef` fails. Ship retries with the same patches (a new SHA). Then `Flush` runs. | The member lands. Nothing is ejected. |
| 10e | At shadow, a pending member is delivered again before compose. | The member is composed with the new patch ids. The wave stays clean. |
| 10f | The lane ref of a pending member is gone. | The member composes from the record. Nothing is ejected. |
| 10g | A console PR is recorded twice. | The second record replaces the first. |
| 10h | At shadow, a second delivery with new patches. | The record is rewritten, and R-d or row 10e follows. |
| 11 | A lane has one deferred id and one marker id. It merges at enforce. | The marker id is promoted. The deferred id stays open. |

Each row is a red test first. Mutations: remove the `delivered` skip, remove the patch id compare in `WriteRecord`, swap the fake CI verdict, and remove the status check of R-c. One more: replace `os.Link` with `os.Rename` under `WriteOnce`. That mutation turns row 10b red, because a new patch set reaches the record.

## 12. Rollout

**Fact.** The first live enforce run is at I9, the operator's flip. The operator sets two rules: two clean shadow waves first, and the first enforce wave runs alone. No field, check or `status` line counts clean shadow waves today.

**Decision.**

| Id | Rule | Acceptance row |
|---|---|---|
| H6.1 | `evolve integration status` prints `enforce ready: N/2 clean shadow waves (pools ...)`. N counts pools with the `clean` verdict of the parent spec §7. | A fixture with one clean and one unclean record prints `1/2` and the clean pool id. |
| H6.2 | `evolve stage flip` refuses `integration.stage` `enforce` while N < 2. The count is the `Promotion` data of the `integration` registry entry. The driver gate reads the same entry. | With N = 1, the flip and the driver gate refuse `enforce` with a typed error that names N. With N = 2 they accept. |
| H6.3 | The first enforce pool runs as the only pool. A later pool waits until the first enforce pool ends. | With one enforce pool open, a second pool start is held. After `Ended`, it starts. |

**Evidence integrity.** The wave record is the evidence of the train. The count N of H6.1 reads only pool records whose digest is committed to the hash chain. A pool record that is missing or edited fails the count. It never counts as clean.

The enforce flip writes a verified flip record. The general rule is in [shadow-stage-contract](shadow-stage-contract.md) (rules E1-E4 and C1-C3). This card does not repeat it.

| Id | Rule | Acceptance row |
|---|---|---|
| H6.4 | N counts only chained pool records. | A fixture with one clean pool whose record was edited prints `0/2`, and the gate refuses. |
| H6.5 | The flip writes a verified flip record (C2 of [shadow-stage-contract](shadow-stage-contract.md)). | After the flip, the record exists and its digest verifies in the chain. |

**Consequence.** The flip is a data change that the gate checks. The operator sees the count before the flip.

## 13. Limits and the PR (c) split

- The fixture soak in `cmd_fleet_soak_pool.go` is not read for this card. PR (c) reads it first.
- The cost of `BatchTest` at enforce is not measured.
- The list of lane post-landing work that enforce skips (the landing intent code) is for PR (c) to trace by symbol.
- `Composer.PrefixCheck` in `prefix.go` has no production caller. Only `prefix_test.go` and `batchtest_test.go` call it. It is dead code from before `Merger`, and part 6 deletes it. `Recomposer` is shelved in the parent spec. `CheckLanded` has one home.

PR (c) splits into parts 0 to 8. Each part has its own red test. Part 0 ranks first.

0. the shadow prerequisite: R-a and R-d, the `Redelivered` field and its readers (§1, §10);
1. the `delivered` state and its reader table;
2. the `EnforceActor` and its ports;
3. the `PlaneSync` port and `syncToOrigin` in `cmd`;
4. the `Promoter` port;
5. the checkpoint identity;
6. the removal of `PrefixCheck`;
7. write-once delivery at enforce: the `WritePolicy` strategy, R-a to R-c in `WriteRecord`, the `admits` defence and the guard table (§10);
8. the rollout gate: H6.1 to H6.5 (§12).
