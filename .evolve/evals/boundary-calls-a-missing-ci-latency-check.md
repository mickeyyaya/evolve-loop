---
score_cap:
  - criterion: "The boundary plan runs no evolve ci latency step, because that verb does not exist"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2641 -run '^TestC2641_001_BoundaryPlanRunsNoMissingCILatencyVerb$'"
  - criterion: "The boundary plan keeps evolve wave latency --check between the quality report and gc"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2641 -run '^TestC2641_002_BoundaryPlanKeepsTheWaveLatencyCheckAdvisoryBeforeGC$'"
  - criterion: "A test pins that each boundary step names a registered verb and subverb"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestBoundarySteps_EachStepNamesARegisteredVerbAndSubverb$'"
---

# Eval: the boundary runs only verbs that exist

> The boundary of evolve boundary run and evolve wave next ran evolve ci latency check. The ci verb has no latency subverb, so the step only printed a usage WARN. The fix removes the step and keeps evolve wave latency --check, which exists. Source: cycle 2641.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dead-step | No ci latency step in the plan | 5/10 | `TestC2641_001_BoundaryPlanRunsNoMissingCILatencyVerb` |
| preserved | The wave latency check stays in order | 6/10 | `TestC2641_002_BoundaryPlanKeepsTheWaveLatencyCheckAdvisoryBeforeGC` |
| pin | Each step names a registered verb and subverb | 6/10 | `TestBoundarySteps_EachStepNamesARegisteredVerbAndSubverb` |
