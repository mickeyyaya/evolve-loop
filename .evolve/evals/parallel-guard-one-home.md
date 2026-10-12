---
score_cap:
  - criterion: "The shared guard package go/internal/paralleltest exists, is enrolled in apicover, and its own tests pass"
    max_if_missing: 8
    evidence: "cd go && grep -qx './internal/paralleltest' .apicover-enforce && go test -count=1 ./internal/paralleltest"
  - criterion: "cmd/evolve, internal/core and internal/phases/ship test binaries all depend on internal/paralleltest"
    max_if_missing: 8
    evidence: "cd go && for p in ./cmd/evolve ./internal/core ./internal/phases/ship; do go list -test -deps $p | grep -qx github.com/mickeyyaya/evolve-loop/go/internal/paralleltest || exit 1; done"
  - criterion: "No package keeps its own copy of the AST guard helpers"
    max_if_missing: 6
    evidence: "! grep -n 'func startsWithParallel\\|func testingTName\\|func processStateMutations' go/cmd/evolve/parallel_default_test.go go/internal/core/parallel_default_test.go go/internal/phases/ship/parallel_default_test.go"
  - criterion: "Each package's real test tree passes the shared guard"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run 'Parallel|SerialByDefault' ./cmd/evolve ./internal/core ./internal/phases/ship"
  - criterion: "The shared guard follows helpers and methods on package-level values, and catches a seam test made parallel in every package"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1869_00[2-5]' ./acs/cycle1869"
---

# Eval: parallel-guard-one-home — one parallel-by-default guard for three packages

> Pins the extraction of the parallel-by-default AST guard into `go/internal/paralleltest`.
> Three fleet lanes wrote three copies of the guard in `cmd/evolve`, `internal/core` and `internal/phases/ship`.
> The ship copy did not follow helpers, so a seam test made parallel without its serial entry passed unseen.
> Source incident: inbox item `parallel-guard-one-home` (2026-10-12), scoped in cycle 1869.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| one-home | shared package exists, enrolled, tested | 8/10 | `go test ./internal/paralleltest` |
| delegation | three test binaries depend on it | 8/10 | `go list -test -deps` |
| no-duplicates | no local AST guard helpers remain | 6/10 | `! grep func startsWithParallel ...` |
| real-tree | each real tree passes the shared guard | 9/10 | `go test -run 'Parallel\|SerialByDefault'` |
| helper-following | helpers, methods and seam mutation caught | 9/10 | `go test -tags acs -run TestC1869_00[2-5]` |
