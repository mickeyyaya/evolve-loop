# Build Explanation — Cycle 2294

## Build Binding
- Cycle: 2294
- Base SHA: 3b9fe5a83a5397f6c1c392a8395b2855fb801498

## Summary
The new verb evolve code bench measures each code index backend against a type-checked truth. It prints precision, recall, p50, p95, bytes and failures for each backend, and the host load average.

## Rationale
The plan of the code index (decisions D7 and D8) makes each route change a measured claim. The truth comes from the gotypes index, so a grep check on a sample guards it against a circular score.

## Changed Areas
- `go/internal/codeindex/bench.go` — adds the bench types, the scoring of file and function pairs, the nearest rank percentiles and the table.
- `go/internal/codeindex/benchcheck.go` — adds GrepCheck and ErrTruthDisagrees, which compare the truth sites with the call lines that grep finds.
- `go/internal/codeindex/benchsample.go` — adds SampleSymbols, which sorts symbols by a hash of their position and adds the probes.
- `go/internal/codeindex/backends.go` — splits Resolve out of New, and adds KnownBackends, so the verb can get the backends one by one.
- `go/internal/codeindex/bench_test.go` — names each new export in a real assertion and keeps the package at 100% coverage.
- `go/internal/adapters/codeindex/gotypes/truth.go` — adds Truth, which lists the module functions with callers and marks the symbols that grep can check.
- `go/internal/adapters/codeindex/gotypes/truth_test.go` — tests Truth on the fixture module.
- `go/cmd/evolve/cmd_code_bench.go` — adds the verb: flags, the unknown backend refusal, the sample, the run and the host load average.
- `go/cmd/evolve/cmd_code_bench_test.go` — tests the verb with a real gotypes index of a small module.
- `go/cmd/evolve/cmd_code.go` — routes the bench verb, extracts codeBackendFactories and adds bench to the usage line.
- `docs/architecture/packages/internal-codeindex.md` — documents the bench design.
- `docs/plans/code-index-backends-2026-10.md` — marks G3 built and records the measured values.

## Design Decisions
A caller is a file and function pair, and scores are micro averages. A symbol is grep-checked only if its bare name has one definition, all its calls are static and none is in a test file. Truth sites of other symbols still score.

## Verification
The cycle 2294 predicates pass, and the codeindex and gotypes packages stay at 100% coverage. A run on the real index gave gotypes callers precision 1.00 and recall 1.00, and the grep check passed on 10 symbols.

## Compatibility
Existing verbs and the New function keep their behavior. The usage line has one more verb.

## Limitations
The truth is the gotypes index, so gotypes scores 1.00 by construction and only the grep check guards it. The bench scores callers only, and trail scores 0.00 on qualified names.
