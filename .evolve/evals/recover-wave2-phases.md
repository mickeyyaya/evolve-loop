---
score_cap:
  - criterion: "All 4 recovered Wave-2 phase descriptors validate via the engine"
    max_if_missing: 6
    evidence: "for p in benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do EVOLVE_PHASE_ROOTS=$(git rev-parse --show-toplevel)/.evolve/phases $(git rev-parse --show-toplevel)/go/bin/evolve phases validate $p || exit 1; done"
  - criterion: "Wave-2 phase artifacts are git-tracked (not gitignore-shadowed)"
    max_if_missing: 5
    evidence: "git ls-files --error-unmatch .evolve/phases/benchmark-gate/phase.json .evolve/phases/fuzz-probe/phase.json .evolve/phases/cleanup-sweep/phase.json .evolve/phases/rollback-plan/phase.json .evolve/profiles/benchmark-gate.json .evolve/profiles/fuzz-probe.json .evolve/profiles/cleanup-sweep.json .evolve/profiles/rollback-plan.json"
  - criterion: "Phase validation rejects unknown phase names (negative contract)"
    max_if_missing: 7
    evidence: "! EVOLVE_PHASE_ROOTS=$(git rev-parse --show-toplevel)/.evolve/phases $(git rev-parse --show-toplevel)/go/bin/evolve phases validate cycle247-no-such-phase"
---

# Eval: Recover Wave-2 quality-gate phases from cycle-246

> Pins the recovery of dangling commit `aea56ca` (cycle-246 Wave-2 work:
> benchmark-gate, fuzz-probe, cleanup-sweep, rollback-plan — 22 config-only
> files) onto main, and the validate-engine contracts those phases depend on.
> Source incident: cycle-246's ship failed (SELF_SHA_TAMPERED), leaving
> ACS-green work (71/71) unreachable from main; cycle-247 recovers it. The
> negative-rejection cap guards the anti-no-op contract: a validate that
> accepts anything would make the positive caps meaningless.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| validate-positives | 4 recovered phases validate exit 0 | 6/10 | per-phase `evolve phases validate` loop |
| tracking-guard | descriptors + profiles git-tracked | 5/10 | `git ls-files --error-unmatch` (dual-check rule, cycle-93+) |
| validate-negative | unknown phase rejected | 7/10 | `! evolve phases validate cycle247-no-such-phase` |
