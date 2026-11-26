---
score_cap:
  - criterion: "A red gate on a composed batch member ejects it through the batch split and a green batch merges"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestGateJourney_|TestL2Gate_' ./cmd/evolve"
  - criterion: "The production batcher names no missing port for the gate and the train gate is the real gate"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestProductionDriver_TheProductionBatcherNamesNoMissingPort|TestIntegrationTrain_TheL2ComposerGateIsTheRealGate' ./cmd/evolve"
  - criterion: "The fixture soak passes and the BatchTest cost per batch is in the package note"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestGateSoak_ ./cmd/evolve && grep -Eq 'BatchTest cost per batch.*[0-9] ?(ms|s)' ../docs/architecture/packages/internal-integration.md"
  - criterion: "Each changed function is fully covered and the no-op gate is gone"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2652_004|TestC2652_006' ./acs/cycle2652"
---

# Eval: The lane batcher tests each composed batch through a real gate

> Pins the gate adapter `l2Gate` of `go/cmd/evolve`. Before this task `LaneBatcher.Missing()` named `Gate`, because the production `Composer` had no gate, and the batch test used `l2NoGate`. The adapter checks out the composed tip, runs `go build` (T1, T2) or `go build` and `go vet` (T3, T4), and returns a `*GateRed` on a red step. Source: inbox item `train-batch-gate-adapter`, cycle 2652.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| red-ejects-green-merges | A red member ejects through the batch split | 4/10 | `go test -run 'TestGateJourney_|TestL2Gate_' ./cmd/evolve` |
| no-missing-gate | Missing() does not name Gate | 5/10 | `go test -run TestProductionDriver_TheProductionBatcherNamesNoMissingPort ./cmd/evolve` |
| soak-and-cost | Soak passes, cost in the package note | 6/10 | `go test -run TestGateSoak_ ./cmd/evolve` |
| coverage-no-comment | 100% on changed functions, no no-op gate | 6/10 | `go test -tags acs -run 'TestC2652_004|TestC2652_006' ./acs/cycle2652` |
