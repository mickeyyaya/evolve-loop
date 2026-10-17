---
score_cap:
  - criterion: "coverChangedPackages drops every package that builds no file under covchanged.DefaultTags, so a go/acs/cycleN package never reaches the profiler"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestCoverChangedPackages_ExcludesTagOnly|TestCoverChangedFailures_ExcludesACSTagOnlyPackageFromProfiler|TestCoverChangedPackages_KeepsOnlyLivePackagesOutsideTestdata)$' ./internal/core"
  - criterion: "A live diff that adds an untested function beside an acs package is refused by name at enforce and warned by name at shadow, with no [setup failed]"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestCoverChangedFailures_UncoveredFunction$' ./internal/core"
  - criterion: "The coverStrictRunner seam turns a changed go/.cover-strict package below its floor into a cover_changed failure at enforce and a warning at shadow, and judges only changed strict packages"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestCoverChangedFailures_StrictRunner|TestCoverChangedFailures_StrictRunnerDefaultMeasuresOnlyTheChangedStrictPackages)$' ./internal/core"
  - criterion: "The pre-existing cover-changed contract and the cycle predicates stay green"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestCoverChanged' ./internal/core && go test -tags acs -count=1 ./acs/cycle1916"
---

# Eval: The build floor's cover-changed measures past acs packages and checks go/.cover-strict

> This eval pins two build-floor repairs. First, `coverChangedPackages` keeps only packages that build under `covchanged.DefaultTags`. Each cycle adds `go/acs/cycleN` (`//go:build acs`); before the fix that package made the single `go test -tags integration -coverprofile` run fail with `[setup failed]`, so no changed function was ever proven (69 such warnings in runtime loop logs). Second, the floor checks changed strict packages against `go/.cover-strict` through the `coverStrictRunner` seam. Source incident: main CI red on 2026-10-17 at f4a9005a7 (cycle 1909): `internal/practicewatch` fell to 96.6% because `markTruncated` had 0% coverage, and the floor did not catch it. Bound in cycle 1916.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| tag-filter | tag-only packages never reach the profiler | 7/10 | `go test -run TestCoverChangedPackages_ExcludesTagOnly ./internal/core` |
| uncovered-added-function | the cycle-1909 shape is refused or warned by name | 8/10 | `go test -run TestCoverChangedFailures_UncoveredFunction ./internal/core` |
| strict-seam | changed strict packages below floor fail or warn | 7/10 | `go test -run TestCoverChangedFailures_StrictRunner ./internal/core` |
| no-regression | the old contract and the cycle predicates stay green | 8/10 | `go test -run TestCoverChanged ./internal/core` |
