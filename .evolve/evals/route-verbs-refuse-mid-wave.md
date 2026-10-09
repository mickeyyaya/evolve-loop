---
score_cap:
  - criterion: "route-console and route-lane refuse with exit 1 while a loop run lease is live and leave the item byte-identical"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestCmd_InboxRouteVerbs_RefuseWhileALoopLaneIsLiveAndLeaveTheItemByteIdentical ./cmd/evolve"
  - criterion: "a stale lease never refuses a route verb"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestCmd_InboxRouteVerbs_AStaleLeaseDoesNotRefuse ./cmd/evolve"
  - criterion: "usage errors keep exit 10 ahead of the mid-wave refusal"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestCmd_InboxRouteVerbs_ARefusalNeverMasksUsageErrors ./cmd/evolve"
---

# Eval: route-console and route-lane refuse mid-wave

> Pins the mid-wave refusal for the two routing verbs that rewrite a tracked inbox file, as edit, withdraw and verify already do via refusedMidWave. Source incident: cycle 1701, where a mid-wave route bounced audit tree-state verification.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| refuse-live | rc 1, item byte-identical | 4/10 | `go test -run TestCmd_InboxRouteVerbs_RefuseWhileALoopLaneIsLiveAndLeaveTheItemByteIdentical` |
| stale-ok | stale lease routes | 6/10 | `go test -run TestCmd_InboxRouteVerbs_AStaleLeaseDoesNotRefuse` |
| usage-first | rc 10 on bad args | 6/10 | `go test -run TestCmd_InboxRouteVerbs_ARefusalNeverMasksUsageErrors` |
