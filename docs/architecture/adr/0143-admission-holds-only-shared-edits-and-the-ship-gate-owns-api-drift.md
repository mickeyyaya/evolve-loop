# ADR-0143: admission holds only shared edits, and the ship gate owns API drift

- **Status:** Accepted (2026-10-19). Stage `shadow` until the data of one wave meets D3 of the plan.
- **Plan:** [wave-latency-2026-10](../../plans/wave-latency-2026-10.md). **Research:** [wave-latency-2026-10](../../research/wave-latency-2026-10.md).
- **Relates to:**
  - [ADR-0139](0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md): this ADR changes the conflict rule of its admission by cell.
  - [ADR-0105](0105-identity-preserving-fleet-rebase.md): the composed-tree gate of the rebase is the check that replaces the reach rule.
- **Evidence:**
  - Waves 130 to 133 used 51% of their slot-hours (research F2.1).
  - In wave 133 a lane that edits `internal/core` held 13 ready items for 71 minutes (F2.3).
  - `internal/core` imports 86 internal packages directly, so a reach of direct imports frees almost nothing (F2.4).
  - No surveyed system holds a change on its forward imports. Merge queues admit every change and test overlapping changes together before the land (research O1 to O3).

## Context

Admission by cell holds a candidate in three cases. It edits a package that a running lane edits, both are hubs, or its edits meet the reach of a running lane. The reach is every package that the edited packages import, except the hubs. Since cycle 1972, a todo that edits only leaf packages has an empty reach. The reach rule prevents one failure. A candidate changes a package that a running lane compiles against, and the two changes break each other after both land.

The ship already checks for that failure. It rebases each lane onto main and runs the composed-tree gate before the push, and since PR #990 a clean rebase keeps its audit. So the reach rule pays a large cost in width for a check that the ship does anyway.

## Decision

1. Admission holds a candidate only when it edits a package that a running lane edits, or when both are hubs.
2. The reach check leaves admission. The composed-tree gate of the ship owns API drift between lanes.
3. The ship-window lease usually makes the ships of two overlapping lanes run one at a time. When a lane cannot take the lease, it ships unleased, and its rebase and composed-tree gate still run.
4. The change starts at stage `shadow`. The shadow records each candidate that the old rule holds and the new rule admits. The ship records each composed-gate failure between two such lanes.
5. The stage moves to `enforce` after one wave if fewer than one in five such admissions fail the composed gate.

## Consequences

- A lane that edits `internal/core` no longer holds the rest of the backlog, so a wave runs more lanes.
- Two lanes can now collide through an import. The second ship rebases, runs its gates again, and goes back to build only when the gates fail.
- A defect that the tests do not cover can pass the composed-tree gate. The reach rule did not catch that defect either, so the risk does not rise.

## Rejected options

The plan scores four designs ([plan §3](../../plans/wave-latency-2026-10.md#lever-a-admission)). The reach rule as it is costs the most width. A reach of direct imports frees almost nothing for core. A reach of exported-API changes needs a signal that exists only after the build. An overlap of reverse dependencies holds almost every pair, because `cmd/evolve` depends on 270 of the 302 internal packages (research O5).

## What changes this decision

- In shadow, one in five or more of the new admissions ends in a composed-gate failure. Then the stage stays `shadow`, and a reach of exported-API changes is the next design.
- A change to the ship-window lease takes the composed-tree gate out of the leased window. Then this decision must be checked again, because the gate is its safety check.
