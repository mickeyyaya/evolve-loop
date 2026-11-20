---
score_cap:
  - criterion: "A green run of the main SHA wins over a newer cancelled run"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestCheckMain_AGreenRunWinsOverANewerCancelledRun' ./internal/integration/"
  - criterion: "A red run of the main SHA keeps main red"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestCheckMain_ARedRunOfTheSHAStaysRed' ./internal/integration/"
  - criterion: "Only cancelled runs and no run at all keep main not green"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCheckMain_OnlyCancelledRunsLeaveMainNotGreen|TestCheckMain_NoRunOfTheSHAIsStillNoRun' ./internal/integration/"
---

# Eval: main CI accepts any green run for the SHA

> A cancelled newer CI run of the same main SHA read as red, so a batch waited or refused.
> These tests drive the real seam, `Train.checkMain`, through both production watchers (`GhCI` and `PeekCI`).
> Source incident: cycle 2571, inbox item integration-accepts-any-green-run-for-the-sha.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| green-wins | A green run wins over a newer cancelled run | 4/10 | `go test -run TestCheckMain_AGreenRunWinsOverANewerCancelledRun` |
| red-stays-red | Any red run keeps main red | 4/10 | `go test -run TestCheckMain_ARedRunOfTheSHAStaysRed` |
| not-green-kept | Cancelled-only and no-run stay not green | 6/10 | `go test -run 'TestCheckMain_OnlyCancelled|TestCheckMain_NoRun'` |
