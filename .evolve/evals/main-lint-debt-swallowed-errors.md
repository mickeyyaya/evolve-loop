---
score_cap:
  - criterion: "golangci-lint reports 0 issues on core, codeindex and gc"
    max_if_missing: 5
    evidence: "cd go && golangci-lint run ./internal/core/... ./internal/codeindex/... ./internal/gc/..."
  - criterion: "A failed bench table write is returned by BenchReport.WriteTable"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2320_00[234]' ./acs/cycle2320"
  - criterion: "Every function in codeindex bench.go is fully covered"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2320_005 ./acs/cycle2320"
  - criterion: "The bench verb reaches WriteTable and its caller is fully covered"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2320_006 ./acs/cycle2320"
---

# Eval: Handle the swallowed errors that main lint reports

> Pins the lint cleanup of cycle 2320. The tabwriter Flush error in `BenchReport.Table` was
> discarded. The router wall bench error was already joined and stays pinned by its own tests.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| lint-clean | 0 issues on three packages | 5/10 | `golangci-lint run` |
| flush-error | WriteTable returns the writer error | 6/10 | `go test -tags acs -run TestC2320_00[234]` |
| coverage | bench.go functions at 100% | 7/10 | `go test -tags acs -run TestC2320_005` |
| caller-proof | verb reaches WriteTable, caller covered | 7/10 | `go test -tags acs -run TestC2320_006` |
