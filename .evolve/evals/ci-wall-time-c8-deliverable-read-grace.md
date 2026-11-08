---
score_cap:
  - criterion: "the two runner ArtifactTimeout tests each pass in under 1 s"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2227_001 ./acs/cycle2227"
  - criterion: "a missing-file deliverable read under the test window returns in under 50 ms"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2227_00[23]' ./acs/cycle2227"
  - criterion: "production keeps the 500 ms window and the restore puts it back"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2227_00[45]' ./acs/cycle2227"
  - criterion: "SetReadGraceWindowForTest and readDeliverableWithGrace have 100% coverage"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2227_006 ./acs/cycle2227"
---

# Eval: deliverable read grace is a value that tests set

> Pins decision D17 of the ci-wall-time plan (component C8, finding F3.25). The
> runner ArtifactTimeout tests spent 8.2 s each on 16 settle probes that each
> waited the 500 ms read grace. The window becomes a value that a test sets and
> restores; production keeps 500 ms. Source: cycle 2227.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| runner-latency | both ArtifactTimeout tests under 1 s | 7/10 | `TestC2227_001` |
| missing-read-fast | missing read under 50 ms with a short window | 7/10 | `TestC2227_002`, `TestC2227_003` |
| production-window | 500 ms default and restore | 6/10 | `TestC2227_004`, `TestC2227_005` |
| coverage | 100% on changed functions | 6/10 | `TestC2227_006` |
