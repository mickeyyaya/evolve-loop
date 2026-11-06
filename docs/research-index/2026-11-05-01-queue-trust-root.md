## 2026-11-05 — the queue trust root: the host record decides what a cycle may do to the inbox queue (ADR-0160)

| Document | What it records |
|---|---|
| [research/queue-trust-root-2026-11.md](research/queue-trust-root-2026-11.md) | Research: six blocked review rounds, the agent-writable files that give authority over the queue, and the faults that the review of PR 1 found. It adds outside work and one stated gap. Findings F1.1 to F1.14, P2.1 to P2.7, refinements R1 to R11. |
| [plans/queue-authority-from-host-facts-2026-11.md](plans/queue-authority-from-host-facts-2026-11.md) | Plan: six designs with scores, and the winner, one trust root. Decisions D1 to D18 and components K1 to B2 with red tests. It corrects an old claim: PR 1 closes the pin file, and PR 3 closes the PASS path. |
| [architecture/adr/0160-the-host-lineage-record-is-the-one-trust-root-for-what-a-cycle-may-do-to-the-queue.md](architecture/adr/0160-the-host-lineage-record-is-the-one-trust-root-for-what-a-cycle-may-do-to-the-queue.md) | Decision (Proposed): the host record is the only source of which items a cycle owns. It has one reader, a typed route reason, a mover refusal and a brake. |
| [architecture/queue-trust-root.md](architecture/queue-trust-root.md) | Spec of record: the host record, its one reader and guard, and the drift signal. It also covers the write boundary, the typed route reason, the mover refusal and the breaker. |
