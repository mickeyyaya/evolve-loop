---
score_cap:
  - criterion: "behavior-baseline, behavior-compare and smell-scan all validate as user phases"
    max_if_missing: 7
    evidence: "EVOLVE_PROJECT_ROOT=\"$PWD\" go/bin/evolve phases validate behavior-baseline && EVOLVE_PROJECT_ROOT=\"$PWD\" go/bin/evolve phases validate behavior-compare && EVOLVE_PROJECT_ROOT=\"$PWD\" go/bin/evolve phases validate smell-scan"
  - criterion: "behavior-compare is the gate: hard-fails when behavior.preserved == false"
    max_if_missing: 8
    evidence: "jq -e '.classify.fail_if_signal[\"behavior.preserved\"] == \"==false\"' .evolve/phases/behavior-compare/phase.json"
  - criterion: "the pair straddles build: baseline captures after tdd, compare diffs after build"
    max_if_missing: 7
    evidence: "jq -e '.after==\"tdd\"' .evolve/phases/behavior-baseline/phase.json && jq -e '.after==\"build\"' .evolve/phases/behavior-compare/phase.json"
  - criterion: "smell-scan is a non-writing evaluate phase that fails on an empty report"
    max_if_missing: 6
    evidence: "jq -e '.archetype==\"evaluate\" and ((.writes_source // false)==false) and .classify.fail_if_empty==true' .evolve/phases/smell-scan/phase.json"
---

# Eval: Wave-1 refactor micro-phases (behavior-baseline/compare + smell-scan)

> Pins the refactor safety net introduced in cycle 217 (micro-phase catalog §3
> Wave 1): a golden-master pair that STRADDLES the build phase —
> `behavior-baseline` (after tdd) captures pre-refactor observable behavior,
> `behavior-compare` (after build) re-runs and diffs it, hard-failing on
> `behavior.preserved == false` — plus `smell-scan` (detect-only Fowler-taxonomy
> smell ranking). Evidence base: Feathers characterization tests, DiffKemp
> ICST'21, RefactoringMiner 3.0 TOSEM, LLM smell detection 2504.16027. The
> straddle is load-bearing: if both halves run on the same side of build, the
> safety net verifies nothing. tdd checks NEW behavior; this pair checks
> PRESERVED behavior — the audit flow cannot substitute for it.
> Source incident: none (feature eval); authored per cycle-131 lesson.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| validate-trio | all three phases pass `phases validate` | 7/10 | `go/bin/evolve phases validate` × 3 |
| preserved-gate | behavior-compare fails on behavior.preserved==false | 8/10 | `jq fail_if_signal check on behavior-compare/phase.json` |
| straddle | baseline after tdd, compare after build | 7/10 | `jq .after checks on both phase.json files` |
| smell-shape | smell-scan evaluate / non-writing / fail_if_empty | 6/10 | `jq shape check on smell-scan/phase.json` |
