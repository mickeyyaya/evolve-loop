# ADR-0127: Events reach other programs through channel logs that the kernel wakes, never through a poll

- **Status:** Proposed (2026-10-09). The operator approved the console plan and the decisions of fix round 1 on 2026-10-09. This ADR moves to Accepted as components E1 to E14 land ([plan](../../plans/event-notification-protocol-2026-10.md) §16).
- **Spec of record:** [event-channels.md](../event-channels.md). It holds the protocol, the command surface, the exit codes and the limits.
- **Plan:** [event-notification-protocol-2026-10.md](../../plans/event-notification-protocol-2026-10.md).
- **Research:** [event-notification-protocol-2026-10.md](../../research/event-notification-protocol-2026-10.md).
- **Amends:**
  - [ADR-0101](0101-signal-center.md), the Signal Center:
    - a `Publisher` listener at every root makes each event reachable outside its process;
    - `newRootSignalCenter(role)` returns a closer that each root calls at exit;
    - the project-level `.evolve/signals.ndjson` retires into the `signals` channel, and the per-cycle file stays ([design](../signal-center-design.md)).
  - [ADR-0064](0064-pipeline-integrity-boundary.md), the protected surface: it adds `/.evolve/events/`, so no phase agent writes a record or a subscription.
- **Evidence:**
  - Every watch verb polls: 1 s, 5 s, 500 ms, 30 s, 15 s and 200 ms (plan §2).
  - The wave plan states the loss: "A phase shorter than the poll interval does not show".
  - In this session, 48 orphan `tail -F` processes ran for up to 24 days. Monitors replayed old events, and events between polls were lost.
  - The load is about 2,000 events per day, and `ledger.appended` is 82% of it (research F1.1, F1.2).
  - Each local transport costs microseconds (research F3.1 to F3.3). Durability, replay and the wake are the limits, not speed.

## Context

The Signal Center (ADR-0101) is in-process: each process has one Center, and its listeners live in that process. A program outside the process can only read files, so every watch verb sleeps and reads again.

On 2026-10-09 the operator asked for registered notification: "allowing other programs to listen to a specific event / loop / errors / etc.", through one command interface. The operator also wrote: "I don't like pulling the status by continuous query. Design the protocol that only notifies when an event is updated."

Later the same day, the operator added three requirements:
- expose the full Signal Center;
- use multi-channel publish/subscribe, with back-pressure for each channel;
- run no central delivery worker.

## Decision

1. **One producer path.** Every event is a Signal Center event. A `Publisher` listener at every root appends it to channels.
2. **Channels.** A closed catalog in `policy.json` `events.channels` gives each channel a name, route, QoS class and retention.
3. **One log for each channel.** The cursor is a byte offset in that channel. Order is total within a channel only.
4. **Appends.** A channel lock with a deadline, a tail repair and one write for each batch.
   - No fsync. The operator accepts the risk of an unsynced tail after an OS crash.
5. **Back-pressure.** A `lossless` channel appends in the drain. A `best_effort` channel uses a bounded queue.
   - A publisher never waits on a subscriber. Every loss is a counted gap record.
6. **Push only.** Every wait is: arm, catch up, wait. Use kqueue with `EV_CLEAR` on darwin and inotify on Linux.
   - Other operating systems and network filesystems are refused. No poll fallback exists.
7. **One filter grammar** serves routes, filters, `--until` and subscriptions. The Signal Center registries are its catalog.
8. **Subscriptions are files.** `subscribe run NAME` delivers each event serially and at least once, with retries.
   - Whoever registers a subscription runs its consumer. Nothing in the loop supervises it.
   - A poison event goes to a dead letter, and delivery continues.
9. **Identity and position.** The event id (`pid.seq.ts`) removes duplicates. The channel and cursor give the position.
10. **No self-loop.** Records carry the dispatch id of their process. A runner skips the records that its commands caused.
11. **Observe, never decide.** No subscriber changes a verdict or blocks a phase. The Publisher never emits.
12. **Retention.** Each channel is a gc category: 30 days; 128 MB lossless; 512 MB for `ledger` and `signals`.
13. **The research amendments.** Dotted names with `*` and `>`, and `--since last` from the tail segment.
    - A `loop.lost` last will for a loop that the reader saw live, from `loop.started` and a kernel exit watch.
    - Consumer groups (E13).
14. **No orphan.** A watcher exits when the reader of its output is gone.
15. **The other pollers.** E14 moves them to push. The GitHub API pollers stay as documented limits.

## Alternatives considered

| Alternative | Why not |
|---|---|
| A broker in the loop process, with a journal for replay | The loop must never stall for a listener. A broker sees only the loop's events and is absent when no loop runs. Its speed gain is microseconds. |
| A hybrid: the logs plus a live broker | Two wake paths must agree, and a supervisor is still needed. A broker can come later as one more consumer of the logs. |
| One journal for all events, with filters at read time | No back-pressure or retention for each topic. A reader of `loop` wakes for every ledger append. |
| A FIFO for each subscriber | The producer then depends on its subscribers. Go cannot poll a FIFO on darwin. |
| Shared memory with a doorbell | Not durable. eventfd is Linux only, and the macOS wait needs macOS 14.4. |
| D-Bus, journald, NATS, Kafka or Redis | A daemon or a new dependency for a load of 2,000 events per day. |
| fsnotify | A new dependency for calls that the `syscall` package has. |
| `gh webhook forward` for CI | GitHub says it is for tests, not production. |
| A last will from the run lease | A lease names the process of a cycle, and a fleet lane is its own process. |

## Consequences

- **No consumer polls.** The GitHub API pollers are the only polls that stay.
- **New producers, kinds and codes** (E11). The spec §14 lists them.
- **Every root calls its closer**, and a root-list guard test keeps the list complete (E10).
- **Disk.** Channel logs live under `.evolve/events/`, which `.evolve/*` in `.gitignore` already ignores. Retention bounds them.
- **The watch verbs keep their flags** and become views over channels (E12).
- **Delivery is at-least-once.** A consumer removes duplicates with the event id.
- **Accepted limits:** an unsynced tail after an OS crash, the stall exposure of a channel lock, and no order across channels. The spec §Limits holds the full list.
