---
score_cap:
  - criterion: "normalizeReasonForFingerprint folds cycle-numbered path variance and attempt/retry denominators so one recurring defect yields ONE fingerprint"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestNormalizeReasonForFingerprint_(CycleNumberedPathsFold|AttemptDenominatorFolds)$' ./internal/core"
  - criterion: "the normalizer stays narrow — two DIFFERENT defects never collapse into one fingerprint, and the existing narrative=<verdict> / go-test-duration pins stay green"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestNormalizeReasonForFingerprint_(DistinctDefectsStayDistinct|ExistingPinsStayGreen|TouchesOnlyTheNarrativeToken)$' ./internal/core"
---

# Eval: fingerprint identity survives path and attempt variance

> Pins the defect-identity projection in
> `normalizeReasonForFingerprint` (go/internal/core/failure_digest.go). It
> stripped exactly two identity-noise tokens (`narrative=<verdict>`, go-test
> durations), so a recurring defect whose reason names a cycle-numbered artifact
> path or a retry attempt index split into N fingerprints — and the
> identical-fingerprint breaker (ceiling 3, standing rule
> `three_consecutive_fails_halt`) never reached its ceiling on a defect that was
> in fact repeating. Source: PR #442 diff-review LOW finding, worked as
> cycle-1440 inbox item `fingerprint-normalizer-path-variance`.
>
> The narrowness criterion carries the HIGHER cap: over-normalization blinds the
> breaker far worse than the variance it fixes — a normalizer that returned a
> constant would satisfy the folding half perfectly while collapsing every
> distinct defect in the repo into one bucket.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| identity-folding | cycle-numbered paths and attempt denominators fold to stable tokens | 6/10 | `go test -run '^TestNormalizeReasonForFingerprint_(CycleNumberedPathsFold\|AttemptDenominatorFolds)$' ./internal/core` |
| blast-radius | distinct defects stay distinct; pre-existing normalization pins stay green | 8/10 | `go test -run '^TestNormalizeReasonForFingerprint_(DistinctDefectsStayDistinct\|ExistingPinsStayGreen\|TouchesOnlyTheNarrativeToken)$' ./internal/core` |
