---
score_cap:
  - criterion: "An exhausted per-model usage window benches that (family, model) only, until its reset"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestProber_AnExhaustedPerModelWindowBenchesThatModelOnly' ./internal/usageprobe/"
  - criterion: "A model bench never appears in Active() or Expired(), so the router and the canary see no new family"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestStore_AModelBenchIsNotAFamilyBench|TestStore_AModelBenchEndsAtItsResetAndNeverBecomesACanaryFamily' ./internal/clihealth/"
  - criterion: "The canary leaves an expired model bench alone instead of probing it as a family"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCanary_AnExpiredModelBenchIsNotProbedAsAFamily' ./cmd/evolve/"
  - criterion: "A window mapped to no routing family benches no model and keeps the WARN"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestProber_AModelWindowMappedToNoFamilyBenchesNoModel' ./internal/usageprobe/"
---

# Eval: Model-level benches from per-model usage windows (store and probe, unwired)

> Pins steps 1-2 of docs/plans/model-level-benches-2026-10.md: the clihealth store gains a model-scoped entry and the usage probe writes it for an exhausted per-model window. Routing, the canary probe and the operator surfaces (steps 3-5) are a follow-up. Source incident: cycle 1981, a drained claude Opus or Fable week only logged a WARN and left the phase to fail at run time.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| probe-writes-model-bench | exhausted per-model window benches (family, model) | 6/10 | `go test -run TestProber_AnExhaustedPerModelWindowBenchesThatModelOnly ./internal/usageprobe/` |
| family-view-isolation | Active() and Expired() hold no model entry | 7/10 | `go test -run TestStore_AModelBenchIsNotAFamilyBench ./internal/clihealth/` |
| canary-safety | canary does not probe a model key as a family | 6/10 | `go test -run TestCanary_AnExpiredModelBenchIsNotProbedAsAFamily ./cmd/evolve/` |
| unmapped-fail-open | no family mapping benches nothing | 5/10 | `go test -run TestProber_AModelWindowMappedToNoFamilyBenchesNoModel ./internal/usageprobe/` |
