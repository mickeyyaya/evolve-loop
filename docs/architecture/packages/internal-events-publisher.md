# internal/events/publisher

## Purpose

`internal/events/publisher` is the Signal Center listener that appends each event to the event channels whose route matches it. It is component E6 of [ADR-0127](../adr/0127-push-only-event-channels.md). The spec of record is [event-channels.md](../event-channels.md) §1, §3, §5 and §6, and the plan is [event-notification-protocol-2026-10.md](../../plans/event-notification-protocol-2026-10.md) §7 and §10.

The package is unwired. No production code calls it yet. Component E10 subscribes it at each root through `newRootSignalCenter(role)`, and component E7 gives it the channel catalog and the queue keys from `policy.json`.

The package uses two other packages:

- `internal/events/channel` (E2) appends the records under the channel lock with a deadline ([internal-events-channel.md](internal-events-channel.md)).
- `internal/events/filter` (E4) parses each route once against `filter.RegisteredCatalog()` and matches each event ([internal-events-filter.md](internal-events-filter.md)).

## API

| Export | What it does |
|---|---|
| `New(Config) (*Publisher, error)` | parses the routes, opens the channel logs and starts one writer goroutine for each `best_effort` channel. It reads no environment. |
| `Config` | `Root` (the `ch` directory), `Role` (the record `source`), `Dispatch` (the `EVOLVE_DISPATCH_ID` that E10 reads) and `Channels`. Also `Log` (the default segment size and the lock deadline), `QueueEvents`, `QueueBytes` and `EnqueueDeadline`. |
| `Channel` | `Name`, `Route` (a filter expression; empty selects every event), `QoS` and `SegmentBytes` (0 uses `Config.Log.SegmentBytes`) |
| `Roles() []string` | the closed set of roles of the spec §1: `cycle`, `loop`, `loop-chain`, `ship`, `subagent`, `simulate`, `phase-observer`, `wave`, `inbox`, `ci` and `subscriber` |
| `QoS`, `QoSLossless`, `QoSBestEffort` | the two QoS classes of §6 |
| `(*Publisher).Listen(signalcenter.Event)` | the Center listener: `center.Subscribe(p.Listen)` |
| `(*Publisher).Close(deadline) int` | drains the queues and writes the pending gap records until the deadline. It returns the count of the losses that no gap record names. |
| `MaxDispatchBytes` | 256: the longest `dispatch` stamp |

`New` refuses these configs with an error that names the channel:

- a route that does not parse (it wraps `filter.ErrUsage` or `filter.ErrRefused`);
- a QoS that is not `lossless` or `best_effort`;
- a channel name that `channel.New` refuses, or a name that is in the list two times;
- a role that is not in `Roles()`. This also refuses an empty role and the reader role `watch`, and it keeps `source` short;
- a `QueueEvents` or a `QueueBytes` below 1.

## Design

- **Observe, never decide.** The Publisher has no Center. It never emits and never calls `Flush` (D3). A loss is a gap record in the channel, not a signal.
- **The record.** Each record is `{source: role, dispatch: id, signal: event}`. A gap record has the same `source` and `dispatch`. An empty `Config.Dispatch` writes no `dispatch` key. The stamp is cut to 256 bytes at a rune boundary.
- **Routes.** `Listen` matches the event against each route with the `source` of the root, so a route can name `source=loop`. One event goes to each channel whose route matches it.
- **`lossless`.** `Listen` appends the record inside the Center drain, in one locked write. It waits at most the lock deadline. A second append of the same channel in this process waits for the first one. Thus the channel never sees two lock calls of one process.
- **`best_effort`.** `Listen` puts the record on a bounded queue and returns. The queue holds at most `QueueEvents` records and `QueueBytes` bytes of record lines. A full queue waits up to `EnqueueDeadline` for space; with 0 it never waits. Each put sends on a wake channel, and the writer goroutine takes the whole queue and appends it as one batch. Nothing polls.

  A take sends one space signal. A put that leaves space sends it again, so each waiting put gets the space. A closed queue refuses a put at once and never waits. A record that does not encode is a `write_error` loss, so the byte count never misses it.
- **A loss.** These results are a counted loss:
  - a lock deadline or `ErrLockPending` (reason `lock_deadline`);
  - any other append error (reason `write_error`);
  - a full or closed queue (reason `queue_full`);
  - a record that does not encode for the byte count (reason `write_error`).

  `ErrRotate` is not a loss, because the batch is on disk.
- **The gap record.** The route keeps one run for each reason, `pid` and contiguous `seq` range. Two losses join only when their `seq` values touch. Thus `first_seq`, `last_seq` and `dropped` always agree, and the result does not depend on the order of the losses. The runs are in `pid` and `first_seq` order.

  The next append of the channel writes the gap records first, in the same write as its events. If that append fails, the runs stay and grow.

  The severity is `INCIDENT` on a `lossless` channel and `WARN` on a `best_effort` channel. This is the pattern of `ndjsonSink.reportDrops`, with a record in place of a signal.
- **Two locks for each route.** One mutex orders the appends of the route. A second mutex keeps the loss counts. Thus a full queue counts its loss at once, also while the writer waits for the channel lock.
- **`Close(deadline)`.** It stops each writer. A writer appends the queue in batches until the queue is empty or the deadline passes. It does not start a batch after the deadline. An append that started before the deadline ends within the lock deadline. The records left in the queue are a `queue_full` loss.

  Then `Close` writes the pending gap records of each channel if the deadline has not passed. It returns the sum of the losses that are still pending. A second `Close` tries the pending gap records again. An event after `Close` is a `lossless` append, or a `queue_full` loss on a `best_effort` channel.

## Invariants

- **One event goes to each matching channel, once.** Pinned by `TestPublisher_RoutesOneEventToEveryMatchingChannel` and `TestPublisher_TheRouteSeesTheSourceOfTheRoot`.
- **A lossless event is on disk when `Flush` returns.** Pinned by `TestPublisher_LosslessEventIsOnDiskWhenFlushReturns`.
- **Each loss is counted, and the next good append writes one gap for each reason and contiguous `seq` run.** Pinned by `TestPublisher_AWriteErrorIsACountedLossWithAnIncidentGap`, `TestPublisher_APendingLockIsALockDeadlineLoss`, `TestPublisher_EachReasonAndSeqRunGetsItsOwnGap`, `TestMergeLosses_OneGapForEachContiguousSeqRunInAnyOrder`, `TestPublisher_ARecordThatDoesNotEncodeIsAWriteErrorLoss`, `TestPublisher_AFailedGapStaysPendingWithTheNewLoss`, `TestPublisher_QueueOverflowWritesOneGapWithTheExactSeqRange`, `TestPublisher_QueueBytesBoundTheQueue`, `TestPublisher_AnEnqueueDeadlineThatPassesIsAQueueFullLoss` and `TestPublisher_ABestEffortWriteErrorIsAWarnGap`.
- **A rotation error is not a loss.** Pinned by `TestPublisher_ARotationErrorIsNotALoss`.
- **A held lock never holds the drain past the lock deadline, and one lock call waits for each channel.** Pinned by `TestPublisher_ALockPastItsDeadlineNeverHoldsTheDrain` and `TestPublisher_OneLockCallWaitsForEachChannel`. These tests hold a real `flock` and carry the `integration` tag.
- **A slow channel never delays the emitter or another channel.** Pinned by `TestPublisher_ABlockedBestEffortChannelNeverDelaysTheEmitterOrAnotherChannel`, `TestPublisher_ASlowReaderNeverDelaysAnAppend` and `TestPublisher_ConcurrentEnqueuersShareTheSpaceOfOneTake`.
- **`Close` drains before the deadline and counts what is left.** Pinned by `TestPublisher_CloseDrainsTheQueueBeforeTheDeadline`, `TestPublisher_CloseCountsWhatIsLeftAtTheDeadline`, `TestPublisher_CloseUnderAHeldLockReturnsTheCount`, `TestPublisher_CloseWritesAPendingGap`, `TestPublisher_CloseReturnsTheCountOfAGapThatDidNotGoOut`, `TestPublisher_CloseAfterTheDeadlineDoesNotTryTheGap`, `TestPublisher_AnEventAfterCloseIsACountedLoss`, `TestPublisher_AListenAfterCloseNeverWaitsTheEnqueueDeadline` and `TestPublisher_CloseTwiceIsSafe`.
- **The roles are a closed set, and a channel can set its own segment size.** Pinned by `TestRoles_IsTheClosedSetOfTheSpecRoots`, `TestNew_RefusesABadConfig` and `TestPublisher_AChannelSegmentSizeOverridesTheDefault`.
- **The dispatch stamp comes from the config and is cut to 256 bytes.** Pinned by `TestPublisher_StampsTheDispatchIDOfItsProcess`, `TestPublisher_AProcessWithNoDispatchIDHasNoDispatchKey` and `TestPublisher_CutsALongDispatchStampTo256Bytes`.
- **The listener never panics and never emits.** Pinned by `TestPublisher_ListenNeverPanicsAndNeverEmits`.
- **Mutation record (2026-10-09).** The mutants of each production branch are killed. The list is in the lane report.

## Findings

- **More than one gap.** The spec says that the next good batch starts with one gap record. When losses of two reasons, or two `seq` runs, are pending, the Publisher writes one gap record for each run. One record can name a wrong reason or count events that are on disk.
- **The reason of a queue left at `Close`.** The spec says that what is left is a counted loss, but it names no reason. The Publisher uses `queue_full`.
- **Route warnings.** `filter.Parse` warns on an unknown code. The Publisher drops the warning; component E7 shows it at policy load.
- **Two encodes of a best-effort record.** The queue encodes a record to count its bytes, and `channel.Append` encodes it again. One encode needs a `channel` API that takes encoded lines and keeps its checks, so this lane keeps two encodes. The cost is small at the measured load of about 2,000 events each day.
- **The drain bound and many Centers.** The bound is k × the lock deadline for k matching lossless channels (spec §5). Two Publishers in one process can have two waiting lock calls for one channel (spec §Limits).
