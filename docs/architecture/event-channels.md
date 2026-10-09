# Event channels: a push-only publish/subscribe protocol

> [ADR-0127](adr/0127-push-only-event-channels.md) (Proposed, 2026-10-09) · plan: [event-notification-protocol-2026-10.md](../plans/event-notification-protocol-2026-10.md) · research: [event-notification-protocol-2026-10.md](../research/event-notification-protocol-2026-10.md) (F1.1 to F7.3, R1 to R24) · the producer path: [signal-center-design.md](signal-center-design.md) ([ADR-0101](adr/0101-signal-center.md)).
> This file is the spec of record for the protocol, the command surface, the exit codes and the limits. The plan holds the decisions, the components and the tests. The ADR holds the decision.
> Written in the Issue / Gap / Solution shape.
>
> The operator requests (2026-10-09) are quoted in the [plan](../plans/event-notification-protocol-2026-10.md) §1.
>
> **Terms used throughout:**
> - **Channel:** a named, append-only log of events, for example `loop` or `errors`. It has its own cursor space, retention and QoS class.
> - **Route:** the filter expression that selects the events of a channel.
> - **Record:** one line of a channel log: a signal record or a gap record.
> - **Cursor:** the byte offset of a record's first byte in its channel. It counts across segments.
> - **Position:** a channel and a cursor. It says where a record is.
> - **Event id:** `<pid>.<seq>.<ts>` of the signal. It says which event a record carries, in every channel.
> - **Segment:** one file of a channel log. Its name carries the cursor of its first byte.
> - **Publisher:** the Signal Center listener that appends each event to its channels.
> - **Reader:** the code that arms a kernel watch, reads new records and waits.
> - **Subscription:** a named, stored set of cursors, with its channels, a filter and a delivery.
> - **Runner:** the process `evolve events subscribe run NAME`, the consumer of one subscription.
> - **Gap record:** a record that names events that a channel does not hold.
> - **Last will:** the synthetic `loop.lost` record that a reader makes when a loop exits without its `loop.exit` event.
> - **Pending-entries list (PEL):** in a consumer group, the records that a runner holds without an ack.

## Table of contents

1. [Issue](#issue)
2. [Gap](#gap)
3. [Solution](#solution)
   1. [Architecture](#1-architecture)
   2. [Channels](#2-channels)
   3. [Records](#3-records)
   4. [Cursors and segments](#4-cursors-and-segments)
   5. [The append protocol](#5-the-append-protocol)
   6. [Back-pressure: two QoS classes](#6-back-pressure-two-qos-classes)
   7. [The wake: arm, catch up, wait](#7-the-wake-arm-catch-up-wait)
   8. [The filter grammar](#8-the-filter-grammar)
   9. [Subscriptions](#9-subscriptions)
   10. [Delivery](#10-delivery)
   11. [Consumer groups](#11-consumer-groups)
   12. [Retention](#12-retention)
   13. [The command surface](#13-the-command-surface)
   14. [Kinds, modules and codes](#14-kinds-modules-and-codes)
   15. [Timers](#15-timers)
   16. [Safety](#16-safety)
   17. [What this does not do](#17-what-this-does-not-do)
4. [Limits](#limits)

## Issue

Every watcher of the pipeline polls, and no program outside one process can subscribe to the Signal Center.

- **Each watch verb sleeps and reads again.** `signals tail --follow` waits 1 s, `wave watch` 5 s, `bridge watch` 500 ms and `ci watch` 30 s. The phase watchdog reads file times every 15 s. The [plan](../plans/event-notification-protocol-2026-10.md) §2 lists each site with its file and line.
- **The Center is in-process only.** Each process has one `Center` with its own `seq` (`go/internal/signalcenter/center.go:136`). `Emit` calls each listener on the goroutine that drains.
- **Many processes append to `.evolve/signals.ndjson`.** Their lines have no shared order.
- **The cost in this session.** 48 orphan `tail -F` processes ran for up to 24 days. Monitors replayed old events when they armed again, and events between two polls were lost. Each consumer built its own filter.
- **The wave plan states the loss.** "A phase shorter than the poll interval does not show" ([evolve-wave-cli-2026-10.md](../plans/evolve-wave-cli-2026-10.md) §Limits).

## Gap

1. **No external subscription.** The listeners of a Center live in its process.
2. **No wake.** Each watcher sleeps and reads again.
3. **No cursor.** A consumer cannot resume after a restart without a loss or a replay.
4. **No channel.** A consumer that wants loop events reads every event of the project.
5. **No back-pressure contract.** The effect of a slow consumer is not defined.
6. **No delivery.** A program cannot register a command that runs on an event.
7. **No end for an orphan.** A watcher whose reader is gone waits forever.
8. **Two producers are absent.** `phase.dispatched` and `ship.landed` exist as kinds, but no code emits them.

## Solution

```
 producers (each process)                     channels (logs)                          subscribers
 ┌────────────────────────┐   route by rule    .evolve/events/ch/<name>/seg-<base>      ┌──────────────────────────┐
 │ signalcenter.Center    │──► Publisher ──►   loop · cycle · errors · ship · inbox ──► │ evolve events watch      │
 │ (+ wave/ship/inbox/ci  │   (listener)       ledger · ci · signals (the firehose)     │ evolve events subscribe  │
 │  lifecycle emitted in) │                    kernel wake: kqueue / inotify            │   run NAME (exec:CMD)    │
 └────────────────────────┘                                                             │ any program (same CLI)   │
                                                                                        └──────────────────────────┘
```

### 1. Architecture

- **One producer path.** Every event is a Signal Center event. Wave, loop, ship, inbox and CI lifecycle events are emitted into the Center (§14).
- **One publisher.** The `Publisher` is one Center `Listener`. `newRootSignalCenter(role)` (`go/cmd/evolve/cmd_cycle.go:352`) subscribes it and returns `close(deadline)`.
- **Every root.** Each process that emits builds its Center through `newRootSignalCenter`. The phase-observer Center gets the Publisher too. A root-list guard test keeps the list complete.
- **Listeners observe and never decide** ([ADR-0101](adr/0101-signal-center.md) decision 5). The Publisher never emits and never calls `Flush`.
- **No broker.** A subscriber reads the channel logs. No process forwards an event, so no process needs a supervisor.
- **Roles.** The `role` names the root: `cycle`, `loop`, `loop-chain`, `ship`, `subagent`, `simulate`, `phase-observer`, `wave`, `inbox`, `ci` or `subscriber`. A reader marks the records that it makes with `watch`.

### 2. Channels

The catalog is config: `.evolve/policy.json` `events.channels`. The compiled defaults are in `internal/policy` (`EventsConfig()`). The catalog is closed: a publisher appends only to a configured channel, and a selector never makes a channel.

Each channel has these keys:

| Key | Meaning | Default |
|---|---|---|
| `route` | a filter expression (§8); an empty route selects every event | see the next table |
| `qos` | `lossless` or `best_effort` (§6) | see the next table |
| `ttl_days` | the retention age (§12) | `gc.logs_ttl_days` (30) |
| `max_total_mb` | the retention size cap (§12) | 128; 512 for `ledger` and `signals` |
| `segment_mb` | the size at which a segment rotates (§5) | 16 |

The default channels:

| Channel | Route | QoS |
|---|---|---|
| `loop` | `module=loop` (the loop and wave lifecycle, halts) | lossless |
| `cycle` | `kind=phase.dispatched,phase.outcome,phase.aborted,cycle.sealed,ship.landed,system.failure` | lossless |
| `errors` | `severity>=WARN` | lossless |
| `ship` | `kind=ship.*` | lossless |
| `inbox` | `module=inbox` | lossless |
| `ci` | `kind=ci.*` | lossless |
| `ledger` | `kind=ledger.appended` | best_effort |
| `signals` | every event (the firehose) | best_effort |

- One event can go to several channels. The Publisher writes it once in each channel.
- The `signals` channel replaces the project-level `.evolve/signals.ndjson`. The per-cycle `runs/cycle-N/signals.ndjson` stays as the cycle record.
- **Names.** A name is one or more tokens separated by dots. A token matches `[a-z0-9_-]+`.
- **Selectors.** `--channel` takes a comma list of names or patterns. `*` matches one token, and `>` matches one or more tokens at the end (NATS subjects, research F4.4). For example, if the catalog has `ci.required` and `ci.release`, `--channel 'ci.>'` selects both.
- **Resolution.** The reader resolves the selectors when it starts. A selector that matches no channel is refused. With a wildcard, the reader also watches `.evolve/events/ch/`, and it resolves the selection again when a new channel directory appears.

### 3. Records

Two record types exist on disk, one JSON object on each line:

```json
{"source":"loop","dispatch":"events/0/sub-alerts/p4242n1t0","signal":{"schema_version":"signal/1.0","seq":41,"pid":9055,"ts":"2026-10-09T17:46:02.114Z","cycle":1841,"module":"orchestrator","origin":"cycleRun.completeCycle","kind":"cycle.sealed","severity":"INFO","reason":"final verdict PASS","fields":{"final_verdict":"PASS"}}}
{"source":"loop","gap":{"reason":"queue_full","severity":"WARN","pid":9055,"first_seq":57,"last_seq":73,"dropped":17,"from":0,"to":0}}
```

- **`signal`** holds the full `signal/1.0` event without a change (`go/internal/signalcenter/event.go:222`).
- **`source`** is the role of the root that published the record (§1).
- **`dispatch`** is the `EVOLVE_DISPATCH_ID` of the process that published the record, if it has one. A record from a process with no tag has no `dispatch` key.
- **`gap`** names events that this channel does not hold: the reason, a severity, the `pid` of the process, the `seq` range and the count. The reasons are `queue_full`, `write_error` and `lock_deadline` (§5, §6). A loss on a `lossless` channel has the severity `INCIDENT`, and a loss on a `best_effort` channel has `WARN`.
- **`from` and `to`** are always in a gap, also when they are 0. A synthetic gap sets them to the two cursors of the move. A stored gap has 0 in both.
- **Valid record.** A record has exactly one of `signal` and `gap`. A stored record never has the source `watch`. The writer refuses any other record, and the reader reads it as `malformed`.
- **Size.** `Normalize` keeps a signal line at 4,096 bytes or less (`MaxLineBytes`, `event.go:33`). `source` comes from a closed set, and the Publisher cuts `dispatch` to 256 bytes. A record line is thus 4,608 bytes or less.

The wire form is what `evolve events watch --json` prints and what a command reads on stdin. The reader adds four keys from the record and from the place where it read the line:

```json
{"schema_version":"evolve.event/1","id":"9055.41.2026-10-09T17:46:02.114Z","channel":"cycle","cursor":184467,"source":"loop","dispatch":"…","signal":{…}}
```

- `evolve.event/1` holds every `signal/1.0` field without a change, inside `signal`. A `signal/1.0` consumer reads `.signal`.
- **The event id** (`id`) is `<signal.pid>.<signal.seq>.<signal.ts>`. It is the same in every channel and in the per-cycle file. One process has one `seq` order, and the time separates a pid that the system used again. The event id is the key to remove duplicates.
- **The position** (`channel` and `cursor`) says where the record is. A consumer resumes and acks by position.
- A gap record's id is `gap.<channel>.<cursor>`.
- A **synthetic record** comes from a reader, not from a log: the last will (`loop.lost`, §7) and the reader gaps (`retention`, `malformed`, `reset`). Its `source` is `watch`, and it has no `cursor`. A synthetic gap has the id `gap.<channel>.<from>`. A consumer never acks a synthetic record. The reader moves the cursor past the lines that a synthetic gap names.

### 4. Cursors and segments

- **Path.** `.evolve/events/ch/<channel>/seg-<base>.ndjson`. `<base>` is the cursor of the segment's first byte, padded with zeros to 20 digits (Kafka base offsets, research F4.1). A name sort is a number sort.
- **Cursor.** The cursor of a record is the byte offset of its first byte in the channel. Cursors increase across segments, and no tip file exists.
- **Lookup.** A cursor belongs to the segment with the largest base at or below it.
- **Stored cursor.** A subscription stores, for each channel, the cursor of the next record to read. That is the cursor of the last acked record plus its line length (the Kafka position, research F4.2).
- **Start.** `--since` selects the first record:

| Value | Start |
|---|---|
| `new` (default) | the end of each channel when the reader arms |
| `all` | the first retained record |
| `last` | the last complete record of each channel, then live. It is the last line of the tail segment, or of the segment before it when the tail has no complete line. |
| `ch:offset[,ch:offset]` | the given cursors |
| an RFC 3339 time | the first record whose `signal.ts` is at or after the time, by a linear scan from the oldest retained segment |

- **Order.** Order is total within a channel. No order exists across channels (Kafka partitions). A reader of several channels merges each batch by `signal.ts`, with one head for each channel, and labels the result "not a total order".
  - The merge never changes the order of one channel. Two processes append to one channel, so its times are not in order. A sort of all events (`signalcenter.MergeByTS`) can thus put a later cursor before an earlier one, and the reader does not use it.
  - The merge applies inside one batch. Two batches are not in time order across channels.
- **The reset rule.** When the reader opens a channel or wakes, it compares its cursor with the channel:
  - A cursor past the end of a segment that has a later segment moves to the base of that later segment. An OS crash can drop the tail of a segment after its rotation.
  - A cursor past the end of the tail segment moves to the true end. The tail lost bytes, or someone wiped the directory.
  - A reader at the end of a segment whose end is below the base of the later segment moves to that base. The bytes between them are lost.
  - Each move gives one synthetic gap record with `reason: reset`, `from` and `to`.
  - A segment that gc removes after the reader listed the segments gives a `retention` gap to the base of the next segment.
  - If the channel directory is gone, the reader makes it again and arms again. The new log starts at cursor 0.

### 5. The append protocol

Each append takes the channel lock `ch/<channel>.lock` with `flock.LockWithin(path, deadline, onSettled)`. The lock file sits beside the channel directory, so its creation never wakes a watcher. With the lock held:

1. Open the tail segment, which is the segment with the highest base, with `O_WRONLY|O_APPEND|O_CREATE`. `fstat` it.
2. If the last byte is not `\n`, put one `\n` before the batch. This repairs a line that a crashed writer tore.
3. Write the repair and the batch of lines in one `write` call.
4. If the segment is larger than `segment_mb`, make the next segment `seg-<base+size>`. If this step fails, the batch is on disk and is not a loss. The append returns `ErrRotate` with the cursor, and the next append tries the rotation again.
5. Release the lock.

**The lock with a deadline** (`internal/adapters/flock`, new in E2):
- A goroutine calls `flock`, which blocks. The caller waits for it or for a one-shot timer, whichever comes first. Nothing polls.
- **The hand-off** is one atomic state with three values: `waiting`, `handed` and `abandoned`. Each side changes it with one compare-and-swap.
  - When `flock` returns, the goroutine tries `waiting` to `handed`. If it wins, the lock and its descriptor belong to the caller.
  - If the goroutine loses, the state is `abandoned`. Only then does the goroutine close its descriptor, which frees the lock at once.
  - At the deadline, the caller tries `waiting` to `abandoned`. If it wins, `LockWithin` returns `ErrLockDeadline`, and the caller never touches the descriptor again.
  - If the caller loses, the state is `handed`, and the caller takes the lock. The caller owns the lock only in the state `handed`.
  - Only one of the two swaps can win. Thus exactly one side owns the descriptor, also when the deadline and the grant come at the same instant.
  - If `flock` fails, the goroutine hands the error over in the same way.
- **The settle signal.** `LockWithin` calls `onSettled` once, when its goroutine is done. That is before it returns a lock, after an abandoned call released the lock, or before it returns an open error.
- At most one lock call waits for each channel and process. While it waits, a new append to that channel fails at once with `ErrLockPending`, which wraps `ErrLockDeadline`. It starts no goroutine and opens no descriptor. It is a counted loss with the reason `lock_deadline`. The flag clears at the settle signal.
- The deadline is `events.lock_deadline_ms` (default 250). E10 keeps it at 250 or less.
- **The drain bound.** One event that matches k lossless channels waits up to k × the lock deadline. The worst case is the six default lossless channels: 6 × 250 ms = 1.5 s.
- The lock deadline bounds only the wait for the lock. A disk that hangs inside the write, after the lock is taken, is not bounded. That is an accepted limit (see Limits).

**Errors:**
- A write error (for example `ENOSPC` or `EIO`) or a lock deadline is a counted loss. The next good append writes one gap record with the reason `write_error` or `lock_deadline`.
- **No fsync.** A process crash cannot repeat or skip a cursor, because the page cache keeps the bytes. An OS crash is a documented limit.
- **One write for each batch.** No other writer appends between two lines of one batch.
- **The same idiom as the ledger**, which takes `flock` around each append (`go/internal/adapters/ledger/ledger.go:118-140`). The deadline is new.

### 6. Back-pressure: two QoS classes

A publisher never waits on a subscriber: no subscriber takes a channel lock. A slow subscriber only lags.

| Class | Path | Loss | For |
|---|---|---|---|
| `lossless` | the Publisher appends inside the Center drain: one locked write for each event and channel | only a write error or a lock deadline, as a counted loss with an `INCIDENT` gap. An event emitted at exit is on disk before `Flush` returns. | low-volume channels (MQTT QoS 1) |
| `best_effort` | a bounded queue in each process; one writer goroutine appends in batches | a full queue, a write error or a lock deadline, as a counted loss with a `WARN` gap | high-volume channels (MQTT QoS 0) |

- **Keys.** `events.queue_events` (1,024), `events.queue_bytes` (4 MiB), `events.enqueue_deadline_ms` (0) and `events.lock_deadline_ms` (250). With 0, the Publisher never waits for queue space.
- **A loss.** The Publisher counts a loss for each channel of the event. It keeps the `pid`, the first and last `seq` and the count. The next good batch for that channel starts with one gap record.
- **The pattern for the count** is the one in `ndjsonSink.reportDrops` (`go/internal/signalcenter/sinks.go:108`). The Publisher writes a record, and it does not emit.
- **`close(deadline)`.** Each root calls it at exit, after `Flush`. It drains the queue until it is empty or the deadline passes. What is left is a counted loss. The gap record goes out if the lock comes before the deadline. Else `close` returns the count, and the root prints it on one stderr line.
- **Measured load.** About 2,000 events per day, with bursts of 50 per second (research F1.1, F1.2). A queue of 1,024 holds 20 seconds of the worst burst.

### 7. The wake: arm, catch up, wait

The `Waiter` (`internal/events/wake`) has two calls: `Arm(targets)` and `Wait(ctx, deadline)`. The targets are the directories, the files and the pids. Each `Arm` sets the whole watch set. A syscall port holds `kqueue`, `kevent`, `inotify`, `epoll`, `pidfd_open` and `statfs`, so a test drives every branch. The start time of a process is not in this port (see the process identity below).

**The wake contract** (the same on darwin and on Linux):
- A directory in `Dirs` wakes when its set of entries changes: an entry is made, deleted or renamed.
- A file in `Files` wakes when its content changes: an append, a truncation, a delete, a rename away, or a replacement by rename.
- A spurious wake is allowed. A wake only means "something can be new".
- A rename can replace a watched file, and the old watch then holds the old file. Thus, after a `Changed` wake on such a file, the caller runs `Arm` again.
- After each `Arm`, also one that only adds a pid, the caller catches up.
- A pid that is gone at the arm (`ESRCH`) is an exit. The next `Wait` returns it at once.
- `Hangup` is terminal. On darwin, `EV_EOF` can come only once, so the caller ends the watch (exit 3) at the first `Hangup`.

1. **Arm** the kernel watch.
2. **Catch up:** read every complete line after the cursor, to the end of the tail segment.
3. **Wait** until the kernel posts an event, the context ends or the deadline passes.
4. After each wake, catch up again. A wake only means "something can be new".

The order closes the window where an append lands between the read and the wait (Kubernetes list-then-watch, research F4.11). For the same reason, `go/internal/dashboard/sse.go:87-90` subscribes before its snapshot.

**darwin (kqueue):**
- `EVFILT_VNODE` with `NOTE_WRITE` on the channel directory: a new segment.
- `EVFILT_VNODE` with `NOTE_WRITE`, `NOTE_EXTEND`, `NOTE_DELETE`, `NOTE_RENAME` and `NOTE_ATTRIB` on the tail segment: an append, a delete, a rename or a truncation. A truncation gives only `NOTE_ATTRIB`.
- `EV_CLEAR` on both. Without it, the event returns on every call and the waiter spins (research F1.8).
- `EVFILT_USER` with `NOTE_TRIGGER` for a cancel from the same process.
- `EVFILT_PROC` with `NOTE_EXIT` for the last will.
- `EVFILT_WRITE` on stdout, with `EV_EOF`, for the output hangup.
- `kevent` with no timeout, or with the time left to the deadline.

**Linux (inotify):**
- One watch on the channel directory: `IN_MODIFY`, `IN_CREATE`, `IN_DELETE`, `IN_MOVED_FROM`, `IN_MOVED_TO`, `IN_DELETE_SELF` and `IN_MOVE_SELF`. A file in `Files` is watched through its directory.
- An `epoll` set holds the inotify descriptor, a self-pipe for a cancel and a `pidfd` for the last will.
- The same set holds stdout with `EPOLLHUP|EPOLLERR` for the output hangup.
- `IN_Q_OVERFLOW` counts as a wake.

**Other rules:**
- **`EINTR`.** If `kevent` or `epoll_wait` returns `EINTR`, `Wait` computes the time left to the deadline again and calls again.
- **Refusals.** The waker reads the filesystem type with `statfs`. It accepts `apfs` and `hfs` on darwin, and `ext4`, `xfs`, `btrfs`, `tmpfs` and `overlay` on Linux. It refuses every other filesystem and every other operating system with exit 1. No poll fallback exists (research F2.4, F2.8).
- **Loud failure.** If the kernel refuses a watch (for example, at the inotify limits), the reader fails with that error and exit 1 (research F2.9).
- **Rotation.** When the directory wakes and a newer segment exists, the reader reads the old segment to its end. Then it arms the new tail segment, catches up and waits.
- **Before any loop.** The reader runs `MkdirAll` on the channel directory before it arms. A watch thus works before any producer ever ran, and the first append wakes it.
- **The output hangup.** If stdout is a pipe or a terminal, the waiter also arms its hangup. When the reader of stdout goes away, the watch exits 3. A write that fails with `EPIPE` also exits 3. The CLI calls `signal.Notify` for `SIGPIPE`: without it, Go exits on a broken pipe at file descriptor 1 (research F7.3). If stdout is a regular file, no hangup is armed.
- **Torn and bad lines.** The reader returns only complete lines. A complete line that does not parse becomes a synthetic gap record with `reason: malformed`, and the reader moves its cursor past the line. A runner delivers that gap and acks past the line.

**The last will.** A loop that exits without its `loop.exit` event is a lost loop. The reader reports it with a synthetic `loop.lost` record. The reader always follows the `loop` channel for this, even if `loop` is not selected. It prints a `loop` record only if a selected channel holds it.

**The process identity.** `loop.started` carries `fields.proc_start`, the start time of the loop process. `proctree.StartOf(pid)` reads it at emit. It is in `proctree`, not in the wake port. E5 and E11 add it:
- On darwin, `procstart_darwin.go` reads `p_starttime` from `kinfo_proc` (`KERN_PROC_PID`). It uses a general form of the raw `sysctl` call that `proctree` uses for `KERN_PROCARGS2` (`go/internal/proctree/procargs_darwin.go`).
- On Linux, `procstart_linux.go` reads the start ticks from `/proc/<pid>/stat`, with the boot id.
- The reader reads the live pid with the same port and compares the two values for equality. Equal means "the same process". Anything else means "gone".

At arm time, for any `--since`:

1. Arm the watch on the `loop` channel, so that each new `loop` record wakes the reader.
2. **The seed scan.** Read the retained `loop` channel backwards. Collect each `loop.started` record with no later `loop.exit` from the same pid.
3. If the retained channel does not start at cursor 0, write one gap record (`reason: retention`). That gap names the history that the seed scan cannot see.
4. For each collected record, read `proc_start` of its pid. Not equal, or `ESRCH`: the loop was gone before this reader looked.
5. Equal: the loop is live. Add it to the live set.
6. For each live loop, arm the exit watch first: `EVFILT_PROC` with `NOTE_EXIT`, or a `pidfd`. Then read `proc_start` again.
7. `ESRCH` at the arm, or a second read that is not equal: the loop died after the seed scan. It is lost.
8. Equal: the watch holds the right process. A later exit wakes the reader.

After arm time:

9. A new `loop.started` record arms an exit watch at once. Then the reader reads `proc_start`; not equal or `ESRCH` means lost.
10. On an exit wake, the reader catches up the `loop` channel to its end.

**The race.** The second read in step 6 comes after the exit watch is armed. A loop that dies after the seed scan thus gives `ESRCH` at the arm, a changed `proc_start` at the second read, or an exit wake. No case gives silence.

**Every decision of a lost loop** first catches up the `loop` channel. A `loop.exit` from the pid there is a normal end, and the reader makes no `loop.lost`.

The two cases of a dead loop:
- **Gone before this reader looked** (step 4). For example, a stale `loop.started` from an old SIGKILL. The reader makes no `loop.lost`, so it raises no alarm and `wave watch` does not exit 4. One exception: if the reader's start cursor is at or before that `loop.started` record (for example `--since all`), the reader reports it once as history. That record is `loop.lost` with `fields.historical=true`, and it never ends `wave watch`.
- **Lost after this reader saw it live** (steps 7 to 10). The record is `loop.lost` with `fields.pid`, `fields.run_id` and `fields.exit`.

**`fields.exit`** is `crash` by default. It is `unknown` if the `loop` channel holds an `INCIDENT` gap record (`lock_deadline` or `write_error`) from that pid after its `loop.started`. That gap can be the lost `loop.exit`, so the crash is not confirmed.

- The routes and the filter select `loop.lost` like any signal: the `loop` and `errors` routes hold it.
- The source is the `loop.started` event, not the run lease. A lease names the process that runs a cycle (`go/internal/core/runlease_hook.go:33`), and a fleet lane runs in its own process (`go/cmd/evolve/cmd_fleet.go:127`). The end of a lane is not a lost loop.

### 8. The filter grammar

One package, `internal/events/filter`, holds the grammar and the matcher. The channel routes, `--filter`, `--until` and subscriptions use it (the Specification pattern).

```
EXPR  = TERM { " " TERM }                 terms are ANDed
TERM  = KEY OP VALUE { "," VALUE }        "=": any value matches; "!=": no value matches
OP    = "=" | "!=" | ">=" | ">" | "<=" | "<"
KEY   = "kind" | "code" | "module" | "severity" | "cycle" | "phase" | "run_id"
      | "origin" | "attempt" | "pid" | "seq" | "source" | "fields." NAME
VALUE = a literal, or a glob with "*" for the keys "kind" and "code"
```

- **Example:** `kind=cycle.sealed,loop.exit severity>=WARN cycle=1841 code=SKILLS_DRIFT_* module=ship`.
- **Order keys.** `severity` orders `INFO`, `WARN` and `INCIDENT`. `cycle`, `attempt`, `pid` and `seq` are numbers.
  - An order operator (`>=`, `>`, `<=`, `<`) applies only to an order key. On another key, it is a usage error (exit 10).
  - An order operator takes exactly one value. With more than one value, it is a usage error (exit 10).
- **Absent keys.** A key that a record does not have is absent. A `cycle` or an `attempt` of 0 is absent, as in the JSON form. An absent key never matches `=` or an order operator, and it always matches `!=`.
  - An empty `kind`, `module`, `severity` or other text value is absent.
  - A record severity that is not `INFO`, `WARN` or `INCIDENT` is absent.
  - `pid` and `seq` are always present, also when they are 0.
- **The catalog.** The vocabulary comes from the Signal Center registries (`go/internal/signalcenter/event.go:68-172`, `registry.go`). `evolve events kinds` prints it. It is the one home: no second vocabulary exists.
- **Checks.**
  - An unknown key is a usage error (exit 10).
  - A `severity` value that is not `INFO`, `WARN` or `INCIDENT` is a usage error (exit 10).
  - An unknown kind or module, or a `kind` glob that matches no registered kind, is refused (exit 1).
  - An unknown code only warns. The code registry is open: a newer build can add a code that a stored filter names.
- **Gap records pass every filter.** A gap is never silent.
- **`--until EXPR`** ends a watch at the first signal record that matches. A synthetic `loop.lost` can match. The watch prints that record and exits 0. A gap record never satisfies `--until`.

### 9. Subscriptions

A subscription is a set of files under `.evolve/events/subs/`. That directory is outside `ch/`, so a cursor write never wakes a channel watcher.

| File | Content | Writer |
|---|---|---|
| `<name>.json` | the config (next table) | the CLI only |
| `<name>.cursor` | the next cursor for each channel | the runner or the ack verb, with `WithPathLock` and `atomicwrite` |
| `<name>.lock` | the runner lock of a fan-out subscription (`TryLock`), with the runner's pid | the runner |
| `<name>.runner-<pid>-<nonce>.lock` | the lock of one group runner (§11) | that runner |
| `<name>.dead.ndjson` | the dead letters | the runner |
| `<name>.pel.json` | the pending-entries list of a group (§11) | the group runners |

The config keys:

| Key | Default | Meaning |
|---|---|---|
| `channels` | none (required) | the channel selectors |
| `filter` | empty | a filter expression (§8) |
| `deliver` | empty: a pull consumer | `exec:CMD` |
| `since` | `new` | the first cursors, set once by `subscribe add` |
| `retries` | 3 | the local tries after the first one |
| `backoff_s` | 10 | the fixed wait before each local retry |
| `timeout_s` | 60 | the deadline of one command run |
| `paused` | false | a paused subscription delivers nothing and holds no retention |
| `group` | false | several runners share the subscription (§11) |
| `ack_wait_s` | 300 | group only: an entry idle for longer can be claimed |
| `max_deliver` | 5 | group only: the claims before a dead letter |

- **Name rule.** `^[a-z0-9][a-z0-9_-]{0,63}$`.
- **Checks at `add`.** For a group, `ack_wait_s` must be more than `timeout_s × (retries + 1) + backoff_s × retries`. The defaults give 300 against 270.
- **Status.** `subscribe status` shows the lag of each channel (the tail cursor less the stored cursor), the dead-letter count and the time of the last delivery.

### 10. Delivery

`evolve events subscribe run NAME` is the consumer of an `exec` subscription. Whoever registered the subscription starts it: a person, a launchd or systemd unit, or Claude's Monitor. Nothing in the loop supervises it.

1. Take the runner lock: `subs/<name>.lock` for fan-out, or `subs/<name>.runner-<pid>-<nonce>.lock` for a group. If another fan-out runner holds the lock, exit 1.
2. Read the stored cursors. Arm, catch up and wait (§7).
3. Take the records in cursor order, merged by time across channels. Skip a record that the filter refuses, and advance its cursor.
4. Skip a record that this subscription caused: its `dispatch` starts with `events/0/sub-<name>/`, or its `signal.pid` is the runner's pid.
5. Skip a record whose event id is in the window of the last 4,096 ids that this runner delivered. Advance its cursor.
6. Run the command once for the record. The next record waits until this one is acked or dead-lettered.

- **The `events` module.** An exec subscription never gets a record of the module `events`, unless its filter names `module=events`. A dead letter of one subscription thus never feeds another.
- **The window.** The same event can be in two selected channels. The window runs the command for it once in one run. The window is in memory, so after a restart a copy can run again. A consumer that must be exact keeps its own set of `EVOLVE_EVENT_ID` values.

**The command:**
- `sysexec.Command(ctx, "/bin/sh", "-c", CMD)` in the project root, with `timeout_s` as the deadline. At the deadline, `sysexec` sends SIGTERM, and SIGKILL after `WaitDelay` (5 s, `go/internal/sysexec/command.go`).
- Stdin: the wire record (§3), one line.
- Environment:
  - `EVOLVE_EVENT_ID`: the event id, the key to remove duplicates;
  - `EVOLVE_EVENT_CHANNEL` and `EVOLVE_EVENT_CURSOR`: the position;
  - `EVOLVE_EVENT_KIND`: the signal kind, or `gap`;
  - `EVOLVE_PROJECT_ROOT`;
  - `EVOLVE_DISPATCH_ID=events/0/sub-<name>/p<runner-pid>n<nonce>` (`go/internal/proctree/dispatchid.go`).
- After the command exits, the runner stops each tagged descendant with `proctree.Reaper`, as the bridge does at dispatch end. If the runner dies, `evolve gc` reaps the tagged leftovers (`go/internal/gc/dispatch_processes.go:36`).
- Each `evolve` process that the command starts stamps the tag into its records (§3). Step 4 then skips them.

**Outcomes:**
- **Exit 0:** the runner stores the cursor after the record. Delivery is at-least-once (research F4.2, F5.1).
- **A failure** (a non-zero exit or the deadline): the runner tries again after `backoff_s`, up to `retries` times.
- **A poison event** (all tries failed):
  1. the runner appends the record, the tries and the last exit code to `subs/<name>.dead.ndjson`;
  2. it acks past the event;
  3. it emits `EVENTS_DEAD_LETTERED` (WARN) through its own Center (role `subscriber`);
  4. it continues with the next record.

**Pull mode:**
- `evolve events watch --sub NAME` resumes from the stored cursors. By default, it acks each record after it writes the line to stdout. That is at-most-once into the pipe: a record in the pipe buffer is acked even if the program that reads it dies first.
- `evolve events watch --sub NAME --manual-ack` acks nothing. The program acks with `subscribe ack NAME CHANNEL:CURSOR` after it handles a record. That is at-least-once.
- `subscribe ack` also moves stored cursors forward or back, for a skip or a replay.

The `Deliverer` interface has two implementations in v1: the command runner and the stdout writer of pull mode. A webhook deliverer is a later phase.

### 11. Consumer groups

Component E13 adds work-queue delivery after E12. Fan-out stays the default: every subscription gets every record that its filter accepts.

- **Locks.** Each group runner holds its own lock, `subs/<name>.runner-<pid>-<nonce>.lock`. A held lock means a live runner.
- **Shared state.** The runners share one group cursor and one PEL, under `WithPathLock`.
- **PEL entries.** Each entry holds the position, the event id, the runner id, the delivery count and the delivery time.
- **Take.** A runner adds a PEL entry and advances the group cursor, then delivers.
- **Ack.** Each message has its own ack, which removes its entry. A poison message thus never blocks its peers (Kafka share groups, research F4.3).
- **Local tries and claims.** A runner tries an entry up to `retries + 1` times, with `backoff_s` between tries. A claim moves the entry to another runner and adds 1 to `delivery_count`.
- **Dead letter.** An entry goes to the dead letters when its local tries fail, or when `delivery_count` is more than `max_deliver`. The second case is a record that kills its runners.
- **Claim triggers.** A runner claims an entry that is idle for longer than `ack_wait_s`. Four push triggers start a claim:
  - each wake of the runner;
  - the kernel exit watch on the pid of the entry's runner;
  - a `TryLock` on the entry's runner lock that succeeds, which proves that the runner is dead;
  - a one-shot deadline at the oldest entry's delivery time plus `ack_wait_s`, for a runner that is alive but stuck.
- **Order.** A group keeps no order across its runners.

### 12. Retention

- **One gc category for each channel** in the gcpolicy catalog (`go/internal/gcpolicy/logs.go:58`). Its home is `events/ch/<name>`, with the glob `seg-*.ndjson`. Its TTL and size cap come from the channel (§2).
- **The tail segment is never deleted**, as for the `current` loop log.
- **The `Protect` hook.** For each subscription that is not paused, gc keeps the segment that contains its cursor and every later segment, if the size cap allows it. For a group, the cursor is the lower of the group cursor and the oldest PEL cursor.
- **A gap from the cap.** If the cap deletes a segment that a cursor needs, the reader makes a synthetic gap record (`reason: retention`, with the two cursors). Then it resumes at the oldest base. A gap is never silent.
- `evolve gc --dry-run` lists the channel segments with their rule.

### 13. The command surface

The verbs live in `go/cmd/evolve/cmd_events*.go`, with a dispatch map in the style of `runWaveWith`. `registry.go` registers the verb `events`.

```
evolve events channels [--json]                      # catalog: name, route, QoS, retention, tail cursor, lag per subscription
evolve events kinds [--json]                         # registered modules, kinds and codes (the filter vocabulary)
evolve events watch --channel C[,C] [--filter EXPR] [--since all|new|last|CURSORS|RFC3339] [--until EXPR] [--json]
evolve events watch --sub NAME [--manual-ack]        # resume a registered subscription (pull consumer)
evolve events subscribe add NAME --channel C[,C] [--filter EXPR] [--deliver exec:CMD] [--retries N] [--since all|new|last|CURSORS|RFC3339]
evolve events subscribe list | show NAME | rm NAME | pause NAME | resume NAME | ack NAME CHANNEL:CURSOR[,CHANNEL:CURSOR]
evolve events subscribe run NAME                     # the consumer of an exec subscription; it blocks and the kernel wakes it
evolve events subscribe status [NAME]                # lag per channel, dead-letter count, last delivery
```

| Exit | Meaning |
|---|---|
| 0 | ok: `--until` matched, or the user stopped the command (SIGINT or SIGTERM) |
| 1 | refused: an unsupported filesystem or system; an unknown kind or module; a selector with no channel; a held runner lock; a refused kernel watch |
| 2 | an I/O error, or the `.evolve` directory is gone |
| 3 | the reader of stdout is gone: a hangup, or `EPIPE` |
| 4 | `wave watch` only: a live `loop.lost` record ended the wave; a historical one never does |
| 10 | a usage error |

- A deleted channel directory is not an exit. The reader writes a `reset` gap, makes the directory again and arms again (§4).
- The stream output is one JSON line for each record, in the wire form (§3). Without `--json`, a line is `<channel> <cursor> ` and then `signalcenter.FormatLine` of the signal.

### 14. Kinds, modules and codes

Component E11 adds these producers and registers them in the closed sets (`go/internal/signalcenter/event.go`):

| Kind | Module | Emitted by | Severity |
|---|---|---|---|
| `phase.dispatched` | `orchestrator` | the phase transition (`go/internal/core/cyclerun_dispatch.go:43`, `resume_execution.go:118`, `evaluate_batch.go:125`) | INFO |
| `ship.landed` | `ship` | the landing intent at `IntentComplete` (`go/internal/phases/ship/landing/intent.go:44`) | INFO |
| `wave.started`, `wave.ended` | `loop` | `evolve wave next` and the loop at a wave end | INFO |
| `loop.started` | `loop` | the loop root at start-up; `fields.proc_start` is the start time of the loop process (§7) | INFO |
| `loop.exit` | `loop` | the loop root at exit | INFO |
| `loop.lost` | `loop` | a reader only, as a synthetic record (code `LOOP_LOST`; fields `pid`, `run_id`, `exit` and `historical`) | INCIDENT |
| `inbox.claimed`, `inbox.released` | `inbox` | the inbox mover lifecycle (`go/internal/inboxmover/lifecycle`) | INFO |
| `ci.completed` | `ci` (new) | `evolve ci watch` and the ship CI watch; a red run has code `CI_RUN_RED` | INFO, or WARN when red |

The module `events` (new) owns `EVENTS_DEAD_LETTERED`. A gap record is not a signal, so it has no kind.

### 15. Timers

- **Allowed: a one-shot deadline.** It fires once, for a known reason, and it never reads state to find a change. The deadlines are:
  - `Wait(ctx, deadline)` and `LockWithin`;
  - the command timeout and the retry back-off;
  - the reaper grace;
  - `enqueue_deadline_ms`;
  - the E13 `ack_wait_s` deadline;
  - the watchdog deadline for "nothing happened".
- **Forbidden: a poll.** A poll is a loop that sleeps and then reads a file, a status or an API again.
- **The guard.** The AST guard test `TestNoPollTimerInTheEventChannelSources` (`go/test/structure/nopoll_guard_test.go`) bans `time.Sleep`, `time.Tick`, `time.NewTicker`, `time.After` and `time.NewTimer`. Its list of directories is data: `internal/events` today. E2 adds `flock.LockWithin`, and E12 and E14 add the migrated watch files.
- **The runtime.** An idle Go process wakes about once a minute for runtime work (research F1.6). That wake never reads our state.

### 16. Safety

- **Observe, never decide.** A subscriber cannot change a verdict or block a phase. Delivery runs in a separate process, after the event is on disk. The loop never waits for a subscriber, and a failed command only goes to the dead letters.
- **No agent producer.** A phase agent never gets `Emit` (ADR-0101 decision 7). E8 adds `/.evolve/events/` to `ProtectedSurfaceManifest` (`go/internal/guards/integrity_surface.go`, [ADR-0064](adr/0064-pipeline-integrity-boundary.md)). Thus no phase agent writes a channel record or registers a command.
- **Operator-owned commands.** An `exec` command runs with the rights of the person who started the runner. The CLI is the only writer of the subscription config.
- **No leaks.** The dispatch tag and the gc backstop reap the descendants of a command.
- **No self-loop.** A runner skips the records that its own commands caused (the dispatch stamp), its own records, and the module `events` by default.
- **No orphan.** A watch exits when the reader of its output is gone (§7).

### 17. What this does not do

- **No host-wide bus.** The scope is one project's `.evolve/`. A `plane` key in the record wrapper is the seam for a later reader of several planes.
- **No webhook in v1.** The `Deliverer` interface is the seam.
- **No `--timeout` in v1.** `--until` covers a wait for an event.
- **No exactly-once delivery.** A consumer removes duplicates with `EVOLVE_EVENT_ID`.
- **No order across channels.**
- **No change to the ledger** or its hash chain. The `ledger` channel only notifies, with `fields.entry_seq` as the pointer.
- **No change to the per-cycle `signals.ndjson`.** It stays the cycle record, and a test pins it to the `signals` channel filtered to cycle N.

## Limits

- **The GitHub API pollers stay.** `ci watch`, `ci classify --rerun` and `pr merge --wait` read the GitHub API. No inbound endpoint exists, and `gh webhook forward` is for tests only (research F6.5). `ci watch` publishes `ci.completed`, so local consumers are pushed. The operator accepted these as documented limits.
- **An OS crash can lose a tail.** With no fsync, a power loss can drop the unsynced end of a segment. A later append can then use a cursor again before a reader sees the loss. The reset rule (§4) covers a reader that sees it. The operator accepted the rest.
- **A stopped writer causes losses on its channel.** A process can stop while it holds a channel lock. Then each lossless append of that channel waits up to the lock deadline, and then counts a loss. The ledger lock has the same exposure, without a deadline. The operator accepted it.
- **No order across channels.** A merged view is ordered by time and labelled as such.
- **A best-effort channel can lose events.** A gap record names each loss.
- **Network filesystems and other operating systems are refused.**
- **No exit watch without `pidfd`.** The Linux exit watch needs `pidfd_open` (Linux 5.3 or later). An older kernel gives `ENOSYS`, and the waiter refuses with `ErrRefused` (exit 1). No fallback exists.
- **darwin and Linux differ in what wakes.** On darwin, a file watch follows the file that the waiter opened. A rename away or a replacement wakes it once, and then the caller arms again. On Linux, the watch is on the directory, so a write to any file in it wakes the reader. Both give spurious wakes, and the caller reads again and finds nothing new.
- **Two inotify instances for each `Arm`.** Each `Arm` makes a new inotify instance before it closes the old one. Thus a Linux reader holds two instances for a short time.
- **Linux inotify limits.** Each process that watches uses one inotify instance, and two during an `Arm`. The kernel defaults are 128 instances for each user, 16,384 queued events, and 8,192 to 1,048,576 watches from the RAM size (research F2.10).
- **The runtime wake.** An idle watcher still wakes about once a minute for Go runtime work.
- **Descriptors.** A darwin reader holds one descriptor for each watched directory and tail segment, plus the kqueue.
- **A hung disk holds the drain.** The lock deadline bounds the wait for the channel lock, not a `write` that the kernel holds. The console accepted it as a known limit; the operator can override it.
- **One waiting lock call for each log, not for each process.** The rule of §5 is kept by one `channel.Log`. Two Publishers in one process have two logs for a channel, and thus can have two waiting lock calls. This is accepted. A later process-wide registry of waiting calls can close it.
- **A deleted lock file splits the lock.** If someone deletes `ch/<channel>.lock` while a call waits, the waiter holds the old inode. A new writer then locks a new file. The current `flock` adapter has the same exposure.
- **A time start is a scan.** `--since` with a time reads from the oldest retained segment. The cost grows with the retention.
- **The duplicate window is in memory.** After a restart, a copy of an event in a second channel can run again.
