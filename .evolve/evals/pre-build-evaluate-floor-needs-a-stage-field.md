---
score_cap:
  - criterion: "phase.json accepts a stage field with a closed set of values (pre-build, post-build) and validation, registry load and the descriptor schema all refuse an unknown value"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestStage_DecodesFromPhaseJSON|TestValidateUserSpec_StageIsAClosedSet|TestApplyUserRouting_AnUnknownStageIsNotRouted|TestLoad_ARegistrySpecWithAnUnknownStageIsAHardError|TestDescriptorSchema_StageEnumIsTheClosedSet' ./internal/phasespec"
  - criterion: "the routing splice refuses a pre-build phase placed after build, with the registered routing-defect signal, and still routes pre-build phases before build, unstaged phases and post-build phases"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestApplyUserRouting_(RefusesAPreBuild|PreBuildPhasesBeforeBuild|OnlyAPreBuildStage|APreBuildPhaseIsLegal)' ./internal/phasespec && go test -count=1 -run 'TestWireOrchestrator_A(Pre|Post)Build' ./cmd/evolve"
  - criterion: "bug-reproduction and fault-localization declare the pre-build stage and the real-tree window pin reads the field, not the phase names"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestRepoPhaseCatalog_(ThePreBuildEvaluatePhasesDeclareTheirStage|EveryTrackedStageIsInTheClosedSet)' ./internal/phasespec && go test -count=1 -run 'TestRealTreeRoutingOrder_' ./cmd/evolve"
  - criterion: "the phasespec package note and the phase.json field table document the stage field"
    max_if_missing: 5
    evidence: "grep -q '| `stage` |' docs/architecture/user-defined-phases.md && grep -q 'pre-build' docs/architecture/packages/internal-phasespec.md"
---

# Eval: Phase specs need a stage field so the routing floor can refuse a red-first Evaluate phase placed after build

> Pins the `stage` field of a phase spec (`pre-build`, `post-build`) and the refusal of a pre-build phase that the routing splice would place after build. Source incident: cycle 1550, where `bug-reproduction` ran after build and reded the lane; the realtree window pin named the phases in code until this field replaced it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| closed-set | stage values validated in spec, registry and schema | 6/10 | `go test -run TestValidateUserSpec_StageIsAClosedSet ./internal/phasespec` |
| refusal | pre-build after build refused with a routing-defect signal | 8/10 | `go test -run TestApplyUserRouting_RefusesAPreBuild ./internal/phasespec` |
| declaration | the two tracked phases declare pre-build, the pin reads the field | 7/10 | `go test -run TestRealTreeRoutingOrder_ ./cmd/evolve` |
| docs | package note and field table name the field | 5/10 | `grep` of the two documents |
