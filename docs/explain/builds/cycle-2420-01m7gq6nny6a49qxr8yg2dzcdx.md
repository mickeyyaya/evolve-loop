# Build Explanation — Cycle 2420

## Build Binding
- Cycle: 2420
- Base SHA: 748cded630a52cd5367613eee56bf373e48a0208

## Summary
The ship scan cache now uses default-deny package purity through `regressiontia.PurePackages` and tallies shadow-stage outcomes through `stagescore.Score`.

## Rationale
Default-allow caching skipped packages that read environment variables or run external commands through `os/exec`. Proving purity over the module-local dependency closure closes that gap while preserving `repoContractSelections` as explicit deny overrides.

## Changed Areas
- `go/internal/regressiontia/regressiontia.go` — exports `Pure` and `PurePackages` to prove that a package and its test variants reach no `escapeHatches` across module-local dependencies.
- `go/internal/phases/ship/scanshadow.go` — replaces the default-allow `cacheable` helper with `scanShadow.cacheable` backed by `preloadPure`, and replaces the hand-maintained escape counter with `stagescore.Score`.
- `go/internal/phases/ship/scanshadow_enforce.go` — preloads purity for `prune` targets, calls `s.cacheable` in `covered`, and records `WouldSkip` hits in `commitSkips`.
- `go/internal/stagescore/stagescore.go` — adds the `stagescore` leaf package with `Decision`, `Outcome`, `Record`, `Tally`, and `Score`.
- `go/.apicover-enforce` — enrolls `./internal/stagescore` in the repo-wide API coverage gate.
- `docs/architecture/packages/internal-stagescore.md` — documents the `internal/stagescore` package exports and scoring rules.

## Design Decisions
Batching `go list -e -test -deps` in `PurePackages` and memoizing results in `scanShadow.pureCache` avoids running `go list` once per package during a full ship scan. Placing `Score` in `internal/stagescore` keeps the scorer independent of `ship` and `wavemeasure`.

## Verification
All 8 cycle-2420 ACS predicates pass, `go test -count=1 ./internal/regressiontia ./internal/stagescore` passes, `TestScanShadow|TestVerdictStore|TestCacheable|TestFloorRecorder` pass in `./internal/phases/ship`, `gofmt` and `go vet` report no issues, and `evolve docs ste-lint --strict` reports zero findings.

## Compatibility
Existing `repoContractSelections` entries continue to deny caching even when a package is pure, and `scanShadow` signal field names remain unchanged.

## Limitations
`internal/wavemeasure/cost.go` still holds its existing false-stop check; consolidating it with `stagescore` was left out of scope to keep the diff minimal.
