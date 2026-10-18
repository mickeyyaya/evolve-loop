---
score_cap:
  - criterion: "The TestCoreCoverageBatch_ tests alone take parseUnifiedDiffToHunks, parseDiffPath, parseHunkHeader, parseDiffRange, defectHead, causeHead and stampingLedger.Verify to 100.0% statement coverage"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1942_005_' ./acs/cycle1942 | grep -q -- '--- PASS: TestC1942_005_'"
  - criterion: "The unmet remainder (the rest of internal/core to 100.0% and the go/.cover-strict registration) is a new tracked, claimable, weighted inbox-root item linked by connects_to/deps"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1942_006_' ./acs/cycle1942 | grep -q -- '--- PASS: TestC1942_006_'"
---

# Eval: internal/core coverage batch — diff parsers, digest heads and the stamping ledger

> Pins one batch of `core-package-coverage-remaining-batches`. On 2026-10-18 (cycle 1942 TDD) the
> package measured 97.4% with `-tags integration`. This batch takes the four diff parsers of
> `mergerung2.go`, `defectHead` and `causeHead` of `failure_digest.go` and `stampingLedger.Verify`
> of `runid.go` to 100.0% with behavior tests. Malformed hunk headers and ranges are the negative
> cases. The 160-byte truncation and the over-long scanner line are the edge cases. The batch does
> not register `./internal/core` in `go/.cover-strict`, because the package is not at 100.0% yet.
> The protected `phase_bindings_selfcheck.go` stays untouched.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| batch-coverage | seven named functions at 100.0% | 8/10 | `TestC1942_005_` |
| remainder | new queued tracked remainder item | 8/10 | `TestC1942_006_` |
