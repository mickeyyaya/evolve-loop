# Build Explanation — Cycle 2633

## Build Binding
- Cycle: 2633
- Base SHA: e144bfba9d3fbb79d29d399556aec7b277e1c18f

## Summary
The code-index bench now scores every backend against two truth sets. It prints `set`, `precision`, `recall`, `p50`, and `p95` for each set.

## Rationale
Previously, `evolve code bench` scored backends only against `gotypes.Truth`. As a result, `gotypes` reached 1.00 recall on its own output by construction. An embedded fixture with interface-method and function-value calls exposes dynamic dispatch gaps while keeping `internal/codeindex` free of backend dependencies.

## Changed Areas
- `go/internal/codeindex/bench.go` — adds `TruthSet` to `BenchRun` and `BenchRow`, stamps it onto each row, and prints `set` first in `BenchReport.WriteTable`.
- `go/internal/codeindex/independent.go` — embeds the fixture module and expected caller sites, exposing `IndependentTruth` and `WriteIndependentFixture`.
- `go/internal/codeindex/testdata/independent/go.mod.txt` — stores the fixture `go.mod` outside a nested module boundary so `go:embed` can bundle it.
- `go/internal/codeindex/testdata/independent/work/work.go` — defines static, interface-method, and function-value call sites for the independent fixture.
- `go/internal/codeindex/testdata/independent/truth.json` — lists the hand-checked caller sites that match `work.go` lines and `gotypes` symbol names.
- `go/internal/codeindex/apicover_named_test.go` — tests `IndependentTruth`, `WriteIndependentFixture`, and `TruthSet` fields, keeping `internal/codeindex` at 100% statement coverage and zero uncovered exports.
- `go/cmd/evolve/cmd_code_bench.go` — updates `runCodeBench` to run `go-types` and then call `runIndependentBench` to score all requested backends on the fixture.
- `docs/architecture/packages/internal-codeindex.md` — documents the independent truth set and the `set` column in the bench section.

## Design Decisions
We embedded the fixture sources (`go.mod.txt`, `work.go`, `truth.json`) with `go:embed` instead of calling `gopls` at bench time. This choice keeps `evolve code bench` deterministic and removes external language-server dependencies. Storing `go.mod.txt` avoids Go's nested-module embed limit while `WriteIndependentFixture` restores `go/go.mod` for `gotypes.Build`.

## Verification
Unit tests in `go/internal/codeindex` pass with 100.0% statement coverage and 45 of 45 exports covered. Targeted `cmd/evolve` tests, `golangci-lint`, `TestTheCodeIndexAbstractionRunsNoProcessAndNamesNoBackend`, and all 8 cycle-2633 ACS predicates pass.

## Compatibility
Legacy `BenchReport.Table()` callers with an empty `TruthSet` still print aligned columns. Absent index handling still fails with `ErrAbsent` before any set runs.

## Limitations
Other backends such as `trail` do not build a graph for the temporary fixture. Their `independent` row records failures or misses against that fixture.
