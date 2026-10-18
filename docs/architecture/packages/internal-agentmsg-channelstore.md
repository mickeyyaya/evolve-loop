# internal/agentmsg/channelstore

> The spec of record: [agent-messaging.md](../agent-messaging.md) §7. The plan (component M2, decision D7): [agent-messaging-2026-10.md](../../plans/agent-messaging-2026-10.md). The decision: [ADR-0129](../adr/0129-cycle-scoped-agent-messaging.md). The substrate: [internal-events-channel.md](internal-events-channel.md).

## Purpose

`internal/agentmsg/channelstore` is the adapter of the `broker.Store` port on the ADR-0127 channel log. It is component M2 of ADR-0129. It stores the messages of cycle N in the channel `msg.N`. It stores the dead letters of cycle N in the channel `msg.dead.N`.

The package is unwired. No production code constructs it yet. Component M7 wires it. The in-memory adapter `memstore` stays the adapter of the unit tests.

The package uses `internal/events/channel`: `New`, `Log.Append`, `Log.Read`, `Log.Segments` and the `message` variant of `Record`.

## API

| Export | What it does |
|---|---|
| `New(root, channel.Config) (*Store, error)` | makes a store over the `ch` directory `root` (`.evolve/events/ch` in production). It refuses an empty root, a segment size below 1 and a lock deadline of 0 or less with `ErrConfig`. |
| `(*Store).Append(ctx, cycle, Record) (int64, error)` | appends one `message` record to `msg.<cycle>` and returns its cursor |
| `(*Store).Read(ctx, cycle, from) (Batch, error)` | returns the records from the cursor, the gaps of the log and the next cursor |
| `(*Store).DeadLetter(ctx, cycle, Dead) error` | appends one `message` record to `msg.dead.<cycle>` |
| `ErrConfig` | the refusal of an incomplete config |
| `ErrCursor` | the refusal of a negative cursor |

## Design

- **The record.** The record is the line of the ADR-0127 log with the `message` variant:

```json
{"source":"msg","dispatch":"r/5/scout/p1n1","message":{"envelope":{…},"recipients":[…]}}
```

  - `source` is `msg`. `dispatch` is the dispatch of the sender.
  - A dead letter has the same form. Its `message` is `{"envelope":…,"recipient":…,"reason":"expired"}`.
- **The channel package does not know the shape of a message.** `channel.Record.Message` is raw JSON. The ports `broker.Record` and `broker.Dead` already carry the JSON tags, so the adapter marshals them as they are.
  - Rejected option: a typed `channel.Message` with the envelope and the principal. It makes the log import the `agentmsg` leaf packages, against the dependency direction of the hexagon.
- **Cursors are byte offsets.** `memstore` counts records, and this adapter counts bytes of the log. The port promises only the order and that `Next` is above the cursor of the last record. Do not compare a cursor of one adapter with a cursor of the other.
- **The map from a batch.** `Read` converts each channel record in order:
  1. A synthetic gap record (`retention`, `reset` or `malformed`) becomes a `broker.Gap` with the same `from`, `to` and reason.
  2. A record whose `message` decodes becomes a `broker.Record` with its cursor.
  3. Any other record becomes a `malformed` gap from its cursor to the start of the next record or to `Next`.
- **Why the span of a gap matters.** The broker names a gap `gap.<from>.<to>` and reports it once. Thus each lost record needs its own span. The span also tells an operator how many bytes the log lost.
- **A cursor past the head.** The channel log answers with one `reset` gap and moves to its end. The broker refuses such a cursor with `ErrCursor` before it reads the gaps, so the gap never reaches a mailbox.
- **A channel that does not exist** reads as an empty batch with `Next` 0, as in `memstore`. The read makes no directory. The adapter treats a read error as a missing channel only if `Segments` also reports that the directory is gone. A lost tail segment is an error, never an empty channel.
- **A rotation error is not a loss.** When `Log.Append` returns `channel.ErrRotate`, the record is on disk. `Append` returns the cursor and no error. The publisher (`written` in `route.go`) does the same.
- **A dead letter has a channel for each cycle.** `DeadLetter` writes to `msg.dead.<cycle>`. Thus two cycles never share a directory, and two brokers never contend for one channel lock (`channel.ErrLockPending`).
  - Rejected option: one channel `msg.dead` for all cycles. It mixes the cycles on disk, and one lock serves all brokers.
- **One log for each channel.** The store opens a `channel.Log` once for each channel and keeps it. `channel.Log` allows one waiting lock call for each process. A new log for each call breaks that rule.
- **One writer at a time.** The broker holds its lock across `Append`, so only one append waits for the channel lock. A second append from the same process, at the same time, fails at once with `channel.ErrLockPending`.
- **The size of a message line.** The signal line limit of the log is 4,608 bytes. A body of the default cap (8,192 bytes) does not fit in it. A message line has its own limit, `channel.MaxMessageBytes`, of 64 KiB: six times the default body cap, with room for the envelope. A message over the limit fails the send with `channel.ErrRecordTooLong`.
- **The config check.** A zero `channel.Config` rotates the segment after every record and gives the lock no wait. `New` refuses it.

## Invariants

- **A message is one `message` record in the channel of its cycle, and a dead letter is one in `msg.dead.<cycle>`.** Pinned by `TestChannelStore_AppendsAMessageRecordToTheCycleChannel` and `TestChannelStore_DeadLettersGoToMsgDead`.
- **A record holds exactly one of `signal`, `gap` and `message`.** Pinned by `TestRecord_HoldsExactlyOneOfSignalGapAndMessage`, in `internal/events/channel`.
- **A read crosses segments, and a cursor from `Append` finds its record.** Pinned by `TestChannelStore_ReadFollowsRotation` and `TestChannelStore_ReadFromACursorReturnsTheRestAndTheNextCursor`.
- **A gap is never silent.** Pinned by `TestChannelStore_ARetentionGapIsAGapOfTheBatchAndNotARecord` and `TestChannelStore_AForeignOrBrokenRecordIsAMalformedGapWithItsSpan`.
- **Each cycle has its own channel and its own cursors.** Pinned by `TestChannelStore_EachCycleHasItsOwnChannelAndIncreasingCursors`.
- **The default body cap fits one record.** Pinned by `TestChannelStore_ABodyOfTheDefaultCapWithEveryByteEscapedFits` and `TestAppend_AMessageRecordHasItsOwnLineLimit`.
- **The real broker works over the adapter.** Pinned by `TestChannelStore_ServesTheBrokerSendAndRead` and `TestChannelStore_AnExpiredMessageGoesToMsgDeadOnce`.
- **A refused record writes nothing.** Pinned by `TestChannelStore_ARecordThatCannotBeEncodedIsRefusedAndWritesNothing`, `TestChannelStore_AMessageLongerThanTheMessageLimitFailsAndIsNotStored` and `TestChannelStore_ACanceledContextFailsEachCallAndStoresNothing`.
- **A cursor from `Append` is the start of its record.** An ack at that cursor neither skips the record nor delivers an earlier one again. Pinned by `TestChannelStore_ACursorFromAppendIsTheStartOfItsRecord`.
- **Dead letters are separate for each cycle.** Pinned by `TestChannelStore_DeadLettersGoToMsgDead`, which reads `msg.dead.5` and `msg.dead.6` and finds no shared `msg.dead`.
- **No poll.** `TestNoPollTimerInTheEventChannelSources` scans `internal/agentmsg`, and this package uses no timer.

## Findings

- **The signal line limit did not fit the default body cap.** `channel.MaxRecordBytes` is 4,608 bytes. A body of 8,192 bytes of plain text is already longer than that. A body of 8,192 bytes of `<` escapes to six bytes for each byte, which is 49,152 bytes. Without a message limit, the adapter refuses such a send. The message limit is the fix.
- **No shared contract suite.** The plan asked for the `memstore` cases as a shared suite if the code allows it. It does not, for two reasons:
  - The cursors differ (a record index against a byte offset), so a shared case cannot assert a cursor value.
  - A suite package with `t.Fatalf` branches cannot reach 100% line coverage without a fake `testing.TB`.
  - The adapter test mirrors the `memstore` cases by name, and it runs the real broker over the adapter instead.
- **Mutants.** 43 mutants ran with `go test -overlay`: 29 of the adapter, 12 of the record change and 2 of the reader. A named test kills each one. The unmutated control passes.
- **An ack reads the log.** `Service.Ack` calls `Read` to check that the cursor is inside the log. Thus each ack reads the log from the cursor to its end. The cycle budget (200 messages by default) bounds that cost.
- **The events reader knows a message record.** `Item.ID` in `internal/events/reader/item.go` gives a message record the id `msg.<channel>.<cursor>`, not a gap id. `placeWill` skips a record with no signal and no gap. A filter passes a message record and never ends a watch on it.
