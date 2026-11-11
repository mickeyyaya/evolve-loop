# internal/codeindex

> The spec: [code-index-cli](../code-index-cli.md). The plan: component K1 of [the code index plan](../../plans/code-index-cli-2026-10.md). The decision: [ADR-0138](../adr/0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md). The operator surface: `evolve code` in [runtime-reference.md](../../operations/runtime-reference.md).

## Purpose

`internal/codeindex` is the layer between the `evolve code` commands and a search engine. It holds the types that every engine returns, the interface that every engine implements, and the ordered set that picks one engine for each query. It runs no process and names no engine. A new search algorithm is one new adapter package and one row in the policy list. The commands do not change.

## Design

- **One result type.** `Hit` has `File`, `Line`, `Symbol` and `Text`. A `Line` of 0 means that the hit is a file or a directory.
- **Four queries.** `Query` is `callers`, `api`, `find` or `map`. `Queries` lists them. `Request` holds a `Query`, one argument and, for `callers`, a `Direction` (`in` or `out`). `ParseDirection` refuses any other word.
- **One interface.** `Backend` has `Name`, `Capabilities` and one method for each query. Each method returns `([]Hit, error)`.
- **Declared capabilities.** `Capabilities` says which queries a backend serves. A call graph serves `callers`. A text search serves `find` only. `Serves` answers for one query.
- **Three typed errors.** `ErrNotFound` means that the query ran and found nothing. `ErrAbsent` means that the index or the engine is missing. `ErrUnsupported` means that no backend serves the query. The commands map them to the exit codes 1, 2 and 3.
- **A chain, not one engine.** `Backends` is an ordered set. `For` returns the first backend that serves a query. `Search` asks that backend only. A `ErrNotFound` from the first backend does not go to the next one, because the operator puts the best engine for each query first. This is the Chain of Responsibility pattern over the Strategy interface.
- **No registry.** The set is built and passed in. There is no `init()` and no global. `New` takes the names from the policy list and a map of `Factory` functions. It fails on an empty list, on a duplicate name and on an unknown name. The unknown-name message lists the known names.
- **`Dispatch`.** It calls the method of a backend that matches a `Request`. `Search` and the conformance suite share it.

## The conformance suite

`internal/codeindex/codeindextest` holds the suite and a fake.

- `Run` takes a `Harness`: a constructor for a working backend, a constructor for a backend with an absent index, and one `Probe` for each query.
- A probe holds a request that must return a named symbol or file, and a request that must return `ErrNotFound`.
- For a served query, `Run` checks four facts. The known request returns a hit that matches. Each hit has a file. The missing request returns `ErrNotFound`. The absent backend returns `ErrAbsent`. For a query that the backend does not serve, `Run` checks that the answer is `ErrUnsupported`.
- `Run` takes a small `Reporter` interface, so a test can pass a recorder and prove that each failure is caught. `*testing.T` fits it.
- `Fake` is an in-memory backend. `NewFake` takes its capabilities and an index. `NewAbsentFake` makes one whose index is absent. `StandardProbes` and `StandardIndex` give the shared cases. The command tests use the same fake.

A backend is done when it passes `Run`. The trail adapter runs the suite with a fake process runner.

## The bench

`RunBench` scores each backend that serves `callers` against a type-checked truth. The code is `bench.go`, `benchcheck.go` and `benchsample.go`.

- **The truth.** A `BenchSymbol` holds a name, the place of its definition and its `TruthSite` list. The `gotypes` adapter makes the list with `gotypes.Truth`.
- **The score.** A caller is a file and function pair. A pair counts once, however many sites it has. Precision and recall are micro averages over all symbols. A rate with no denominator is 0. Rounds do not multiply the counts: the first round gives the answer that is scored.
- **The latency.** Each call of each round gives one sample. `P50` and `P95` use the nearest rank. `MedianBytes` is the median size of the answers. A call that fails counts in `Failures`. `ErrNotFound` is an empty answer, not a failure.
- **The grep check.** `GrepCheck` reads the files of a symbol and lists the lines that call its bare name. A definition line is not a call. The check fails with `ErrTruthDisagrees` when the lines and the truth sites differ in either direction. `RunBench` runs it on the first `GrepSymbols` symbols that have grep files, and returns no table when it fails.
- **The sample.** `SampleSymbols` sorts by a hash of the definition position, takes the first `n`, and adds each symbol that a probe names. The sample does not depend on the input order or on the names.
- **The report.** `BenchReport.Table` prints one row for each backend and the host load average. A query other than `callers` has no truth and no row.

## Invariants

- The package imports no process runner and names no engine. Pinned by `TestTheCodeIndexAbstractionRunsNoProcessAndNamesNoBackend` in `go/test/structure`.
- The first backend that serves a query answers it. Pinned by `TestBackends_ForReturnsTheFirstBackendThatServesTheQuery` and `TestBackends_SearchAsksOnlyTheFirstServingBackendAndDoesNotFallThroughOnNotFound`.
- An unknown backend name fails loudly. Pinned by `TestNew_AnUnknownNameFailsLoudlyAndListsTheKnownNames`.
- Every export is named by a test. The package and `codeindextest` are enrolled in `go/.apicover-enforce` and held at 100% statement coverage in `go/.cover-strict`.

## Wiring

`go/cmd/evolve/cmd_code.go` builds the `Backends` set from the policy block `code_index` (`internal/policy`, `CodeIndexConfig`) and maps the errors to exit codes. It appends one record to the usage log for each call through [`internal/codeindex/usagelog`](internal-codeindex-usagelog.md).

`go/cmd/evolve/cmd_code_freshness.go` compares the index `HEAD` with the local `origin/main` ref before each query. It warns on stderr when they differ, or refuses with exit 21 when `code_index.stale` is `refuse`. `go/cmd/evolve/cmd_code_refresh.go` moves the index worktree to `origin/main` and calls `Backends.Refresh`. `evolve sync-main` and `evolve code refresh` run it. The spec is [code-index-cli.md](../code-index-cli.md), section 4.
