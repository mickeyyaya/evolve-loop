# Build Explanation — Cycle 2548

## Build Binding
- Cycle: 2548
- Base SHA: f25d49e4ae89c9d1c528d1c4fbc1687743635999

## Summary
The `test-integration-darwin` target in `go/Makefile` now runs `go test` with `-p 3` so 3-core and 5-core macOS runners share one package schedule.

## Rationale
`go test` defaults `-p` to the host CPU count, which splits macOS CI runs into 3-slot and 5-slot schedules and drives 41.3% of package wall variance. Pinning `TEST_PARALLEL` through a target-specific variable on `test-integration-darwin` removes that schedule split at zero runner cost while leaving `test-integration` on the host default.

## Changed Areas
- `go/Makefile` — adds `TEST_PARALLEL`, `DARWIN_PARALLELISM := 3`, and `PARALLEL_FLAG` to `INTEGRATION_TEST`, and sets `TEST_PARALLEL = $(DARWIN_PARALLELISM)` only on `test-integration-darwin`.
- `go/internal/ciparity/darwin_budget_test.go` — adds `TestMakeIntegrationDarwin_FixesParallelismWithoutChangingTheFullSuite` and extracts `captureGoTestArgs`. The test verifies `-p 3` on `test-integration-darwin`, no `-p` on `test-integration`, and command-line overrides.
- `docs/research/ci-wall-variance-2026-11.md` — marks R3 `ci-macos-fixed-parallelism` done in cycle 2548. It also records why `-p 3` fixes the schedule while the span awaits live CI runs and cannot reach 1.0.

## Design Decisions
- **Target-specific `TEST_PARALLEL` over a darwin-only recipe copy:** Reusing `INTEGRATION_TEST` through `TEST_PARALLEL = $(DARWIN_PARALLELISM)` matches `TEST_TIMEOUT = $(DARWIN_TEST_TIMEOUT)`, avoids duplicating `coversplit` orchestration, and allows `TEST_PARALLEL=<n>` overrides.
- **No edit to `.github/workflows/go.yml`:** The macOS CI job already calls `make test-integration-darwin` with a 45-minute step timeout that outlasts `DARWIN_TEST_TIMEOUT := 30m`.
- **Rejected runner-label pinning:** Pinning the macOS job to the XLarge runner label costs money on every run, whereas `-p 3` requires no workflow change.

## Verification
`go test -tags acs -count=1 ./acs/cycle2548 ./acs/cycle813`, `go test -tags integration -count=1 -run TestMakeIntegrationDarwin ./internal/ciparity`, `evolve docs ste-lint --strict`, `gofmt -l .`, `go vet ./...`, and `evolve acs suite --cycle 2548` verify the Makefile recipes, ciparity integration tests, and STE-100 compliance.

## Compatibility
`test-integration` and all non-darwin callers of `INTEGRATION_TEST` receive no `-p` flag and retain the host CPU default. Callers of `test-integration-darwin` can override `-p` with `TEST_PARALLEL=<n>`.

## Limitations
Live CI measurements on a 3-core and a 5-core macOS runner are absent in this local build environment. Each test binary still keeps `GOMAXPROCS` at the host CPU count, and the 5-core runner has 14 GB of memory against 7 GB. Thus the 5-core span cannot equal the 3-core span.
