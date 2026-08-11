---
score_cap:
  - criterion: "PhaseSpec.ClassifyWithDefaults() applies evaluate-archetype defaults (fail_if_empty=true, verdict_on_pass=PASS) when fields are zero"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestClassifyWithDefaults ./internal/phasespec/"
  - criterion: "At least 25 phase.json files have boilerplate fields stripped (kind, optional, model, writes_source, prompt_context, fail_if_empty, verdict_on_pass not repeated when they match archetype defaults)"
    max_if_missing: 3
    evidence: "grep -rL '\"kind\": \"llm\"' .evolve/phases/*/phase.json | wc -l"
  - criterion: "evolve phase inventory shows same phase list before and after (no phases lost)"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 ./internal/phasespec/... ./internal/phaseconfig/..."
---

# Eval: Phase JSON archetype defaults extraction

35/35 phase.json files repeat identical boilerplate: `kind:llm`, `optional:true`, `model:auto`,
`writes_source:false`, `prompt_context:["goal"]`, `fail_if_empty:true`, `verdict_on_pass:"PASS"`.
Only the evaluate archetype's `routing`, `classify.fail_if_signal`, `classify.require_sections`,
`inputs`, `outputs`, and `after` fields are actually unique per phase.

The fix: add a `PhaseSpec.ClassifyWithDefaults(archetype)` accessor and a loader-side
`ApplyArchetypeDefaults(spec)` function. Remove the redundant fields from phase.json files.

## Graders

### [code] ClassifyWithDefaults populates fail_if_empty and verdict_on_pass for evaluate archetype

```bash
cd go && go test -count=1 -run TestClassifyWithDefaults ./internal/phasespec/ 2>&1 | tail -3
# expected: PASS
```

### [code] Phase.json files no longer contain the evaluate-archetype boilerplate (bulk)

```bash
# Phases with archetype:evaluate should NOT have explicit "kind":"llm" after stripping
stripped=$(grep -rL '"kind": "llm"' .evolve/phases/*/phase.json 2>/dev/null | wc -l | tr -d ' ')
echo "Phases without explicit kind:llm: $stripped"
# expect >= 25 (most phases stripped)
test "$stripped" -ge 25 && echo "PASS" || echo "FAIL: only $stripped phases stripped"
```

### [code] Phase inventory test suite still green (no regressions)

```bash
cd go && go test -count=1 ./internal/phasespec/... ./internal/phaseconfig/... 2>&1 | grep -E "^(ok|FAIL)"
# expected: both ok
```

### [code] Negative: a phase with non-default archetype is NOT given evaluate defaults

```bash
# 'intent' phase (control archetype) must retain its own classify rules, not get evaluate defaults
cd go && go test -count=1 -run TestApplyArchetypeDefaults_ControlPhase ./internal/phasespec/ 2>&1 | tail -3
# expected: PASS
```

### [model] Spot-check: adversarial-review/phase.json is minimal (only unique fields remain)

Verify that `.evolve/phases/adversarial-review/phase.json` contains only:
- `name`, `archetype`, `routing`, `inputs`, `outputs`, `classify.require_sections`, `classify.fail_if_signal`, `after`

And does NOT repeat: `kind`, `optional`, `model`, `writes_source`, `prompt_context`,
`classify.fail_if_empty`, `classify.verdict_on_pass`.
