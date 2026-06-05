---
score_cap:
  - criterion: "phase-registry.json config.max_optional_insertions == 6 (refactor recipe needs 6 insertions)"
    max_if_missing: 8
    evidence: "python3 -c \"import json,sys; d=json.load(open('docs/architecture/phase-registry.json')); sys.exit(0 if d['config']['max_optional_insertions']==6 else 1)\""
  - criterion: "router persona carries the goal-type recipe table with the bugfix chain wired"
    max_if_missing: 6
    evidence: "grep -q '^## Goal-Type Recipes' agents/evolve-router.md && grep -E '^\\|' agents/evolve-router.md | grep -i bugfix | grep -q fault-localization"
  - criterion: "registry still loads end-to-end through the Go loader after the config edit"
    max_if_missing: 7
    evidence: "EVOLVE_PROJECT_ROOT=\"$PWD\" go/bin/evolve phases list"
---

# Eval: Wave-1 advisor integration (router recipes + insertion cap)

> Pins the advisor-integration half of micro-phase catalog §4, shipped in cycle
> 217: the goal-type recipe table appended to `agents/evolve-router.md`
> (classify-then-route — the router gets a standing prior for which micro-phases
> fit bugfix/feature/refactor/security/performance/release/docs cycles instead
> of re-deriving it every cycle) and `max_optional_insertions` raised 4→6 in the
> `phase-registry.json` config block (the refactor recipe composes 6 optional
> insertions: smell-scan + behavior-baseline + behavior-compare + mutation-gate
> + cleanup-sweep + one slack). Recipes are guidance, not law — ClampPlanToFloor
> remains the deterministic safety net. Regressing the cap to 4 silently
> truncates refactor recipes; losing the table reverts the router to ad-hoc
> composition. Source incident: none (feature eval); authored per cycle-131
> lesson (missing `.evolve/evals/<slug>.md` = automatic CRITICAL FAIL at audit).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| insertion-cap | max_optional_insertions == 6 | 8/10 | `python3 json check on phase-registry.json` |
| recipe-table | Goal-Type Recipes section + bugfix chain wired | 6/10 | `grep section + bugfix row on evolve-router.md` |
| loader-green | registry loads via `evolve phases list` | 7/10 | `go/bin/evolve phases list` |
