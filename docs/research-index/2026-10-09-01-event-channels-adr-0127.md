## 2026-10-09 — event channels (ADR-0127)

| Document | What it records |
|---|---|
| [research/event-notification-protocol-2026-10.md](research/event-notification-protocol-2026-10.md) | Research: kernel file notification (kqueue, inotify, fsnotify, FSEvents, pidfd) and the cost of local IPC. It also covers topic and channel prior art: Kafka, NATS, Redis Streams, MQTT, D-Bus, Kubernetes and etcd. Findings F1.1 to F7.3, an adopt-or-reject table, refinements R1 to R24. |
| [architecture/event-channels.md](architecture/event-channels.md) | Design, the spec of record: channel logs with byte cursors, a locked append, two QoS classes and the kernel wake (arm, catch up, wait). It also covers the filter grammar, subscriptions, delivery, consumer groups, retention, the `evolve events` verbs, the exit codes and the limits. |
| [plans/event-notification-protocol-2026-10.md](plans/event-notification-protocol-2026-10.md) | Plan: the poll audit, three candidate designs with scores, the operator decisions, decisions D1 to D30 and the Signal Center through the channels. It also lists the migrations, and components E0 to E14 with their red tests. |
| [architecture/adr/0127-push-only-event-channels.md](architecture/adr/0127-push-only-event-channels.md) | Decision: events reach other programs through channel logs that the kernel wakes, never through a poll, with the Signal Center as the one producer path. |
