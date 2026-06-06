---
score_cap:
  - criterion: "evolve-auditor.md respects the ADR-0015 300-line ceiling (cycle-77 regression predicate GREEN)"
    max_if_missing: 3
    evidence: "bash acs/regression-suite/cycle-77/001-auditor-stage8-cold-move.sh"
  - criterion: "Relocated late sections preserved (moved to reference, not deleted)"
    max_if_missing: 5
    evidence: "grep -qF 'Reflection-sycophancy defect check' agents/evolve-auditor-reference.md agents/evolve-auditor.md"
---

# Eval: Trim evolve-auditor.md to the 300-line ceiling

> Pins the ship-gate contract restored in cycle 230: `agents/evolve-auditor.md`
> must stay within the ADR-0015 300-line ceiling enforced by the cycle-77
> regression predicate, and trims must MOVE content to
> `agents/evolve-auditor-reference.md`, never delete it. Source incident:
> commit 48f8ff7 (auditor C0 block) grew the persona to 319 lines, flipping
> the regression gate RED and blocking every ship across cycles 228–229.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-ceiling | cycle-77 regression predicate exits 0 (≤300 lines + reference section + pointer + ADR) | 3/10 | `bash acs/regression-suite/cycle-77/001-auditor-stage8-cold-move.sh` |
| move-not-delete | relocated section text survives in persona or reference | 5/10 | `grep -qF 'Reflection-sycophancy defect check' agents/evolve-auditor-reference.md agents/evolve-auditor.md` |
