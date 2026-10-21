## 2026-10-19 — a wider wave, a faster build and gates that test once (ADR-0143)

| Document | What it records |
|---|---|
| [research/wave-latency-2026-10.md](research/wave-latency-2026-10.md) | Research: waves 130 to 133 used 51% of their slot-hours. A lane that edits `internal/core` held 13 ready items for 71 minutes. Build is 46% of lane time, and the gates are 28%. It also surveys merge queues, test caches, routers and pools. Findings F2.1 to F5.3 and O1 to O11. |
| [plans/wave-latency-2026-10.md](plans/wave-latency-2026-10.md) | Plan: three levers with scored designs. The choices are optimistic admission, a headroom-aware build order and one verdict store. Decisions D1 to D7 and components W0 to W5. |
| [architecture/adr/0143-admission-holds-only-shared-edits-and-the-ship-gate-owns-api-drift.md](architecture/adr/0143-admission-holds-only-shared-edits-and-the-ship-gate-owns-api-drift.md) | Decision (Accepted, stage shadow): admission holds only shared edits and the hub, and the composed-tree gate of the ship owns API drift. |
