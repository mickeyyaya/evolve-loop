---
score_cap:
  - criterion: "bridgechain.WithCandidateFilter skips a refused seat, launches the next admitted seat and errors with zero launches when every seat is refused"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestWalking_CandidateFilterSkipsARefusedSeatAndLaunchesTheNext|TestWalking_CandidateFilterThatRefusesEverySeatLaunchesNothingAndErrors)$' ./internal/bridgechain"
  - criterion: "a filtered walk keeps the exit-85 bench and cannot be bypassed by a ChainAttempt request"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestWalking_FilteredWalkStillBenchesEachQuotaWall|TestWalking_ChainAttemptCannotBypassTheCandidateFilter)$' ./internal/bridgechain"
  - criterion: "LaunchSeat returns the served CLI and tier, not the plan head"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestWalking_LaunchSeatReportsTheServedFallbackSeatNotThePlanHead$' ./internal/bridgechain"
  - criterion: "handoff.Independents admits another family or a stronger tier and fails closed on an empty author tier or an unattributed author"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestIndependents_AdmitsOnlyAnotherFamilyOrAStrongerTier$' ./internal/handoff"
  - criterion: "policy.TierNames equals modelcatalog.CanonicalTiers"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestTierNames_EqualTheCanonicalCatalogTiers$' ./internal/policy"
---

# Eval: Handoff judge seat walker with an independence filter

> Pins the seam the handoff judge must use: one seat walker (`bridgechain.Walking`) with a candidate filter, the served seat returned to the caller, and `handoff.Independents` as the judge filter. Source incident: branch dev/cl-c1 was blocked in architecture review round 3 because the judge walked its own seats with `ChainAttempt` and skipped the exit-85 bench and the fresh session. Cycle 2515 ships the seam and its tests; the judge itself is not in this tree.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| filter-admission | refused seat never launches; all refused errors | 4/10 | `go test -run TestWalking_CandidateFilter... ./internal/bridgechain` |
| no-second-walker | bench kept, ChainAttempt cannot bypass | 5/10 | `go test -run TestWalking_FilteredWalkStillBenchesEachQuotaWall ./internal/bridgechain` |
| served-seat | seat is the routed driver | 6/10 | `go test -run TestWalking_LaunchSeatReports... ./internal/bridgechain` |
| independence | author family and tier fail closed | 6/10 | `go test -run TestIndependents_... ./internal/handoff` |
| vocabulary | one tier vocabulary | 3/10 | `go test -run TestTierNames_Equal... ./internal/policy` |
