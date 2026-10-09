# Event notification protocol: research dossier (2026-10)

> The plan that uses this research: [event-notification-protocol-2026-10.md](../plans/event-notification-protocol-2026-10.md). The design: [event-channels.md](../architecture/event-channels.md). The decision record: [ADR-0127](../architecture/adr/0127-push-only-event-channels.md). The producer path: [signal-center-design.md](../architecture/signal-center-design.md).
>
> The refinements R1 to R24 at the end map to the plan's decisions.

Date: 2026-10-09. Method: I checked each claim against a primary source where one exists. Local probes ran on macOS 26.6.2 (Darwin 25.6.0) with Go 1.27.1. "Synthesis" marks my own inference.

## Table of contents

1. [Local evidence](#1-local-evidence)
2. [Kernel file-change notification](#2-kernel-file-change-notification)
3. [Local transports and IPC cost](#3-local-transports-and-ipc-cost)
4. [Topics, channels and subscriptions](#4-topics-channels-and-subscriptions)
5. [Delivery semantics](#5-delivery-semantics)
6. [Command hooks](#6-command-hooks)
7. [Concurrent appends](#7-concurrent-appends)
8. [Adopt or reject](#8-adopt-or-reject)
9. [Refinements to adopt](#9-refinements-to-adopt)

## 1. Local evidence

**F1.1** The runtime plane writes about 2,000 signal lines per day. Cycles 1835 to 1842 wrote 41 to 274 lines each, over 30 to 120 minutes. A line has about 390 bytes on average, and the largest line has 1,229 bytes. Source: local probe of `runtime/.evolve/runs/cycle-18*/signals.ndjson` and `ledger.jsonl` (1,430 to 1,908 ledger lines per day, 2026-10-05 to 2026-10-08).
*Implication:* the transport is not the limit. Durability, replay and the wake are the limits.

**F1.2** `ledger.appended` is 82% of the signal lines (770 of 943 in cycles 1839 to 1842). The peak is 50 events in one second, from one routing decision. Source: the same probe, with `jq -r .kind`.
*Implication:* one high-volume kind dominates. Its own best-effort channel keeps the low-volume channels quiet.

**F1.3** The kinds `phase.dispatched` and `ship.landed` exist in the closed set, but no production code emits them. Source: `go/internal/signalcenter/event.go:129` and `:135`, and a search of the non-test Go code.
*Implication:* a push view of phase changes and ships needs two new producers.

**F1.4** On this host, `socket(AF_UNIX, SOCK_SEQPACKET)` fails with errno 43 ("Protocol not supported"), `sun_path` has 104 bytes and `PIPE_BUF` is 512. Sources: a local probe, `man 4 unix`, `getconf PIPE_BUF /`.
*Implication:* a seqpacket broker does not port to macOS, and a FIFO cannot carry a 4 KiB line as one atomic write.

**F1.5** A socket at `.evolve/events/broker.sock` in a cycle worktree has a 110-byte path on this host. Source: a local probe (`runtime/.evolve/worktrees/cycle-cd3ae73e-1775`).
*Implication:* a socket in the project tree can pass the 104-byte limit.

**F1.6** When all processors are idle, the Go runtime monitor sleeps until the next timer or for `forcegcperiod/2`, which is one minute. Source: Go 1.27.1 `src/runtime/proc.go:6527` and `:6575-6593`.
*Implication:* a blocked watcher with no timer wakes about once a minute for runtime work, and never for our state.

**F1.7** Go does not put a regular file or a directory into its netpoller. On darwin it also excludes FIFOs, because "closing the last writer does not cause a kqueue event". Source: Go 1.27.1 `src/os/file_unix.go:170-191`.
*Implication:* on darwin the waker owns its kqueue. The Go poller cannot watch the log file.

**F1.8** A local kqueue probe gave these results:
- An append raises `NOTE_WRITE|NOTE_EXTEND` (0x6) on the segment descriptor.
- A new file in the directory raises `NOTE_WRITE` (0x2) on the directory descriptor.
- With `EV_CLEAR`, a second call returns nothing. Without `EV_CLEAR`, the same event returns on every call.
- A `kevent` call with no timeout blocked for 0.41 s and woke on `NOTE_EXIT` of a child.
- An absent pid gives `ESRCH`.

Source: the probe below, run on this host with `python3 -I probe.py <empty-dir>`. It needs no package outside the Python standard library.

```python
import os, select, subprocess, sys, time
d = sys.argv[1]
seg = os.path.join(d, "seg-00000000000000000000.ndjson")
open(seg, "a").close()
kq = select.kqueue()
dfd, ffd = os.open(d, os.O_RDONLY), os.open(seg, os.O_RDONLY)
V, W, X, DEL = select.KQ_FILTER_VNODE, select.KQ_NOTE_WRITE, select.KQ_NOTE_EXTEND, select.KQ_NOTE_DELETE
clear = select.KQ_EV_ADD | select.KQ_EV_CLEAR
kq.control([select.kevent(dfd, V, clear, W), select.kevent(ffd, V, clear, W | X | DEL)], 0, 0)
show = lambda label: print(label, [hex(e.fflags) for e in kq.control(None, 8, 0)])
show("armed, no write:")            # []
with open(seg, "a") as f: f.write('{"a":1}\n')
show("after an append:")            # ['0x6'] on the segment
show("again, with EV_CLEAR:")       # []
open(os.path.join(d, "seg-00000000000000000008.ndjson"), "a").close()
show("a new segment:")              # ['0x2'] on the directory
level = select.kqueue()             # no EV_CLEAR: the same event returns on every call
lfd = os.open(seg, os.O_RDONLY)
level.control([select.kevent(lfd, V, select.KQ_EV_ADD, W | X)], 0, 0)
with open(seg, "a") as f: f.write('{"b":2}\n')
print("no EV_CLEAR:", len(level.control(None, 8, 0)), len(level.control(None, 8, 0)))  # 1 1
child = subprocess.Popen(["/bin/sleep", "0.4"])
pq = select.kqueue()
pq.control([select.kevent(child.pid, select.KQ_FILTER_PROC, select.KQ_EV_ADD | select.KQ_EV_ONESHOT, select.KQ_NOTE_EXIT)], 0, 0)
t0 = time.monotonic(); evs = pq.control(None, 1, None)          # blocks with no timeout
print("NOTE_EXIT after %.2fs" % (time.monotonic() - t0), [hex(e.fflags & 0xffffffff) for e in evs])
child.wait()
try: pq.control([select.kevent(999999, select.KQ_FILTER_PROC, select.KQ_EV_ADD, select.KQ_NOTE_EXIT)], 0, 0)
except OSError as e: print("absent pid:", e.errno)  # 3 (ESRCH)
```

*Implication:* the wake design works on the host, and `EV_CLEAR` is mandatory.

**F1.9** A run lease names the process that runs a cycle: `startRunLease` writes `OwnerPID: os.Getpid()`. A fleet lane runs `evolve cycle run` as its own process. Sources: `go/internal/core/runlease_hook.go:33`, `go/cmd/evolve/cmd_fleet.go:127`.
*Implication:* a lease pid can be a lane, not the loop. The last will takes the loop pid from a `loop.started` event.

## 2. Kernel file-change notification

**F2.1** kqueue `EVFILT_VNODE` reports `NOTE_DELETE`, `NOTE_WRITE`, `NOTE_EXTEND`, `NOTE_ATTRIB`, `NOTE_LINK`, `NOTE_RENAME` and `NOTE_REVOKE` on a watched descriptor. `EV_CLEAR` resets the state after the user gets the event. With a NULL timeout, `kevent` waits indefinitely. Sources: https://keith.github.io/xcode-man-pages/kqueue.2.html , https://man.freebsd.org/cgi/man.cgi?query=kqueue&sektion=2
*Implication:* a watcher in `kevent` uses no CPU until the kernel posts an event.

**F2.2** `EVFILT_PROC` with `NOTE_EXIT` fires when a process exits, and an absent pid fails with `ESRCH`. `EVFILT_USER` with `NOTE_TRIGGER` is an event that only user code fires. Sources: the two kqueue pages above.
*Implication:* one kqueue can wait on a channel directory, its tail segment, a producer's exit and a cancel event.

**F2.3** inotify reports `IN_MODIFY`, `IN_CREATE`, `IN_MOVED_TO`, `IN_DELETE_SELF` and `IN_MOVE_SELF`. The descriptor works with `select`, `poll` and `epoll`. Source: https://man7.org/linux/man-pages/man7/inotify.7.html
*Implication:* on Linux, one directory watch for each channel covers appends and new segments.

**F2.4** inotify drops events past `max_queued_events`, and then always queues `IN_Q_OVERFLOW`. Identical unread events merge into one. inotify misses changes from other machines on network filesystems: "Applications must poll". Source: inotify(7), as in F2.3.
*Implication:* a wake is only a hint, so the reader reads the log to its end. A network filesystem is refused.

**F2.5** fsnotify uses kqueue on macOS and inotify on Linux, and it needs Go 1.23. It warns that "kqueue requires opening a file descriptor for every file". NFS, SMB, `/proc` and `/sys` give no events, and the README advises a watch on the parent directory. Sources: https://github.com/fsnotify/fsnotify , https://pkg.go.dev/github.com/fsnotify/fsnotify (v1.10.1, 2026-05-04)
*Implication:* the same calls are in the standard `syscall` package: `Kqueue`, `Kevent`, `InotifyInit1` and `InotifyAddWatch` (Go 1.27.1 `src/syscall`). The module stays with its two dependencies (`go/go.mod`).

**F2.6** FSEvents reports at directory level and merges notifications. Apple says that it is "not designed for finding out when a particular file changes", and that "the kqueues mechanism is more appropriate". Source: https://developer.apple.com/library/archive/documentation/Darwin/Conceptual/FSEvents_ProgGuide/TechnologyOverview/TechnologyOverview.html
*Implication:* use kqueue, not FSEvents.

**F2.7** `pidfd_open` arrived in Linux 5.3. The descriptor becomes readable when the process ends, and `poll`, `select` and `epoll` can watch it. `waitid` works only for a child. Source: https://man7.org/linux/man-pages/man2/pidfd_open.2.html
*Implication:* a watcher can wait for the loop's exit on Linux with no poll.

**F2.8** A journald reader waits on a descriptor from `sd_journal_get_fd`. `sd_journal_reliable_fd` returns 0 on network filesystems such as NFS. The library then asks for timeouts, which is a poll fallback. Source: https://www.mankier.com/3/sd_journal_get_fd
*Implication:* the journald readers use the same model: a log on disk and a kernel wake. We refuse the network case instead of a poll.

**F2.9** In 2016, `journalctl --follow` stopped with no error, and a comment traced the cause to the per-user inotify watch limit. Source: https://bugs.archlinux.org/task/51417
*Implication:* a watch that cannot be added must fail loudly.

**F2.10** The Linux kernel sets three inotify defaults. A user can have 128 instances, and an instance can queue 16,384 events. The watch limit comes from the RAM size, between 8,192 and 1,048,576. Source: https://raw.githubusercontent.com/torvalds/linux/master/fs/notify/inotify/inotify_user.c (`inotify_user_setup`)
*Implication:* each process that watches uses one instance, so about 128 watchers can run for each user. This is a documented limit.

## 3. Local transports and IPC cost

**F3.1** An io_uring IPC RFC measured 64-byte point-to-point latency: a pipe 212 ns, a Unix socket 436 ns, shared memory with eventfd 222 ns. At fan-out to 16 receivers, a Unix socket took 32,504 ns and shared memory 5,970 ns. Source: https://lkml.iu.edu/hypermail/linux/kernel/2603.1/11652.html (D. Hodges, 2026-03-13)
*Implication:* data that is written once wins at fan-out.

**F3.2** An older one-machine test measured median latency: a Unix socket 1,439 ns, a pipe 4,255 ns, eventfd 4,353 ns and TCP loopback 7,287 ns. Source: https://kamalmarhubi.com/blog/2015/06/10/some-early-linux-ipc-latency-data
*Implication:* each local transport is in the microsecond range. Our load is 2,000 events per day.

**F3.3** ipc-bench measured ping-pong rates with 100-byte messages. A Unix socket gave 130,372 per second and a pipe 162,441. A FIFO gave 265,823, and shared memory 4,702,557. Source: https://github.com/goldsborough/ipc-bench
*Implication:* the transport is never our bottleneck.

**F3.4** An AF_UNIX `SOCK_SEQPACKET` socket exists on Linux since 2.6.4, and `sun_path` has 108 bytes there. A connect needs write permission on the socket file, but the page warns that "some systems ignore socket permissions". Source: https://man7.org/linux/man-pages/man7/unix.7.html
*Implication:* with F1.4, a socket broker needs its own message frames, a short path outside the project and its own access check.

**F3.5** A full pipe blocks its writer, or returns `EAGAIN`. A write above `PIPE_BUF` can interleave with other writers, and a writer gets `SIGPIPE` when all readers close. A FIFO open for write with no reader fails with `ENXIO` under `O_NONBLOCK`. Sources: https://man7.org/linux/man-pages/man7/pipe.7.html , https://man7.org/linux/man-pages/man7/fifo.7.html
*Implication:* a FIFO for each subscriber makes the producer depend on its subscribers.

**F3.6** eventfd is Linux only: its standards line says "Linux, GNU". The macOS futex-style wait, `os_sync_wait_on_address`, needs macOS 14.4. Sources: https://man7.org/linux/man-pages/man2/eventfd.2.html , https://developer.apple.com/documentation/os/os_sync_wait_on_address , https://lists.llvm.org/pipermail/libcxx-commits/2026-June/124948.html
*Implication:* a shared-memory doorbell needs two different native APIs, and it is not durable.

**F3.7** `EVFILT_USER` needs all participants to share one kqueue. A FreeBSD proposal adds `EVFILT_USERMEM`, so that separate processes can wake through shared memory. Source: https://reviews.freebsd.org/D37102
*Implication:* `EVFILT_USER` can cancel a wait inside one process, but it cannot wake a different process.

**F3.8** Chromium does not use Mach ports for IPC on macOS. It cannot block on a socket and a Mach port at once, and it found that the gain over pipes was "negligible". Source: https://www.chromium.org/developers/design-documents/os-x-interprocess-communication/
*Implication:* Mach ports add nothing here.

**F3.9** The Docker daemon serves its Engine API on a Unix socket by default. `docker events` streams "real-time events from the server", but "only the last 256 log events are returned" for a past window. A repeated filter key is a logical OR, and different keys are a logical AND. Sources: https://docs.docker.com/reference/cli/dockerd/ , https://docs.docker.com/reference/cli/docker/system/events/
*Implication:* adopt the filter rule. Reject a history that lives only in memory.

## 4. Topics, channels and subscriptions

**F4.1** A Kafka topic has partitions, and order holds within one partition only. A consumer group gives each partition to one consumer. A segment file has the name of its base offset, for example `00000000000000000000.log`. Sources: https://docs.confluent.io/kafka/design/consumer-design.html , https://aiven.io/docs/products/kafka/concepts/partition-segments
*Implication:* adopt order within a channel and segment names from a base offset. Reject partitions, because one host carries a low volume.

**F4.2** Kafka's position is "one larger than the highest offset the consumer has seen". The committed position is the "offset that the consumer will recover to". A commit after the work gives "at-least-once" delivery. Source: https://kafka.apache.org/34/javadoc/org/apache/kafka/clients/consumer/KafkaConsumer.html
*Implication:* a subscription acks its cursor only after the handler succeeds.

**F4.3** Kafka commits offsets in sequence, so a slow message, or one that fails, "blocks the entire partition". Share groups (KIP-932) are ready for production in Kafka 4.2 (February 2026). They add per-record accept, release and reject, a time-limited lock and a delivery limit (default 5). Sources: https://karafka.io/docs/Basics-Consumer-Groups-vs-Share-Groups/ , https://www.conduktor.io/blog/kafka-isnt-a-queue
*Implication:* a fan-out subscription keeps one ordered cursor. A consumer group (E13) acks each message, so a poison event never blocks its peers.

**F4.4** NATS subjects are tokens separated by dots. `*` matches exactly one token. `>` matches one or more tokens and must be the last token. Source: https://docs.nats.io/nats-concepts/subjects
*Implication:* adopt dotted channel names and the two wildcards for `--channel`.

**F4.5** A JetStream consumer is "a server-side, stateful view of a stream". "An acknowledgment advances the consumer's cursor", and a message that is not acked in time comes again (at-least-once). Retention is limits (the default), interest or work-queue. Discard is old (the default) or new. Sources: https://docs.nats.io/nats-concepts/jetstream , https://docs.nats.io/nats-concepts/jetstream/streams
*Implication:* adopt named, stateful subscriptions with acks. A channel log with a TTL and a size cap is limits retention with discard-old.

**F4.6** A JetStream pull consumer holds at most `MaxAckPending` messages without an ack (default 1,000). `MaxDeliver` caps the redeliveries, and the last entry of `BackOff` repeats. An advisory reports a message that used all its deliveries, and a dead-letter store is built from that advisory. Sources: https://docs.nats.io/nats-concepts/jetstream/consumers , https://docs.nats.io/reference/jetstream/advisory/max-deliver , https://www.synadia.com/blog/jetstream-reliable-delivery-dlq-replay
*Implication:* adopt a retry cap, then a dead letter, then the next message.

**F4.7** In Redis Streams, `XREADGROUP` with `>` returns messages that the group never received. A message without an ack stays in the pending-entries list (PEL), with its owner, its idle time and a delivery count. `XCLAIM` and `XAUTOCLAIM` move idle messages to a live consumer, and `MAXLEN` trims regardless of the groups. Sources: https://redis.io/docs/latest/develop/data-types/streams/ , https://redis.io/docs/latest/commands/xpending/
*Implication:* adopt the shape of the PEL for E13.

**F4.8** MQTT has three QoS levels: 0 is "at most once", 1 is "at least once" and 2 is "exactly once". A topic filter can use `+` (one level) and `#` (all levels after it). The broker keeps a retained message and sends it to each new subscription that matches. A will message goes out when a client disconnects unexpectedly. Sources: https://mosquitto.org/man/mqtt-7.html , https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html
*Implication:* adopt a retained last event (`--since last`) and a last will (`loop.lost`). Lossless and best-effort channels play QoS 1 and QoS 0.

**F4.9** HiveMQ advises against a subscription to every message (`#`), because the client often cannot keep up. Source: https://www.hivemq.com/blog/mqtt-essentials-part-5-mqtt-topics-best-practices/
*Implication:* the firehose channel is best effort, and narrow channels are the normal choice.

**F4.10** D-Bus sends a broadcast signal only to a connection whose match rule matches it, which "avoids waking up client processes to deal with signals that are not relevant". A match rule is a list of `key='value'` pairs, and an absent key matches any value. Source: https://dbus.freedesktop.org/doc/dbus-specification.html
*Implication:* adopt the idea that a subscriber wakes only for its channels. Reject the bus: it keeps no history and needs a daemon.

**F4.11** A Kubernetes client lists and then watches from the `resourceVersion` of the list, "without missing any events". A version that is too old gives `410 Gone`, and the client must list again. etcd 3 keeps 5 minutes of changes by default. Source: https://kubernetes.io/docs/reference/using-api/api-concepts/
*Implication:* adopt "arm, then catch up". A cursor that retention passed gets an explicit gap.

**F4.12** etcd promises watch events that are ordered, unique and reliable: they "never drop any subsequence of events within the available history window". A watch from a compacted revision is cancelled. Sources: https://etcd.io/docs/v3.5/learning/api/ , https://etcd.io/docs/v3.5/learning/api_guarantees/
*Implication:* the channel gives the same promise inside its retention window, and a gap record outside it.

**F4.13** In Server-Sent Events, an `id` field sets the last event ID, and a client sends `Last-Event-ID` when it connects again. The spec asks for a comment line about every 15 seconds for older proxies. Source: https://html.spec.whatwg.org/multipage/server-sent-events.html
*Implication:* the cursor plays the SSE last event ID: it is a resume position. A local watcher needs no keepalive.

**F4.14** CloudEvents 1.0.2 requires `id`, `source`, `specversion` and `type`. "Producers MUST ensure that source + id is unique", and `type` is used "for routing, observability, policy enforcement". Source: https://github.com/cloudevents/spec/blob/v1.0.2/cloudevents/spec.md
*Implication:* the event id (`pid.seq.ts`) is our unique key, and the signal kind plays `type`. A later webhook phase can map a record to this envelope.

## 5. Delivery semantics

**F5.1** With a transactional outbox, a message goes out "if and only if" its transaction commits. The relay can send a message twice, so the consumers must be idempotent. Source: https://microservices.io/patterns/data/transactional-outbox.html
*Implication:* the channel log is the outbox, and `EVOLVE_EVENT_ID` is the idempotency key.

**F5.2** A KurrentDB catch-up subscription reads the history and then moves to live delivery, with a "caught up" signal (server 23.10 or later). The handler stores its own checkpoint. A persistent subscription parks a message that passes its retry limit. Sources: https://docs.kurrent.io/clients/grpc/subscriptions , https://docs.kurrent.io/server/v25.0/features/persistent-subscriptions
*Implication:* adopt history, then live, with a cursor that the client keeps. A dead letter plays the parked message.

**F5.3** Redis closes a Pub/Sub client whose output buffer passes 32 MB, or 8 MB for 60 seconds. Source: https://redis.io/docs/latest/develop/reference/clients/
*Implication:* a broker must cut off a slow subscriber. With a log, a slow subscriber only lags.

**F5.4** "GitHub does not automatically redeliver failed deliveries." GitHub advises a script on a schedule that redelivers the failures through the REST API. Source: https://docs.github.com/en/webhooks/using-webhooks/handling-failed-webhook-deliveries
*Implication:* the deliverer owns its retries and its dead letters. A later webhook phase needs the same.

## 6. Command hooks

**F6.1** The git post-commit hook "cannot affect the outcome of git commit", and post-receive runs "after the real work is done". A non-zero exit of a pre-commit or pre-receive hook stops the operation. Source: https://git-scm.com/docs/githooks
*Implication:* a subscriber is a post hook. It observes and never decides.

**F6.2** systemd path units use inotify and "cannot be used to monitor files or directories changed by other machines on remote NFS file systems". Triggers have a rate limit: `TriggerLimitIntervalSec` 2 s and `TriggerLimitBurst` 200. Source: https://man.archlinux.org/man/systemd.path.5.en
*Implication:* several writes can give one wake. The runner reads to the end and runs one command for each event.

**F6.3** entr "waits for the child process to finish before responding to subsequent file system events". With `-r`, it reloads a persistent child in its own process group. Source: https://man.archlinux.org/man/entr.1.en
*Implication:* deliver one event at a time, in order, for each subscription.

**F6.4** watchexec merges many file events into one (the debounce default is 50 ms). Its `--poll` mode (default 30 s) is for network shares where native watches fail. Sources: https://github.com/watchexec/watchexec , https://raw.githubusercontent.com/watchexec/watchexec/main/doc/watchexec.1.md
*Implication:* a wake can stand for many writes. We refuse network filesystems instead of a poll mode.

**F6.5** `gh webhook forward` "is only designed for use during testing and development", and it "is not supported for use in production environments". Only one person can forward for each repository at a time. Source: https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/using-the-github-cli-to-forward-webhooks-for-testing
*Implication:* CI status still comes from the GitHub API. This is a documented limit at one producer.

## 7. Concurrent appends

**F7.1** POSIX says that with `O_APPEND` "the file offset shall be set to the end of the file" before each write, with no other change between the two steps. For regular files, the standard advises "some form of concurrency control". Source: https://pubs.opengroup.org/onlinepubs/9799919799/functions/write.html
*Implication:* each channel takes a lock around its append. The repository already writes a line of 4 KiB or less in one `O_APPEND` write (`go/internal/signalcenter/event.go:29-33`).

**F7.2** A `flock` lock belongs to an open file description, and it is advisory. NFS clients emulate it with `fcntl` byte-range locks. Source: https://man7.org/linux/man-pages/man2/flock.2.html
*Implication:* every writer takes the same lock. The refusal of network filesystems also covers the lock.

Synthesis: because the lock belongs to the open file description, a goroutine that waits in `flock` past a deadline can keep its own descriptor. It closes that descriptor when its call returns, and the lock is free at once. This gives a lock with a deadline and no poll.

**F7.3** Go exits with `SIGPIPE` on a write to a broken pipe at file descriptor 1 or 2, unless the program calls `Notify` for `SIGPIPE`. With `Notify`, the write fails with `EPIPE`. Sources: https://pkg.go.dev/os/signal (section SIGPIPE), Go 1.27.1 `src/os/signal/doc.go:91-104`
*Implication:* a watcher calls `Notify` for `SIGPIPE`, so a broken output gives its own exit code (3). A blocked watcher never writes, so it also arms a hangup wake on stdout.

## 8. Adopt or reject

| Prior art | Verdict | Reason | Finding |
|---|---|---|---|
| kqueue `EVFILT_VNODE` with `EV_CLEAR` | adopt | A wake with no CPU use; the stdlib has it; the probe confirms it | F1.8, F2.1 |
| inotify, one directory watch per channel | adopt | One watch covers appends and new segments | F2.3, F2.4 |
| fsnotify | reject | A new dependency for two calls that the stdlib has | F2.5 |
| FSEvents | reject | Directory level, and it merges events | F2.6 |
| `EVFILT_PROC` and `pidfd` exit watch | adopt | The last will with no poll | F2.2, F2.7 |
| journald reader model | adopt | A log on disk plus a kernel wake, read by each reader | F2.8 |
| A poll fallback on a network filesystem | reject | The hard requirement; refuse instead | F2.4, F2.8, F6.4 |
| A Unix-socket broker | reject for now | It needs a supervisor and the log as a fallback; a later consumer of the log | F1.5, F3.4, F5.3 |
| `SOCK_SEQPACKET` | reject | macOS refuses it | F1.4 |
| A FIFO for each subscriber | reject | The producer then depends on the subscriber | F1.4, F3.5 |
| Shared memory with a doorbell | reject | Not durable; two native APIs | F3.1, F3.6 |
| `EVFILT_USER` across processes | reject | It works inside one kqueue only; adopt it for cancel | F3.7 |
| Mach ports | reject | No gain over pipes | F3.8 |
| The D-Bus bus | reject | No history, and a daemon | F4.10 |
| D-Bus match rules | adopt the idea | A subscriber wakes only for its channels | F4.10 |
| Docker filter rule | adopt | OR within a key, AND across keys | F3.9 |
| Docker history in memory | reject | 256 events only | F3.9 |
| Kafka order within a partition | adopt | Order within a channel only | F4.1 |
| Kafka partitions | reject | One host, low volume | F4.1 |
| Kafka base-offset segment names | adopt | Order by name; a cursor finds its segment | F4.1 |
| Kafka cumulative commit | adopt for fan-out | One ordered cursor for each subscription | F4.2 |
| Kafka share groups | adopt in E13 | An ack for each message; no head-of-line block | F4.3 |
| NATS subject wildcards `*` and `>` | adopt | Channel selection | F4.4 |
| JetStream durable consumers and acks | adopt | Named, stateful subscriptions | F4.5 |
| JetStream `MaxDeliver` and back-off | adopt | A retry cap, then a dead letter | F4.6 |
| JetStream interest and work-queue retention | reject | A slow subscriber must not hold the disk without a cap | F4.5 |
| Redis pending-entries list (PEL) | adopt in E13 | Owner, idle time and a delivery count | F4.7 |
| Redis Pub/Sub cut-off | reject | A log lets a slow subscriber lag instead | F5.3 |
| MQTT QoS 0 and 1 | adopt | `best_effort` and `lossless` | F4.8 |
| MQTT QoS 2 | reject | Exactly-once needs a handshake; consumers dedupe by event id | F4.8 |
| MQTT retained message | adopt | `--since last` reads the last record of the tail segment | F4.8 |
| MQTT last will | adopt | `loop.lost` from a kernel exit watch | F4.8, F2.2 |
| Kubernetes list, then watch | adopt | Arm, then catch up, then wait | F4.11 |
| Kubernetes `410 Gone` | adopt as a gap record | A gap is never silent | F4.11 |
| etcd watch guarantees | adopt | Ordered, unique, reliable inside the window | F4.12 |
| SSE `Last-Event-ID` | adopt | The cursor is the resume position | F4.13 |
| SSE keepalive | reject | Local readers have no proxy | F4.13 |
| CloudEvents envelope | defer | The webhook phase can map to it | F4.14 |
| Transactional outbox | adopt | The log is the outbox; the event id is the key | F5.1 |
| Catch-up subscription | adopt | History, then live, with a client cursor | F5.2 |
| git post hooks | adopt | Subscribers observe and never decide | F6.1 |
| entr serial delivery | adopt | One event at a time, in order | F6.3 |
| watchexec poll mode | reject | The hard requirement | F6.4 |
| `gh webhook forward` | reject | Not for production | F6.5 |
| GitHub redelivery practice | adopt | The deliverer owns retries and dead letters | F5.4 |
| `O_APPEND` with a per-channel lock | adopt | No interleaved lines; the ledger uses the same lock | F7.1, F7.2 |
| A lock wait raced against a one-shot timer | adopt | A deadline with no poll; the abandoned call frees the lock when it returns | F7.2 |
| `Notify` for `SIGPIPE`, and a hangup wake on stdout | adopt | A watcher with no reader exits 3 | F7.3 |
| The run lease as the source of the last will | reject | A lease pid can be a fleet lane | F1.9 |

## 9. Refinements to adopt

1. **R1** Each channel is one append-only log. A subscriber reads the log; no process forwards events. [F2.8, F5.1]
2. **R2** The cursor is a byte offset in a channel. Segment names carry the base offset. [F4.1, F4.2]
3. **R3** Order holds within a channel. A reader merges channels by time and says so. [F4.1]
4. **R4** Every wait is arm, then catch up, then wait. A wake only means "read to the end". [F2.4, F4.11]
5. **R5** On darwin, use `EVFILT_VNODE` with `EV_CLEAR` on the directory and the tail segment. [F1.8, F2.1]
6. **R6** On Linux, use one inotify watch for each channel directory. An overflow counts as a wake. [F2.3, F2.4]
7. **R7** Refuse network filesystems and other operating systems. Never fall back to a poll. [F2.4, F2.8, F6.4]
8. **R8** A watch that the kernel refuses fails loudly with its error. [F2.9]
9. **R9** Use the stdlib `syscall` package. Add no dependency. [F2.5]
10. **R10** `EVFILT_USER` cancels a wait inside one process only. [F3.7]
11. **R11** Channel names have dots. `*` matches one token and `>` matches the rest. [F4.4]
12. **R12** Two QoS classes: `lossless` (QoS 1) and `best_effort` (QoS 0, with an explicit gap). [F4.8, F4.9]
13. **R13** `--since last` starts at the last record of each channel, so a new subscriber learns the current state. [F4.8]
14. **R14** A watcher arms an exit watch on each loop that `loop.started` names, and reports `loop.lost`. [F1.9, F2.2, F2.7, F4.8]
15. **R15** Ack after success. The event id is the idempotency key, and the cursor is the position. [F4.2, F5.1]
16. **R16** A capped retry schedule, then a dead letter, then the next event. [F4.6, F5.4]
17. **R17** One event at a time for each subscription, in cursor order. [F6.3]
18. **R18** Subscribers observe and never decide, as git post hooks do. [F6.1]
19. **R19** A consumer group keeps a pending-entries list (PEL) and acks each message. [F4.3, F4.7]
20. **R20** Filters: OR within a key, AND across keys. A subscriber wakes only for its channels. [F3.9, F4.10]
21. **R21** Retention by TTL and size cap; a cursor that retention passed gets a gap record. [F4.5, F4.11, F4.12]
22. **R22** The GitHub API pollers stay as documented limits. [F6.5]
23. **R23** A watcher exits when the reader of its output is gone. [F7.3]
24. **R24** A lock wait in the drain has a deadline: `flock` in a goroutine, raced against a timer. [F7.2]
