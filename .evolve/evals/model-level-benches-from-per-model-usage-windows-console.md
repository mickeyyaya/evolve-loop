---
score_cap:
  - criterion: "A model bench demotes only the claude-tmux seats whose tier resolves to the benched model, through the production bench step"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestApplyModelBench_' ./internal/bridgechain"
  - criterion: "The scope-word to model map is manifest data (model_match) decided by the window spec alone"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestWindowSpec_ModelMatch|TestWindowSpec_Validate' ./internal/quotastate"
  - criterion: "The canary probes an expired model bench with its model and the pre-wave step reaches it"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestCanary_|TestRunPreWaveProbes_ProbesAnExpiredModelBench' ./cmd/evolve"
  - criterion: "The store lists expired model benches and clears one model bench without touching the family bench"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestStore_ExpiredModels|TestStore_ClearModel' ./internal/clihealth"
  - criterion: "An unmapped scope word writes no model bench row at the usage probe, and the production prober reads model_match from the manifest"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestProber_.*Mapped|TestProber_AnUnmapped|TestProber_OnlyTheMapped' ./internal/usageprobe && go test -count=1 -run 'TestNewUsageProber_' ./cmd/evolve"
  - criterion: "A model canary probe that booted another model keeps the model bench"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestCanary_AModelProbeThatBootedAnotherModelKeepsTheBenchUntouched' ./cmd/evolve"
  - criterion: "An unmapped scope word benches no model through the usage-evidence path and through any Prober that reads windows via the shared reader"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestNew_AnExhausted|TestReadWindowsNow_' ./internal/usageevidence"
  - criterion: "The unmapped rule has one mechanism (the reader's Unmapped flag), no Prober seam (cycle 2225 audit M1)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC2225_012' ./acs/cycle2225"
  - criterion: "Every export of the touched enforced packages is named by a test (cycle 2225 audit apicover-enforce)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC2225_013' ./acs/cycle2225"
  - criterion: "A model canary probe refused by the seat token holder keeps the model bench (cycle 2225 audit L1)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestCanary_AModelProbeRefusedByTheSeatTokenHolderKeepsTheBenchUntouched' ./cmd/evolve"
---

# Eval: Model-level benches reach routing and the canary

> Pins steps 3 to 5 of docs/plans/model-level-benches-2026-10.md: an exhausted per-model usage window demotes only the tiers that resolve to that model, an unmapped scope word benches nothing, and the canary probes an expired model bench with `doctor live --model`. Source incident: cycle 2102 audit FAIL, which built all five components at once.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| router-demotion | Opus bench demotes deep and top, keeps balanced | 6/10 | `go test -run TestApplyModelBench_ ./internal/bridgechain` |
| manifest-map | model_match is data in the window spec | 6/10 | `go test -run TestWindowSpec_ModelMatch ./internal/quotastate` |
| canary | Expired model bench probed with its model | 7/10 | `go test -run TestCanary_ ./cmd/evolve` |
| store | ExpiredModels and ClearModel | 5/10 | `go test -run TestStore_ClearModel ./internal/clihealth` |
| unmapped-word-store | An unmapped word writes no model row; production wires model_match (cycle 2160 audit H1) | 6/10 | `go test -run TestNewUsageProber_ ./cmd/evolve` |
| model-mismatch | ExitModelMismatch keeps the model bench (cycle 2160 audit M1) | 5/10 | `go test -run TestCanary_AModelProbeThatBootedAnotherModelKeepsTheBenchUntouched ./cmd/evolve` |
| unmapped-word-evidence-path | usageevidence.New and the shared reader bench no unmapped word (cycle 2160 audit round 2 H1) | 6/10 | `go test -run 'TestNew_AnExhausted|TestReadWindowsNow_' ./internal/usageevidence` |
| one-mechanism | Prober has no ModelMapped seam (cycle 2225 audit M1) | 6/10 | `go test -tags acs -run TestC2225_012 ./acs/cycle2225` |
| apicover | no unnamed export in touched enforced packages (cycle 2225 audit) | 6/10 | `go test -tags acs -run TestC2225_013 ./acs/cycle2225` |
| seat-token | seat token refusal keeps the model bench (cycle 2225 audit L1) | 4/10 | `go test -run TestCanary_AModelProbeRefusedByTheSeatToken ./cmd/evolve` |
