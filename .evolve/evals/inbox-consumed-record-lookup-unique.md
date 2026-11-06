---
score_cap:
  - criterion: "Two consumed records with one id give an error that names both paths"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2187_00[12]' ./acs/cycle2187"
  - criterion: "One record gives that record and none gives the unchanged not-found error"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2187_003' ./acs/cycle2187"
  - criterion: "Every function in consumed_lookup.go has 100 percent statement coverage and the file has no comment"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2187_004' ./acs/cycle2187"
---

# Eval: The consumed-record lookup refuses more than one record for one id

> Pins `inboxmover.FindConsumedRecord(inboxDir, id)`. The lookup returns the path of the one consumed record for an id, refuses two or more with an error that names every path, and returns `ErrNotFound` unchanged when none match. Source incident: an ACS predicate read the first of two same-id consumed records and could pass on the wrong one (inbox item inbox-consumed-record-lookup-unique, cycle 2187).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| duplicate-refusal | Two same-id records give an error naming both paths | 4/10 | `go test -tags acs -run 'TestC2187_00[12]' ./acs/cycle2187` |
| single-and-none | One record gives its path, none gives ErrNotFound | 5/10 | `go test -tags acs -run TestC2187_003 ./acs/cycle2187` |
| coverage-no-comment | 100% statement coverage, zero comments | 6/10 | `go test -tags acs -run TestC2187_004 ./acs/cycle2187` |
