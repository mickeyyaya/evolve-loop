---
score_cap:
  - criterion: "At enforce, a tdd deliverable whose cycle predicate has an inverted-idiom finding is rejected; the reason names the predicate, the kind and the acsassert.FileNotContains remedy, does not call itself advisory, and the reviewer logs blocking=true"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1802_001_' ./acs/cycle1802"
  - criterion: "At enforce, a tdd deliverable whose cycle predicate has a go-run-exit-code finding is rejected; the reason names the predicate, the kind, the unreachable exit code and the go build remedy"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1802_002_' ./acs/cycle1802"
  - criterion: "An enforce rejection names the blocking proof finding even when advisory absence-message findings fill the five-finding cap ahead of it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1802_003_' ./acs/cycle1802"
  - criterion: "At enforce, an absence-message finding alone is approved and still logged with blocking=false"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1802_004_' ./acs/cycle1802"
  - criterion: "At enforce, flaky-shape findings of all four classes (concurrency, async-wait, environment, resource-leak), beside an absence-message finding, are approved and logged with blocking=false"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1802_005_' ./acs/cycle1802"
  - criterion: "At enforce, a clean cycle predicate package and an absent one are both approved"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1802_006_' ./acs/cycle1802"
  - criterion: "At shadow, every kind (inverted-idiom, go-run-exit-code, absence-message, flaky shape) is logged with stage=shadow and blocking=false and the deliverable is approved"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1802_007_' ./acs/cycle1802"
  - criterion: "The evalgate unit suite, updated for the new blocking contract, passes and is race clean"
    max_if_missing: 6
    evidence: "cd go && go vet ./internal/evalgate && go test -race -count=1 ./internal/evalgate"
---

# Eval: Gate E blocks the provable unsatisfiable kinds at enforce

> Pins the blocking contract of evalgate Gate E (`unsatisfiable-predicate-shape`).
> The `inverted-idiom` and `go-run-exit-code` kinds are proofs: such a predicate is
> red on every tree, so letting it through tdd burns the build (the cycle-1488,
> 1788 and 1793 class). Commit dd8954ad9 landed the lint and Gate E but left every
> kind advisory (`block` was constant false), so a cycle's own unsatisfiable
> predicate was only logged. This eval holds the fix: at the eval_gate enforce
> stage the proof kinds reject the tdd deliverable with the finding and remedy in
> the reason, while the heuristic `absence-message` kind and every Gate D flaky
> finding stay advisory, and shadow logs every kind and approves. Source: inbox
> item unsatisfiable-proof-kinds-block-at-enforce, cycle 1802.

## Graders

- [code] `cd go && go test -tags acs -count=1 ./acs/cycle1802` exits 0: enforce rejects inverted-idiom and go-run-exit-code findings with the finding and remedy named; absence-message, flaky-shape, clean and absent packages are approved; shadow approves and logs every kind.
- [code] `cd go && go vet ./internal/evalgate && go test -race -count=1 ./internal/evalgate` exits 0.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| proof-blocks | inverted-idiom rejected at enforce with finding + remedy | 8/10 | `go test -tags acs -run TestC1802_001_ ./acs/cycle1802` |
| proof-blocks | go-run-exit-code rejected at enforce with finding + remedy | 8/10 | `go test -tags acs -run TestC1802_002_ ./acs/cycle1802` |
| finding-visible | blocking finding named beyond the advisory cap | 6/10 | `go test -tags acs -run TestC1802_003_ ./acs/cycle1802` |
| heuristic-advisory | absence-message never blocks | 7/10 | `go test -tags acs -run TestC1802_004_ ./acs/cycle1802` |
| gate-d-advisory | every flaky-shape class never blocks | 7/10 | `go test -tags acs -run TestC1802_005_ ./acs/cycle1802` |
| no-false-block | clean and absent predicate packages approved | 5/10 | `go test -tags acs -run TestC1802_006_ ./acs/cycle1802` |
| shadow-dial | shadow logs and approves every kind | 7/10 | `go test -tags acs -run TestC1802_007_ ./acs/cycle1802` |
| unit-suite | evalgate vet + race-clean suite | 6/10 | `go test -race -count=1 ./internal/evalgate` |
