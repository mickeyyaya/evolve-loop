# Build Explanation — Cycle 2320

## Build Binding
- Cycle: 2320
- Base SHA: 1eda02ad44b94999f572adfcfb07c454d69c3ad8

## Summary
The bench table writer now returns the error of its writer. The bench verb reports that error and exits with a failure code.

## Rationale
The tabwriter Flush error was discarded in the table builder. A new WriteTable method returns that error, and Table stays as a thin wrapper for in-memory use.

## Changed Areas
- `go/internal/codeindex/bench.go` — adds WriteTable, which returns the Flush error and the error of the load line write; Table calls it.
- `go/cmd/evolve/cmd_code_bench.go` — the bench verb calls WriteTable through a new runCodeBench helper and returns a failure code when the write fails.
- `go/internal/gc/branchsweep_test.go` — drops an unused assignment of the lint report.
- `go/internal/codeindex/bench_amplified_test.go` — tests the Flush error, the load line error and a working writer.
- `go/cmd/evolve/cmd_code_bench_test.go` — tests that a stdout that fails makes the verb fail.

## Design Decisions
Table is kept because tests and a predicate use it. A strings.Builder cannot fail, so Table ignores the error with a blank assignment.

## Verification
Unit tests cover every branch of WriteTable and the verb. The ACS predicates of cycle 2320 and golangci-lint cover the rest.

## Compatibility
The table text is the same as before. The Table signature is unchanged.

## Limitations
The router wall bench error was already joined; this change does not touch it.
