# internal/events/reader

> Spec of record: [event-channels.md](../event-channels.md) §3 (records), §4 (cursors), §7 (the wake and the last will) and §2 (the selectors). Plan: [event-notification-protocol-2026-10.md](../../plans/event-notification-protocol-2026-10.md), component E5. Decision: [ADR-0127](../adr/0127-push-only-event-channels.md).

## Purpose

`internal/events/reader` follows one or more event channels without a poll. It owns:

- the reader (`Reader`, `New`, `Next`);
- its configuration (`Config`, `Channel`) and its ports (`Ports`, `Waiter`);
- the start of a read (`Since`, `ParseSince`);
- the result of a read (`Item`, `Item.ID`) and the two ends of a call (`ErrDeadline`, `ErrHangup`).

Nothing in production calls the package yet. The CLI (E9) and the subscriptions (E8) use it, and E10 wires them.

## Design

- **Arm, catch up, wait.** The first `Next` makes the channel directories and resolves the selectors. It computes the start cursors, arms the waiter and runs the seed scan of the last will. Each `Next` then catches up every channel. It returns the records that it found. If it found none, it waits, and after the wake it catches up again.
- **One read of each channel for each catch-up.** A read is `channel.ReadN` with a budget of 1 MiB. A full read returns its records at once. The next `Next` reads again before it waits, so a channel is caught up only when a read finds nothing new.
- **The watch set follows the channels.** Before each catch-up, the reader computes the targets: the channel directories, the tail segment of each channel and the pids of the live loops. If the targets differ from the armed set, the reader arms again. This covers a rotation, a new channel, a new live loop and a lost loop. A `Changed` wake always arms again, because a wiped and new directory has the same path but a new inode.
- **Ports.** `Waiter` is the consumer-side interface of `wake.Waiter` (`Arm`, `Wait`). `Ports.StartOf` is `proctree.StartOf`, and `Ports.Now` is the clock. Tests give fakes, so no unit test sleeps.
- **No timer.** The reader never makes a timer. It gives the deadline of `Next` to `Wait` without a change. A zero deadline means no timeout. The clock is read only for the time of a `loop.lost` record.
- **The ends of a call.** A `Deadline` wake returns `ErrDeadline`. A `Hangup` wake returns `ErrHangup`, and it is terminal (exit 3 in the CLI). An error of `Arm`, `Wait`, a read, a directory or `StartOf` is returned.
- **The cursor rules.** `channel.Read` applies them, and the reader passes each synthetic gap through: `retention`, `reset` and `malformed`. A missing channel directory is not an error. The reader makes the directory again, arms again and reads from its cursor. The read then gives a `reset` gap to cursor 0.
- **The contract of `Next`.** `Next` returns items or an error, never both.
  - A catch-up reads every channel first, and it moves the cursors only when all reads are good. Thus a failed read moves no cursor and loses no item. The next `Next` reads the same records again, and a `loop.lost` of the failed catch-up comes again.
  - When a catch-up finds items, `Next` returns them at once. The next `Next` arms a changed watch set before its read.
- **`Item.Next`** is the channel cursor after the item. It is the cursor of the next stored record, the `from` of the next synthetic gap, or the end of the read. A consumer stores it to ack the item. For a `loop.lost` record, `Next` is the current cursor of its channel, so an ack of it moves nothing.
- **`Item.ID`** is `<pid>.<seq>.<ts>` for a signal, `gap.<channel>.<cursor>` for a stored gap and `gap.<channel>.<from>` for a synthetic gap (§3).
- **The merge (the decided rule).** The reader merges the reads of one catch-up by `signal.ts`, with one head for each channel. It never changes the order of one channel, so the cursors of a channel stay in order. A gap comes before a signal, and a signal with a time that does not parse comes last.
  - The merge applies inside one batch only. Two batches are not in time order across channels, so the result is not a total order.
  - The synthetic records of the last will come after the records of their batch. Thus the `Next` of a `loop.lost` is never before an item that the consumer did not get.
  - The reader does not use `signalcenter.MergeByTS`. That function sorts all events together. The times in one channel are not in order, because two processes append to it. A sort can thus put a later cursor of a channel before an earlier one. An ack of the later one then skips the earlier one.
- **The duplicate window.** The reader keeps the last 4,096 keys of the signals that it returned (D26). The key is the `source` and the event id. One process can hold two Centers, each with its own `seq` (the E6 note), so the same id from another source is another event. A gap is never removed.
- **Selectors.** The selectors are resolved against the catalog. A selector with `*` or `>` also watches the channel root. At each catch-up, the reader then resolves the selectors again against the catalog and the channel directories. A new channel starts at cursor 0, so no record that came before its arm is lost. A selector that matches no channel is `filter.ErrRefused`, and a malformed one is `filter.ErrUsage`.
- **`Since`.** `ParseSince` reads `new` (and the empty value), `all`, `last`, an RFC 3339 time and `CHANNEL:CURSOR[,CHANNEL:CURSOR]`. A malformed value, a negative cursor and a channel named twice are `filter.ErrUsage`.
  - `new` is `channel.End`, `all` is cursor 0 (a retention gap then names the deleted history), and `last` is `channel.Last`.
  - A time starts at the first signal record with `signal.ts` at or after it, by a linear scan from cursor 0. A time after each record starts at the end.
  - A cursor list starts each named channel at its cursor, and each other selected channel at cursor 0.

## The last will

- **The `loop` channel is always followed**, also when no selector names it. It prints a `loop.lost` record only on a selected channel whose route holds it. A channel without a route in the catalog holds no `loop.lost`.
- **The seed scan.** After the first arm, the reader reads the retained `loop` channel from cursor 0 to its end. It collects each `loop.started` record with no later `loop.exit` from the same `source` and pid.
- **The retention gap (design-review note a).** The reader writes one synthetic `retention` gap on the `loop` channel only when its start cursor on that channel is below the oldest retained base. When `loop` is selected, the read of `loop` gives that gap, so the will does not write a second one.
- **The first read.** For each collected record, the reader reads `proc_start` of the pid with `StartOf`. `ESRCH` or another value means the loop was gone before the reader looked. That loop gives `loop.lost` with `fields.historical=true` only when the start cursor on `loop` is at or before its `loop.started` record. The seed runs once, so the history comes once. A record with no `proc_start` is never live.
- **The live set (design-review note b).** A live loop is keyed by its pid and its `proc_start`. Thus a `loop.started` that the seed scan and a catch-up both see is armed once.
- **The second read.** After each arm that adds a live loop, the reader reads `proc_start` again. `ESRCH` or another value marks the loop as lost. A `loop.started` record after the seed gets no first read: it is live at once, and the arm and the second read decide.
- **A decision first catches up `loop`.** An exit wake and a failed second read only mark a loop. The next catch-up reads the `loop` channel to its end, and a `loop.exit` from the same source and pid ends the loop with no loss. Only then does the reader make `loop.lost`.
- **`loop.lost`** has the source `watch`, the module `loop`, the code `LOOP_LOST` and the severity `INCIDENT`. Its origin is `events.reader`, and it has the pid and the run id of the loop. Its fields are `pid`, `run_id` and `exit`, and `historical` for a history record. `exit` is `unknown` when an `INCIDENT` gap from the same source and pid follows the `loop.started` record. Else it is `crash`.
- **Kinds are data.** `loop.started`, `loop.exit` and `loop.lost` are not registered yet (E11 registers them). The reader compares the kind text, so it does not depend on the registry.

## Invariants

- The order is arm, one read, wait, and after a wake one more arm and one read (`TestReader_CallOrderIsArmThenOneReadThenWait`). An append between the arm and the read is not lost (`TestReader_AnAppendBetweenTheArmAndTheReadIsNotLost`).
- No deadline is invented and no clock is read on a watch with no deadline (`TestReader_WithNoDeadlineCreatesNoTimer`).
- A rotation loses no line, and the new tail is armed (`TestReader_FollowsRotationWithoutALostLine`).
- Each cursor rule gives one gap (`TestReader_ACursorBelowRetentionYieldsOneGapRecord`, `TestReader_ACursorPastTheTailYieldsAResetGapAtTheTrueEnd`, `TestReader_ACursorPastASegmentEndMovesToTheNextBase`, `TestReader_AWipedDirectoryYieldsAResetGapAndArmsAgain`, `TestReader_AMalformedLineYieldsAGapAndTheCursorMovesPastIt`).
- One event in two channels comes once, and the same id from another source comes twice (`TestReader_TheSameEventInTwoChannelsIsDeliveredOnce`, `TestReader_TheSameIDFromAnotherSourceIsAnotherEvent`, `TestWindow_RemembersTheLast4096IDs`).
- Each `Since` form starts where the spec says (`TestReader_SinceLastStartsAtTheLastRecord`, `TestReader_SinceNewStartsAtTheEnd`, `TestReader_SinceATimeStartsAtTheFirstRecordAtOrAfterIt`, `TestReader_ACursorListStartsANamedChannelThereAndAnUnnamedOneAtZero`).
- A wildcard selection adds a new channel from cursor 0 (`TestReader_AWildcardSelectionAddsANewChannel`).
- The merge keeps the order of each channel (`TestReader_MergesChannelsByTimeAndKeepsTheOrderOfEachChannel`).
- A failed read loses no item and no `loop.lost`, and `Next` never returns items with an error (`TestReader_AFailedReadOfALaterChannelLosesNoItemOfAnEarlierOne`, `TestLastWill_AFailedChannelReadLosesNoLastWill`, `TestReader_NextReturnsItemsOrAnErrorNeverBoth`).
- A `loop.lost` is the last item of its batch, and its `Next` skips no record (`TestLastWill_ALossIsTheLastItemAndItsNextSkipsNoRecord`).
- The twelve last-will tests of the plan, and these: `TestLastWill_ASecondReadWithAnotherProcStartIsLost`, `TestLastWill_OneProcessWithTwoLoopStartedIsArmedOnce`, `TestLastWill_ALoopExitFromAnotherSourceIsNotTheEnd`, `TestLastWill_SinceLastAtTheStaleRecordReportsItAsHistory`, `TestLastWill_ACursorListWithoutLoopGivesNoHistory` and `TestLastWill_ANewChannelWithoutARouteHoldsNoLoss`.
- With the real kernel, an append wakes the reader and a dead child process gives `loop.lost` (`TestReader_ARealAppendWakesTheReader`, `TestLastWill_ARealLoopThatDiesGetsALastWill`, integration tier).

## Findings

- **The merge rule is decided.** The spec (§4) now says that a reader merges with one head for each channel and not through `signalcenter.MergeByTS`. The reason is in §Design.
- **Open questions for the spec.** The spec does not say these, and the reader does them:
  - A cursor list that does not name a selected channel starts that channel at cursor 0. When the list does not name `loop` and `loop` is not selected, the will starts at the end of `loop`, so no history comes.
  - The `signal.pid` of `loop.lost` is the pid of the lost loop, and its `seq` is 0.
  - The reader adds a channel directory that appears under the root and is not in the catalog. Such a channel has no route, so it holds no `loop.lost`.
