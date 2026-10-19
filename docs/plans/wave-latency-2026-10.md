# Plan: a wider wave, a faster build and gates that test once

- **Status:** accepted, 2026-10-19. The operator chose the three decisions of §4 on 2026-10-19.
- **Research:** [wave-latency-2026-10](../research/wave-latency-2026-10.md). **Decision:** [ADR-0143](../architecture/adr/0143-admission-holds-only-shared-edits-and-the-ship-gate-owns-api-drift.md).
- **Related:** [ADR-0139](../architecture/adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md) (the wave orchestrator), [ADR-0105](../architecture/adr/0105-identity-preserving-fleet-rebase.md) (the identity rebase), [ADR-0136](../architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md) (quota pools per tier).

## Table of contents

- [1. The problem](#1-the-problem)
- [2. The levers](#2-the-levers)
- [3. Candidate designs for each lever](#3-candidate-designs-for-each-lever)
- [4. Decisions](#4-decisions)
- [5. Components and order](#5-components-and-order)
- [6. Verification](#6-verification)
- [7. Limits](#7-limits)

## 1. The problem

Waves 130 to 133 shipped 18 cycles in 12.8 hours: 12.0 hours of waves and 46 minutes of boundaries. That is about 1.4 ships per hour. The research finds three causes, in order of size:

1. Half of the slots were idle (F2.1). The pool backlog cause is fixed (F2.2). The admission reach is the cause that remains (F2.3 to F2.5).
2. Build is 46% of lane time. An agy build takes about 2.4 times the model time of a claude build (F3.1, F3.3).
3. The gates are 28% of lane time, and the ship tests again what the floor tested (F3.2, F4.2).

## 2. The levers

| Lever | Expected gain | Evidence |
|---|---|---|
| A. Admission | up to 2 more live lanes while a lane edits `internal/core` | F2.3, F2.4, F2.5 |
| B. Build family | about 14 minutes less model time for each build that moves from agy to claude, while quota allows it | F3.3, F3.5 |
| C. One verdict store | the ship skips a package that the floor tested green on the same content key. That saves about 3 minutes for each ship and some CPU load. | F4.1, F4.2, F4.3 |

## 3. Candidate designs for each lever

Each criterion scores 1 (poor) to 5 (good). The criteria are the gain, the risk and the change size.

### Lever A: admission

| Design | Gain | Risk | Change | Total |
|---|---|---|---|---|
| A1. Keep the reach rule | 1 | 5 | 5 | 11 |
| A2. Reach of direct imports only | 1 | 4 | 4 | 9 |
| A3. Optimistic admission: hold only shared edits and the hub; the ship gate owns API drift | 5 | 3 | 4 | **12** |
| A4. Reach of exported-API changes only | 4 | 4 | 1 | 9 |
| A5. Overlap of reverse dependencies | 1 | 5 | 3 | 9 |

A3 wins. A2 frees almost nothing for a core lane, because core imports 86 packages directly (F2.4). A4 needs a signal that exists only after the build. A3 costs a rebase and a gate run when two lanes really collide, and the ship already pays that cost (F2.5).

Current practice agrees (research O1 to O5, ADR-0143).

### Lever B: build family

| Design | Gain | Risk | Change | Total |
|---|---|---|---|---|
| B1. Keep agy first | 1 | 5 | 5 | 11 |
| B2. Claude first always | 5 | 2 | 5 | 12 |
| B3. Headroom-aware order: claude first while the Claude week window has headroom, agy first after | 4 | 4 | 3 | **11, chosen** |

B2 scores highest on this table, but it spends the Claude week window with no limit. The week window ran out on 2026-10-18 in the burn window. The operator chose B3: it takes the gain while the window is low and gives it back before the window runs out. B3 is the standard router shape: filter on quota, then order by measured speed (O8). The speed to measure is the lane time to a PASS for each family. The time of one call does not predict the result (O9).

### Lever C: verdict store

| Design | Gain | Risk | Change | Total |
|---|---|---|---|---|
| C1. Each gate runs its own tests | 1 | 5 | 5 | 11 |
| C2. The floor and the audit write the store; the ship reads it at stage shadow, then enforce | 4 | 4 | 3 | **11, chosen** |
| C3. One gate run per tree SHA, shared by all phases | 5 | 2 | 1 | 8 |

C2 ties C1 on the total. It is the only design with both a gain and a risk score of 4 or more. It reuses the store and the content key that the ship shadow already proved (0 escapes in 1826 results, F4.3). C3 changes the phase order of the gates. The Go test cache is not an option, because our tests read files outside the module (O6).

## 4. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Admission holds a candidate only when it edits a package that a running lane edits, or when both are hubs. It drops the reach check. The ship-window lease usually serializes the ships of overlapping lanes. | F2.3 to F2.5, O1 to O5 | A1, A2, A4, A5: see §3. |
| D2 | D1 starts at stage `shadow`. The admission shadow records each candidate that the reach rule holds and D1 admits, and the ship records each rebase conflict between two such lanes. | the stage pattern of ADR-0139 | Enforce at once: the conflict rate is unknown. |
| D3 | After one wave of shadow, D1 moves to `enforce` if fewer than one in five new admissions fail the ship gate. | F2.5 | A fixed date: the data decides. |
| D4 | The build seat drops a family whose week window is at or above `headroom_pct` (default 60%). It orders the rest by the measured lane time to a PASS of each family over the last 20 builds, from W0. With no measure yet, the order is claude, agy, agy-claude. A quota exit puts the family in the present cooldown. | F3.3, F3.5, O8, O9 | B1, B2: see §3. An order by call latency: O9. |
| D5 | Until D4 lands, the policy sets the builder to claude first at the next boundary, because the week window is 14% used. | F3.5 | Wait for D4: the gain starts now. |
| D6 | The build floor and the audit write their package verdicts to the store of `internal/testverdict` with the content key of the ship. The key also holds the toolchain, the test flags and the gate id. The store refuses a record with no gate id, and a FAIL is never reused. The ship skips a package with a green verdict on the same key. At `shadow` the ship looks up, still runs, and records each mismatch. After one wave with 0 mismatches it moves to `enforce`, and a new verb, `evolve verdicts wipe`, empties the store. | F4.2, F4.3, O6, O7 | C1, C3: see §3. The Go test cache: O6. |
| D7 | `evolve wave latency` prints the slot use, the lane time by phase, the model and host split and the gate time of a wave. | research §7 | A script outside the repository: no one can run it again. |

### Consequences

- Good: a lane that edits `internal/core` no longer holds the rest of the backlog.
- Good: build and gates take less time in each lane, so a wave ends sooner and a lane runs more cycles.
- Bad: D1 can admit two lanes whose changes collide. The second ship then rebases and runs its gates again. D2 measures this first.
- Bad: D4 and D5 spend more of the Claude week window. The headroom threshold bounds it.

### What changes our mind

- In shadow, one in five or more of the D1 admissions ends in a composed-gate failure. Then D1 stays at shadow, and A4 is the next design.
- A controlled run shows that claude builds do not take less model time than agy builds for the same task. Then D4 returns to agy first.

## 5. Components and order

| # | Component | Home | Route |
|---|---|---|---|
| W0 | `evolve wave latency` (D7) | `cmd/evolve`, `internal/loopwave` | lane |
| W1 | Optimistic admission at shadow (D1, D2) | `internal/admission` | lane |
| W2 | The move of W1 to enforce (D3) | `.evolve/policy.json` | operator, after one wave |
| W3 | Claude first for the builder now (D5) | `.evolve/policy.json` through `evolve cli-routing set` | console, at the next boundary |
| W4 | The headroom-aware family order (D4) | `internal/cliroute`, `internal/usageprobe` | console |
| W5 | One verdict store for the floor, the audit and the ship (D6) | `internal/testverdict`, `internal/core`, `internal/phases/ship` | console |

## 6. Verification

- **W0:** on the run dirs of wave 133 the report prints an average of 2.26 live lanes and build as the largest phase.
- **W1:**
  - with a running lane that edits `internal/core`, a candidate that edits `internal/router` is admitted;
  - two candidates that edit one package still never run at once, and the hub rule is unchanged;
  - at shadow, the admitted set is unchanged, and the shadow records the new admission.
- **W4:**
  - with the week window at 14% and no lane-time measure, the builder chain starts with claude;
  - at 60% claude leaves the chain head;
  - with a measure, the family with the shorter lane time to a PASS goes first;
  - when the usage probe fails, the chain keeps the policy order.
- **W5:**
  - a package that the floor tested green is skipped at the ship on the same key;
  - a package with a changed input, toolchain or flag is run;
  - a stored FAIL is never reused;
  - a shadow mismatch blocks the move to enforce, and `evolve verdicts wipe` empties the store.
- **Wave level:** after W1 to W5, measure the next three waves. The average live lanes must rise above 2.1, and the ships per hour above 1.4 (§1).

## 7. Limits

- The supply limit of F2.6 stays. Most of the inbox is console-owned, so a wider admission cannot fill slots that have no admissible work.
- D1 trusts the composed-tree gate of the ship. A defect that the tests do not cover can still pass it.
- The lease does not guarantee one ship at a time: a lane that cannot take it ships unleased. The pending item `ship-window-holds-only-the-rebase-and-push` must keep the composed-tree gate inside the leased window.
- The model time of D4 is an estimate from different tasks (F3.3).
