# internal/treewalk

## Purpose

`internal/treewalk` snapshots a directory tree before tests run and reports every new file or directory after they finish.
Both `make test` and `make test-integration` run this guard through `evolve treewalk`.

## Design

- **Allow list for CI outputs.** `make test-integration` writes `coverage.txt`, `bin/` tools, and optional JSON or rusage logs under `go/`. `Filter` drops paths equal to or beneath an allowed prefix so the guard flags only real leaks.
- **Component-safe prefix match.** `isAllowed` cleans each prefix with `filepath.Clean` and matches either an exact path or a path followed by `filepath.Separator`. A prefix such as `bin` matches `bin/evolve` and never `binary`.
- **Order and immutability.** `Filter` preserves the order of `paths` and never mutates its input slices.
- **CLI validation.** `evolve treewalk check --allow PREFIX` rejects an empty prefix with usage exit 10, because an empty prefix matches every path.

## Invariants

- **Whole-component prefix match only.** `TestFilterDropsPathsAtOrUnderAnAllowedPrefix` and `TestFilterKeepsOrderAndLeavesItsInputsUntouched` pin `Filter`.
- **CLI `--allow` flag validation.** `cmd_treewalk_allow_test.go` pins repeatable `--allow`, `--allow=PREFIX`, empty-prefix rejection, and filtered leak counts.
- **CI target parity.** `makefile_treewalk_integration_test.go` pins that `make test-integration` fails on leaked paths and passes on a clean tree with its own outputs present.
