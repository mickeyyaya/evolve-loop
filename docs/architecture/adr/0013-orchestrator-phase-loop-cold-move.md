# ADR-0013: Orchestrator Phase Loop Body Cold-Move

**Status:** Accepted
**Date:** 2026-05-18
**Cycle:** 75
**Related:** [ADR-0010 scout STOP](0010-scout-stop-criterion-tightening.md), [ADR-0011 intent STOP](0011-intent-stop-criterion-tightening.md), [ADR-0012 commit-claim coherence](0012-commit-claim-coherence.md)

---

## Context

Token-cost ranking across cycles 70–74 identified `agents/evolve-orchestrator.md` as the top-1 sustained cost driver: $5.82 total, $1.16/cycle average, running on every cycle without exception.

The `## Phase Loop` section (lines 98–158, ~60 lines) was explicitly marked *legacy* in the file itself:

> *Legacy reference — actual sequence driven by phase-registry.json when `EVOLVE_USE_PHASE_REGISTRY=1` (default)*

This section duplicates content present in two canonical locations:
- The **Verdict Decision Tree** (orchestrator.md lines 244–253) — actionable PASS/WARN/FAIL routing
- The **registry-dispatch section** of `orchestrator-reference.md` — canonical dispatch loop

Removing it reduces orchestrator.md by ~55 lines (16.4%), lowering hot-path prompt tokens loaded into every orchestrator turn.

### ACS Constraint

`acs/regression-suite/cycle-42/002-p-new-16-orchestrator-stop-criterion.sh` AC3 asserts that the `## Phase Loop` heading EXISTS in orchestrator.md. The heading must be preserved; only the body may be moved.

### ADR Numbering

ADR-0012 is taken (`docs/architecture/adr/0012-commit-claim-coherence.md`). This record is correctly numbered ADR-0013.

---

## Decision

Move the Phase Loop body (lines 101–158) to `agents/evolve-orchestrator-reference.md` as `## Section: legacy-phase-loop`. Replace the body in orchestrator.md with a 2-line pointer to the reference section. Keep the heading and legend line in orchestrator.md.

### Files changed

| File | Change |
|------|--------|
| `agents/evolve-orchestrator.md` | Phase Loop body (lines 101–158) replaced with 2-line pointer |
| `agents/evolve-orchestrator-reference.md` | Appended `## Section: legacy-phase-loop` with full moved body |
| `acs/cycle-75/001-orchestrator-phase-loop-reduction.sh` | AC1–AC3 regression predicates |

---

## Consequences

### Positive

- Orchestrator hot-path prompt reduced by ~55 lines (341 → 286, −16.1%)
- All duplicated content preserved with attribution at the canonical cold-path reference location
- No behavior change: phase-registry.json drives actual dispatch; the legacy sequence is documentation only

### Negative / Risks

- Operators reading orchestrator.md must follow the pointer for full legacy sequence detail (one redirect)
- Reference doc grows by ~60 lines (215 → 277); reference is cold-path (not loaded each turn)

### Preserved invariants

- `## Phase Loop` heading survives: regression-suite/cycle-42 AC3 satisfied
- `MERGE_RC` patterns survive: Verdict Decision Tree retains both patterns (cycle-43 ACS safe)
- cycle-57 ACS022 satisfied: registry-dispatch remains in reference doc
- C73/C74 STOP CRITERION patches not reverted

---

## Rollback

```bash
git revert <patch-sha>
```

Revert restores orchestrator.md to 341 lines and removes the `## Section: legacy-phase-loop` from reference. AC2 of `acs/cycle-75/001-orchestrator-phase-loop-reduction.sh` will FAIL after revert, confirming the predicate is non-tautological.
