---
score_cap:
  - criterion: "the four pinned pairs hold: 2071/2072 and 2049/2051 merge, 2054/2093 and 2073/2089 stay apart"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestNormalizeReasonForFingerprint_PinnedPairs|TestNormalizeReasonForFingerprint_CountOfFailingChecksStaysIdentity)$' ./internal/core"
  - criterion: "cycle=N, glued cycleN, hex lane hash and finding/file counts all fold to one fingerprint"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestNormalizeReasonForFingerprint_CycleAndCountShapesFold$' ./internal/core"
  - criterion: "the digest assembled from two cycles of one build-floor defect carries one fingerprint"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2425_00[34]_' ./acs/cycle2425"
---

# Eval: fold cycle and count variance in the fingerprint normalizer

> Pins the defect-identity projection in `normalizeReasonForFingerprint`
> (go/internal/core/failure_digest.go) for component AD3-1 of the 2026-11-03
> architecture decisions. Cycles 2049 and 2051 hit one build-floor defect but
> carried two fingerprints, because `./acs/cycle2049` has no separator, a lane
> hash can start with a letter, and `N finding(s) in M file(s)` kept its file
> count. The narrowness criterion carries the higher cap: folding too much
> blinds the identical-fingerprint breaker, so 2054/2093 and 2073/2089 must
> stay apart. Source incident: cycle 2408 inbox item
> `arch-decisions-ad3-1-fold-cycle-n-cycle-n-ulid-suffixes`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| pinned-pairs | two pairs merge, two stay apart | 7/10 | `go test -run TestNormalizeReasonForFingerprint_PinnedPairs` |
| shape-folds | each new shape folds | 6/10 | `go test -run TestNormalizeReasonForFingerprint_CycleAndCountShapesFold` |
| digest-caller | the real digest assembler merges one defect | 6/10 | `go test -tags acs -run TestC2425_00[34]_ ./acs/cycle2425` |
