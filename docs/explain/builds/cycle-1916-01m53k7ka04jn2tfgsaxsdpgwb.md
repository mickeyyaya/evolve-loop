# Build Explanation — Cycle 1916

## Build Binding
- Cycle: 1916
- Base SHA: fb7761a757eb659f91cf99fe9955220fb8bb1461

## Summary
The cover-changed build floor now profiles only the packages that build under `covchanged.DefaultTags`. It also checks the changed packages that `go/.cover-strict` lists against their floors.

## Rationale
Each cycle adds a `go/acs/cycleN` package with only `//go:build acs` files. Before the change, `coverChangedPackages` passed that package to `go test -tags integration -coverprofile`. The run failed with `[setup failed]`, so the floor proved no changed function. The cycle-1909 `markTruncated` gap reached main CI through this hole. A `go/build` `ImportDir` probe with `BuildTags = covchanged.DefaultTags` drops a package that builds no file under those tags, because the probe returns `*build.NoGoError`. The strict check reuses `coverstrict.ParseFloors` and `coverstrict.Measure` on the profile that the floor already made.

## Changed Areas
- `go/internal/core/cover_changed_floor.go` — adds `buildsUnderCoverTags` to `coverChangedPackages`, splits the function check into `coverChangedFunctionFailures`, and adds the `coverStrictRunner` seam with its default `changedStrictResults` and the report `coverStrictFailures`.
- `go/internal/core/cover_changed_floor_strict_default_test.go` — tests every branch of `changedStrictResults` and the warning on a runner error.
- `go/internal/core/parallel_default_test.go` — lists the new serial tests that swap `coverChangedProfiler`, `coverStrictRunner` or `os.Stderr`.
- `go/internal/core/cover_changed_floor_strict_test.go` — the TDD phase added the strict-seam tests at enforce, shadow and a met floor.
- `go/internal/core/cover_changed_floor_tagfilter_test.go` — the TDD phase added the tag-filter tests and the live uncovered-function test.
- `go/internal/core/cover_changed_floor_test.go` — the TDD phase gave the testdata fixture real Go files, so it keeps its intent under the tag filter.
- `go/acs/cycle1916/predicates_test.go` — the TDD phase added four predicates that run the named core tests.
- `.evolve/evals/cover-changed-floor-never-measures-acs-package-setup-fails.md` — the TDD phase added the eval with the score caps.
- `docs/architecture/packages/internal-covchanged.md` — records the tag filter and the strict check of the build floor.

## Design Decisions
The tag filter skips only `*build.NoGoError`. Another `ImportDir` error, such as a parse error, keeps the package, so `go test` reports the real fault. The strict default reads `<module>/.cover-strict` and judges only the listed packages in the change set. An absent file is silent. A read, parse, `go.mod` or profile error is a WARN, the same as the other plumbing faults of the floor. `coverstrict.Check` was rejected, because it judges every listed package and walks the whole module for staleness. The floor profile covers only the changed packages and is new by construction. A strict failure goes through `coverChangedResult`, so enforce refuses and shadow warns.

## Verification
`go test -count=1 ./internal/core` passes. `go test -tags acs -count=1 ./acs/cycle1916` passes 4/4. A `-coverprofile` run of the cover-changed tests reports 100% for every function in `cover_changed_floor.go`.

## Compatibility
The floor stage key `cover_changed`, the headline of the function check and the plumbing-warning behavior stay the same. A module without `.cover-strict` behaves as before.

## Limitations
The strict check judges only the changed packages. An unchanged strict package that drops because of a change elsewhere stays the job of `make cover-strict` and CI.
