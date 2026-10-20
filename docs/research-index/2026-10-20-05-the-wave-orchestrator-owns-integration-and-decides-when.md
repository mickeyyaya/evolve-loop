## 2026-10-20 — the wave orchestrator owns integration and decides when the wave reaches main (ADR-0147)

| Document | What it records |
|---|---|
| [research/wave-integration-2026-10.md](research/wave-integration-2026-10.md) | Research: about one lane in four needed a rebase recovery at landing, and `main` moved 100 times in two days. It also surveys merge trains, merge queues, batching and culprit finding. Findings F2.1 to F2.7 and M1 to M16. |
| [plans/wave-integration-2026-10.md](plans/wave-integration-2026-10.md) | Plan: five designs with scores. The orchestrator orders, composes and tests the deliverables on one integration branch, and flushes a batch as one PR. Decisions D1 to D23, with four test layers L1 to L4, and components I0 to I11. |
| [architecture/adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md](architecture/adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md) | Decision (Proposed): lanes deliver and do not push `main`; the orchestrator owns the order, the compose, the test and the push. |
