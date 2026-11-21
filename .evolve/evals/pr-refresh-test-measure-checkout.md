---
score_cap:
  - criterion: "the research doc names the share of the checkout and of the generators in the 61 s to 156 s of the PRRefresh test, and no longer says the split is not measured"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC2590_001_' ./acs/cycle2590"
  - criterion: "the PRRefresh real-generators test passes inside a 20 s go test timeout"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags integration -timeout 20s -run '^TestPRRefresh_TheRealGenerators' ./cmd/evolve/"
  - criterion: "the research doc passes the STE lint"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run 'TestC2590_003_' ./acs/cycle2590"
---

# Eval: TestPRRefresh measures the checkout and the generators apart, then reuses one fixture

> Pins research item R7 of docs/research/ci-wall-variance-2026-11.md. The test
> `TestPRRefresh_TheRealGeneratorsOfACloneCommitOnlyTheStaleIndex` took 61.1 s
> and 156.2 s in CI, and 50.8 s on a macOS developer host in cycle 2590. The doc
> said the checkout probably costs more than the clone, and called it not measured.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| measured-split | The doc names the checkout share and the generator share | 6/10 | `go test -tags acs -run TestC2590_001_ ./acs/cycle2590` |
| wall-budget | The test passes under a 20 s timeout | 7/10 | `go test -tags integration -timeout 20s -run ^TestPRRefresh_TheRealGenerators ./cmd/evolve/` |
| ste | The doc passes the STE lint | 5/10 | `go test -tags acs -run TestC2590_003_ ./acs/cycle2590` |
