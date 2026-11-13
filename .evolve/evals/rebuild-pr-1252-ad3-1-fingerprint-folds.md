---
score_cap:
  - criterion: "cycle=N, cycle-N-ulid and finding counts fold out of the failure fingerprint"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2345_00[1-4]' ./acs/cycle2345"
  - criterion: "recycle=12, check-failure counts and distinct defects keep distinct fingerprints"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2345_00[5-7]' ./acs/cycle2345"
---

# Eval: Fold per-cycle tokens in the failure fingerprint

> Pins AD3-1 of PR 1252, rebuilt on current main. The fingerprint of a failure
> reason must not change with `cycle=N`, with the `cycle-N-<ulid>` suffix of an
> explanation path, or with a finding count. Two different defects must keep two
> fingerprints. The predicates call `core.AssembleFailureDigest`, the production
> entry. Source: closed PR 1252, branch cloud/ad3-1-fingerprint-folds (cycle 2345).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| folds | the three per-cycle tokens fold out | 4/10 | `go test -tags acs -run 'TestC2345_00[1-4]' ./acs/cycle2345` |
| preservation | distinct defects stay distinct | 5/10 | `go test -tags acs -run 'TestC2345_00[5-7]' ./acs/cycle2345` |
