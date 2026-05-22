# ADR-8: Add `retrospective)` Phase Case to role-gate.sh

**Status:** Accepted  
**Date:** 2026-05-17  
**Cycle:** 71  
**Implemented in:** `legacy/scripts/guards/role-gate.sh`, `acs/cycle-71/001-role-gate-retrospective.sh`

---

## Context

The `retrospective` phase was introduced to run lesson extraction after FAIL/WARN cycles. It writes to:

- `$ws/retrospective-report.md` (workspace artifact)
- `$REPO_ROOT/.evolve/instincts/lessons/*.yaml` (lesson persistence)
- `$REPO_ROOT/.evolve/state.json` (state update)

However, `role-gate.sh` had no `retrospective)` case in its `allow_for_phase()` function. The `learn)` case was its structural mirror but was not shared. As a result, the retrospective phase could not write to `instincts/lessons/*.yaml` without triggering a DENY from role-gate — causing silent failures or gate bypasses.

This was identified in cycle-70 retrospective §9 as a structural gap.

---

## Decision

Add a `retrospective)` case to `role-gate.sh:allow_for_phase()` that mirrors the `learn)` case's permissions:

```bash
retrospective)
    match_any "$path" \
        "$ws/retrospective-report.md" \
        "$REPO_ROOT/.evolve/instincts/lessons/*.yaml" \
        "$REPO_ROOT/.evolve/state.json"
    ;;
```

---

## Alternatives considered

1. **Merge `retrospective` into `learn)`** — rejected; phases are distinct pipeline stages with different workspace artifacts and different error semantics. Conflating them would allow `learn` to write `retrospective-report.md` and vice versa.

2. **Add retrospective writes to the `build)` allowlist** — rejected; build phase has worktree-wide write access. Letting retrospective run as build would bypass the per-phase write scoping entirely.

3. **EVOLVE_BYPASS_ROLE_GATE=1 workaround** — rejected; bypass is an emergency hatch, not a permanent solution. It logs WARN and defeats the gate's audit-trail purpose.

---

## Consequences

- **Positive:** Retrospective phase can now write lessons cleanly without gate bypass.
- **Positive:** Change is additive — no existing cases modified, no regression risk.
- **Neutral:** The `retrospective)` allowlist is slightly narrower than `learn)` (no `orchestrator-report.md`), which is correct since retrospective doesn't produce that artifact.

---

## Rollback

```bash
git revert <merge-sha>   # reverts the retrospective) case addition
```

The gate falls back to denying retrospective writes to `instincts/lessons/*.yaml`, which was the pre-fix behavior. No data loss — lesson files already written are unaffected.

---

## Verification

ACS predicate: `acs/cycle-71/001-role-gate-retrospective.sh` — exits 0 when all three conditions hold:

1. `retrospective)` case present in `role-gate.sh`
2. `learn)` case still present (no regression)
3. `retrospective)` case allows `instincts/lessons/*.yaml` writes
