## 2026-11-27 — Merge train and lane concurrency

| Document | What it records |
|---|---|
| [research/merge-train-lane-concurrency-2026-11.md](research/merge-train-lane-concurrency-2026-11.md) | Why wave 191 ran one lane of five, and what to change. The train of ADR-0147 is a multi-lane design; the one lane came from `--only`, shared packages and the id-family rule of admission. It holds 13 local findings: the self-land load on `main`, 46 ship recoveries in 53 cycles and a shadow eject rate near 3%. It surveys Zuul, SubmitQueue, Trunk and CAID, compares four approaches and lists 8 refinements and 3 operator questions. No plan uses it yet. |
