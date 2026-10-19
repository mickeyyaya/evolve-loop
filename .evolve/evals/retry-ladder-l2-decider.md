---
score_cap:
  - criterion: "The rung table: R0 to R1, R0 to R2 on a repeated fingerprint or no effort step, R2 to stop-as-spec on an earlier fingerprint, no headroom at R0 and at R1"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestNext_CapabilityLadderTable|TestNext_EffortStepRules|TestNext_TierStepRules|TestNext_R2EffortIsTheLargerOfTheRaisedEffortAndTheTierDefault' ./internal/retryladder/"
  - criterion: "No input gives more than two raised tries, a tier above max_tier or an effort above max_effort"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestNext_PropertyNoInputBeatsTheBounds ./internal/retryladder/"
  - criterion: "halt and spec never climb; unknown never passes R1; infra and quota keep the rung"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestNext_HaltAndSpecNeverClimb|TestNext_UnknownNeverPassesR1|TestNext_InfraAndQuotaKeepTheRung' ./internal/retryladder/"
  - criterion: "With no headroom the decider records LADDER_NO_HEADROOM, goes from R0 to R1, and stops at R1"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run TestC1977_004 ./acs/cycle1977/"
  - criterion: "Every decision records the shadow critic rung and the critic never changes the decision"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestNext_RecordsTheShadowCriticRungWithoutFollowingIt ./internal/retryladder/"
  - criterion: "Unset bounds are max_tier deep and max_effort high"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestBounds_DefaultsAreDeepAndHigh ./internal/retryladder/"
  - criterion: "The retryladder package reuses internal/convergence and still does not import internal/core"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run TestC1977_008 ./acs/cycle1977/"
---

# Eval: Retry ladder L2, the rung decider

> Pins `retryladder.Next` of ADR-0142 component L2 (spec `docs/architecture/retry-ladder.md` §2 and §4, plan D9 and D12). The decider is a pure function of the failure class, the fingerprint history, the base seat, the healthy-headroom table and the bounds. Source: the aborted builds of cycles 1946 to 1966 (research F2.11), where a stronger try could fix only the capability failures.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| rung-table | the flow of spec §2 as a table | 7/10 | `go test -run TestNext_CapabilityLadderTable` |
| bounds-property | at most two raises, tier and effort inside the bounds | 8/10 | `go test -run TestNext_PropertyNoInputBeatsTheBounds` |
| no-climb-classes | halt, spec and unknown stop the climb | 8/10 | `go test -run TestNext_HaltAndSpecNeverClimb` |
| no-headroom | the signal, R0 to R1, stop at R1 | 7/10 | `go test -tags acs -run TestC1977_004` |
| shadow-critic | the critic rung is recorded and never followed | 6/10 | `go test -run TestNext_RecordsTheShadowCriticRungWithoutFollowingIt` |
| default-bounds | deep and high when unset | 6/10 | `go test -run TestBounds_DefaultsAreDeepAndHigh` |
| one-tier-table | convergence reused, core not imported | 8/10 | `go test -tags acs -run TestC1977_008` |
