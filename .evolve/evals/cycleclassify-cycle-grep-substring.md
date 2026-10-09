---
score_cap:
  - criterion: "gitLogMatchesCycle anchors the cycle number: cycle 4 does not match a cycle 42 or cycle 400 commit"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1850_00[12]' ./acs/cycle1850"
  - criterion: "Anchoring does not over-narrow: cycle 42 and cycle 4 still match their own commits"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1850_00[34]' ./acs/cycle1850"
  - criterion: "The hang test uses setHangClassifierForTest(t, true) and no longer sets the removed env flag; duplicate helpers are gone"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1850_00[56]' ./acs/cycle1850"
---

# Eval: cycleclassify grep anchored to the whole cycle number

> Pins the fix for gitLogMatchesCycle running `git log --grep=cycle N` as a substring match, which let the opt-in hang reclassifier report cycle 4 as shipped from a commit about cycle 42. Also pins the hang test switching to the live classifier toggle. Source: inbox item cycleclassify-cycle-grep-substring (cycle 1850).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| anchored-negative | cycle 4 vs cycle 42 / 400 commits is false | 5/10 | `go test -tags acs -run 'TestC1850_00[12]' ./acs/cycle1850` |
| anchored-positive | cycle 42 and cycle 4 still match | 6/10 | `go test -tags acs -run 'TestC1850_00[34]' ./acs/cycle1850` |
| test-hygiene | live toggle in hang test, helpers removed | 7/10 | `go test -tags acs -run 'TestC1850_00[56]' ./acs/cycle1850` |
