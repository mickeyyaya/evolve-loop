---
score_cap:
  - criterion: "TestC1920_005 rejects a prior suite whose TestC1915_002 lacks Vertex AI while a later function contains it"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1921_001 ./acs/cycle1921"
  - criterion: "TestC1920_005 still accepts a prior suite whose TestC1915_002 requires Vertex AI, including as the last function"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1921_00[24]' ./acs/cycle1921"
  - criterion: "The real cycle1920 suite stays green after the trim fix"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1921_005 ./acs/cycle1921"
---

# Eval: Confine TestC1920_005 to TestC1915_002

> Cycle 1920 audit defect `d4fb06ad157c4f825fbd17c22de815d16`: the trim in `TestC1920_005` searches for `"func "` from index 0 of a body that starts with `func TestC1915_002_`, finds index 0 and never trims. Any later function that contains `"Vertex AI"` (TestC1915_004 does) satisfies the check. The predicates build the cycle1920 test binary and run it inside a fixture repo that holds a synthetic prior suite.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| leaked-scope | later-function match is rejected | 7/10 | `go test -tags acs -run TestC1921_001 ./acs/cycle1921` |
| preservation | valid suites still pass | 6/10 | `go test -tags acs -run 'TestC1921_00[24]' ./acs/cycle1921` |
| real-suite | cycle1920 stays green | 6/10 | `go test -tags acs -run TestC1921_005 ./acs/cycle1921` |
