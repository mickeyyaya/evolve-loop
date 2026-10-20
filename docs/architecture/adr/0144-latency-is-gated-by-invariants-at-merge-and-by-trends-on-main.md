# ADR-0144: latency is gated by invariants at merge and by trends on main

- **Status:** Proposed (2026-10-20).
- **Plan:** [latency-gates-2026-10](../../plans/latency-gates-2026-10.md). **Research:** [latency-gates-2026-10](../../research/latency-gates-2026-10.md).
- **Relates to:**
  - [ADR-0143](0143-admission-holds-only-shared-edits-and-the-ship-gate-owns-api-drift.md): its admission rule gets a property test as a hard gate.
  - [ADR-0139](0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md): the pool and the wave measures that the gates test.
- **Evidence:** research §2, G4, G1, G2 and G5 ([research](../../research/latency-gates-2026-10.md)).

## Context

A later change can undo a latency fix with no test that notices. The plan gives the problem and the reasons ([plan §1](../../plans/latency-gates-2026-10.md#1-the-problem)).

## Decision

1. Each latency fix gets a hard gate that tests its behavior or counts its work. A hard gate never reads the wall clock.
2. A seeded simulation of a wave drives the real pool and the real admission with no clock. Its invariants are a hard gate.
3. CI run times, CI queue times, package wall times and the wave measures are soft gates. A breach needs a relative threshold, an absolute floor and two breaches in a row.
4. A test package fails CI only when it runs more than min(2 × the ceiling of its size class, `TEST_TIMEOUT`).
5. Each soft breach files one inbox item for its measure. The item closes when the measure recovers.

## Consequences

- A regression that a test can prove cannot merge.
- A regression of measured time shows up as an item within a few runs or waves, with its numbers.
- The simulation bound and the ratchet need a new baseline when the kind of work changes.

## Rejected options

The plan scores four designs and gives the reasons: [plan §2](../../plans/latency-gates-2026-10.md#2-candidate-designs).
