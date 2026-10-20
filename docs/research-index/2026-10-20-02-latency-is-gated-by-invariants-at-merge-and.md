## 2026-10-20 — latency is gated by invariants at merge and by trends on main (ADR-0144)

| Document | What it records |
|---|---|
| [research/latency-gates-2026-10.md](research/latency-gates-2026-10.md) | Research: the latency fixes, the existing test tools and current practice for gates on noisy runners (F3.1 to F3.5, G1 to G12). |
| [plans/latency-gates-2026-10.md](plans/latency-gates-2026-10.md) | Plan: hard gates on behavior, a seeded wave simulation and soft trend gates (D1 to D5, T1 to T7). |
| [architecture/adr/0144-latency-is-gated-by-invariants-at-merge-and-by-trends-on-main.md](architecture/adr/0144-latency-is-gated-by-invariants-at-merge-and-by-trends-on-main.md) | Decision (Proposed): hard gates never read the wall clock, and measured times file an item after two breaches. |
