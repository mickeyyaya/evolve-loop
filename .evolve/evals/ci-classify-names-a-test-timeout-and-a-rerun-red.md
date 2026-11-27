---
score_cap:
  - criterion: "A go test timeout panic classifies as the timeout class, not unknown, with the running test names in its key"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2660_001_' ./acs/cycle2660"
  - criterion: "A log with no timeout panic stays unknown"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2660_002_' ./acs/cycle2660"
  - criterion: "A main run whose attempt 1 failed key K and whose attempt 2 passed makes recurred_on_main yes for K"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2660_003_|TestC2660_006_' ./acs/cycle2660"
  - criterion: "An attempt-1 failure of another key, or an empty main run list, does not make recurred_on_main yes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2660_004_|TestC2660_005_' ./acs/cycle2660"
  - criterion: "Each changed function of internal/ciwatch is covered 100%"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2660_007_' ./acs/cycle2660"
---

# Eval: evolve ci classify names a go test timeout panic and counts a red attempt that went green on re-run

> Pins two fixes of `ciwatch.ClassifyTarget`. Source incident: PR #1841 classify output on 2026-11-26,
> where a `panic: test timed out after 30m0s` log read as class unknown and a main attempt 1 that went
> green on re-run read as recurred_on_main no.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| timeout-class | timeout panic gets label timeout | 6/10 | `go test -tags acs -run TestC2660_001_ ./acs/cycle2660` |
| timeout-negative | no panic stays unknown | 7/10 | `go test -tags acs -run TestC2660_002_ ./acs/cycle2660` |
| attempt-recurrence | attempt 1 red, attempt 2 green recurs | 6/10 | `go test -tags acs -run 'TestC2660_003_\|TestC2660_006_' ./acs/cycle2660` |
| recurrence-negative | other key or empty list does not recur | 7/10 | `go test -tags acs -run 'TestC2660_004_\|TestC2660_005_' ./acs/cycle2660` |
| coverage | changed functions 100% | 5/10 | `go test -tags acs -run TestC2660_007_ ./acs/cycle2660` |
