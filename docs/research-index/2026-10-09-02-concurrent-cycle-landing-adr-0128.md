## 2026-10-09 — concurrent cycle landing (ADR-0128)

| Document | What it records |
|---|---|
| [research/concurrent-cycle-landing-2026-10.md](research/concurrent-cycle-landing-2026-10.md) | Research: a census of 46 fleet-rebase recoveries, at 24.2 min and 1.49 LLM phases each. It shows why cycle 1843 was not proven identical and why the prefix queue of ADR-0078 is off. It also covers merge queues, agent harnesses, test selection, the cost model and five approaches. Findings F2.1 to F9.3, refinements R1 to R33. |
| [architecture/fleet-landing-queue.md](architecture/fleet-landing-queue.md) | Design, the spec of record: the durable landing queue, the composition with `merge-tree`, the overlap proof and the tiers T1 to T4. It also covers the scoped gates, the interaction review, ejection, budgets, signals, config keys, failure modes and limits. |
| [plans/concurrent-cycle-landing-2026-10.md](plans/concurrent-cycle-landing-2026-10.md) | Plan: operator decisions O1 to O13, console decisions CD1 to CD4, decisions D1 to D53, and components Q0 to Q19 with red tests. It also holds the migration from the current ladder, the rollout from shadow to enforce with its metrics, and the risks. |
| [architecture/adr/0128-landing-queue-tiered-reverification.md](architecture/adr/0128-landing-queue-tiered-reverification.md) | Decision: fleet lanes land through one queue, and their overlap with the peers picks the re-verification. It amends ADR-0078, ADR-0105 and ADR-0049. |
