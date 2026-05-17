# ADR-0011: Intent STOP CRITERION Tightening (C74 Calibration)

**Status:** Accepted  
**Date:** 2026-05-17  
**Cycle:** 74

---

## Context

5-cycle turn-cost measurement (C69–C73) identified intent as a chronic overrun with monotonically worsening trend:

| Cycle | Turns | Profile max | Overrun % |
|-------|-------|-------------|-----------|
| C69   | 9     | 10          | -10% (compliant) |
| C70   | 12    | 10          | +20%      |
| C71   | 13    | 10          | +30%      |
| C72   | 13    | 10          | +30%      |
| C73   | 15    | 10          | +50%      |

**Average overrun (C70–C73): +32.5%. 4/5 cycles overrun. Slope: +1 turn/cycle.**

The existing "Maximum 2 turns" line in `agents/evolve-intent.md` § Turn budget is a design-intent declaration, not a runtime exit trigger. There is no `## STOP CRITERION` section. The agent reads the advisory, then proceeds to read prior intent.md files, inspect cycle state, and draft/refine YAML frontmatter iteratively — each loop adding 1-2 turns. With no turn-numbered "stop now and write" trigger, the agent continues until the `max_turns=10` profile ceiling — which is advisory-only (ADR-0009) and is itself exceeded (12-15 turns observed).

C69 (9 turns, compliant, no file-read spiral) demonstrates intent CAN complete within budget when output is written promptly.

Intent has no WebSearch/WebFetch in its profile (structurally stripped in v9.0.2), so no research-spiral pattern can occur. No web research deadline is needed — unlike Scout (ADR-0010).

---

## Decision

Add a `## STOP CRITERION` section to `agents/evolve-intent.md` with:

| Trigger | Turn | Rationale |
|---------|------|-----------|
| Emergency Exit | 5+  | 50% of profile max; allows 3 context reads before forcing output |
| Hard Stop      | 7   | 70% of profile max; absolute cap below advisory max |

**No web research deadline:** tools stripped; spiral pattern not possible.

Calibration follows the C73 scout ratio (Emergency Exit ≈ 47% of profile max; Hard Stop ≈ 67%) applied to intent profile max=10. Scout used 7/15 and 10/15; intent uses 5/10 and 7/10.

---

## Consequences

**Positive:**
- Creates a forcing function before the agent commits to a multi-turn refinement chain
- Partial `intent.md` with `TURN-BOUNDED` prefix is more useful to Scout than a timeout
- Expected turn reduction: from avg 12.4 (C70–C73) toward 5-7 turns
- Estimated cost saving: ~$0.36/cycle (8 turns × ~600 output tokens × $75/MTok)

**Negative / Risk:**
- Tighter constraints may produce more TURN-BOUNDED partial intents in ambiguous-goal cycles
- Builder/auditor overrun patterns deferred — intent + scout addressed; triage/builder/auditor remain

---

## Rollback

```bash
git revert <sha-of-this-commit>
```

The revert removes the `## STOP CRITERION` section from `agents/evolve-intent.md`, restoring the advisory-only "Maximum 2 turns" as the sole turn constraint.

---

## References

- ADR-0009: `docs/architecture/adr/0009-p2-turn-budget-inert.md` — confirms `max_turns` is advisory-only
- ADR-0010: `docs/architecture/adr/0010-scout-stop-criterion-tightening.md` — scout calibration template
- Intent report C74: `.evolve/runs/cycle-74/intent.md`
- Scout report C74: `.evolve/runs/cycle-74/scout-report.md`
- Token economics research: `knowledge-base/research/` (P1-P8 roadmap)
