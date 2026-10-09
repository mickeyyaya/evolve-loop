# internal/events/channel

## Purpose

`internal/events/channel` keeps one event channel as an append-only log on disk. It is component E2 of [ADR-0127](../adr/0127-push-only-event-channels.md). The spec of record is [event-channels.md](../event-channels.md) §3 to §5, and the plan is [event-notification-protocol-2026-10.md](../../plans/event-notification-protocol-2026-10.md) §10.

The package is unwired. No production code calls it yet. The publisher (E6) appends with it, and the reader (E5) reads with it. Component E10 wires them.

The package uses two parts of other packages:

- `signalcenter.ReadLines(path, from)` (E1) reads the complete lines of a file from a byte offset. It does not return a torn tail. `signalcenter.ReadStream` now calls the same reader and gives the same results as before.
- `flock.LockWithin(path, wait, onSettled)` (E2) takes the exclusive lock, or returns `flock.ErrLockDeadline` when the wait ends first. It calls `onSettled` once, when its goroutine is done.

## API

| Export | What it does |
|---|---|
| `New(root, name, Config) (*Log, error)` | makes the log of the channel `name` under the `ch` directory `root`. It refuses a name that is not dot-separated tokens of `[a-z0-9_-]`. |
| `Config` | `SegmentBytes`, the size at which a segment rotates, and `LockDeadline`, the longest wait for the channel lock |
| `(*Log).Append([]Record) (int64, error)` | writes the batch under the channel lock and returns the cursor of its first record |
| `(*Log).Read(from) (Batch, error)` | returns the records from the cursor `from` to the end, with the synthetic gaps of the cursor rules, and the next cursor |
| `(*Log).ReadN(from, maxBytes) (Batch, error)` | the same as `Read`, but the lines of one batch are `maxBytes` or less. A single line that is longer comes alone. `Read` calls it with no limit. |
| `(*Log).Last() (int64, error)` | returns the cursor of the last complete record, for `--since last` |
| `(*Log).End() (int64, error)` | returns the end of the last complete line of the tail, for `--since new` |
| `(*Log).Dir() string`, `(*Log).Segments() ([]Segment, error)` | the channel directory, and its segments in base order with the tail last |
| `Segment`, `SegmentOf([]Segment, cursor) (Segment, bool)` | a segment (`Base`, `Path`), and the segment that holds a cursor: the largest base at or below it |
| `Record`, `Gap`, `Batch` | a stored or synthetic record, a gap, and the result of a read |
| `MaxRecordBytes` | 4,608: the longest record line, without its newline |
| `ErrRecordTooLong` | the refusal of a record line that is longer than `MaxRecordBytes` |
| `ErrInvalidRecord` | the refusal of a record that has not exactly one of `signal` and `gap`, or that has the source `watch` |
| `ErrLockPending` | the refusal of an append while a lock call of this process for the channel still waits. It wraps `flock.ErrLockDeadline`. |
| `ErrRotate` | the batch is on disk, but the next segment was not made. It is not a loss. |
| `SourceWatch` | the source of a synthetic record |
| `ReasonReset`, `ReasonMalformed`, `ReasonRetention`, `ReasonQueueFull`, `ReasonWriteError`, `ReasonLockDeadline` | the closed set of gap reasons: the first three for a reader, the last three for a publisher |

## Design

- **Files.** The segments are `<root>/<name>/seg-<base>.ndjson`. The base is the cursor of the first byte of the segment, with zeros to 20 digits, so a name sort is a number sort. The lock is `<root>/<name>.lock`, beside the channel directory. Thus the lock file never changes the directory that a reader watches.
- **The append.** `Append` encodes the batch first. Before it takes the lock, it refuses an invalid record, an encode error and a record line that is too long. Thus a refused batch writes nothing. Then it does these steps under the lock:
  1. It opens the tail segment, the segment with the highest base, with `O_WRONLY|O_APPEND|O_CREATE`.
  2. It reads the size with `fstat`. If the last byte is not a newline, it puts one newline before the batch. This repairs a line that a crashed writer tore.
  3. It writes the repair and the batch in one `write` call.
  4. If the segment is now larger than `SegmentBytes`, it makes the next segment `seg-<base+size>`. If that fails, `Append` returns the cursor and `ErrRotate`. The next append finds the same tail over the cap and tries again.
- **One waiting lock call.** The log keeps an atomic flag for a lock call that waits. While the flag is set, `Append` fails at once with `ErrLockPending`. It starts no goroutine and opens no descriptor. The settle signal of `LockWithin` clears the flag.
- **The read.** `Read` lists the segments once. It finds the segment of the cursor (the largest base at or below it) and reads each segment to its end with `signalcenter.ReadLines`. The cursor of a record is the base of its segment plus its offset in the segment.
- **The cursor rules.** `Read` gives one synthetic gap record (`source: watch`, with `from` and `to`) for each move of the cursor:
  - a cursor below the oldest segment moves to its base (`retention`);
  - a cursor past the end of a segment that has a later segment moves to the base of that later segment (`reset`);
  - a cursor past the end of the tail moves to the end of the last complete line (`reset`);
  - a cursor in a channel with no segment moves to 0 (`reset`);
  - a complete line that does not parse moves the cursor past the line (`malformed`).
  - a segment that gc removed after the listing moves the cursor to the base of the next segment (`retention`). A removed tail is an error that wraps `fs.ErrNotExist`.
- **A valid record** is a JSON object with exactly one of `signal` and `gap`. The writer refuses the source `watch`.
- **Synthetic records have the cursor 0.** Tell them apart by `Source == SourceWatch`, never by `Cursor`.
- **A missing channel directory** is an error that wraps `fs.ErrNotExist` from `Read`, `ReadN`, `Last`, `End` and `Segments`. The reader (E5) makes the directory again and arms again.
- **Gap `from` and `to`** are always in the JSON, also when they are 0.
- **`Last`** reads the tail segment, and the segment before it when the tail has no complete line. With no complete line in the two, it returns the end of the channel.
- **Errors.** An error names the channel and the step. A lock deadline and `ErrLockPending` wrap `flock.ErrLockDeadline`, so the publisher can count the loss as `lock_deadline`. `ErrRotate` is not a loss. Each other append error is a `write_error`.

## The lock with a deadline

`flock.LockWithin` starts a goroutine that calls the blocking `flock`. The caller waits for the goroutine or for a one-shot timer, whichever comes first. Nothing polls.

- One atomic state has three values: `waiting`, `handed` and `abandoned`. The goroutine changes `waiting` to `handed` with one compare-and-swap. The caller changes `waiting` to `abandoned` at the deadline with one compare-and-swap. Only one swap can win.
- If the goroutine wins, it calls `onSettled`, and then the lock goes to the caller. This is also so when the deadline comes at the same instant.
- If the caller wins, `LockWithin` returns `ErrLockDeadline`. The goroutine then gets the lock later, releases it at once and calls `onSettled`.
- An error of `flock` goes to the caller in the same way.

## Invariants

- **Each line of a batch is whole, and no other writer writes between the lines of a batch.** Pinned by `TestAppend_ConcurrentProcessesWriteUniqueWholeLines`. Four processes each append 120 batches of two records, with rotations between them.
- **The tail opens with `O_APPEND` under the lock.** Pinned by `TestAppend_OpensTheTailWithAppendUnderTheLock`.
- **A torn tail gets one newline before the batch, and an intact tail gets none.** Pinned by `TestAppend_RepairsATornTailBeforeTheBatch` and `TestAppend_AnIntactTailGetsNoRepair`.
- **A segment rotates only when it is larger than the cap, and cursors increase across a rotation.** Pinned by `TestAppend_RotatesPastTheCapAndCursorsKeepIncreasing` and `TestRead_ACursorFindsItsSegmentAcrossRotation`.
- **Each cursor rule gives one gap.** Pinned by `TestRead_ACursorPastTheTailMovesToTheTrueEndWithOneResetGap`, `TestRead_ACursorPastASegmentEndMovesToTheNextBase`, `TestRead_ACursorBelowTheOldestSegmentYieldsARetentionGap`, `TestRead_AChannelWithNoSegmentEndsAtZero` and `TestRead_AMalformedLineYieldsAGapAndTheCursorMovesPastIt`.
- **Exactly one side owns the lock at the deadline.** Pinned by `TestLockWithin_ADeadlineAtTheGrantInstantHasExactlyOneOwner`.
- **An abandoned call releases the lock when it gets it.** Pinned by `TestLockWithin_AnAbandonedCallReleasesTheLockWhenItArrives`.
- **A refused record writes nothing, and one lock call waits for each channel and process.** Pinned by `TestAppend_AnEncodeErrorWritesNothing`, `TestAppend_RefusesARecordThatReadWouldReject` and `TestAppend_OneLockCallWaitsForEachChannelAndProcess`.
- **A rotation error is not a loss.** Pinned by `TestAppend_ARotationErrorKeepsTheBatchAndTheNextAppendRotates`.
- **A bounded read keeps the cursors whole.** Pinned by `TestReadN_ABatchStaysWithinMaxBytesAndCursorsContinue`.
- **Mutation record (2026-10-09).** The mutants of each change are killed, except the equivalent ones. The list is in the lane report.

## Findings

- **Two cursor rules are now in the spec.** A reader at the end of a segment whose end is below the next base moves to that base with a `reset` gap. `--since last` reads the segment before the tail when the tail has no complete line ([event-channels.md](../event-channels.md) §4).
