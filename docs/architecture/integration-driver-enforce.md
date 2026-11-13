# Integration driver: the EnforceActor card

> Parent spec: [integration-driver](integration-driver.md) §9. Package note: [internal-integration](packages/internal-integration.md).
> **Status:** Proposed (2026-11-14), a console proposal for PR (c). This card has no code. The code follows in a later cycle, after the card passes review.
> Decision record: [solutions/wave-integration-i14-driver-pr-c](../../solutions/wave-integration-i14-driver-pr-c/recommendation.md).

**Terms used throughout:**
- **EnforceActor:** the `Actor` that lands a pool at stage `enforce`. PR (b) has only `ShadowActor`.
- **Delivered:** a lane shipped its patch to a wave record. The patch is not yet on `main`.
- **Lane identity:** the pool id, the lane cycle number and the scope of one fleet lane.
- **Brief:** the file that `Ejection.WriteBrief` writes. It tells the next lane why a patch conflicted.
- **Fake CI:** a test double of the `CI` port that returns a set verdict.
- **Bare origin:** a bare git repository that serves as `origin` in a test.

## 1. Goal and scope

At `enforce`, a lane ship writes its record and stops before `main`. Nothing lands it ([integration-driver](integration-driver.md) §2). The `EnforceActor` lands it.

The card decides the seven open inputs of §9 of the parent spec. Each decision has a fact, a decision and a consequence. Acceptance of the inbox item `wave-integration-i14-driver-pr-c`:

1. A journey with a fake CI and a bare origin lands one member by fast-forward. It requeues a conflict with its brief.
2. No second lane starts on a delivered item.
3. `evolve loop --resume` of a lane checkpoint cannot land on `main` at enforce.

This card changes no driver code. The code files `driver.go`, `postship.go` and `cmd_loop_wavesync.go` stay as they are.

## 2. EnforceActor

`EnforceActor` implements the `Actor` interface of the parent spec §3. It holds the enforce ports. `Driver` holds none.

```go
type EnforceActor struct {
    Merger   Merger
    Plane    PlaneSync
    Landing  LandingFeed
    Requeue  Requeuer
    Briefs   BriefStore
    Promote  Promoter
}
```

| Method | What it does at enforce |
|---|---|
| `Resume` | Reads the record. Adopts a fence that was pushed with an old record by the discard path. Deletes leaked `integration/p<pool>-*` refs. Never lands. |
| `Flush` | Takes one `PendingBatch`. Calls `Merger.Land` with a lane batcher as `Compose`. Lands by fast-forward through `Main.FastForward`. Writes the outcome into the record. |
| `WaveEnd` | Loops `Flush` until `Hold` is empty. Calls `Plane.Sync` once. Writes `Ended`. A member still `pending` after `MaxMoves` stays pending and raises `_DRIVER_FAILED`. |

The stage table of the parent spec §3 stays the source for the compose, gate and refs steps. This card adds the decisions below.

## 3. F2: the delivered inbox state

**Fact.** Post-ship sees `isLanded` false at a delivered ship (`go/internal/phases/ship/postship.go:153`). It releases the item. A second lane then starts on the same work.

**Decision.** A delivered ship moves the inbox item to a new state `delivered`. The state has these rules:

- The menu builder skips a `delivered` item. So no second lane starts on it.
- The stale-claim sweep does not release a `delivered` item. The sweep reads the state and skips it.
- `EnforceActor` ends the state. On `merged`, it promotes the item (Finding 3). On ejection, it releases the item with the brief (F3).
- A `delivered` item with no member in any wave record is an orphan. The driver reports it once as a failure. It never releases it by itself.

**Consequence.** The state is data on the inbox item. No flag is added. Shadow keeps today's release, because shadow lands nothing.

## 4. F3: the brief reaches the next lane

**Fact.** `inboxbatch.Item` has no brief field. A brief-only continuation is unlikely to work, because readers skip an empty `SnapshotSHA`.

**Decision.**

- `inboxbatch.Item` gets a field `BriefPath`. `inboxBatchesSection` renders it into the prompt of the next lane.
- `Requeue` returns the item to the queue with `BriefPath` set. It goes through `inboxmover`. `Train.Brief` is not used, because it serves a PR author.
- Each ejection adds one to the failure count of the item. The convergence count of ADR-0126 bounds the retries.
- The requeue keeps the original item text. The brief is added, never substituted.

**Consequence.** A conflicted member is requeued with its brief. The next lane reads what conflicted and where.

## 5. F11: the mid-wave sync

**Fact.** `core.FastForwardPlaneMain` does not land, and the plane must follow `origin/main` after a merge.

**Decision.** PR (c) extracts `syncToOrigin(ctx, g, rel, warn) (SyncOutcome, error)`. The stamp step is a strategy parameter.

- Mid-wave, the sync is a plain `merge --ff-only`. No stamp runs.
- The stamp path stays at the boundary.
- A blocked mid-wave sync waits for the boundary. `_MERGED` stays.
- `SyncOutcome` is `Current`, `Ahead`, `Diverged`, `Blocked` or `Moved`. Mid-wave, `Moved` and `Current` proceed.

**Consequence.** A merge never waits for a boundary stamp. A blocked sync never retracts `_MERGED`, because `main` did merge.

## 6. F12: a local bookkeeping commit after a merge

**Fact.** A plane with a local bookkeeping commit after a merge reads `Diverged`. `bookkeeping.go` holds two pure predicates and no flush.

**Decision.** The merge step builds the flush that plan D12 names.

1. Rebase the local-only commits onto the new `origin/main`.
2. Flush them with `evolve ship`, not a bare push.
3. Run `Plane.Sync`.

Any other `Diverged` raises `_DRIVER_FAILED`. The plane sync refuses `Ahead` only after the flush ran.

**Consequence.** Plane `main` equals `origin/main` after each merge. Zero-ship reads 1 in the journey.

## 7. Finding 3: promotion at merged

**Fact.** `rec.Scope` is not the promotion set. A deferred id must stay open. A marker id must be promoted.

**Decision.** Post-ship moves its landed branch into one extracted function, `PromoteLanded`. Two callers use it:

- post-ship, when `isLanded` is true;
- `EnforceActor`, at `merged`.

The function takes the lane result, not `rec.Scope`. The delivery step records the result of the promotion rules in the member, so that `EnforceActor` replays it.

**Consequence.** One home for promotion. A deferred id stays open, and a marker id is promoted, on both paths.

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

## 10. The test journey

A real git repo and a bare origin are the fixture. The CI port is a fake CI that returns green or red as set.

| Step | Setup | Expected |
|---|---|---|
| 1 | Two members, A and B. Both deliver with the item state `delivered`. | A second lane started on A or B is refused by the menu. The stale-claim sweep does not release them. |
| 2 | A touches `a.txt`. B and a console commit both touch `b.txt`. | `Flush` composes A onto the live head. B conflicts. |
| 3 | The fake CI returns green for the batch of A. | `Main.FastForward` lands A by fast-forward on the bare origin. `main` equals the batch tip. |
| 4 | B is ejected. | `WriteBrief` writes the brief. `Requeue` returns B to the queue with `BriefPath`. The failure count of B is 1. |
| 5 | A console merge lands during the watch. | `Watch` returns `Moved`. The batch composes again and lands. |
| 6 | The plane has a local bookkeeping commit. | The F12 flush runs. Plane `main` equals `origin/main`. Zero-ship reads 1. |
| 7 | `evolve loop --resume` of a lane checkpoint at `enforce`. | The lane keeps its identity and writes a record. A stale-epoch checkpoint refuses. The tip of `main` is unchanged. |
| 8 | A batch is open at the boundary. | The batch continues in the next wave. No second lane starts on a delivered item. |

Each row is a red test first. Mutations: remove the `delivered` skip, remove the identity check in ship, and swap the fake CI verdict.

## 11. Limits and the PR (c) split

- The fixture soak in `cmd_fleet_soak_pool.go` is not read for this card. PR (c) reads it first.
- The cost of `BatchTest` at enforce is not measured.
- The first live run is at I9, the operator's flip.
- The list of lane post-landing work that enforce skips (`landing_intent.go:38`) is for PR (c) to trace.

PR (c) splits into six parts. Each part has its own red test:

- the `delivered` state;
- the `EnforceActor` and its ports;
- `syncToOrigin`;
- `PromoteLanded`;
- the checkpoint identity;
- the removal of `PrefixCheck`.
