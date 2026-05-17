# ADR-0010: Scout STOP CRITERION Tightening (C73 Calibration)

**Status:** Accepted  
**Date:** 2026-05-17  
**Cycle:** 73

---

## Context

5-cycle turn-cost measurement (C68-C72) identified scout as the dominant chronic overrun:

| Cycle | Turns | Budget | % |
|-------|-------|--------|---|
| C68 | 8 | 15 | 53% |
| C69 | 44 | 15 | 293% |
| C70 | 36 | 15 | 240% |
| C71 | 32 | 15 | 213% |
| C72 | 31 | 15 | 207% |

**Average: 201% of budget. 4/5 cycles overrun. 4 consecutive cycles (C69-C72).**

The existing Hard Stop at turn 14 was not constraining behavior. Root cause: the 5 completion gates (system-health, inbox-audit, backlog, build-plan, research-cache) drive extensive data gathering before writing begins. By turn 14, the scout has already committed to a multi-turn research chain it completes before writing.

Profile recalibration (raising `max_turns` from 15 to 35) was considered and rejected: `max_turns` is advisory-only (confirmed by ADR-0009; `claude -p` has no `--max-turns` flag). Raising the ceiling accepts the overrun without fixing it.

C68 (8 turns, compliant, no web research spiral) demonstrates scout CAN complete within budget when research is bounded.

---

## Decision

Tighten the STOP CRITERION in `agents/evolve-scout.md` as follows:

| Item | Before | After |
|------|--------|-------|
| Emergency Exit trigger | Turn 12 | Turn 7 |
| Hard Stop trigger | Turn 14 | Turn 10 |
| Web research deadline | None (3-call cap only) | Must complete by turn 5 |

The web research deadline (turn 5) is a new constraint that fires before the Emergency Exit to interrupt the research-spiral before it becomes entrenched.

**Rationale for chosen turn numbers:**
- C68 completed in 8 turns (compliant); incremental mode target is 6-8 turns
- Hard Stop at 10 gives 2 turns of buffer above the 8-turn target
- Web research deadline at turn 5 ensures web fetches happen early, not as a driver of additional turns

---

## Consequences

**Positive:**
- Creates an earlier forcing function before scout commits to a deep research chain
- Web research deadline interrupts the spiral at the source (not after it starts)
- Profile recalibration deferred: advisory-only fields unchanged; actual behavior changes via text constraint

**Negative / Risk:**
- Tighter constraints may produce more TIME-BOUNDED partial reports in complex cycles
- Intent overrun (3/5 cycles, 114% avg) deferred to C74 — unaddressed for now

---

## Rollback

```bash
git revert <sha-of-this-commit>
```

The revert restores Emergency Exit to turn 12, Hard Stop to turn 14, and removes the web research deadline clause from `agents/evolve-scout.md`.

---

## References

- ADR-0009: `docs/architecture/adr/0009-p2-turn-budget-inert.md` — confirms `max_turns` is advisory-only
- Scout report C73: `.evolve/runs/cycle-73/scout-report.md`
- Token economics research: `knowledge-base/research/` (P1-P8 roadmap)
