---
score_cap:
  - criterion: "A field written by the code under test, reached through a holder literal passed to a call, is not own setup"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2518_001' ./acs/cycle2518"
  - criterion: "A literal aliased through a local with no call between the alias and the assertion is still own setup"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2518_002' ./acs/cycle2518"
  - criterion: "The two cycle-2506 core tests assert the field directly and carry no asserts-own-setup finding"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2518_003' ./acs/cycle2518"
---

# Eval: asserts-own-setup alias blind spot

> Pins the two-sided fix of the asserts-own-setup rule in go/internal/testaudit/rules.go. The rule flagged
> a field that the code under test writes (false positive) and a local alias hid a real tautology (false
> negative). Source incident: cycle 2506 audit M1, where a lane moved two assertions into locals to
> dodge the false finding.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| holder-escape | holder literal passed to a call clears the contained literal | 6/10 | `go test -tags acs -run TestC2518_001 ./acs/cycle2518` |
| alias-follow | alias of a literal stays own setup without a call between | 6/10 | `go test -tags acs -run TestC2518_002 ./acs/cycle2518` |
| restored-assertions | core tests assert the field directly | 5/10 | `go test -tags acs -run TestC2518_003 ./acs/cycle2518` |
