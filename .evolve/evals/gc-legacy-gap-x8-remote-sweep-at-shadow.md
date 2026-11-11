---
score_cap:
  - criterion: "At shadow the remote sweep reports the branch it would delete and deletes nothing"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2296_001' ./acs/cycle2296"
  - criterion: "At enforce the sweep deletes only a merged branch whose tip matches the pull request head"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2296_002|TestC2296_005|TestC2296_007' ./acs/cycle2296"
  - criterion: "A verified bundle of the remote tip comes before the API delete, and a failed verify keeps the branch"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2296_003|TestC2296_004' ./acs/cycle2296"
  - criterion: "The production evolve gc command reaches the remote sweep through gh"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2296_009' ./acs/cycle2296"
  - criterion: "Every changed function of internal/gc, internal/gcpolicy and cmd/evolve is covered at 100%"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2296_010|TestC2296_011|TestC2296_012' ./acs/cycle2296"
---

# Eval: remote branch sweep of gc (X8, D10)

> Pins decision D10 and component X8 of the plan `docs/plans/gc-closes-the-legacy-gap-2026-11.md`. The sweep reads pull requests through `gh`, saves each remote tip in a verified bundle, and deletes through the API. It continues cycle 2283, whose build failed its handoff on an unrelated integration test.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| shadow-reports-only | shadow deletes nothing and writes no bundle | 4/10 | `go test -run TestC2296_001` |
| enforce-merged-tip-only | only a merged branch with a matching tip goes | 4/10 | `go test -run TestC2296_002` |
| bundle-first | verify before delete; failed verify keeps the branch | 5/10 | `go test -run TestC2296_003` |
| production-caller | `evolve gc` reaches the sweep through `gh` | 5/10 | `go test -run TestC2296_009` |
| coverage | changed functions at 100% | 6/10 | `go test -run TestC2296_010` |
