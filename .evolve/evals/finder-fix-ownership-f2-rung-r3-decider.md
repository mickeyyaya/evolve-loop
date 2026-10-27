---
score_cap:
  - criterion: "The replay of cycles 1972 and 1983 climbs R1, R2, then R3"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestNext_ReplayCycles1972And1983 ./internal/retryladder/"
  - criterion: "A repeated R1 fingerprint skips R2 and goes to R3"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestNext_R3SkipsR2WhenTheR1FingerprintRepeats ./internal/retryladder/"
  - criterion: "No effort step and no tier headroom skips R2 and goes to R3"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestNext_R3SkipsR2WhenNoEffortStepAndNoTierHeadroom ./internal/retryladder/"
  - criterion: "The R3 builder runs on another family only when that family has headroom at the base tier"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestNext_R3BuilderFamilyChoice ./internal/retryladder/"
  - criterion: "The ladder stops after R3, and R3 on a repeated fingerprint stops as spec"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestNext_R3IsTheLastRung|TestNext_R3KeepsTheClassRulesOfEveryOtherRung' ./internal/retryladder/"
  - criterion: "The contract correction loop dispatches a fresh R3 try through the production caller"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestContractCorrection_EnforceEscalatesToAFreshRaisedTryAfterOneCorrection ./internal/core/"
---

# Eval: Finder fix ownership F2, rung R3 in the ladder decider

> Pins rung R3 of `retryladder.Next` (ADR-0146 items 3, 8 and 9; plan `docs/plans/finder-fix-ownership-2026-10.md` component F2). Source incident: cycles 1972 and 1983 each failed the audit after two repair rounds (research F2.2).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| replay | cycles 1972 and 1983 give R1, R2, R3 | 6/10 | `go test -run TestNext_ReplayCycles1972And1983` |
| skip-by-fingerprint | a repeated R1 fingerprint goes to R3 | 7/10 | `go test -run TestNext_R3SkipsR2WhenTheR1FingerprintRepeats` |
| skip-by-headroom | no effort step and no tier headroom goes to R3 | 7/10 | `go test -run TestNext_R3SkipsR2WhenNoEffortStepAndNoTierHeadroom` |
| family-choice | another family with headroom builds R3 | 7/10 | `go test -run TestNext_R3BuilderFamilyChoice` |
| last-rung | the ladder stops after R3 | 7/10 | `go test -run TestNext_R3IsTheLastRung` |
| caller | the correction loop dispatches R3 | 8/10 | `go test -run TestContractCorrection_EnforceEscalatesToAFreshRaisedTryAfterOneCorrection ./internal/core/` |
