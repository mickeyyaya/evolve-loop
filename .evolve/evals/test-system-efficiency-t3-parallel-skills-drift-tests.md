---
score_cap:
  - criterion: "Every skills-drift test calls t.Parallel as its first statement"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2150_001 ./acs/cycle2150"
  - criterion: "The serial list names no skills-drift test"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2150_002 ./acs/cycle2150"
  - criterion: "The skills-drift tests run in parallel and pass under -race -count=3 -shuffle=on"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2150_003 ./acs/cycle2150"
  - criterion: "The changed skillsdrift.go functions stay at 100% coverage"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2150_004 ./acs/cycle2150"
---

# Eval: Parallel skills-drift tests with injected seams

> Pins test-system plan T3. The skills-drift tests swapped the package vars runCmd and worktreeSkillsCheckLimit, so eleven of them sat in serialTests. The predicates require t.Parallel in each test, an empty serial list for them, a real pause event for each under race, count and shuffle, and full coverage of the changed functions. Source: cycle 2150.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| parallel-first | each test starts with t.Parallel | 6/10 | `go test -tags acs -run TestC2150_001 ./acs/cycle2150` |
| serial-list | no skills-drift entry in serialTests | 6/10 | `go test -tags acs -run TestC2150_002 ./acs/cycle2150` |
| race-stream | pause and pass events for each test, three times | 7/10 | `go test -tags acs -run TestC2150_003 ./acs/cycle2150` |
| coverage | changed functions at 100% | 5/10 | `go test -tags acs -run TestC2150_004 ./acs/cycle2150` |
