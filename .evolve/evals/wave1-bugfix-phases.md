---
score_cap:
  - criterion: "fault-localization validates as a user phase through the real Go loader"
    max_if_missing: 7
    evidence: "EVOLVE_PROJECT_ROOT=\"$PWD\" go/bin/evolve phases validate fault-localization"
  - criterion: "reproduce-bug validates as a user phase through the real Go loader"
    max_if_missing: 7
    evidence: "EVOLVE_PROJECT_ROOT=\"$PWD\" go/bin/evolve phases validate reproduce-bug"
  - criterion: "reproduce-bug carries the FAIL_TO_PASS gate (a repro that doesn't fail pre-patch is a failed phase)"
    max_if_missing: 8
    evidence: "jq -e '.classify.fail_if_signal[\"repro.failing\"] == \"==false\"' .evolve/phases/reproduce-bug/phase.json"
  - criterion: "fault-localization routes on bugfix goal cycles"
    max_if_missing: 6
    evidence: "jq -e '.routing.insert_when[]? | select(.field==\"scout.goal_type\" and (.op==\"==\" or .op==\"eq\") and .value==\"bugfix\")' .evolve/phases/fault-localization/phase.json"
---

# Eval: Wave-1 bugfix micro-phases (fault-localization + reproduce-bug)

> Pins the bugfix recipe chain introduced in cycle 217 (micro-phase catalog §3
> Wave 1, carryoverTodo micro-phase-wave-1): `fault-localization` (hierarchical
> suspect narrowing, Agentless arXiv 2407.01489) and `reproduce-bug` (FAIL_TO_PASS
> reproduction before any patch, TestPrune 2510.18270, +9.4–12.9% resolution
> lift). Both are ADR-0035 zero-Go user phases under `.evolve/phases/`. The
> `repro.failing == "==false"` gate is the strongest single signal gate of the
> wave — losing it silently degrades reproduce-bug to unverified prose.
> Source incident: none (feature eval); authored per cycle-131 lesson (missing
> `.evolve/evals/<slug>.md` = automatic CRITICAL FAIL at audit).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| validate-fl | fault-localization passes `phases validate` | 7/10 | `go/bin/evolve phases validate fault-localization` |
| validate-rb | reproduce-bug passes `phases validate` | 7/10 | `go/bin/evolve phases validate reproduce-bug` |
| fail-to-pass-gate | repro.failing FAIL_TO_PASS gate present | 8/10 | `jq fail_if_signal check on reproduce-bug/phase.json` |
| bugfix-routing | fault-localization inserts on bugfix goals | 6/10 | `jq insert_when check on fault-localization/phase.json` |
