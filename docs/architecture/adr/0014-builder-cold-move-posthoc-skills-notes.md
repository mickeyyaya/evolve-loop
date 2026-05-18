# ADR-0014: Builder Cold-Move — POSTHOC, Skills Invoked, Builder Notes

**Status:** Accepted
**Date:** 2026-05-18
**Cycle:** 76
**Related:** [ADR-0013 orchestrator phase loop cold-move](0013-orchestrator-phase-loop-cold-move.md), [ADR-0012 commit-claim coherence](0012-commit-claim-coherence.md)

---

## Context

Token-optimization campaign Stage 7. `agents/evolve-builder.md` ranked as the largest persona at 380 lines. It has a companion reference doc (`agents/evolve-builder-reference.md`, 225 lines) established in prior campaign work.

Three sections were identified as cold-path: consulted only when a specific compliance situation arises, yet loaded on every Builder turn at the cost of ~47 lines of prompt tokens per invocation.

---

## Decision

Cold-move three sections from `evolve-builder.md` to `evolve-builder-reference.md`, replacing each with a 1–2 line pointer to the reference section.

### Cold-moves

| Section | Hot-path before | Hot-path after | Net removed |
|---------|----------------|----------------|-------------|
| POSTHOC enforcement (metric table + INERT example) | 36 lines | ~3 lines | −33 lines |
| Skills Invoked output template | 9 lines | 1 line | −8 lines |
| Step 9 Retrospective template | 12 lines | 1 line | −11 lines |

**Total removed from builder.md:** ~52 lines (actual: 54 lines, −14.2%)

### Files changed

| File | Change |
|------|--------|
| `agents/evolve-builder.md` | −54 lines (380 → 326); three sections compressed to pointers |
| `agents/evolve-builder-reference.md` | +71 lines (225 → 296); three new `## Section:` blocks added |

### Key invariant preserved

The behavioral rule in the hot path is retained for each cold-move:
- POSTHOC: "Use `pending <!-- POSTHOC: <cmd> -->` placeholders. INERT marks MUST include `re_attempt_by_cycle`." ✓
- Skills Invoked: pointer to `tool-hygiene-rules` section remains ✓
- Retrospective: write instruction + ≤20 line constraint remain ✓

---

## Consequences

- Builder hot-path tokens reduced by ~14.2% per turn
- Cold-path detail accessible via reference doc when compliance situation arises
- Consistent with Campaign D pattern: compress hot-path, expand reference
- All 7 acceptance criteria pass (AC1–AC7 in scout-report.md)
