---
score_cap:
  - criterion: "evolve cycle models --cycle N prints the tier, selector and serving model of each phase from llm-calls.ndjson"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2023_001' ./acs/cycle2023 && go test -count=1 -run 'TestRunCycleModels_PrintsTierSelectorAndServedModelPerPhase|TestRunCycle_ModelsIsADispatchedSubcommand' ./cmd/evolve"
  - criterion: "a launch through a fake driver that reports a model records it on the llm-calls line"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2023_002' ./acs/cycle2023"
  - criterion: "records without a serving model print unreported and records with one do not"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2023_003' ./acs/cycle2023 && go test -count=1 -run 'TestRunCycleModels_ARecordWithoutAServedModelPrintsUnreported|TestRunCycleModels_EachAttemptOfAPhaseGetsItsOwnRow' ./cmd/evolve"
  - criterion: "a cycle with no call records exits 1 with a stderr reason"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2023_004' ./acs/cycle2023 && go test -count=1 -run 'TestRunCycleModels_NoRecordsExitsOne' ./cmd/evolve"
---

# Eval: evolve cycle models (lane part)

> Pins the read-only operator verb `evolve cycle models --cycle N`, which replaces reading CLI transcripts by hand
> to learn which model served each phase. The recording half (`llmcalls.Record.ServedModel`) already exists in the
> bridge; this eval pins the reader, its `unreported` fallback and its empty-cycle exit. Source: inbox record
> cli-cycle-models-lane (cycle 2023).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| reader-exists | Per-phase tier, selector and served model are printed | 4/10 | `go test -tags acs -run TestC2023_001 ./acs/cycle2023` |
| recording-held | Fake-driver launches record the served model | 5/10 | `go test -tags acs -run TestC2023_002 ./acs/cycle2023` |
| unreported-fallback | Missing served model prints unreported, a reported one does not | 6/10 | `go test -tags acs -run TestC2023_003 ./acs/cycle2023` |
| empty-cycle | No records exits 1 with a reason | 6/10 | `go test -tags acs -run TestC2023_004 ./acs/cycle2023` |
