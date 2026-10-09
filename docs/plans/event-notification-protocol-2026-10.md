# Plan: event channels, a push-only publish/subscribe protocol for `evolve events`

- **Status:** design, opened 2026-10-09 in the docs lane `cl-events` (branch `docs/events-subscriptions`, base `9b47ca6ae`). The operator approved the console plan on 2026-10-09. This file is its mirror in `docs/`, with its research amendments, the operator decisions of fix round 1 and the review fixes.
- **Spec of record:** [event-channels.md](../architecture/event-channels.md). It holds the protocol, the command surface, the exit codes and the limits. This plan holds the decisions, the components and the tests.
- **Decision record:** [ADR-0127](../architecture/adr/0127-push-only-event-channels.md).
- **Research:** [event-notification-protocol-2026-10.md](../research/event-notification-protocol-2026-10.md) (findings F1.1 to F7.3, refinements R1 to R24).
- **Producer path:** [signal-center-design.md](../architecture/signal-center-design.md) ([ADR-0101](../architecture/adr/0101-signal-center.md)).

## Table of contents

1. [Request](#1-request)
2. [Facts: the poll audit](#2-facts-the-poll-audit)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Operator decisions](#4-operator-decisions)
5. [Candidate designs](#5-candidate-designs)
6. [Decisions](#6-decisions)
7. [The Signal Center through the channels](#7-the-signal-center-through-the-channels)
8. [The command surface](#8-the-command-surface)
9. [Migrations](#9-migrations)
10. [Components](#10-components)
11. [TDD protocol](#11-tdd-protocol)
12. [Verification](#12-verification)
13. [Patterns and forces](#13-patterns-and-forces)
14. [Risks and limits](#14-risks-and-limits)
15. [Open questions](#15-open-questions)
16. [Status](#16-status)

## 1. Request

The operator wrote this on 2026-10-09:

1. *"Design a CLI cmd that supports registered notification, allowing other programs to listen to a specific event / loop / errors / etc. Those behaviors and use cases must go through the same command interfaces / APIs / CLI cmds."*
2. *"I don't like pulling the status by continuous query. Design the protocol that only notifies when an event is updated."*

The coordinator added four requirements on the same day:

- **Push, never poll.** A consumer that waits uses no CPU and wakes only when a new event exists. This includes the wait for a producer that has not started.
- **Expose the Signal Center.** Every Center event reaches external subscribers through the same verbs and the same filter grammar.
- **Multi-channel publish/subscribe.** Producers publish to named channels. Each user or program chooses its channels. Back-pressure is set for each channel.
- **IPC and pub/sub research.** The dossier covers both, and the research amendments in §6 follow from it.

## 2. Facts: the poll audit

| Verb or part | Poll today | Evidence |
|---|---|---|
| `evolve signals tail --follow` | yes: a 1 s timer; each tick lists `runs/` and the leases again | `go/cmd/evolve/cmd_signals.go:27`; `cmd_signals_tail.go:61-64`, `:71-78`, `:270`, `:298-307` |
| `evolve signals tail` (no `--follow`) | no: one read | `cmd_signals_tail.go:75` |
| `evolve wave watch` | yes: a 5 s sleep, a diff of two snapshots, one `ps` process for each tick | `cmd_wave.go:27`, `:101-125`; `cmd_wave_status.go:144-154`; `internal/wave/events.go:45-77` |
| `evolve ledger tail` | no: one read of the whole ledger | `cmd_ledger.go:252-291` |
| `evolve ci watch` | yes: `gh run list` every `ci_watch.poll_s` (default 30 s) | `internal/ciwatch/ciwatch.go:139-147`, `:151-166`; `cmd_ci_watch.go:153` |
| `evolve ci classify --rerun` | yes: `time.After(poll)` | `internal/ciwatch/classify_source.go:240-264` |
| `evolve pr merge --wait` | yes: a poll of the CI status | `cmd_pr.go:352-371` |
| `evolve bridge watch --follow` | yes: a 500 ms ticker on the channel feed | `cmd_bridge_watch.go:121-122`, `:134-180` |
| `evolve loop-stop --wait` | yes: a 200 ms lease poll | `cmd_loop_stop.go:19`, `:69-88` |
| `evolve loop --detach` (boot wait) | yes: a 200 ms ticker on the leases | `cmd_loop_detach.go:26`, `:109-133` |
| `evolve dashboard` | yes: a 2 s scan of file times, then SSE to the browser | `internal/dashboard/server.go:38-40`, `:141-153`; `sse.go:20-54` |
| phase watchdog | yes: reads file times every 15 s | `internal/phasewatchdog/phasewatchdog.go:34`, `:64-65`, `:195` |
| channel producer (bridge) | yes: a 2 s ticker on the phase output log | `internal/bridge/channel/producer.go:53`, `:106-127` |
| channel supervisor (bridge) | yes: a 500 ms ticker on the feed | `internal/bridge/channel/supervisor.go:86`, `:124-160` |
| phase observer | yes: a ticker on the phase output | `internal/adapters/observer/observer.go:117` |
| `dispatchevents` | no: a writer only | `internal/dispatchevents/events.go:132-159` |
| `panewatch` | no: an atomic writer; its readers read once | `internal/panewatch/panewatch.go:54-60`; `cmd_bridge_sessions.go:100-111` |

More facts:

- The wave plan chose a poll (D12), and it states the cost: "A phase shorter than the poll interval does not show" ([evolve-wave-cli-2026-10.md](evolve-wave-cli-2026-10.md)).
- Liveness today is a lease heartbeat with a 10-minute TTL and a pid probe (`go/internal/runlease/runlease.go:62`, `:147-158`).
- A lease names the process that runs a cycle (`go/internal/core/runlease_hook.go:33`), and a fleet lane runs in its own process (`go/cmd/evolve/cmd_fleet.go:127`).
- The load is about 2,000 events per day, and `ledger.appended` is 82% of it. The peak is 50 events in one second (research F1.1, F1.2).
- `phase.dispatched` and `ship.landed` exist as kinds, but no code emits them (research F1.3).

## 3. Goals and non-goals

Goals:

- **G1.** Other programs subscribe to channels and get each new event, through one command surface.
- **G2.** No consumer, verb or delivery runner polls. A blocked consumer uses no CPU, and only the kernel wakes it.
- **G3.** Every Signal Center event can reach an external subscriber, with the full `signal/1.0` envelope.
- **G4.** A consumer that acks after its work resumes with no loss inside the retention window. That is an exec runner, or a pull consumer with `--manual-ack`. Every loss is a named gap.
- **G5.** A slow subscriber never slows a publisher. It only lags.
- **G6.** A registered command runs once for each event that matches, at least once, with a dead letter for a poison event.
- **G7.** The current watch verbs become views over channels, and their flags keep their meaning.
- **G8.** A watcher whose reader is gone exits. No orphan watcher stays.

The spec holds the non-goals ([event-channels.md](../architecture/event-channels.md) §17 and §Limits). They are:

- no host-wide bus and no webhook in v1;
- no exactly-once delivery;
- no change to the ledger or the per-cycle `signals.ndjson`;
- no network filesystem and no poll fallback.

The plan adds one non-goal: no feature flag and no env flag, because the channel catalog is config in `policy.json`.

## 4. Operator decisions

| # | Decision | Source |
|---|---|---|
| O1 | v1 delivery is pull consumers plus `exec:CMD`. Webhook is a later phase; only its seam is designed. | brief |
| O2 | The scope is one journal home for each project `.evolve/`. A seam for a plane tag stays. | brief |
| O3 | Push, never poll: no consumer, verb or delivery worker sleeps and reads again. | brief (hard requirement) |
| O4 | Expose the full Signal Center through the protocol. | coordinator, 2026-10-09 |
| O5 | Multi-channel publish/subscribe. Each channel has its own log, cursors, retention and QoS. | coordinator, 2026-10-09 |
| O6 | No central delivery worker. Whoever registers a subscription runs its consumer. | coordinator, 2026-10-09 |
| O7 | A poison event goes to a dead letter, and delivery continues. | operator |
| O8 | The per-cycle `signals.ndjson` stays. The project-level `.evolve/signals.ndjson` retires into a firehose channel. | operator |
| O9 | The research amendments: dotted channel names with wildcards, `--since` modes, a last will, and consumer groups (E13). | console plan, approved 2026-10-09 |
| O10 | Fan-out is the default delivery. Work-queue groups come in E13 (was Q1). | operator, 2026-10-09 |
| O11 | Network filesystems are refused (was Q2). | operator, 2026-10-09 |
| O12 | The per-cycle `signals.ndjson` stays, pinned to the `signals` channel by a test (was Q3). | operator, 2026-10-09 |
| O13 | Accept the risk of an unsynced tail after an OS crash: no fsync and no hash. The reset rule stays (was Q4). | operator, 2026-10-09 |
| O14 | `--since last` reads the last record of the tail segment. No separate file holds it (was Q5). | operator, 2026-10-09 |
| O15 | The Publisher stamps the process's `EVOLVE_DISPATCH_ID` in each record. A runner skips the records that its commands caused (was Q6). | operator, 2026-10-09 |
| O16 | No `--timeout` in v1; `--until` covers it (was Q7). | operator, 2026-10-09 |
| O17 | Component E14 moves the other pollers to push after E12. The GitHub API pollers stay as documented limits (was Q8). | operator, 2026-10-09 |
| O18 | Accept the stall exposure of a channel lock, as for the ledger (was Q9). | operator, 2026-10-09 |
| O19 | Retention: 30 days; 128 MB for each lossless channel; 512 MB for `ledger` and `signals` (was Q10). | operator, 2026-10-09 |

## 5. Candidate designs

### 5.1 A: a broker in the loop process, with journal replay

The loop hosts a Unix socket server. A consumer connects, sends a filter and a cursor, gets the history from a journal and then live events from memory.

**Steelman.** It has the lowest latency. The server filters, so a consumer wakes only for its events. Access control has one home. Docker, Kubernetes and NATS all use a broker (research F3.9, F4.5, F4.11). A remote API later is natural.

**Rejected:**
- The loop is the process that must never stall or crash for a listener. A broker adds connection state to its heap and its failure modes.
- It sees only the loop's own events. Ship, the console verbs, subagents and lanes need a second path, which is the journal.
- With no loop, no broker exists. A consumer then needs a file watch to find the socket, which is design B.
- macOS refuses `SOCK_SEQPACKET`, and a socket path in a worktree passes 104 bytes (research F1.4, F1.5).
- The speed gain is in microseconds (research F3.1 to F3.3). The load is 2,000 events per day.

### 5.2 B: channel logs and a kernel wake, with no broker (the winner)

Each process appends to channel logs under a lock. Each reader arms a kernel watch, reads what is new and waits. Channels give each reader a wake only for its topics, as D-Bus match rules do (research F4.10).

### 5.3 C: a hybrid

B stays the source of truth. A broker in the loop adds a fast live path and server filters.

**Steelman.** It keeps the durability of B and adds the speed of A, and it can serve remote consumers.

**Rejected for v1:**
- Two wake paths must agree, and the switch from catch-up to live needs a dedupe.
- The broker still needs a supervisor.
- The gains are not needed at this load.
- A broker can come later as one more consumer of the logs (research amendment).

### 5.4 Other designs

| Design | Why not |
|---|---|
| One journal for all events, with filters at read time | No back-pressure or retention for each topic. A reader of `loop` wakes for every ledger append (82% of the load, research F1.2). The operator chose channels (O5). |
| A FIFO for each subscriber | The producer depends on its subscribers. macOS `PIPE_BUF` is 512, and Go cannot poll a FIFO on darwin (research F1.4, F1.7, F3.5). |
| Shared memory with a doorbell | Not durable. eventfd is Linux only, and the macOS wait needs macOS 14.4 (research F3.1, F3.6). |
| D-Bus, systemd journal, NATS, Kafka or Redis | A daemon or a new dependency. The module has two dependencies (`go/go.mod`). |

### 5.5 Scores

| Force | A: broker in the loop | B: channel logs (winner) | C: hybrid |
|---|---|---|---|
| No poll | yes, but it needs a file watch to find the broker | yes | yes |
| No stall from a slow consumer | needs a buffer and a cut-off in the loop | yes: readers never take the lock | as A on the live path |
| Resume after a restart | yes, through the journal | yes: a stored cursor | yes, with a seam |
| Works with no loop | no | yes: the first append wakes the reader | partly |
| Crash safety | the broker dies with the loop | the log is the truth | as A on the live path |
| darwin and Linux | socket path limit; no seqpacket | kqueue and inotify in the stdlib | both sets of limits |
| Complexity | a server, message frames, reconnects and a journal | an appender, a reader and a waker | the sum of A and B |
| Testability | a socket server and its clients | a temp directory and a fake `Waiter` | two paths and their seam |

## 6. Decisions

| # | Decision | Reason |
|---|---|---|
| D1 | Every event is a Signal Center event. Wave, loop, ship, inbox and CI lifecycle events are emitted into the Center. | One producer path, one vocabulary (O4). |
| D2 | The `Publisher` is one Center listener. `newRootSignalCenter(role)` wires it and returns `close(deadline)`. | One topology for every root (`cmd_cycle.go:352`). |
| D3 | The Publisher never emits and never calls `Flush`. | Listeners observe and never decide (ADR-0101). |
| D4 | The channel catalog is config: `policy.json` `events.channels`, with compiled defaults. | Phase settings come from config (house rule). |
| D5 | A channel has a name, a route in the filter grammar, a QoS class and a retention. | One grammar, one home (R20). |
| D6 | Names are dotted. `*` matches one token, and `>` matches the rest. A wildcard selection follows new channels. | NATS subjects (R11). |
| D7 | Each channel is its own log. The cursor is the byte offset of a line in its channel. | No tip file; a process crash cannot repeat a cursor (R2). |
| D8 | Order is total within a channel. Readers of several channels merge by time and label it. | Kafka partition semantics (R3). |
| D9 | An append takes the channel lock with a deadline, repairs a torn tail and writes the batch in one write. No fsync. | The ledger idiom with a bound (R24, O13). |
| D10 | `lossless` channels append inside the Publisher; `best_effort` channels go through a bounded queue. A loss is a counted gap. | Back-pressure for each channel (O5, R12). |
| D11 | Every wait is arm, then catch up, then wait. `EINTR` retries with the time left. | No lost wake, no spin (R4 to R6). |
| D12 | Other operating systems and network filesystems are refused. No poll fallback. | The hard requirement (O3, O11, R7). |
| D13 | `--since` takes `all`, `new`, `last`, cursors or an RFC 3339 time. `last` is the last record of the tail segment. | O14. |
| D14 | The last will seeds its live set with a scan of `loop.started` records. Then it arms an exit watch and compares `proc_start` exactly. A loop that was gone before the reader looked raises no alarm. | The console plan used the run lease, but a lease pid can be a fleet lane (§2). A stale record must not raise an alarm. |
| D15 | A subscription is a config file and a cursor file under `.evolve/events/subs/`. Only the CLI writes the config. | Each field has one owner. |
| D16 | `subscribe run NAME` is the consumer. Whoever registers the subscription runs it. | O6. |
| D17 | Delivery is serial, at-least-once and acked after success. The event id is the idempotency key; the position is the channel and cursor. | R15. |
| D18 | A fixed retry schedule, then a dead letter, an ack past the event and a WARN. | O7, R16. |
| D19 | Each channel is a gc category with a TTL, a size cap and a `Protect` hook. | The log catalog (R21, O19). |
| D20 | Consumer groups (E13): one lock for each runner, a shared cursor, a PEL, an ack for each message, claims and `max_deliver`. | Kafka share groups, Redis groups (R19). |
| D21 | The per-cycle `signals.ndjson` stays as the cycle record. | O8, O12; it is synchronous and it never drops. |
| D22 | The GitHub API pollers stay: `ci watch`, `ci classify --rerun` and `pr merge --wait`. `ci watch` publishes `ci.completed`. | No inbound endpoint (R22, O17). |
| D23 | The Publisher stamps the process's dispatch id. A runner skips the records that its subscription caused. | O15. |
| D24 | An exec subscription skips the module `events` unless its filter names it. | A dead letter must not feed another subscription. |
| D25 | A watcher exits 3 when the reader of its output is gone. | G8; 48 orphan watchers in this session (R23). |
| D26 | A reader removes duplicates across channels with a window of 4,096 event ids. | One event can be in two selected channels. |
| D27 | The reset rule moves a cursor past an end to the true end, with a gap. | O13; a gap is never silent. |
| D28 | Pull mode acks after the stdout write by default, and `--manual-ack` gives at-least-once. | G4. |
| D29 | A gap passes every filter, but it never satisfies `--until`. | A gap is not the event that a wait waits for. |
| D30 | E14 moves the other pollers to push: a process exit watch, a file wake or a channel event. | O17. |

## 7. The Signal Center through the channels

- **The bridge is a Center listener** (D2). It never blocks `Emit` on a subscriber. For a `best_effort` channel, the listener only puts the event on a queue. For a `lossless` channel, the drain does one locked write of 4,608 bytes or less, with a lock deadline. The per-cycle sink already does one write for each event in the drain.
- **The drop semantics** come from `ndjsonSink.reportDrops` (`go/internal/signalcenter/sinks.go:108`): count the losses and report them once, on the next good write. Here the report is a gap record with the `pid`, the `seq` range and the count. The Publisher does not emit it, because a listener never emits (D3).
- **The record shape.** A stored record is `{"source":<role>,"dispatch":<id>,"signal":<signal/1.0>}`. The wire record adds `schema_version`, `id`, `channel` and `cursor` ([event-channels.md](../architecture/event-channels.md) §3).
  - `signal.seq` stays the order of one process, and `cursor` is the order of one channel.
  - `cycle`, `run_id` and `phase` stay in `signal` without a change.
- **One catalog.** `evolve events kinds` prints the closed `Module` and `Kind` sets and the registered codes (`event.go:68-172`, `registry.go`). `evolve signals codes` still generates `signal-codes.md` from the same registry. No second vocabulary exists.
- **The other sources go into the Center first.** Recommendation: yes. The reasons:
  - one interface (`Emit`), one validation (`Normalize`) and one vocabulary;
  - the per-cycle record and the orchestrator's summary see the same events;
  - most of these events already go through the Center: `ledger.appended`, `loop.wave`, `loop.halt`, `cycle.sealed` and `quota.paused`.

  The cost is two new producers and some new kinds (§10, E11). A rich payload stays in its artifact, with `fields.path` as the pointer, as ADR-0101 says.
- **Many Centers, one order for each channel.** Each process has its own Center and `seq`. The channel lock orders their appends, so each channel has one total order.
- **`signals tail` becomes a view** over the `signals` channel (§9). The per-cycle `signals.ndjson` stays as the cycle record, for three reasons:
  - it is written in the drain and it never drops, and the `signals` channel is best effort;
  - triage, the dashboard, the wave facts and `jq` recipes read it;
  - it lives and dies with its run directory.
- **The project-level `.evolve/signals.ndjson` retires** (O8).
  - The root topology wraps the per-cycle sink in a cycle filter, so cycle 0 events never reach it.
  - An empty path is not the way, because the sink counts an empty path as a drop and emits `signalcenter.sink_dropped` (`sinks.go:66-69`, `:108`).
  - The `loop` and `errors` channels keep the cycle 0 events, and they are lossless.

## 8. The command surface

The spec of record is [event-channels.md](../architecture/event-channels.md) §13: the verbs, the flags, the exit codes and the output form. This plan does not repeat it.

## 9. Migrations

Component E12 moves the watch verbs. Each verb becomes a view over channels and keeps its flags.

| Today | Becomes | Flags kept |
|---|---|---|
| `signals tail --follow` (1 s poll) | a view over `signals`; `--cycle N` adds `cycle=N` and ends at that cycle's `cycle.sealed` | `--cycle`, `--kind`, `--code`, `--follow`, `--json` (prints `.signal`) |
| `wave watch` (5 s poll) | a view over `loop` and `cycle` with the same lines; it ends at `loop.exit` (exit 0) or `loop.lost` (exit 4) | `--json`, `--project-root` |
| `bridge watch --follow` (500 ms ticker) | the `wake` package on its feed file; the feed is not a channel | `--workspace`, `--agent`, `--follow` |
| phase watchdog (15 s poll) | wakes on writes; keeps one deadline, because "nothing happened" needs a timer | its config |
| Claude Monitors | `evolve events watch --channel loop,cycle --json` | none |
| `ledger tail` (no poll) | no change; the live stream is `evolve events watch --channel ledger` | `--evolve-dir`, `--n` |

Component E14 moves the other pollers after E12 (O17). Each one uses the check-then-watch rule (§13).

| Today | Becomes |
|---|---|
| `loop-stop --wait` (200 ms) | a watch of the `loop` channel until `loop.exit` or `loop.lost`, with its current `--timeout` as a one-shot deadline |
| `loop --detach` boot wait (200 ms) | a watch of the `loop` channel until the child's `loop.started`; the child's exit already ends the wait; the boot wait is a one-shot deadline |
| `evolve dashboard` (2 s scan) | the `wake` package on the paths of its fingerprint; a wake runs one refresh |
| bridge channel producer (2 s) | the `wake` package on the phase output log |
| bridge channel supervisor (500 ms) | the `wake` package on the feed; its current timeout is a one-shot deadline |
| phase observer (ticker) | the `wake` package on the phase output; the stall threshold is one deadline |

The GitHub API pollers stay (D22): `ci watch`, `ci classify --rerun` and `pr merge --wait`. They are documented limits in the spec.

## 10. Components

Each component is small and lands unwired first. Each new package is at 100 in `go/.cover-strict` and listed in `go/.apicover-enforce`.

| # | Component | Reuses |
|---|---|---|
| E0 | Research dossier, plan, spec and ADR (this lane) | the research lane |
| E1 | `signalcenter.ReadLines(path, from)`: a byte reader that tolerates a torn tail, split out of `readStreamFrom` | `stream.go:19` |
| E2 | `internal/events/channel`: segment names, the locked append with tail repair, rotation, `Read(from)` and the last record. Sub-component: `flock.LockWithin`, the lock with a deadline. | `adapters/flock`, E1 |
| E3 | `internal/events/wake`: `Waiter{Arm, Wait(ctx, deadline)}` over a syscall port, in `wake_darwin.go`, `wake_linux.go` and `wake_other.go` (refuses) | the `proctree/procargs_*` build-tag pattern |
| E4 | `internal/events/filter`: the grammar, the matcher and the catalog from the registries | `event.go`, `registry.go` |
| E5 | `internal/events/reader`: arm, catch up and wait over channels; rotation; gap and reset records; the duplicate window; the last will. Also `proctree.StartOf(pid)`, the process start time: `procstart_darwin.go` (`KERN_PROC_PID` `kinfo_proc` with a general `sysctl` helper) and `procstart_linux.go` (the start ticks of `/proc/<pid>/stat` and the boot id) | E2 to E4, `MergeByTS`, `proctree` |
| E6 | `internal/events/publisher`: routes; the lossless path; the best-effort queue; gap records; the dispatch stamp; `Close(deadline)` | E2, E4, the `reportDrops` pattern |
| E7 | `policy_events.go` (`EventsConfig()`), the gcpolicy channel categories and `Protect` | `policy_ciwatch.go`, `gcpolicy/logs.go` |
| E8 | `internal/events/subs`: the config and cursor stores, the exec runner, pull acks, dead letters and the protected surface entry. Note (a): E8 adds the additive `Filter.Names(key, value) bool` for the `module=events` skip (D24), so the grammar keeps one home. Note (b): a stored filter whose kind a later build removed gives `ErrRefused` at load. E8 decides between two rules: refuse to start the runner with exit 1 and a message that names the kind (recommended), or dead-letter each record. | `atomicwrite`, `WithPathLock`, `sysexec.Command`, `proctree` |
| E9 | The CLI: `events channels`, `kinds`, `watch` and `subscribe` | the dispatch style of `cmd_wave.go` |
| E10 | Wiring: `newRootSignalCenter(role)` returns a closer at every root; a root-list guard test | `cmd_cycle.go:352` |
| E11 | Producers: `phase.dispatched`, `ship.landed`, `wave.*`, `loop.started`, `loop.exit`, `inbox.claimed`, `inbox.released`, `ci.completed`, and their codes | `signal_cycle.go`, `inboxmover/lifecycle` |
| E12 | Migrations: `signals tail`, `wave watch`, `bridge watch`, the phase watchdog; retire `.evolve/signals.ndjson` | the files in §2 |
| E13 | Consumer groups: runner locks, the PEL, an ack for each message, claims, `max_deliver` | E8 |
| E14 | Push for the other pollers: `loop-stop --wait`, the detach boot wait, the dashboard, the bridge producer and supervisor, the phase observer | E3, E5 |

- Each component is its own lane and pull request, merged at a wave boundary.
- Each one follows the usual chain: red tests first, then the simplifier, then the Go and architecture reviewers, then the full floor.
- E1 to E9 ship unwired. E10 to E14 change live behavior.

## 11. TDD protocol

The rules for each component:

1. Write one test for one acceptance criterion. Run it, and see it fail on its named assertion. Keep the red output in the lane scratchpad.
2. Write the smallest code that passes it.
3. Run a mutant of each fix in a scratch copy, and see the test fail.
4. Unit tests fake the clock, the syscall port, the process table and the command runner. No test sleeps.
5. New packages reach 100% line and API coverage. Changed functions in mixed packages reach 100% each.
6. Run `go test -count=1 -race` and the full floor before the review.

The red tests, named by their acceptance criteria:

| # | Red tests |
|---|---|
| E1 | `TestReadLines_ATornTailIsNotReturnedUntilItsNewlineArrives`, `TestReadLines_NextIsTheOffsetAfterTheLastCompleteLine`, `TestReadLines_AMalformedCompleteLineIsCountedNotFatal` |
| E2 | `TestAppend_ConcurrentProcessesWriteUniqueWholeLines`, `TestAppend_OpensTheTailWithAppendUnderTheLock`, `TestAppend_RepairsATornTailBeforeTheBatch`, `TestAppend_RotatesPastTheCapAndCursorsKeepIncreasing`, `TestRead_ACursorFindsItsSegmentAcrossRotation`, `TestRead_LastIsTheLastRecordOfTheTailOrTheSegmentBefore` |
| E2 lock | `TestLockWithin_GetsAFreeLockAtOnce`, `TestLockWithin_ReturnsAtTheDeadlineWhileAnotherHolderKeepsTheLock`, `TestLockWithin_AnAbandonedCallReleasesTheLockWhenItArrives`, `TestLockWithin_ADeadlineAtTheGrantInstantHasExactlyOneOwner` |
| E3 | `TestWaiter_AnAppendWakesTheWaiter`, `TestWaiter_ASecondWaitAfterOneWriteTimesOut`, `TestWaiter_AWatchOnAnAbsentDirectoryWakesOnTheFirstSegment`, `TestWaiter_ProcessExitWakesTheWaiter`, `TestWaiter_CancelReturnsAtOnce`, `TestWaiter_AnEINTRRetriesWithTheTimeLeft`, `TestWaiter_AStdoutHangupWakesTheWaiter`, `TestWaiter_RefusesANetworkFilesystem`, `TestWaiter_AKernelRefusalFailsLoudly`, `TestNewWaiter_RefusesASystemWithoutABackend` |
| E4 | `TestParse_KeysAreANDedAndValuesAreORed`, `TestParse_AKindGlobThatMatchesNoKindIsRefused`, `TestParse_AnUnknownCodeOnlyWarns`, `TestParse_AGlobOnAnotherKeyIsAUsageError`, `TestMatch_SeverityOrdersInfoWarnIncident`, `TestMatch_AnAbsentKeyMatchesOnlyNotEqual`, `TestMatch_GapRecordsPassEveryFilter`, `TestUntil_AGapRecordNeverMatches`, `TestMatch_NotEqualIsTheComplementOfEqualForPresentKeys` (a `rapid` property; no gaps, no absent keys) |
| E5 | `TestReader_CallOrderIsArmThenOneReadThenWait`, `TestReader_WithNoDeadlineCreatesNoTimer`, `TestReader_FollowsRotationWithoutALostLine`, `TestReader_ACursorBelowRetentionYieldsOneGapRecord`, `TestReader_ACursorPastTheTailYieldsAResetGapAtTheTrueEnd`, `TestReader_ACursorPastASegmentEndMovesToTheNextBase`, `TestReader_AWipedDirectoryYieldsAResetGapAndArmsAgain`, `TestReader_AMalformedLineYieldsAGapAndTheCursorMovesPastIt`, `TestReader_TheSameEventInTwoChannelsIsDeliveredOnce`, `TestReader_SinceLastStartsAtTheLastRecord`, `TestReader_AWildcardSelectionAddsANewChannel` |
| E5 will | `TestLastWill_AWatchArmedMidLoopGetsALastWill`, `TestLastWill_AStaleLoopStartedGivesNoAlarm`, `TestLastWill_SinceAllReportsAStaleLoopOnceAsHistory`, `TestLastWill_ALoopThatDiesBetweenTheSeedScanAndTheArmIsLost`, `TestLastWill_AReusedPidWithAnotherProcStartIsGone`, `TestLastWill_ArmsTheExitWatchBeforeTheSecondProcStartRead`, `TestLastWill_ALoopExitFoundOnCatchUpIsNoLoss`, `TestLastWill_AnIncidentGapFromThePidMakesTheExitUnknown`, `TestLastWill_ASeedBeyondRetentionWritesAGap`, `TestLastWill_ALoopThatStartsLaterIsWatched`, `TestLastWill_TwoLiveLoopsAreWatchedApart`, `TestLastWill_ALaneExitIsNotALostLoop` |
| E6 | `TestPublisher_LosslessEventIsOnDiskWhenFlushReturns`, `TestPublisher_QueueOverflowWritesOneGapWithTheExactSeqRange`, `TestPublisher_AWriteErrorIsACountedLossWithAnIncidentGap`, `TestPublisher_ALockPastItsDeadlineNeverHoldsTheDrain`, `TestPublisher_OneLockCallWaitsForEachChannel`, `TestPublisher_CloseUnderAHeldLockReturnsTheCount`, `TestPublisher_ASlowReaderNeverDelaysAnAppend`, `TestPublisher_CloseDrainsTheQueueBeforeTheDeadline`, `TestPublisher_RoutesOneEventToEveryMatchingChannel`, `TestPublisher_StampsTheDispatchIDOfItsProcess`, `TestPublisher_CutsALongDispatchStampTo256Bytes` |
| E7 | `TestEventsConfig_CompiledDefaultsMatchTheChannelTable`, `TestEventsConfig_AnUnknownKeyFails`, `TestEventsConfig_ARouteMustParse`, `TestLogCatalog_HasOneCategoryForEachChannel`, `TestPlan_ProtectKeepsTheCursorSegmentAndEveryLaterOne`, `TestPlan_ProtectUsesTheOldestPELCursorOfAGroup`, `TestPlan_NeverDeletesTheTailSegment` |
| E8 | `TestRunner_DeliversSeriallyInCursorOrder`, `TestRunner_AcksOnlyAfterExitZero`, `TestRunner_RetriesThenDeadLettersAndContinues`, `TestRunner_RedeliversWhatWasNotAckedAfterACrash`, `TestRunner_SkipsTheRecordsThatItsCommandsCaused`, `TestRunner_NeverGetsTheEventsModuleUnlessTheFilterNamesIt`, `TestRunner_ASecondFanOutRunnerIsRefused`, `TestRunner_TwoGroupRunnersHoldTheirOwnLocks`, `TestRunner_ATimeoutSendsSIGTERMThenSIGKILL`, `TestRunner_APausedSubscriptionDeliversNothing`, `TestRunner_SetsTheEventIDAndThePositionInTheEnvironment`, `TestRunner_TagsTheCommandAndReapsItsDescendants` |
| E8 subs | `TestPullAck_TheDefaultAcksAfterTheWrite`, `TestPullAck_ManualAckLeavesTheCursorUntilTheAck`, `TestSubs_AddChecksTheAckWaitOfAGroup`, `TestSubs_AddStoresTheSinceCursors`, `TestSubs_ACursorWriteDoesNotWakeAChannelWatcher`, `TestProtectedSurface_CoversTheEventsTree` |
| E9 | `TestEventsCLI_ExitCodesMatchTheTable`, `TestEventsWatch_UntilPrintsTheMatchAndExitsZero`, `TestEventsWatch_AGapNeverEndsAnUntilWatch`, `TestEventsWatch_EachSinceModeStartsWhereTheSpecSays`, `TestEventsWatch_AnEmptySelectorIsRefused`, `TestEventsWatch_AnUnreadableChannelExitsTwo`, `TestEventsWatch_EPIPEExitsThree`, `TestEventsWatch_AStdoutHangupExitsThree`, `TestEventsWatch_SIGINTExitsZero`, `TestEventsWatch_SubResumesFromTheStoredCursor`, `TestEventsSubscribe_AddWritesTheConfigOnly` |
| E10 | `TestRootSignalCenter_EveryRootSubscribesThePublisher`, `TestRootSignalCenter_EveryRootDefersItsCloser`, `TestRootSignalCenter_CycleZeroEventsNeverReachTheCycleSink` |
| E11 | `TestDispatchPhase_EmitsPhaseDispatched`, `TestLanding_EmitsShipLandedAtIntentComplete`, `TestWaveNext_EmitsWaveStarted`, `TestLoop_EmitsLoopStartedWithItsProcStart`, `TestLoop_EmitsLoopExitAtExit`, `TestInboxLifecycle_EmitsClaimedAndReleased`, `TestCIWatch_EmitsCICompletedAndARedRunWarns`, `TestRegistry_NewKindsAndCodesAreRegistered` |
| E12 | `TestSignalsTail_FollowIsAViewOverTheSignalsChannel`, `TestWaveWatch_PrintsTheSameLinesFromChannels`, `TestWaveWatch_ALostLoopExitsFour`, `TestWaveWatch_AStaleOrHistoricalLossNeverExitsFour`, `TestBridgeWatch_FollowWakesOnTheFeedWithoutATicker`, `TestPhaseWatchdog_WakesOnWritesAndKeepsOneDeadline`, `TestCycleRecord_EqualsTheSignalsChannelForTheCycle`, `TestNoPollTimerInTheEventChannelSources` (E12 adds its files to the directory list) |
| E13 | `TestGroup_EachMessageHasItsOwnAck`, `TestGroup_LocalTriesDoNotCountAsDeliveries`, `TestGroup_ClaimsTheEntryOfADeadRunnerOnItsExit`, `TestGroup_ClaimsWhenTheRunnerLockIsFree`, `TestGroup_ClaimsAStuckEntryAtTheAckWaitDeadline`, `TestGroup_DeadLettersPastMaxDeliver` |
| E14 | `TestLoopStopWait_EndsOnLoopExitWithoutAPoll`, `TestLoopDetach_ConfirmsTheBootOnLoopStarted`, `TestDashboard_RefreshesOnAWake`, `TestBridgeChannelProducer_WakesOnTheOutputLog`, `TestBridgeSupervisor_WakesOnTheFeed`, `TestPhaseObserver_WakesOnWritesAndKeepsOneDeadline` |

How "no poll" is proved with no sleep:

- **A fake `Waiter`** records the calls. The order must be `Arm`, one read, `Wait`. The read count stays at 1 until the test releases a wake, and then it is exactly 2.
- **A fake clock** must never create a timer when a watch has no deadline (`TestReader_WithNoDeadlineCreatesNoTimer`).
- **A real-kernel test** on darwin and on Linux CI: after one write, a second `Wait` with a short deadline returns at the deadline. This proves `EV_CLEAR` and no spin.
- **The AST guard** of the spec (§15) is the test `TestNoPollTimerInTheEventChannelSources` in `go/test/structure/nopoll_guard_test.go`. E3 added it. It uses the shared walker (`structure.SourceFiles`, `structure.SelectorUses`) that the `sysexec` process guard also uses.

The proofs that drive real processes:

- The multi-process append race runs the test binary again N times, as `TestAppend_TwoProcessStress` does (`go/internal/adapters/ledger/ledger_crossproc_test.go:35`).
- The end-to-end tests build the binary as `go/test/e2e/version_smoke_test.go:27` does, and run `evolve events` as a subprocess.

## 12. Verification

- **No poll:** the four proofs in §11.
- **Order and crash safety:**
  - a multi-process append race gives unique lines with no interleave;
  - a torn tail is repaired;
  - cursors increase across rotation;
  - a cursor past an end gets one reset gap.
- **QoS:**
  - a lossless event emitted at exit is on disk after `close`;
  - an overflow of a best-effort queue gives a gap record with the exact `seq` range;
  - a write error or a lock deadline gives a counted loss and never holds the drain;
  - a slow subscriber never delays a publisher.
- **Delivery:**
  - delivery is serial and in order, with no self-loop;
  - the retry schedule, the dead letter and the ack past a poison event work;
  - after a crash, a restart delivers again what was not acked.
- **End to end:**
  - start `evolve events watch --channel loop --until kind=loop.exit` before `.evolve/events` exists, run a simulated cycle, and see exit 0 on the event;
  - register an `exec` subscription on `errors`, emit one WARN, and see the command get the event JSON exactly once;
  - close the reader of a watch's stdout, and see exit 3.
- **Live, after E12:** a real wave drives Claude's watch through `events watch --channel loop,cycle`. It prints no replay on a new arm (`--since new` or `--sub`) and no duplicate.

## 13. Patterns and forces

| Pattern | Where | Force |
|---|---|---|
| Observer | the Center and the `Publisher` listener | Producers stay unchanged; the Publisher observes only. |
| Specification | the filter terms and the channel routes | One grammar composes every selection. |
| Ports and adapters | the syscall port (`kqueue`, `kevent`, `inotify`, `epoll`, `pidfd_open`, `statfs`), the process start time (`proctree.StartOf`), the process lister and signaler, the clock | Every kernel and process boundary has a fake, so E3 reaches 100%. |
| Strategy | `Deliverer`: the command runner and the stdout writer | Two real implementations in v1; the webhook is the third. |
| Transactional outbox | the channel log | The log is the record, and the event id is the idempotency key. |
| Catch-up subscription | the reader: arm, catch up, wait | History, then live, with no gap at the seam. |
| Check, then watch | every migrated wait verb | Take the cursor, read the state once, then watch from the cursor. |

Not abstracted:

- The segment names and the QoS classes are data, not types.
- The filesystem allow list is a table in each build-tag file.
- No broker interface. A broker is a later consumer of the logs.

## 14. Risks and limits

The limits of record are in the spec: [event-channels.md](../architecture/event-channels.md) §Limits. The operator accepted three of them (O13, O17, O18).

## 15. Open questions

None. The operator decided Q1 to Q10 on 2026-10-09 (§4, O10 to O19).

## 16. Status

| # | Status |
|---|---|
| E0 | ☑ the research dossier, this plan, the spec and ADR-0127. Merged in #823. |
| E4 | ☑ `internal/events/filter`: the grammar, the matcher, the selectors and the catalog, at 100% coverage. Merged in #824. It is unwired. |
| E1 | ☑ `signalcenter.ReadLines`, with its red tests and mutants. Merged in #825 (with E2). |
| E2 | ☑ `internal/events/channel` and `flock.LockWithin` built, unwired, at 100% coverage. Merged in #825. Spec findings: [internal-events-channel.md](../architecture/packages/internal-events-channel.md) §Findings |
| E3 | ◐ `internal/events/wake`: the kqueue and inotify backends, the syscall ports, the refusals and the no-poll guard (moved to `go/test/structure`). Coverage is 100% on darwin and Linux. The PR is open. It is unwired. |
| E1 to E14, except the rows above | ☐ not started |
