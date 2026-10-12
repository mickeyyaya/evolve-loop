# Build Explanation — Cycle 1869

## Build Binding
- Cycle: 1869
- Base SHA: 117cbf6122246af6aa4f5a62c6b97d50be9be8c9

## Summary
One package, `go/internal/paralleltest`, now holds the parallel-by-default guard. The `cmd/evolve`, `internal/core` and `internal/phases/ship` test trees call it with their own serial list. Before this build, each of the three packages had its own copy of the guard.

## Rationale
Three fleet lanes wrote three copies of the same AST guard. The copies drifted. The ship copy did not follow helpers, so a seam test made parallel without its serial entry passed. The core copy followed only `_test.go` helpers by name. The cmd/evolve copy did not flag `os.NewFile` on descriptors 0-2. One shared copy gives every package the union of the three rule sets. A fix in one place now reaches all three trees.

## Changed Areas
- `go/internal/paralleltest/paralleltest.go` — the shared guard. `Violations(sources, Config)` judges every top-level test. `Sources(dir)` reads the Go files of a directory. `Config` carries the serial list and the process-global setters of a package. The guard follows helpers transitively, and it follows methods called on package-level values.
- `go/internal/paralleltest/doc.go` — the package doc that the `docgo` regression predicate requires of every internal package.
- `go/internal/paralleltest/paralleltest_test.go` — the fixture tests of the three former copies, merged into one table per rule, plus tests for method following and for each failure of `Sources`.
- `go/internal/paralleltest/apicover_named_test.go` — names the three exports for the apicover gate.
- `go/.apicover-enforce` — enrolls `./internal/paralleltest`.
- `go/cmd/evolve/parallel_default_test.go` — keeps only the four process-global setters of `cmd/evolve` and the real-tree test.
- `go/internal/core/parallel_default_test.go` — keeps only the serial reasons, the serial list and the real-tree test.
- `go/internal/phases/ship/parallel_default_test.go` — keeps the serial list, the real-tree test and the seam-mutation test. The seam-mutation test now also proves that the helper `swapRepoContractTest` is followed.
- `go/acs/cycle1869/predicates_test.go` — the TDD phase's acceptance predicates for this task.
- `.evolve/evals/parallel-guard-one-home.md` — the TDD phase's eval for this task.

## Design Decisions
The base of the shared guard is the cmd/evolve copy, because only that copy followed helpers through a call graph (breadth-first, with a visited set). The TDD satisfiability probe showed that this copy already passes predicates 002, 004 and 005 (`.evolve/runs/cycle-1869/test-report.md`). The ship rule for `os.NewFile` was added to it. The setter names became `Config.GlobalSetters`, because only cmd/evolve swaps those globals.

Method following matches the method name, and only when the receiver expression is a package-level variable that the caller does not shadow. A rejected option was to resolve the declared type of each variable. A variable declared without a type (for example `var x = newY()`) has no type in the AST, so type resolution would miss it (synthesis). The name match can over-report; the real trees of all three packages pass with it.

The core copy used a fixed-point set of state-changing helpers. That option was rejected because it named only the first helper in a chain, and predicate 002 requires the name of the helper that mutates.

Violation messages say "listed as serial" in place of each package's variable name (`serialTests`, `serialRequiredTests`). One message text serves all three packages.

## Verification
- `go test -count=1 ./internal/paralleltest` passes with 93.6% statement coverage.
- `go test -count=1 -run 'Parallel|SerialByDefault' ./cmd/evolve ./internal/core ./internal/phases/ship` passes.
- `go test -tags acs -count=1 ./acs/cycle1869` passes all eight predicates.
- `APICOVER_PKGS=./internal/paralleltest make apicover-enforce` reports 3 exported, 3 covered.

## Compatibility
The serial lists of the three packages did not change. The real trees pass the shared guard without new entries. Only the text of the violation messages changed.

## Limitations
A method that writes only through its own receiver (`b.n++`) is not flagged, because the receiver is a local name. The guard does not follow a helper that a test calls through a package qualifier from an external `_test` package.
