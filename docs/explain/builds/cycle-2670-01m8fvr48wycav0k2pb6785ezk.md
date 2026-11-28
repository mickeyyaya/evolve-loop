# Build Explanation — Cycle 2670

## Build Binding
- Cycle: 2670
- Base SHA: 59e7b0750b9210903d273be167b7434d045f8691

## Summary
The code-index bench now scores every backend against two truth sets. It prints `set`, `precision`, `recall`, `p50`, and `p95` for each set.

## Rationale
Previously, `evolve code bench` scored backends only against `gotypes.Truth`, so `gotypes` reached 1.00 recall by construction. An embedded fixture with interface-method and function-value calls exposes dynamic dispatch gaps while `internal/codeindex` stays free of backend dependencies.

## Changed Areas
- `go/internal/codeindex/bench.go` — adds `TruthSet` to `BenchRun` and `BenchRow`, stamps it onto each row, and prints `set` first in `BenchReport.WriteTable`.
- `go/internal/codeindex/independent.go` — embeds the fixture module (`go.mod.txt`, `work.go.txt`, `truth.json`) and exposes `IndependentTruth` and `WriteIndependentFixture`.
- `go/internal/codeindex/testdata/independent/go.mod.txt` — holds the fixture `go.mod` outside a nested module boundary so `go:embed` bundles it cleanly.
- `go/internal/codeindex/testdata/independent/work/work.go.txt` — stores the fixture source with `.txt` suffix so new-package graduation does not mistake `testdata` for a live Go package.
- `go/internal/codeindex/testdata/independent/truth.json` — lists the hand-checked caller sites that match `work.go` lines and `gotypes` symbol names.
- `go/internal/codeindex/apicover_named_test.go` — tests `IndependentTruth`, `WriteIndependentFixture`, and `TruthSet` fields to keep 100% statement coverage and zero uncovered exports.
- `go/cmd/evolve/cmd_code_bench.go` — updates `runCodeBench` to run `go-types` and then call `runIndependentBench` so every requested backend runs on both truth sets.
- `go/cmd/evolve/cmd_code_bench_independent_test.go` — adds CLI tests for per-set rows, table headers, `gotypes` recall below 1.00 on `independent`, and absent-index failures.
- `go/cmd/evolve/cmd_code_bench_test.go` — updates bench tests and adds error-propagation tests for `runCodeBench` and `runIndependentBench`.
- `docs/architecture/packages/internal-codeindex.md` — documents the independent truth set and the `set` column in the bench section.

## Design Decisions
We embedded the fixture sources (`go.mod.txt`, `work.go.txt`, `truth.json`) with `go:embed` instead of calling `gopls` at bench time. This choice keeps `evolve code bench` deterministic and removes external language-server dependencies. Storing `work.go.txt` avoids treating the fixture directory as an ungraduated Go package. `WriteIndependentFixture` restores `go/work/work.go` and `go/go.mod` in the temporary bench directory.

## Verification
Unit tests in `go/internal/codeindex/...`, targeted `go/cmd/evolve` tests, `TestTheCodeIndexAbstractionRunsNoProcessAndNamesNoBackend` in `go/test/structure`, `go vet`, and `gofmt` pass with 100.0% statement coverage on `internal/codeindex`. The cycle-2670 ACS suite and `evolve phase verify build` run before handoff.

## Compatibility
Legacy `BenchReport.Table()` callers with an empty `TruthSet` still print aligned columns. Absent index handling still fails with `ErrAbsent` before any set runs.

## Limitations
Backends that do not index the temporary fixture directory (such as `trail` without a fixture graph) record misses or failures on their `independent` row.
