---
score_cap:
  - criterion: "A test name that no _test.go file declares does not resolve"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1968_001' ./acs/cycle1968"
  - criterion: "A path:line past the end of the file does not resolve"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1968_002|TestC1968_003' ./acs/cycle1968"
  - criterion: "An ACS predicate resolves only when the cycle package declares it"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1968_004' ./acs/cycle1968"
  - criterion: "A command resolves only when its result is recorded; an unknown kind never resolves"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1968_005|TestC1968_006' ./acs/cycle1968"
  - criterion: "The handoff package note lists the evidence kinds"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1968_007' ./acs/cycle1968"
  - criterion: "The new package graduates into the apicover gate and carries no comment"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1968_008|TestC1968_009' ./acs/cycle1968"
  - criterion: "The resolver table tests pass in the package"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./internal/handoff/evidence"
---

# Eval: Phase handoff H3, the evidence resolvers

> Pins `internal/handoff/evidence.Resolve`, which decides in code whether an evidence pointer of a
> handoff ledger entry resolves (ADR-0141, plan H3). A pointer that does not resolve makes its entry
> unmet. The negative cases carry the weight: an always-nil resolver fails the undeclared test, the
> past-the-end line, the foreign cycle predicate, the absent command and the unknown kind. Source:
> inbox item handoff-h3-evidence-resolvers, cycle 1968.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| undeclared-test | no _test.go declares the cited name | 4/10 | `go test -tags acs -run TestC1968_001 ./acs/cycle1968` |
| line-bound | line past end of file | 4/10 | `go test -tags acs -run 'TestC1968_002|TestC1968_003' ./acs/cycle1968` |
| cycle-scope | predicate declared by the cycle package only | 5/10 | `go test -tags acs -run TestC1968_004 ./acs/cycle1968` |
| recorded-command | command result present or absent | 5/10 | `go test -tags acs -run 'TestC1968_005|TestC1968_006' ./acs/cycle1968` |
| package-note | note lists the kinds | 5/10 | `go test -tags acs -run TestC1968_007 ./acs/cycle1968` |
| apicover-graduation | enrolled, named, comment-free | 6/10 | `go test -tags acs -run 'TestC1968_008|TestC1968_009' ./acs/cycle1968` |
| table-tests | the package table tests | 4/10 | `go test ./internal/handoff/evidence` |
