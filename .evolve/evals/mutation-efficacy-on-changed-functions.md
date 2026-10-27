---
score_cap:
  - criterion: "A changed function that sends a signal to a process, a file that holds such a call, and the kill-guard packages never reach the runner"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2067_00[134]' ./acs/cycle2067"
  - criterion: "A function with no kill call reaches the runner with its line range; an unparsable body is refused"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2067_00[25]' ./acs/cycle2067"
  - criterion: "8 killed, 2 lived and 1 not covered give TQ1 0.80 with Wilson bounds over 10 mutants and not_covered=1; timed out counts as killed"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2067_00[78]' ./acs/cycle2067"
  - criterion: "TQ3 counts a test with zero unique kills and stays finite with no measured time"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2067_0(09|10)' ./acs/cycle2067"
  - criterion: "The index starts at shadow and emits no breach event"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2067_011' ./acs/cycle2067"
  - criterion: "The runner gets a scratch path outside the plane and main trees; a scratch path inside either is refused; no real gremlins starts"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2067_01[2345]' ./acs/cycle2067"
  - criterion: "Every function of internal/qualitypanel is at 100% coverage and apicover passes"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2067_01[67]' ./acs/cycle2067"
---

# Eval: Mutation efficacy on changed functions (TQ1) and test value (TQ3)

> Pins the Q5 reader of ADR-0153: `qualitypanel.MeasureMutation` scores the mutants of the changed
> functions and never mutates code that signals a process. A kill-guard mutant once killed every user
> process. The runner is a seam, so no test starts the real gremlins. Source: cycle 2067, inbox item
> mutation-efficacy-on-changed-functions.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| kill-exclusion | body, file and package exclusion | 3/10 | `TestC2067_001`, `_003`, `_004` |
| scratch-isolation | scratch outside plane and main, no real gremlins | 3/10 | `TestC2067_012` to `_015` |
| tq1-formula | 8/2/1 gives 0.80 and Wilson bounds | 5/10 | `TestC2067_007`, `_008` |
| shadow | no breach at shadow | 5/10 | `TestC2067_011` |
| tq3 | zero unique kills counted | 6/10 | `TestC2067_009`, `_010` |
| coverage | 100% coverage and apicover | 6/10 | `TestC2067_016`, `_017` |
