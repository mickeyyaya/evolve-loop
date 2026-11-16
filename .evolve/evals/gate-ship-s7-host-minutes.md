---
score_cap:
  - criterion: "Verdict counts host minutes beside LLM minutes"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestVerdict_CountsHostMinutes$' ./internal/cyclespend"
  - criterion: "the sampler records host load for each gate span"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestSampler_RecordsLoadForEachGateSpan$' ./internal/cyclespend"
  - criterion: "wave readings carry host minutes from the workspace gate spans"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestReadCycles_TheGateSpansOfACycleGiveItsHostMinutesBesideItsLLMMinutes$' ./internal/wave"
  - criterion: "wave status prints and encodes host minutes beside LLM minutes"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestWaveStatus_(TextShowsTheHostMinutesBesideTheLLMMinutes|JSONCarriesTheHostMinutesOfEachCycleAndTheWaveMedian)$' ./cmd/evolve"
---

# Eval: Count host minutes and host load in the readings

> Pins decision D7 and component S7 of the gate-ship retry-waste plan (cycle 2428). The verdict, the sampler and the wave readings count host minutes beside LLM minutes, so the host waste of a lane stays visible.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| verdict | Verdict counts host minutes | 7/10 | `go test -run TestVerdict_CountsHostMinutes ./internal/cyclespend` |
| sampler | load recorded for each gate span | 7/10 | `go test -run TestSampler_RecordsLoadForEachGateSpan ./internal/cyclespend` |
| reading | wave readings carry host minutes | 6/10 | `go test -run TestReadCycles_TheGateSpansOfACycleGiveItsHostMinutesBesideItsLLMMinutes ./internal/wave` |
| printer | wave status shows host minutes | 6/10 | `go test -run TestWaveStatus_TextShowsTheHostMinutesBesideTheLLMMinutes ./cmd/evolve` |
