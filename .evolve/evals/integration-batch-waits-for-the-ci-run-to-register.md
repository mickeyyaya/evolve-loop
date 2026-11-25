---
score_cap:
  - criterion: "a batch whose CI run registers 2 minutes after the push waits and lands"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2635_001 ./acs/cycle2635"
  - criterion: "a batch whose run never registers stops with a message that names the bound"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2635_002|TestC2635_004' ./acs/cycle2635"
  - criterion: "both cases run on an injected clock and a fake run lister"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2635_003 ./acs/cycle2635"
---

# Eval: the integration batch waits for the CI run to register

> `GhCI.runIDs` gave up after 5 polls, about 20 seconds with the 5 second settle pause of production. A CI run that registers 2 minutes after the push made `evolve integration batch` stop with `no CI run`. The wait now has a bound in time, read from an injected clock, and the stop message names the bound. Source: inbox item integration-batch-waits-for-the-ci-run-to-register, cycle 2635.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| late-registration | the batch waits 2 minutes and lands | 6/10 | `go test -tags acs -run TestC2635_001 ./acs/cycle2635` |
| bound-named | the stop message names the bound | 7/10 | `go test -tags acs -run 'TestC2635_002\|TestC2635_004' ./acs/cycle2635` |
| injected-clock | both cases use a fake clock and run lister | 5/10 | `go test -tags acs -run TestC2635_003 ./acs/cycle2635` |
