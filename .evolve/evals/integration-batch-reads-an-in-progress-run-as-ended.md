---
score_cap:
  - criterion: "A batch whose CI run is in progress and then completes with success waits through Settle and lands the PR"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2239_001' ./acs/cycle2239"
  - criterion: "A run that stays in progress past the deadline ends with the named integration.ErrRunTimeout, with and without a deadline on the context"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2239_002' ./acs/cycle2239"
  - criterion: "A completed run with an empty conclusion is still refused with the neither-green-nor-red error and is never reported as a timeout"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2239_003' ./acs/cycle2239"
  - criterion: "A run that is in progress and then fails is read as red after the wait"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2239_004' ./acs/cycle2239"
---

# Eval: Integration batch reads an in-progress run as ended

> Pins the verdict read of `integration.GhCI.Watch`. Source incident: `evolve integration batch 1377` exited
> with `run 42557152023 ended with the conclusion "", which is neither green nor red` and landed nothing,
> because `gh run view --json conclusion` printed an empty conclusion for a run that was still in progress.
> The predicates drive `integration.Merger.Land`, the production caller of `CI.Watch`, with a scripted `gh`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wait-then-land | in progress, then success lands the PR | 3/10 | `go test -tags acs -run TestC2239_001 ./acs/cycle2239` |
| named-timeout | in progress past the deadline ends with `ErrRunTimeout` | 4/10 | `go test -tags acs -run TestC2239_002 ./acs/cycle2239` |
| empty-conclusion | completed with an empty conclusion is refused | 4/10 | `go test -tags acs -run TestC2239_003 ./acs/cycle2239` |
| wait-then-red | in progress, then failure is red | 5/10 | `go test -tags acs -run TestC2239_004 ./acs/cycle2239` |
