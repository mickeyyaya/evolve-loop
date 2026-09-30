---
score_cap:
  - criterion: "Starved compares realized lanes with the sized width"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestWaveObservation_StarvedComparesRealizedWithSizedWidth|TestStarvationTracker_FiresOnShortfallAgainstSizedWidth' ./internal/fleet"
  - criterion: "the production observer files a todo for a quota-shrunk wave that misses its sized width"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestObserveWorkSupply_' ./cmd/evolve"
---

# Eval: starvation compares sized width

> A quota shrink explains only the lanes it removed; waves 19 and 20 (1 and 0 of 2 sized lanes) were counted not-starved. go/acs/cycle544's old rule is superseded with a documented reason (design doc §7.7 W4).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| sized-width | shortfall vs sized width is starved | 7/10 | `go test ./internal/fleet` |
| caller | observer reaches it in cmd/evolve | 7/10 | `go test -run TestObserveWorkSupply_ ./cmd/evolve` |
