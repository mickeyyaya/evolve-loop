## 2026-10-13 — fleet landing at scale: 200 concurrent lanes (ADR-0134)

| Document | What it records |
|---|---|
| [research/fleet-landing-at-scale-2026-10.md](research/fleet-landing-at-scale-2026-10.md) | Research: why one ship takes five minutes (`cmd/evolve` in the importer backstop). The cost of today's path at N = 2, 20 and 200, and the overlap rate of the import graph. It also surveys merge queues, speculative chains, batches and bisection, content-keyed test selection and git mechanics. Findings F2.1 to F3.5 and P4.1 to P8.4, refinements R1 to R12. |
| [plans/fleet-landing-at-scale-2026-10.md](plans/fleet-landing-at-scale-2026-10.md) | Plan: five approaches with weighted scores, and the winner with what it takes from the others. The winner is partitioned speculative chains with content-keyed verification and group commit. Decisions D1 to D17 and components S0 to S17 with red tests. The rollout from shadow to enforce, and the simulation at N = 2, 20 and 200. |
| [architecture/adr/0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md](architecture/adr/0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md) | Decision (Proposed): a content key proves each package run, and candidates verify before the head. Candidates with no edge land in any order, and one push lands the verified run. It amends ADR-0128. |
