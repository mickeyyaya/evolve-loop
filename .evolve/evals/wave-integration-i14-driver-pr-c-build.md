---
score_cap:
  - criterion: "EnforceActor lands a composed batch by fast-forward and promotes each landed member"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2414_002' ./acs/cycle2414"
  - criterion: "A conflict or a red CI ejects the member with a brief and a requeue, and main stays put on red"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2414_003|TestC2414_004' ./acs/cycle2414"
  - criterion: "Write-once delivery refuses an existing record at enforce and shadow delivery keeps overwriting"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2414_009|TestC2414_011|TestC2414_012' ./acs/cycle2414"
  - criterion: "SyncToOrigin reports Current, Moved, Ahead, Diverged and Blocked and never autostashes operator work"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2414_014|TestC2414_015|TestC2414_016' ./acs/cycle2414"
---

# Eval: Build the integration EnforceActor (PR (c), non-protected scope)

> Pins the behavior of the EnforceActor card (docs/architecture/integration-driver-enforce.md): the actor, the write-once delivery policy, the plane sync outcomes and the removal of PrefixCheck. Source incident: cycle 2414, the item wave-integration-i14-driver-pr-c-build. The protected parts (the delivered inbox state, the resume identity) are console work and carry no predicate.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| landing | Flush fast-forwards and promotes | 5/10 | `go test -tags acs -run TestC2414_002 ./acs/cycle2414` |
| ejection | brief and requeue, no main move on red | 6/10 | `go test -tags acs -run 'TestC2414_003|TestC2414_004' ./acs/cycle2414` |
| write-once | enforce refuses overwrite, shadow overwrites | 6/10 | `go test -tags acs -run 'TestC2414_009|TestC2414_011|TestC2414_012' ./acs/cycle2414` |
| plane sync | five outcomes, no autostash | 6/10 | `go test -tags acs -run 'TestC2414_014|TestC2414_015|TestC2414_016' ./acs/cycle2414` |
