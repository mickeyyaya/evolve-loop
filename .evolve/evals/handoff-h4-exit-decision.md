---
score_cap:
  - criterion: "A round that meets no new ask returns cap before the round limit"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1969_001' ./acs/cycle1969"
  - criterion: "Retry carries exactly the ids of the unmet required asks"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1969_002' ./acs/cycle1969"
  - criterion: "Pass needs each required ask met with resolved evidence"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1969_003' ./acs/cycle1969"
  - criterion: "An optional unmet ask does not block pass"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1969_004' ./acs/cycle1969"
  - criterion: "A spent redispatch cap ends in cap and a full pass still wins"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1969_005|TestC1969_006' ./acs/cycle1969"
  - criterion: "The handoff package note documents the exits"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1969_007' ./acs/cycle1969"
  - criterion: "The new package graduates into the apicover gate and carries no comment"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1969_008|TestC1969_009' ./acs/cycle1969"
  - criterion: "The exit decision table tests pass in the package"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./internal/handoff/exit"
---

# Eval: Phase handoff H4, the exit decision

> Pins `internal/handoff/exit.Decide`, the pure function that returns pass, retry (with the ids of
> the unmet required asks) or cap from the request, the check results, the round history and the
> policy (ADR-0141, plan H4). The negative cases carry the weight: an always-pass decision fails the
> unresolved-evidence and missing-result cases, an always-retry decision fails the no-gain and
> spent-cap cases, and a decision that returns every ask id fails the exact-id case. Source: inbox
> item handoff-h4-exit-decision, cycle 1969.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-gain-stop | no new ask met caps before the limit | 3/10 | `go test -tags acs -run TestC1969_001 ./acs/cycle1969` |
| exact-retry-ids | retry names the unmet required asks only | 4/10 | `go test -tags acs -run TestC1969_002 ./acs/cycle1969` |
| resolved-pass | pass needs met plus resolved evidence | 3/10 | `go test -tags acs -run TestC1969_003 ./acs/cycle1969` |
| optional-free | optional unmet ask never blocks | 5/10 | `go test -tags acs -run TestC1969_004 ./acs/cycle1969` |
| spent-cap | spent redispatch cap ends in cap | 5/10 | `go test -tags acs -run 'TestC1969_005|TestC1969_006' ./acs/cycle1969` |
| package-note | note documents the exits | 5/10 | `go test -tags acs -run TestC1969_007 ./acs/cycle1969` |
| apicover-graduation | enrolled, named, comment-free | 6/10 | `go test -tags acs -run 'TestC1969_008|TestC1969_009' ./acs/cycle1969` |
| table-tests | the package table tests | 4/10 | `go test ./internal/handoff/exit` |
