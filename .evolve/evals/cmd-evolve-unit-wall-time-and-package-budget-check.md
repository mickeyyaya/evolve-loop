---
score_cap:
  - criterion: "The package wall-time budget is one named value, addedtests.PackageWallBudget, well below addedtests.PackageTimeout"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1930_001 ./acs/cycle1930"
  - criterion: "testlatency -package-budget exits 1 for a package over the named budget and names the package, its wall time and the budget"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1930_00[23]' ./acs/cycle1930"
  - criterion: "make test-budget pipes go test -json into testlatency -package-budget"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1930_005 ./acs/cycle1930"
  - criterion: "The existing --budget and --fail-over-budget flags keep their contract"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./cmd/testlatency"
---

# Eval: Cut ./cmd/evolve unit-test wall time and add a package wall-time budget check

> Pins the package wall-time budget check from cycle 1930. The build floor
> moved every go-test run onto addedtests.PackageTimeout (20m) after cycles
> 1787, 1791, 1792 and 1798 killed a green ./cmd/evolve at a 120s literal
> deadline. That fix removed the only alarm for a slow package. This eval
> keeps one named budget, a testlatency flag that applies it, and a make
> target that runs it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| named-budget | One named value, at most a quarter of PackageTimeout | 6/10 | `go test -tags acs -run TestC1930_001 ./acs/cycle1930` |
| over-budget-red | Exit 1 and the package, wall time and budget in the report | 8/10 | `go test -tags acs -run 'TestC1930_00[23]' ./acs/cycle1930` |
| make-wiring | `make test-budget` reaches `testlatency -package-budget` | 7/10 | `go test -tags acs -run TestC1930_005 ./acs/cycle1930` |
| preservation | The existing budget flags keep their contract | 5/10 | `go test -count=1 ./cmd/testlatency` |
