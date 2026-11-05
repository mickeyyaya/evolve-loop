---
score_cap:
  - criterion: "the ship runs 0 packages at enforce for an unchanged key and every package at shadow, counted in one test"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestVerdictStore_TheShipRunsNoPackageAtEnforceOnAnUnchangedKeyAndEveryPackageAtShadow$' ./internal/phases/ship"
  - criterion: "a stored FAIL is never reused by the verdict store"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestStore_AStoredFailIsNeverReused$' ./internal/testverdict"
  - criterion: "at enforce a package the floor found red runs again and fails the ship"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestVerdictStore_AtEnforceAStoredFloorRedRunsAgainAndFailsTheShip$' ./internal/phases/ship"
  - criterion: "headroom moves a family at the limit to the tail of the chain"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestHeadroom_AFamilyAtTheLimitMovesToTheTailAndTheRestKeepTheirPolicyOrder$' ./internal/cliroute"
  - criterion: "a shorter lane time to a PASS goes first, also against an earlier policy place"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestHeadroom_AShorterLaneTimeToAPassBeatsAnEarlierPolicyPlace$' ./internal/cliroute"
---

# Eval: Latency gates T3, the W5 count test and the W4 fixture tests

> Pins the four proofs of component T3 of docs/plans/latency-gates-2026-10.md (ADR-0144): the W5 package count of the ship at enforce and at shadow, the stored FAIL that is never reused, and the W4 headroom order. Source: inbox item latency-gates-t3-wave-fix-proofs, cycle 2167.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| paired-count | 0 packages run at enforce, all at shadow | 6/10 | `go test -run TestVerdictStore_TheShipRunsNoPackageAtEnforceOnAnUnchangedKeyAndEveryPackageAtShadow` |
| fail-never-reused | a stored FAIL is a miss | 7/10 | `go test -run TestStore_AStoredFailIsNeverReused` |
| red-runs-again | a floor red runs again at enforce | 6/10 | `go test -run TestVerdictStore_AtEnforceAStoredFloorRedRunsAgainAndFailsTheShip` |
| tail-position | a family at the limit goes to the tail | 6/10 | `go test -run TestHeadroom_AFamilyAtTheLimitMovesToTheTailAndTheRestKeepTheirPolicyOrder` |
| shorter-first | the shorter lane time to a PASS goes first | 6/10 | `go test -run TestHeadroom_AShorterLaneTimeToAPassBeatsAnEarlierPolicyPlace` |
