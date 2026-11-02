---
score_cap:
  - criterion: "A failed cycle with a floor rejection at minute 30 and 20 more minutes has fail point 30, waste 20 and ratio 0.6"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestVerdictOf_AFailedCycleWithAFloorRejectionAtMinute30AndTwentyMoreMinutesHasFailPoint30Waste20Ratio06 ./internal/cyclespend"
  - criterion: "A shipped cycle has minutes to ship and no fail point"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestVerdictOf_AShippedCycleHasMinutesToShipAndNoFailPoint ./internal/cyclespend"
  - criterion: "wave status JSON lists the per-cycle values and the wave medians"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestWaveStatus_JSONCarriesTheLLMVerdictOfEachCycleAndTheWaveMedians ./cmd/evolve"
  - criterion: "wave status text lists the per-cycle values and the wave median"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestWaveStatus_TextListsThePerCycleValuesAndTheWaveMedian ./cmd/evolve"
---

# Eval: LLM minutes to verdict, fail point, waste and fail-fast ratio per cycle

> Pins the per-cycle verdict measure of cycle 2060. A cycle gets its LLM minutes to the verdict. A failed cycle also gets the fail point (the first blocking signal), the waste after it and the fail-fast ratio. `evolve wave status` lists the values and the wave median. Source: inbox item `cycle-llm-minutes-verdict`; in wave 143 all 7 FAIL cycles were decided at the build floor, and each one spent its correction after that point.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fail-point-arithmetic | fail point 30, waste 20, ratio 0.6 | 4/10 | `go test -run TestVerdictOf_AFailedCycle... ./internal/cyclespend` |
| ship-has-no-fail-point | minutes to ship, no fail point | 6/10 | `go test -run TestVerdictOf_AShippedCycle... ./internal/cyclespend` |
| status-json | per-cycle values and medians in `wave status --json` | 5/10 | `go test -run TestWaveStatus_JSONCarriesTheLLMVerdict... ./cmd/evolve` |
| status-text | per-cycle values and median in `wave status` text | 6/10 | `go test -run TestWaveStatus_TextListsThePerCycleValues... ./cmd/evolve` |
