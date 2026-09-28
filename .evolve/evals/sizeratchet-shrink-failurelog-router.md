---
score_cap:
  - criterion: "Every listed failurelog/router function is <= 50 lines and its offenders.json entry is gone"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -run TestC1731_00[12] -count=1 -v ./acs/cycle1731/..."
  - criterion: "Existing failurelog and router test suites still pass unmodified (behavior unchanged)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/failurelog/... ./internal/router/..."
  - criterion: "The build explanation document's function-count claims match the diff (all three prune functions; named siblings counted correctly)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -run TestC1731_004 -count=1 -v ./acs/cycle1731/..."
---

# Eval: Shrink the 8 sizeratchet offenders in failurelog and router

> Pins the size-ratchet-shrink acceptance for
> `sizeratchet-shrink-failurelog-router` (cycle 1731, inbox
> `2026-09-28T05-12-00Z-sizeratchet-shrink-failurelog-router.json`). The ratchet
> (`go/internal/sizeratchet`, cycle 1726) lists eight offenders across
> `go/internal/failurelog` and `go/internal/router` — `PruneByClassification`,
> `PruneExpired`, `PruneExpiredCarryoverTodos`, `Record`, `ClampPlanModelRouting`,
> `ClampPlanToFloorWith`, `Digest`, `shouldRun` — each over the 50-line limit.
> The task is a behavior-preserving refactor: extract named steps until every
> one fits, then drop its `offenders.json` entry. All eight already have
> existing test coverage in their packages, so no new characterization tests
> were required; the regression suites for both packages are the behavior-pin.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| size-and-offender-drop | All 8 functions <= 50 lines and removed from offenders.json | 8/10 | `go test -tags acs -run TestC1731_00[12] ./acs/cycle1731/...` |
| behavior-unchanged | failurelog/router suites pass unmodified | 6/10 | `go test ./internal/failurelog/... ./internal/router/...` |
| explanation-counts | Explanation doc count words match the diff (cycle 1731 audit H1: "four sibling functions" naming two; "two of the three" prune functions sharing a skeleton all three shared) | 5/10 | `go test -tags acs -run TestC1731_004 ./acs/cycle1731/...` |
