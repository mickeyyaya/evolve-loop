# Build Explanation — Cycle 2592

## Build Binding
- Cycle: 2592
- Base SHA: a1d94de45f161e7196b00f806015f048d8447aba

## Summary
The `make test-integration` target now snapshots `go/` before tests run and checks for leaked paths afterward, matching `make test`.
A repeatable `--allow` flag and `treewalk.Filter` let `test-integration` keep its own coverage, binary, JSON, sample, and rusage outputs without masking leaks.

## Rationale
Without the treewalk guard in `test-integration`, CI on `main` passed while composed lanes failed `make test` on `.evolve` directories left under `go/`.
Adding an allow-list filter to `treewalk` and sharing the snapshot and check macros in `go/Makefile` closes the CI parity gap with minimal change.

## Changed Areas
- `go/internal/treewalk/treewalk.go` — adds `Filter` and `isAllowed` to drop paths equal to or under cleaned whole-component prefixes while keeping input order and slices unchanged.
- `go/cmd/evolve/cmd_treewalk.go` — adds the repeatable `--allow` flag (`allowList`) to `runTreewalkCheck`, rejects empty prefixes with exit 10, filters `Diff` output, and documents `--allow` in `treewalkUsage`.
- `go/Makefile` — defines `TREEWALK_SNAPSHOT`, `TREEWALK_CHECK`, and `TREEWALK_ALLOW`. It wraps `test-integration` and `test-integration-darwin`, normalizes allow paths, and cleans `coverage.txt` on failure.
- `docs/architecture/packages/internal-treewalk.md` — documents the allow-list design, prefix boundary invariant, and CLI validation rules for `internal/treewalk`.

## Design Decisions
- **Whole-component prefix match:** `isAllowed` cleans each prefix and checks equality or a separator boundary, so `bin` never allows `binary`.
- **Physical path normalization in the Makefile:** On macOS `/var` links to `/private/var`. The recipe resolves existing absolute paths with `pwd -P` before stripping `root_abs`.
- **Always run `treewalk check` before exit:** `test-integration` runs the check even when tests fail. It reports leaks alongside test errors and removes `coverage.txt`.

## Verification
`go test -count=1 ./internal/treewalk`, `go test -count=1 -run Treewalk ./cmd/evolve`, `go test -count=1 -tags integration -run 'TestMake' ./internal/ciparity`, `go test -tags acs -count=1 ./acs/cycle2592`, and `evolve acs suite --cycle 2592` verify `Filter`, `--allow`, `make test-integration`, `make test`, and 100% statement coverage of all changed functions.

## Compatibility
`evolve treewalk check` without `--allow` keeps identical behavior, and `make test` keeps its un-filtered snapshot and check commands.

## Limitations
`test-integration` allows `coverage.txt`, `coverage.partial.txt`, `runner-samples.ndjson`, `$(BIN_DIR)`, `$(TEST_JSON)`, and `$(TESTRUSAGE_OUT)`. Any new CI output under `go/` needs an allow entry.
